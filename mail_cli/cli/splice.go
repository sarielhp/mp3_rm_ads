package cli

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"mail_cli/app"
	"mail_cli/cache/msg"
	"mail_cli/email"
	"mail_cli/mailclient"

	"github.com/sarielhp/clihelp"
)

const maxConfirmRetries = 5
const confirmRetryDelay = 2 * time.Second

var (
	flagSpliceMove         bool
	flagSpliceCopy         bool
	flagSpliceN            int
	flagSpliceFolder       string
	flagSpliceFolderSuffix string
	flagSpliceFolderYear   string
	flagSplicePattern      string
	flagSpliceAllow        bool
)

// SpliceCmd returns the clihelp.Command for the splice command.
func SpliceCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "splice",
		Description: "Move messages from a folder into the keep/YYYY/MM/<folder> structure. The root \"keep\" is fixed. Use -f to change the target folder name, or -F to change the target folder name and automatically suffix it with the year and month.",
		UsageLine:   "mail_cli splice <folder> [flags]",
		Parameters: []clihelp.Param{
			{Name: "<folder>", Description: "The source folder name (e.g. \"receipts\" or \"acc1:folder\")."},
		},
		Options: []clihelp.Option{
			clihelp.Bool(&flagSpliceMove, "--move", false, "Actually move the messages instead of dry run"),
			clihelp.Bool(&flagSpliceCopy, "--copy", false, "Copy messages instead of moving them"),
			clihelp.Int(&flagSpliceN, "-n <num>", 10, "Number of messages to process"),
			clihelp.String(&flagSpliceFolder, "-f, --folder <name>", "", "Destination folder name (without year/month suffix)"),
			clihelp.String(&flagSpliceFolderSuffix, "-F, --folder-suffix <name>", "", "Destination folder name with year/month suffix attached"),
			clihelp.String(&flagSpliceFolderYear, "-Y, --folder-year <name>", "", "Destination folder name under year directory with year/month suffix attached"),
			clihelp.Bool(&flagSpliceAllow, "--allow", false, "Allow sourcing from folders that start with keep/"),
		},
		Examples: []clihelp.Example{
			{Line: "mail_cli splice receipts"},
			{Line: "mail_cli splice receipts --move"},
			{Line: "mail_cli splice receipts -f Archive"},
			{Line: "mail_cli splice receipts -F Receipts"},
			{Line: "mail_cli splice receipts -n 50 --move"},
			{Line: `mail_cli splice receipts -p "*order*" --move`},
		},
		Args: clihelp.ExactArgs(1),
		Run: func(ctx *clihelp.Context) error {
			args := ctx.Args
			defer func() {
				flagSpliceMove = false
				flagSpliceCopy = false
				flagSplicePattern = ""
			}()

			if flagSplicePattern == "" {
				flagSplicePattern = app.FlagScanPattern
			}

			if flagSpliceMove && flagSpliceCopy {
				return fmt.Errorf("cannot specify both --move and --copy")
			}

			folderArg := args[0]
			numMessages := flagSpliceN

			srcClient, srcFolder, err := session.ResolveClientAndLabel(folderArg)
			if err != nil {
				return err
			}

			if err := ValidateSpliceArgs(srcFolder, numMessages, flagSpliceAllow); err != nil {
				return err
			}

			if err := srcClient.Validate(); err != nil {
				return err
			}

			uniqueMatch, err := resolveUniqueLabel(srcClient, srcFolder)
			if err != nil {
				return fmt.Errorf("failed to resolve source folder %q: %w", srcFolder, err)
			}

			destClient := srcClient
			var destFolderBase string
			parts := strings.Split(uniqueMatch, "/")
			destFolderBase = parts[len(parts)-1]
			isSuffix := false
			isYearSuffix := false

			if flagSpliceFolderYear != "" {
				dc, df, err := session.ResolveClientAndLabel(flagSpliceFolderYear)
				if err != nil {
					return err
				}
				destClient = dc
				destFolderBase = df
				isYearSuffix = true
			} else if flagSpliceFolderSuffix != "" {
				dc, df, err := session.ResolveClientAndLabel(flagSpliceFolderSuffix)
				if err != nil {
					return err
				}
				destClient = dc
				destFolderBase = df
				isSuffix = true
			} else if flagSpliceFolder != "" {
				dc, df, err := session.ResolveClientAndLabel(flagSpliceFolder)
				if err != nil {
					return err
				}
				destClient = dc
				destFolderBase = df
			}

			if err := destClient.Validate(); err != nil {
				return err
			}

			labelCache := NewSanitizeLabelCache()
			baseDir := srcClient.Config().DownloadDir

			messageIDs, err := srcClient.FetchAndDownloadEmails(uniqueMatch, labelCache.Get(uniqueMatch))
			if err != nil {
				return fmt.Errorf("failed to fetch emails for %q: %w", uniqueMatch, err)
			}

			if flagSplicePattern != "" {
				messageIDs = filterByPattern(messageIDs, baseDir)
			}

			if len(messageIDs) == 0 {
				if flagSplicePattern != "" {
					fmt.Printf("%s No messages in folder %q match pattern %q.\n", app.PrefixWarn, uniqueMatch, flagSplicePattern)
				} else {
					fmt.Printf("%s No messages found in folder %q.\n", app.PrefixWarn, uniqueMatch)
				}
				return nil
			}

			if numMessages > 0 && len(messageIDs) > numMessages {
				messageIDs = messageIDs[:numMessages]
			}

			isCopy := flagSpliceCopy
			isMove := flagSpliceMove
			dryRun := !isCopy && !isMove

			var destFoldersSet map[string]bool
			if dryRun {
				destFoldersSet, err = runSpliceDryRun(srcClient, destClient, uniqueMatch, messageIDs, session, baseDir, isYearSuffix, isSuffix, destFolderBase)
			} else if isCopy {
				destFoldersSet, err = runSpliceCopy(srcClient, destClient, uniqueMatch, messageIDs, session, baseDir, isYearSuffix, isSuffix, destFolderBase)
			} else {
				destFoldersSet, err = runSpliceMove(srcClient, destClient, uniqueMatch, messageIDs, session, baseDir, isYearSuffix, isSuffix, destFolderBase)
			}
			if err != nil {
				return err
			}

			printSpliceTotals(session, srcClient, destClient, uniqueMatch, destFoldersSet, labelCache)
			return nil
		},
	}
}

func runSpliceDryRun(srcClient, destClient mailclient.MailClient, uniqueMatch string, messageIDs []string, session *app.Session, baseDir string, isYearSuffix, isSuffix bool, destFolderBase string) (map[string]bool, error) {
	destCache := make(map[string]map[string]bool)
	destFoldersSet := make(map[string]bool)

	for i, msgID := range messageIDs {
		if i > 0 {
			fmt.Println("---------------")
		}

		rawBytes, err := msg.Read(baseDir, msgID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  [%d/%d] Error loading %s: %v\n", i+1, len(messageIDs), msgID, err)
			continue
		}
		parsedEmail := email.ParseReader(bytes.NewReader(rawBytes), msgID, "")
		if parsedEmail == nil {
			fmt.Fprintf(os.Stderr, "  [%d/%d] Error parsing %s\n", i+1, len(messageIDs), msgID)
			continue
		}
		date := parsedEmail.EmailDate
		year := date.Format("2006")
		month := date.Format("01")

		dest := computeDest(year, month, isYearSuffix, isSuffix, destFolderBase)
		destFoldersSet[dest] = true
		srcDisplay := app.FormatAccountLabel(session, srcClient, uniqueMatch)
		destDisplay := app.FormatAccountLabel(session, destClient, dest)

		if isSameAccount(srcClient, destClient) && strings.EqualFold(uniqueMatch, dest) {
			fmt.Printf("[%d/%d] %s %s | Subject: %q\n", i+1, len(messageIDs), app.PrefixWarn, strings.TrimSpace(msgID), parsedEmail.Subject)
			fmt.Printf("    %s\n", app.ColorYellow.Sprint("(source equals destination, skipping)"))
			continue
		}

		existsInDest := isMessageInDest(destClient, dest, msgID, parsedEmail, destCache)

		fmt.Printf("[%d/%d] %s %s | Subject: %q\n", i+1, len(messageIDs), app.PrefixInfo, strings.TrimSpace(msgID), parsedEmail.Subject)
		fmt.Printf("    %s\n       →   %s\n", srcDisplay, destDisplay)
		if existsInDest {
			fmt.Printf("    %s\n", app.ColorYellow.Sprint("(already exists in destination, skipping)"))
		}
	}

	return destFoldersSet, nil
}

func runSpliceCopy(srcClient, destClient mailclient.MailClient, uniqueMatch string, messageIDs []string, session *app.Session, baseDir string, isYearSuffix, isSuffix bool, destFolderBase string) (map[string]bool, error) {
	destCache := make(map[string]map[string]bool)
	destFoldersSet := make(map[string]bool)

	for i, msgID := range messageIDs {
		if i > 0 {
			fmt.Println("---------------")
		}

		rawBytes, err := msg.Read(baseDir, msgID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  [%d/%d] Error loading %s: %v\n", i+1, len(messageIDs), msgID, err)
			continue
		}
		parsedEmail := email.ParseReader(bytes.NewReader(rawBytes), msgID, "")
		if parsedEmail == nil {
			fmt.Fprintf(os.Stderr, "  [%d/%d] Error parsing %s\n", i+1, len(messageIDs), msgID)
			continue
		}
		date := parsedEmail.EmailDate
		year := date.Format("2006")
		month := date.Format("01")

		dest := computeDest(year, month, isYearSuffix, isSuffix, destFolderBase)
		destFoldersSet[dest] = true
		srcDisplay := app.FormatAccountLabel(session, srcClient, uniqueMatch)
		destDisplay := app.FormatAccountLabel(session, destClient, dest)

		if isSameAccount(srcClient, destClient) && strings.EqualFold(uniqueMatch, dest) {
			fmt.Printf("[%d/%d] %s %s | Subject: %q\n", i+1, len(messageIDs), app.PrefixWarn, strings.TrimSpace(msgID), parsedEmail.Subject)
			fmt.Printf("    %s\n", app.ColorYellow.Sprint("(source equals destination, skipping)"))
			continue
		}

		existsInDest := isMessageInDest(destClient, dest, msgID, parsedEmail, destCache)
		if existsInDest {
			fmt.Printf("[%d/%d] %s %s | Subject: %q\n", i+1, len(messageIDs), app.PrefixInfo, strings.TrimSpace(msgID), parsedEmail.Subject)
			fmt.Printf("    %s\n       →   %s\n", srcDisplay, destDisplay)
			fmt.Printf("    %s\n", app.ColorYellow.Sprint("(already exists in destination, skipping copy)"))
			continue
		}

		if err := destClient.EnsureLabelExists(dest); err != nil {
			printSpliceError(i, len(messageIDs), msgID, parsedEmail.Subject)
			fmt.Printf("    %s\n       →   %s (failed to create folder: %v)\n", srcDisplay, destDisplay, err)
			continue
		}

		if err := spliceCopyOrUpload(srcClient, destClient, msgID, uniqueMatch, dest, rawBytes); err != nil {
			printSpliceError(i, len(messageIDs), msgID, parsedEmail.Subject)
			if isSameAccount(srcClient, destClient) {
				fmt.Printf("    %s\n       →   %s (failed to copy: %v)\n", srcDisplay, destDisplay, err)
			} else {
				fmt.Printf("    %s\n       →   %s (failed to upload to target account: %v)\n", srcDisplay, destDisplay, err)
			}
			continue
		}

		delete(destCache, dest)
		printSpliceSuccess(i, len(messageIDs), msgID, parsedEmail.Subject)
		fmt.Printf("    %s\n       →   %s\n", srcDisplay, destDisplay)
		fmt.Printf("    %s\n", app.ColorGreen.Sprint("(copied to destination)"))
	}

	return destFoldersSet, nil
}

func runSpliceMove(srcClient, destClient mailclient.MailClient, uniqueMatch string, messageIDs []string, session *app.Session, baseDir string, isYearSuffix, isSuffix bool, destFolderBase string) (map[string]bool, error) {
	destCache := make(map[string]map[string]bool)
	destFoldersSet := make(map[string]bool)

	for i, msgID := range messageIDs {
		if i > 0 {
			fmt.Println("---------------")
		}

		rawBytes, err := msg.Read(baseDir, msgID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  [%d/%d] Error loading %s: %v\n", i+1, len(messageIDs), msgID, err)
			continue
		}
		parsedEmail := email.ParseReader(bytes.NewReader(rawBytes), msgID, "")
		if parsedEmail == nil {
			fmt.Fprintf(os.Stderr, "  [%d/%d] Error parsing %s\n", i+1, len(messageIDs), msgID)
			continue
		}
		date := parsedEmail.EmailDate
		year := date.Format("2006")
		month := date.Format("01")

		dest := computeDest(year, month, isYearSuffix, isSuffix, destFolderBase)
		destFoldersSet[dest] = true
		srcDisplay := app.FormatAccountLabel(session, srcClient, uniqueMatch)
		destDisplay := app.FormatAccountLabel(session, destClient, dest)

		if isSameAccount(srcClient, destClient) && strings.EqualFold(uniqueMatch, dest) {
			fmt.Printf("[%d/%d] %s %s | Subject: %q\n", i+1, len(messageIDs), app.PrefixWarn, strings.TrimSpace(msgID), parsedEmail.Subject)
			fmt.Printf("    %s\n", app.ColorYellow.Sprint("(source equals destination, skipping)"))
			continue
		}

		existsInDest := isMessageInDest(destClient, dest, msgID, parsedEmail, destCache)

		if err := destClient.EnsureLabelExists(dest); err != nil {
			printSpliceError(i, len(messageIDs), msgID, parsedEmail.Subject)
			fmt.Printf("    %s\n       →   %s (failed to create folder: %v)\n", srcDisplay, destDisplay, err)
			continue
		}

		if !existsInDest {
			if err := spliceCopyOrUpload(srcClient, destClient, msgID, uniqueMatch, dest, rawBytes); err != nil {
				printSpliceError(i, len(messageIDs), msgID, parsedEmail.Subject)
				if isSameAccount(srcClient, destClient) {
					fmt.Printf("    %s\n       →   %s (failed to copy to destination: %v)\n", srcDisplay, destDisplay, err)
				} else {
					fmt.Printf("    %s\n       →   %s (failed to upload to target account: %v)\n", srcDisplay, destDisplay, err)
				}
				continue
			}
		}

		delete(destCache, dest)

		confirmed, confirmErr := confirmMessageInDest(destClient, dest, msgID, parsedEmail)
		if !confirmed || confirmErr != nil {
			printSpliceError(i, len(messageIDs), msgID, parsedEmail.Subject)
			fmt.Printf("    %s\n       →   %s (unconfirmed in destination; source message NOT deleted: %v)\n", srcDisplay, destDisplay, confirmErr)
			continue
		}

		srcStillThere, srcErr := verifySourceMessage(srcClient, uniqueMatch, msgID, baseDir)
		if !srcStillThere || srcErr != nil {
			printSpliceWarn(i, len(messageIDs), msgID, parsedEmail.Subject)
			fmt.Printf("    %s\n       →   %s (source message gone before deletion; may have been moved already: %v)\n", srcDisplay, destDisplay, srcErr)
			continue
		}

		if err := spliceDeleteSource(srcClient, destClient, msgID, uniqueMatch, dest, session); err != nil {
			printSpliceWarn(i, len(messageIDs), msgID, parsedEmail.Subject)
			fmt.Printf(" (confirmed in destination, but failed to remove from source: %v)\n", err)
			continue
		}

		printSpliceSuccess(i, len(messageIDs), msgID, parsedEmail.Subject)
		fmt.Printf("    %s\n       →   %s\n", srcDisplay, destDisplay)
		if existsInDest {
			fmt.Printf("    %s\n", app.ColorGreen.Sprint("(already existed in destination, confirmed and removed from source)"))
		} else {
			fmt.Printf("    %s\n", app.ColorGreen.Sprint("(confirmed in destination, removed from source)"))
		}
	}

	return destFoldersSet, nil
}

// computeDest builds the destination folder path from the date components and user flags.
func computeDest(year, month string, isYearSuffix, isSuffix bool, destFolderBase string) string {
	if isYearSuffix {
		return "keep/" + year + "/" + destFolderBase + "-" + year + "-" + month
	} else if isSuffix {
		return "keep/" + year + "/" + month + "/" + destFolderBase + "-" + year + "-" + month
	}
	return "keep/" + year + "/" + month + "/" + destFolderBase
}

// spliceCopyOrUpload copies (same account) or uploads (cross-account) a message.
func spliceCopyOrUpload(srcClient, destClient mailclient.MailClient, msgID, srcFolder, destFolder string, rawBytes []byte) error {
	if isSameAccount(srcClient, destClient) {
		return srcClient.CopyEmail([]string{msgID}, srcFolder, destFolder)
	}
	return destClient.UploadRawEmail(rawBytes, destFolder)
}

// spliceDeleteSource removes the message from the source folder after a successful move.
// For same-account moves, it moves within the same account. For cross-account, it trashes.
func spliceDeleteSource(srcClient, destClient mailclient.MailClient, msgID, srcFolder, destFolder string, session *app.Session) error {
	if isSameAccount(srcClient, destClient) {
		return srcClient.MoveEmail([]string{msgID}, srcFolder, destFolder)
	}
	trashTarget := "Trash"
	if session.ResolveTrashTarget != nil {
		if t, err := session.ResolveTrashTarget(srcClient); err == nil && t != "" {
			trashTarget = t
		}
	}
	return srcClient.MoveEmail([]string{msgID}, srcFolder, trashTarget)
}

// filterByPattern filters message IDs whose subject matches the given pattern.
func filterByPattern(messageIDs []string, baseDir string) []string {
	var filteredIDs []string
	for _, msgID := range messageIDs {
		rawBytes, err := msg.Read(baseDir, msgID)
		if err != nil {
			slog.Warn("splice: pattern filter skipping message due to read error", slog.String("msgID", msgID), slog.Any("error", err))
			continue
		}
		parsedEmail := email.ParseReader(bytes.NewReader(rawBytes), msgID, "")
		if parsedEmail == nil {
			continue
		}
		if email.MatchPattern(parsedEmail.Subject, flagSplicePattern) {
			filteredIDs = append(filteredIDs, msgID)
		}
	}
	return filteredIDs
}

// printSpliceTotals prints the final message counts for source and destination folders.
func printSpliceTotals(session *app.Session, srcClient, destClient mailclient.MailClient, uniqueMatch string, destFoldersSet map[string]bool, labelCache SanitizeLabelCache) {
	srcFinalIDs, _ := srcClient.FetchAndDownloadEmails(uniqueMatch, labelCache.Get(uniqueMatch))
	srcDisplayFolder := app.FormatAccountLabel(session, srcClient, uniqueMatch)

	fmt.Printf("\n%s Folder totals after splice:\n", app.PrefixInfo)
	fmt.Printf("    %s: %d message(s)\n", srcDisplayFolder, len(srcFinalIDs))

	for df := range destFoldersSet {
		destFinalIDs, err2 := destClient.FetchAndDownloadEmails(df, labelCache.Get(df))
		if err2 == nil {
			destDisplayFolder := app.FormatAccountLabel(session, destClient, df)
			fmt.Printf("    %s: %d message(s)\n", destDisplayFolder, len(destFinalIDs))
		}
	}
}

// print helpers for splice status lines

func printSpliceError(i, total int, msgID, subject string) {
	fmt.Printf("[%d/%d] %s %s | Subject: %q\n", i+1, total, app.PrefixError, msgID, subject)
}

func printSpliceWarn(i, total int, msgID, subject string) {
	fmt.Printf("[%d/%d] %s %s | Subject: %q\n", i+1, total, app.PrefixWarn, msgID, subject)
}

func printSpliceSuccess(i, total int, msgID, subject string) {
	fmt.Printf("[%d/%d] %s %s | Subject: %q\n", i+1, total, app.PrefixSuccess, msgID, subject)
}
