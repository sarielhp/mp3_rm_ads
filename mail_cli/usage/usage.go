package usage

import (
	"os"
	"path/filepath"

	"mail_cli/app"
	"mail_cli/cfg_g"
	"mail_cli/cli"

	"github.com/sarielhp/clihelp"
)

// GetApp returns the global clihelp.App instance for mail_cli.
func GetApp() *clihelp.App {
	return cli.InitCLI(&app.Session{Config: &cfg_g.Config{}})
}

// PrintUsage prints the global CLI help overview to os.Stderr.
func PrintUsage() {
	homeDir, _ := os.UserHomeDir()
	configPath := filepath.Join(homeDir, ".config", app.AppName, "config.json")
	cliApp := GetApp()
	cliApp.ConfigPath = configPath
	cliApp.Version = app.Version
	cliApp.RenderGlobal(clihelp.Options{Writer: os.Stderr})
}

// PrintDetailedUsage prints detailed usage for a command or subcommand to os.Stdout.
func PrintDetailedUsage(cmd, subcmd string) {
	var path []string
	if cmd != "" {
		path = append(path, cmd)
	}
	if subcmd != "" {
		path = append(path, subcmd)
	}
	cliApp := GetApp()
	cliApp.RenderCommand(clihelp.Options{Writer: os.Stdout}, path...)
}

// Render writes help text for the specified path to the given options destination.
func Render(opts clihelp.Options, path ...string) bool {
	cliApp := GetApp()
	return cliApp.Render(opts, path...)
}

// RenderMarkdown generates markdown documentation in the given directory.
func RenderMarkdown(opts clihelp.MarkdownOptions) (bool, error) {
	cliApp := GetApp()
	return clihelp.RenderMarkdown(cliApp, opts)
}
