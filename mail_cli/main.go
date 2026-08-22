package main

import (
	"fmt"
	"os"
	"path/filepath"

	"mail_cli/app"
	"mail_cli/backend/gmail"
	"mail_cli/cfg_g"
	"mail_cli/cfghandler"
	"mail_cli/cli"
	"mail_cli/organize"
	"mail_cli/sieve"
	"mail_cli/spam"
	"mail_cli/tui"
	"mail_cli/usage"

	"github.com/sarielhp/clihelp"
)

func main() {
	if os.Getenv("CLIHELP_GEN") != "" {
		if changed, err := usage.RenderMarkdown(clihelp.MarkdownOptions{Dir: "docs/clihelp"}); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating markdown documentation: %v\n", err)
			os.Exit(1)
		} else if changed {
			fmt.Println("Generated updated documentation in docs/clihelp/")
		}
		return
	}

	homeDir, errHome := os.UserHomeDir()
	if errHome != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to detect user home directory: %v\n", errHome)
		os.Exit(1)
	}
	config, err := cfg_g.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to load config: %v\n", err)
		os.Exit(1)
	}
	config.ConfigDir = filepath.Join(homeDir, ".config", app.AppName)

	logAppend := false
	for _, arg := range os.Args {
		if arg == "-logappend" || arg == "--logappend" {
			logAppend = true
			break
		}
	}
	app.InitLogger(config.ConfigDir, logAppend)
	defer app.CloseLogger()

	preprocessedArgs := preprocessArgs(os.Args, config)

	session := &app.Session{
		Config: config,
		// Keep old fields for backward compatibility
		GetClient: func(cfg *cfg_g.Config) (MailClient, error) {
			return getClientForSelected(cfg)
		},
		GetClientForAccountIndex: func(cfg *cfg_g.Config, index int) (MailClient, error) {
			return getClientForAccountIndex(cfg, index)
		},
		PreCheck: func(cfg *cfg_g.Config) error {
			return runPreFlightCheck(cfg)
		},
		RunScan: func(cfg *cfg_g.Config, labelPrefix, moveSpam, moveInbox string) (int, error) {
			return runScanOnAccounts(cfg, labelPrefix, moveSpam, moveInbox)
		},
		RunShow: func(cfg *cfg_g.Config, labelPrefix, msgID string) error {
			return runShowOnAccounts(cfg, labelPrefix, msgID)
		},
		RunUnspam: func(cfg *cfg_g.Config, id string) error {
			client, err := getClientForSelected(cfg)
			if err != nil {
				return err
			}
			return spam.UnspamByID(client, cfg, id)
		},
		RunUnspamFolder: func(cfg *cfg_g.Config, folder string) error {
			client, err := getClientForSelected(cfg)
			if err != nil {
				return err
			}
			return spam.UnspamFolder(client, cfg, folder)
		},
		RunLearningReset: func(cfg *cfg_g.Config) error {
			return spam.ResetLearning(cfg)
		},
		RunTests: func(cfg *cfg_g.Config) error {
			return gmail.RunRESTTests(cfg)
		},
		ExportRules: func(cfg *cfg_g.Config, filePath string) error {
			return exportRulesToFile(cfg, filePath)
		},
		ImportRules: func(cfg *cfg_g.Config, filePath string) error {
			return importRulesFromFile(cfg, filePath)
		},
		ExportSieve: func(cfg *cfg_g.Config, outputPath string) error {
			return sieve.ExportScript(cfg, outputPath)
		},
		MarkSpam: func(cfg *cfg_g.Config, id string) error {
			client, err := getClientForSelected(cfg)
			if err != nil {
				return err
			}
			return spam.MarkByID(client, cfg, id)
		},
		LearnHam: func(cfg *cfg_g.Config, folder string, force bool) error {
			client, err := getClientForSelected(cfg)
			if err != nil {
				return err
			}
			if force {
				cfg.ForceLearn = true
			}
			return spam.LearnHam(client, cfg, folder)
		},
		CalendarAdd: func(cfg *cfg_g.Config, client MailClient, labelPrefix, msgID string) error {
			return gmail.PerformCalendarAdd(cfg, client, labelPrefix, msgID)
		},
		CalendarWeek: func(cfg *cfg_g.Config) error {
			return gmail.PerformCalendarWeek(cfg)
		},
		CalAddAll: func(cfg *cfg_g.Config, client MailClient) error {
			return gmail.PerformCalAddAll(cfg, client)
		},
		ConfigShow: func(cfg *cfg_g.Config) error {
			return cfghandler.HandleConfigShow(cfg)
		},
		ConfigValidate: func(cfg *cfg_g.Config) error {
			return cfghandler.PerformConfigValidation(cfg)
		},
		ConfigSet: func(cfg *cfg_g.Config, key, value string, accountSpecific bool) error {
			return cfghandler.HandleConfigSet(cfg, key, value, accountSpecific)
		},
		ConfigReset: func(cfg *cfg_g.Config, key string) error {
			return cfghandler.HandleConfigReset(cfg, key)
		},
		InitTUI: func(cfg *cfg_g.Config, labelPrefix string) error {
			return tui.InitTuiMode(cfg, labelPrefix, &tui.Backend{
				RunPreFlightCheck:    runPreFlightCheck,
				GetClientForSelected: getClientForSelected,
				GetClientForAccount: func(c *cfg_g.Config, acc AccountConfig) (MailClient, error) {
					return NewMailClient(acc, c)
				},
				GetClientForAccountIndex: func(c *cfg_g.Config, index int) (MailClient, error) {
					return getClientForAccountIndex(c, index)
				},
				InitTuiLogger:  app.InitTuiLogger,
				CloseTuiLogger: app.CloseTuiLogger,
				ResolveArchiveTarget: func(client MailClient) (string, error) {
					return organize.ResolveArchiveTarget(client)
				},
				ResolveTrashTarget: func(client MailClient) (string, error) {
					return organize.ResolveTrashTarget(client)
				},
				SanitizeLabelForCache: sanitizeLabelForCache,
			})
		},
		ArchiveAll: func(cfg *cfg_g.Config, client MailClient, sourcePrefix, targetFolder string) error {
			return organize.All(cfg, client, sourcePrefix, targetFolder)
		},
		ArchiveByID: func(cfg *cfg_g.Config, client MailClient, targetFolder, id string) error {
			return organize.ArchiveByID(cfg, client, targetFolder, id)
		},
		ResolveArchiveTarget: func(client MailClient) (string, error) {
			return organize.ResolveArchiveTarget(client)
		},
		ResolveTrashTarget: func(client MailClient) (string, error) {
			return organize.ResolveTrashTarget(client)
		},
		RunLast: func(cfg *cfg_g.Config, n int) error {
			return runLastOnAccounts(cfg, n)
		},
	}

	cliApp := cli.InitCLI(session)
	if err := cliApp.Execute(preprocessedArgs[1:]); err != nil {
		clihelp.PrintError(err)
		os.Exit(1)
	}
}

func sanitizeLabelForCache(label string) string {
	return cfg_g.SanitizeLabelForCache(label)
}

func runPreFlightCheck(config *Config) error {
	return app.RunPreFlightCheck()
}
