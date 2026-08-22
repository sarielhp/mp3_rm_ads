package cli

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"mail_cli/app"
	"mail_cli/cache"

	"github.com/sarielhp/clihelp"
)

var (
	flagLabelsListExt    bool
	flagLabelsListNoZero bool
	flagLabelsListFull   bool
	flagLabelsListReal   bool
)

// LabelsCmd returns the clihelp.Command for the labels command.
func LabelsCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "labels",
		Description: "Manage and inspect server labels and folder hierarchies.",
		UsageLine:   "mail_cli labels <subcommand> [args...]",
		Subcommands: []clihelp.Command{
			labelsListCmd(session),
			labelsPrintCmd(session),
			labelsRenameCmd(session),
			labelsFixCmd(session),
			labelsDelCmd(session),
			labelsSearchCmd(session),
			labelsCacheCmd(session),
			labelsCreateCmd(session),
		},
		Examples: []clihelp.Example{
			{Line: "mail_cli labels list"},
			{Line: "mail_cli labels list --ext"},
			{Line: "mail_cli labels search receipts"},
			{Line: `mail_cli labels create "Receipts/2026"`},
			{Line: `mail_cli labels rename "OldName" "NewName"`},
		},
	}
}

func labelsListCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "list",
		Aliases:     []string{"ls"},
		Title:       "labels list",
		Description: "List all folders and labels in a hierarchical tree view. Pass --full to show full paths, --ext to show unread and total message counts, --nozero to omit empty folders, or --real to query the server in real-time.",
		UsageLine:   "mail_cli labels list [flags]",
		Options: []clihelp.Option{
			clihelp.Bool(&app.FlagLabelsListAll, "-a, --all", false, "List all labels, including those with zero messages"),
			clihelp.Bool(&flagLabelsListNoZero, "-z, --nozero", false, "Only list folders that have at least one message in them"),
			clihelp.Bool(&flagLabelsListFull, "-f, --full", false, "Show full label names (without hierarchical view)"),
			clihelp.Bool(&flagLabelsListExt, "-e, --ext", false, "Show (unread/total) message counts"),
			clihelp.Bool(&flagLabelsListReal, "--real", false, "Fetch real-time counts from server (bypasses cache)"),
		},
		Subcommands: []clihelp.Command{
			labelsListFullCmd(session),
		},
		Run: func(ctx *clihelp.Context) error {
			args := ctx.Args
			isFull := flagLabelsListFull
			for _, a := range args {
				if strings.EqualFold(a, "full") {
					isFull = true
				}
			}
			if isFull {
				isExt, isNoZero := parseExtNoZero(args, flagLabelsListExt, flagLabelsListNoZero)
				realOnly := findArg(args, "real") || flagLabelsListReal
				return runLabelsListFull(session, isExt, isNoZero, realOnly)
			}
			client, err := labelsClient(session)
			if err != nil {
				return err
			}
			_, isNoZero := parseExtNoZero(args, flagLabelsListExt, flagLabelsListNoZero)
			if isNoZero && client.Config() != nil {
				client.Config().HideZeroLabels = true
			}
			return client.ListLabels()
		},
		Examples: []clihelp.Example{
			{Line: "mail_cli labels list"},
			{Line: "mail_cli labels list --ext"},
			{Line: "mail_cli labels list --full --ext"},
			{Line: "mail_cli labels list --nozero"},
		},
	}
}

func labelsListFullCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "full",
		Title:       "labels list full",
		Description: "List labels with their full name (without hierarchical view).",
		UsageLine:   "mail_cli labels list full [flags]",
		Options: []clihelp.Option{
			clihelp.Bool(&app.FlagLabelsListAll, "-a, --all", false, "List all labels, including those with zero messages"),
			clihelp.Bool(&flagLabelsListExt, "-e, --ext", false, "Show (unread/total) message counts"),
			clihelp.Bool(&flagLabelsListNoZero, "-z, --nozero", false, "Only list folders that have at least one message in them"),
			clihelp.Bool(&flagLabelsListReal, "--real", false, "Fetch real-time counts from server (bypasses cache)"),
		},
		Subcommands: []clihelp.Command{
			labelsListFullExtCmd(session),
			labelsListFullNoZeroCmd(session),
		},
		Run: func(ctx *clihelp.Context) error {
			args := ctx.Args
			isExt, isNoZero := parseExtNoZero(args, flagLabelsListExt, flagLabelsListNoZero)
			realOnly := findArg(args, "real") || flagLabelsListReal
			return runLabelsListFull(session, isExt, isNoZero, realOnly)
		},
		Examples: []clihelp.Example{
			{Line: "mail_cli labels list full"},
			{Line: "mail_cli labels list full --ext"},
		},
	}
}

func labelsListFullExtCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "ext",
		Description: "List labels with their full name and (unread/total) message counts.",
		UsageLine:   "mail_cli labels list full ext [flags]",
		Options: []clihelp.Option{
			clihelp.Bool(&app.FlagLabelsListAll, "-a, --all", false, "List all labels, including those with zero messages"),
			clihelp.Bool(&flagLabelsListNoZero, "-z, --nozero", false, "Only list folders that have at least one message in them"),
			clihelp.Bool(&flagLabelsListReal, "--real", false, "Fetch real-time counts from server (bypasses cache)"),
		},
		Subcommands: []clihelp.Command{
			labelsListFullExtRealCmd(session),
		},
		Run: func(ctx *clihelp.Context) error {
			args := ctx.Args
			isExt, isNoZero := parseExtNoZero(append([]string{"ext"}, args...), flagLabelsListExt, flagLabelsListNoZero)
			realOnly := findArg(args, "real") || flagLabelsListReal
			return runLabelsListFull(session, isExt, isNoZero, realOnly)
		},
		Examples: []clihelp.Example{
			{Line: "mail_cli labels list full ext"},
		},
	}
}

func labelsListFullExtRealCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "real",
		Description: "List labels with real (uncached) counts from the server.",
		UsageLine:   "mail_cli labels list full ext real",
		Args:        clihelp.NoArgs,
		Run: func(ctx *clihelp.Context) error {
			return runLabelsListFull(session, true, true, true)
		},
		Examples: []clihelp.Example{
			{Line: "mail_cli labels list full ext real"},
		},
	}
}

func labelsListFullNoZeroCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "nozero",
		Description: "List non-empty labels with their full name.",
		UsageLine:   "mail_cli labels list full nozero [flags]",
		Options: []clihelp.Option{
			clihelp.Bool(&app.FlagLabelsListAll, "-a, --all", false, "List all labels, including those with zero messages"),
			clihelp.Bool(&flagLabelsListExt, "-e, --ext", false, "Show (unread/total) message counts"),
			clihelp.Bool(&flagLabelsListReal, "--real", false, "Fetch real-time counts from server (bypasses cache)"),
		},
		Run: func(ctx *clihelp.Context) error {
			args := ctx.Args
			isExt, isNoZero := parseExtNoZero(append([]string{"nozero"}, args...), flagLabelsListExt, flagLabelsListNoZero)
			realOnly := findArg(args, "real") || flagLabelsListReal
			return runLabelsListFull(session, isExt, isNoZero, realOnly)
		},
		Examples: []clihelp.Example{
			{Line: "mail_cli labels list full nozero"},
		},
	}
}

func labelsPrintCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "print",
		Aliases:     []string{"pr"},
		Description: "Print all labels/folders, one per line (full path only).",
		UsageLine:   "mail_cli labels print",
		Args:        clihelp.NoArgs,
		Run: func(ctx *clihelp.Context) error {
			client, err := labelsClient(session)
			if err != nil {
				return err
			}
			items, err := client.GetLabelItems()
			if err != nil {
				return err
			}
			var paths []string
			for _, item := range items {
				paths = append(paths, item.FullName)
			}
			sort.Strings(paths)
			for _, p := range paths {
				fmt.Println(p)
			}
			return nil
		},
		Examples: []clihelp.Example{
			{Line: "mail_cli labels print"},
		},
	}
}

func labelsRenameCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "rename",
		Title:       "labels rename <old_name> <new_name>",
		Description: "Rename a label/folder and update all existing routing rules that reference it.",
		UsageLine:   "mail_cli labels rename <old_name> <new_name>",
		Parameters: []clihelp.Param{
			{Name: "<old_name>", Description: "The full path of the label to rename."},
			{Name: "<new_name>", Description: "The new full path for the label."},
		},
		Args: clihelp.ExactArgs(2),
		Run: func(ctx *clihelp.Context) error {
			args := ctx.Args
			client, err := labelsClient(session)
			if err != nil {
				return err
			}
			oldName := args[0]
			newName := args[1]
			if err := client.RenameLabel(oldName, newName); err != nil {
				return err
			}
			fc, targetAcc, _, configPath, err := cfgResolveAccount(session)
			if err != nil {
				return err
			}
			updateRuleLabels(fc, targetAcc, oldName, newName)
			if err := cfgSaveConfig(configPath, fc); err != nil {
				return err
			}
			return nil
		},
		Examples: []clihelp.Example{
			{Line: `mail_cli labels rename "Archive/2025" "Archive/2026"`},
		},
	}
}

func labelsFixCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "fix",
		Description: "Ensure that all parent folders exist for every nested folder path in the account.",
		UsageLine:   "mail_cli labels fix",
		Args:        clihelp.NoArgs,
		Run: func(ctx *clihelp.Context) error {
			client, err := labelsClient(session)
			if err != nil {
				return err
			}
			return client.FixLabels()
		},
		Examples: []clihelp.Example{
			{Line: "mail_cli labels fix"},
		},
	}
}

func labelsDelCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "del",
		Aliases:     []string{"delete"},
		Title:       "labels del <lbl_name>",
		Description: "Delete a label/folder from the server.",
		UsageLine:   "mail_cli labels del <lbl_name>",
		Parameters: []clihelp.Param{
			{Name: "<lbl_name>", Description: "The full path of the label to delete."},
		},
		Args: clihelp.ExactArgs(1),
		Run: func(ctx *clihelp.Context) error {
			client, err := labelsClient(session)
			if err != nil {
				return err
			}
			return client.DeleteLabel(ctx.Args[0])
		},
		Examples: []clihelp.Example{
			{Line: "mail_cli labels del Receipts/Old"},
		},
	}
}

func labelsSearchCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "search",
		Title:       "labels search <substring...>",
		Description: "Search cached labels by one or more substring patterns (all must match).",
		UsageLine:   "mail_cli labels search <substring> [additional_substrings...]",
		Parameters: []clihelp.Param{
			{Name: "<substring>", Description: "Substring to search for in folder paths."},
			{Name: "[additional_substrings...]", Description: "Optional extra substrings — all must match for a label to be listed."},
		},
		Args: clihelp.MinimumNArgs(1),
		Run: func(ctx *clihelp.Context) error {
			args := ctx.Args
			cacheDir, err := ResolveCacheDir(session.Config)
			if err != nil {
				return err
			}
			cs := &cache.DiskCacheStore{DownloadDir: cacheDir}
			items, cacheErr := cs.GetLabelItems()
			age, ageErr := cs.CachedLabelItemsAge()

			if cacheErr != nil || ageErr != nil {
				client, err := labelsClient(session)
				if err != nil {
					return err
				}
				items, err = client.GetLabelItems()
				if err != nil {
					return err
				}
			} else if age >= 24*time.Hour {
				go func() {
					client, err := session.GetClient(session.Config)
					if err != nil {
						slog.Warn("async refresh: client creation failed", slog.String("error", err.Error()))
						return
					}
					if err := client.Validate(); err != nil {
						slog.Warn("async refresh: validation failed", slog.String("error", err.Error()))
						return
					}
					if _, err := client.GetLabelItems(); err != nil {
						slog.Warn("background labels cache refresh failed", slog.String("error", err.Error()))
					}
				}()
			}

			matches, _ := SearchLabels(session.Config, args)
			if matches == nil {
				var lowerPatterns []string
				for _, p := range args {
					lowerPatterns = append(lowerPatterns, strings.ToLower(p))
				}
				for _, item := range items {
					lowerName := strings.ToLower(item.FullName)
					allMatched := true
					for _, lp := range lowerPatterns {
						if !strings.Contains(lowerName, lp) {
							allMatched = false
							break
						}
					}
					if allMatched {
						matches = append(matches, item.FullName)
					}
				}
				sort.Strings(matches)
			}
			for _, m := range matches {
				fmt.Println(m)
			}
			return nil
		},
		Examples: []clihelp.Example{
			{Line: "mail_cli labels search receipts"},
			{Line: "mail_cli labels search archive 2026"},
		},
	}
}

func labelsCacheCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "cache",
		Title:       "labels cache",
		Description: "Manage the local labels cache: fetch and refresh cached label/folder data from the server.",
		UsageLine:   "mail_cli labels cache <subcommand>",
		Subcommands: []clihelp.Command{
			labelsCacheUpdateCmd(session),
		},
		Examples: []clihelp.Example{
			{Line: "mail_cli labels cache update"},
		},
	}
}

func labelsCacheUpdateCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "update",
		Aliases:     []string{"up", "sync"},
		Description: "Fetch the latest labels and folder hierarchies from the server and refresh the local cache.",
		UsageLine:   "mail_cli labels cache update",
		Args:        clihelp.NoArgs,
		Run: func(ctx *clihelp.Context) error {
			client, err := labelsClient(session)
			if err != nil {
				return err
			}
			_, err = client.GetLabelItems()
			if err != nil {
				return err
			}
			fmt.Printf("%s Labels cache updated\n", app.PrefixSuccess)
			return nil
		},
		Examples: []clihelp.Example{
			{Line: "mail_cli labels cache update"},
		},
	}
}

func labelsCreateCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "create",
		Title:       "labels create <lbl_name>",
		Description: "Create a new label or folder path on the server.",
		UsageLine:   "mail_cli labels create <lbl_name>",
		Parameters: []clihelp.Param{
			{Name: "<lbl_name>", Description: "The name or path of the label/folder to create."},
		},
		Args: clihelp.ExactArgs(1),
		Run: func(ctx *clihelp.Context) error {
			client, err := labelsClient(session)
			if err != nil {
				return err
			}
			targetLabel := ctx.Args[0]
			if strings.TrimSpace(targetLabel) == "" {
				return fmt.Errorf("label name cannot be empty")
			}

			labels, err := client.GetMatchingLabels("")
			if err != nil {
				return err
			}
			for _, l := range labels {
				if strings.EqualFold(l, targetLabel) {
					return fmt.Errorf("label %q already exists", targetLabel)
				}
			}

			if err := client.EnsureLabelExists(targetLabel); err != nil {
				return err
			}
			fmt.Printf("%s Successfully created label %q\n", app.PrefixSuccess, targetLabel)
			return nil
		},
		Examples: []clihelp.Example{
			{Line: `mail_cli labels create "Work/Projects"`},
		},
	}
}
