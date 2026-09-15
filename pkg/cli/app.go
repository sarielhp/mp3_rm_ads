package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/sarielhp/clihelp"
)

var embeddedVersion string

func SetEmbeddedVersion(v string) {
	embeddedVersion = v
}

func getVersion() string {
	if embeddedVersion != "" {
		return embeddedVersion
	}
	if data, err := os.ReadFile("VERSION"); err == nil {
		return strings.TrimSpace(string(data))
	}
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		if data, err := os.ReadFile(filepath.Join(execDir, "VERSION")); err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return "dev"
}

func buildCLIApp(action *string, opts *CLIOptions) *clihelp.App {
	keepVal := -1
	countVal := -1

	return &clihelp.App{
		Name:                "pod",
		Description:         "Automatic Ad Segment Remover & Podcast Manager",
		UsageLine:           "pod [OPTIONS] <COMMAND>",
		Version:             getVersion(),
		GlobalNote:          "Run 'pod <command> --help' or 'abs help <command>' for command-specific options.",
		AbbrevCommands:      true,
		Pager:               true,
		InteractiveFallback: true,
		PersistentOptions: []clihelp.Option{
			clihelp.Bool(&opts.ShowExamples, "-E, --examples", false, "Show command examples"),
		},
		Examples: []clihelp.Example{
			{
				Line:        "pod server feeds",
				Description: "Check podcast feeds directly for newly published episodes",
			},
			{
				Line:        "pod server download",
				Description: "Download new episodes from server",
			},
			{
				Line:        "pod server opml export podcasts.opml",
				Description: "Export server podcast RSS feeds into an OPML file",
			},
			{
				Line:        "pod queue latest 10",
				Description: "Queue the 10 latest uncleaned episodes for ad removal",
			},
			{
				Line:        "pod queue run",
				Description: "Process ad removal on queued episodes",
			},
			{
				Line:        "pod tui",
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
