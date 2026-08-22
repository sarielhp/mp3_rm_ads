package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"mail_cli/app"

	"github.com/sarielhp/clihelp"
	"github.com/spf13/pflag"
)

// InitCLI initializes and returns the *clihelp.App with all subcommands and global options.
func InitCLI(session *app.Session) *clihelp.App {
	homeDir, _ := os.UserHomeDir()
	configPath := filepath.Join(homeDir, ".config", app.AppName, "config.json")

	cliApp := &clihelp.App{
		Name:        app.AppName,
		Description: "Gmail / JMAP Spam Checker CLI tool",
		Version:     app.Version,
		ConfigPath:  configPath,
		GlobalNote:  fmt.Sprintf("GitHub: [%s](%s)", app.GitHubURL, app.GitHubURL),
		Shortcuts: []clihelp.Command{
			{Name: "ss", Description: "Shortcut alias for: scan spam"},
			{Name: "sb", Description: "Shortcut alias for: spam bye"},
		},
		PersistentOptions: []clihelp.Option{
			clihelp.Bool(&app.FlagVerbose, "-v, --verbose", false, "Enable verbose output"),
			clihelp.String(&app.FlagAccountName, "-A, --account <name>", "", "Specify target account name (e.g. personal-gmail, work-jmap)"),
			clihelp.Bool(&app.FlagLogAppend, "--logappend", false, "Keep and append to existing log files instead of deleting them at startup"),
			clihelp.Bool(&app.FlagFixConfig, "--fix", false, "Automatically fix received_folder configuration issues"),
			clihelp.String(&app.FlagScanPattern, "-p, --pattern <pattern>", "", "Only process messages whose subject contains this pattern."),
			{
				Flags:       "--read-only",
				Description: "Run in read-only / dry-run mode (no server modifications; aliases: --dry-run, --ro)",
				Binder: func(fs *pflag.FlagSet) error {
					fs.BoolVar(&app.FlagReadOnly, "read-only", false, "Run in read-only / dry-run mode (no server modifications)")
					fs.BoolVar(&app.FlagReadOnly, "dry-run", false, "Alias for --read-only")
					fs.BoolVar(&app.FlagReadOnly, "ro", false, "Alias for --read-only")
					return nil
				},
			},
		},
		GlobalFlags: []clihelp.Option{
			{
				Flags:       "-m, --move [From]",
				Description: "Move identified spam emails to Spam folder. Optional: specify From address to move a single unique message.",
				Binder: func(fs *pflag.FlagSet) error {
					fs.StringVarP(&app.FlagMoveSpamStr, "move", "m", "", "Move identified spam emails to Spam folder. Optional: specify From address to move a single unique message.")
					if f := fs.Lookup("move"); f != nil {
						f.NoOptDefVal = "true"
					}
					return nil
				},
			},
			clihelp.String(&app.FlagMoveInboxStr, "--inbox-move <From>", "", "Move identified emails from a specific From address back to the Inbox folder."),
			clihelp.Bool(&app.FlagVersion, "--version", false, "Print the version number and exit"),
		},
		BeforeRun: func(ctx *clihelp.Context) error {
			app.CmdExecutionStarted = true
			if session.Config != nil {
				session.Config.Verbose = app.FlagVerbose
				session.Config.FixConfig = app.FlagFixConfig
				if app.FlagReadOnly {
					session.Config.ReadOnly = true
				}
				if app.FlagAccountName != "" {
					session.Config.SelectedAccount = app.FlagAccountName
				}
			}
			return nil
		},
		Run: func(ctx *clihelp.Context) error {
			if app.FlagVersion {
				fmt.Printf("mail_cli version %s\n", app.Version)
				os.Exit(0)
			}

			if session.PreCheck != nil {
				if err := session.PreCheck(session.Config); err != nil {
					return err
				}
			}
			if session.RunScan != nil {
				moved, err := session.RunScan(session.Config, "inbox", app.FlagMoveSpamStr, app.FlagMoveInboxStr)
				if err != nil {
					return err
				}
				if moved > 0 {
					fmt.Printf("\nTotal emails moved during scan: %s\n", app.ColorBoldYellow.Sprint(moved))
				}
			}
			return nil
		},
		Commands: []clihelp.Command{
			ScanCmd(session),
			ShowCmd(session),
			TestCmd(session),
			WlistCmd(session),
			BlistCmd(session),
			RuleCmd(session),
			LabelsCmd(session),
			SpamCmd(session),
			FilterCmd(session),
			AccountsCmd(session),
			LearnHamCmd(session),
			UnspamCmd(session),
			LearningCmd(session),
			ArcCmd(session),
			ConfigCmd(session),
			CacheCmd(session),
			CalendarCmd(session),
			CalAddCmd(session),
			TuiCmd(session),
			ColorCmd(session),
			SpliceCmd(session),
			MigrateCmd(session),
			SplitCmd(session),
			DownloadCmd(session),
			UploadCmd(session),
			LastCmd(session),
		},
	}

	return cliApp
}
