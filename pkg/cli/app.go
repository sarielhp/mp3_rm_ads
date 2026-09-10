package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/sarielhp/clihelp"
)

func getVersion() string {
	if data, err := os.ReadFile("VERSION"); err == nil {
		return strings.TrimSpace(string(data))
	}
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		if data, err := os.ReadFile(filepath.Join(execDir, "VERSION")); err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return "0.2.11"
}

func buildCLIApp(action *string, opts *CLIOptions) *clihelp.App {
	keepVal := -1
	countVal := -1

	return &clihelp.App{
		Name:                "abs",
		Description:         "Automatic Ad Segment Remover & Podcast Manager",
		UsageLine:           "abs [OPTIONS] <COMMAND>",
		Version:             getVersion(),
		GlobalNote:          "Run 'abs <command> --help' or 'abs help <command>' for command-specific options.",
		AbbrevCommands:      true,
		Pager:               true,
		InteractiveFallback: true,
		PersistentOptions: []clihelp.Option{
			clihelp.Bool(&opts.ShowExamples, "-E, --examples", false, "Show command examples"),
		},
		Examples: []clihelp.Example{
			{
				Line:        "abs server feeds",
				Description: "Check podcast feeds directly for newly published episodes",
			},
			{
				Line:        "abs server download",
				Description: "Update feeds and download new episodes from server",
			},
			{
				Line:        "abs server opml export podcasts.opml",
				Description: "Export server podcast RSS feeds into an OPML file",
			},
			{
				Line:        "abs queue run",
				Description: "Process ad removal on queued episodes",
			},
			{
				Line:        "abs tui",
				Description: "Launch interactive terminal UI browser",
			},
		},
		Commands: []clihelp.Command{
			buildConfigCommand(opts, action),
			buildInfoCommand(opts, action),
			buildOffloadCommand(opts, action),
			buildPlayerCommand(opts, action),
			buildQueueCommand(opts, action),
			buildRmAdsCommand(opts, action),
			buildServerCommand(opts, action, &countVal, &keepVal),
			buildTUICommand(opts, action),
		},
	}
}
