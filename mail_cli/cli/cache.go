package cli

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"mail_cli/app"
	"mail_cli/cfg_g"

	"github.com/sarielhp/clihelp"
)

var flagPruneWipe bool

// CacheCmd returns the clihelp.Command for the cache command.
func CacheCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "cache",
		Description: "Manage the local email download cache.",
		UsageLine:   "mail_cli cache <subcommand> [args...]",
		Subcommands: []clihelp.Command{
			{
				Name:        "prune",
				Title:       "cache prune [days] [--wipe]",
				Description: "Prune cached emails and spam scores older than the specified number of days (default: 30). With --wipe, purges all cached messages regardless of age.",
				UsageLine:   "mail_cli cache prune [days] [-w, --wipe]",
				Parameters: []clihelp.Param{
					{Name: "[days]", Description: "Age threshold in days for pruning cached emails (default: 30)."},
				},
				Options: []clihelp.Option{
					clihelp.Bool(&flagPruneWipe, "-w, --wipe", false, "Purge all cached email files regardless of age."),
				},
				Examples: []clihelp.Example{
					{Line: "mail_cli cache prune"},
					{Line: "mail_cli cache prune 7"},
					{Line: "mail_cli cache prune --wipe"},
				},
				Args: clihelp.MaximumNArgs(1),
				Run: func(ctx *clihelp.Context) error {
					args := ctx.Args
					days := 30
					if len(args) > 0 {
						var err error
						days, err = strconv.Atoi(args[0])
						if err != nil {
							return fmt.Errorf("invalid number of days: %s", args[0])
						}
					}

					wipe := flagPruneWipe
					if wipe || days <= 0 {
						days = 0
					}

					cutoff := time.Now().AddDate(0, 0, -days)
					cacheDir := session.Config.DownloadDir

					var folderSubdir string
					if session.GetClient != nil {
						client, err := session.GetClient(session.Config)
						if err == nil {
							dir := cfg_g.SanitizeLabelForCache(client.Config().SelectedAccount)
							if dir != "" {
								folderSubdir = dir
							}
						}
					}

					pruneDir := cacheDir
					if folderSubdir != "" {
						pruneDir = filepath.Join(cacheDir, folderSubdir)
					}

					if _, err := os.Stat(pruneDir); os.IsNotExist(err) {
						fmt.Printf("Cache directory does not exist: %s\n", pruneDir)
						return nil
					}

					var totalDeleted int
					err := filepath.WalkDir(pruneDir, func(path string, d fs.DirEntry, err error) error {
						if err != nil {
							return err
						}
						if d.IsDir() && d.Name() == ".scores" {
							scoresPath := path
							entries, err := os.ReadDir(scoresPath)
							if err != nil {
								return err
							}
							for _, e := range entries {
								if !e.IsDir() {
									info, err := e.Info()
									if err != nil {
										continue
									}
									if info.ModTime().Before(cutoff) {
										fullPath := filepath.Join(scoresPath, e.Name())
										if err := os.Remove(fullPath); err == nil {
											totalDeleted++
											slog.Info("Deleted score file", slog.String("path", fullPath))
										}
									}
								}
							}
							return filepath.SkipDir
						}
						if !d.IsDir() && strings.HasSuffix(d.Name(), ".eml") {
							info, err := d.Info()
							if err != nil {
								return err
							}
							if info.ModTime().Before(cutoff) {
								fullPath := path
								if err := os.Remove(fullPath); err == nil {
									totalDeleted++
									slog.Info("Deleted cached email", slog.String("path", fullPath))
								}
							}
						}
						return nil
					})
					if err != nil {
						return fmt.Errorf("error pruning cache: %w", err)
					}

					fmt.Printf("Pruned %d cached file(s) older than %d day(s) from %s.\n", totalDeleted, days, pruneDir)
					return nil
				},
			},
			{
				Name:        "reset",
				Description: "Recreate the cache directory for the current account, wiping all cached data.",
				UsageLine:   "mail_cli cache reset",
				Args:        clihelp.NoArgs,
				Run: func(ctx *clihelp.Context) error {
					cacheDir := session.Config.DownloadDir

					var folderSubdir string
					if session.GetClient != nil {
						client, err := session.GetClient(session.Config)
						if err == nil {
							dir := cfg_g.SanitizeLabelForCache(client.Config().SelectedAccount)
							if dir != "" {
								folderSubdir = dir
							}
						}
					}

					resetDir := cacheDir
					if folderSubdir != "" {
						resetDir = filepath.Join(cacheDir, folderSubdir)
					}

					if err := os.RemoveAll(resetDir); err != nil {
						return fmt.Errorf("failed to remove cache directory %s: %w", resetDir, err)
					}
					if err := os.MkdirAll(resetDir, 0755); err != nil {
						return fmt.Errorf("failed to recreate cache directory %s: %w", resetDir, err)
					}
					fmt.Printf("%s Cache directory reset successfully: %s\n", app.PrefixSuccess, resetDir)
					return nil
				},
				Examples: []clihelp.Example{
					{Line: "mail_cli cache reset"},
				},
			},
		},
		Examples: []clihelp.Example{
			{Line: "mail_cli cache prune"},
			{Line: "mail_cli cache prune 7"},
			{Line: "mail_cli cache prune --wipe"},
			{Line: "mail_cli cache reset"},
		},
	}
}
