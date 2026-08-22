package cli

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"mail_cli/app"

	"github.com/sarielhp/clihelp"
)

// MigrateCmd returns the clihelp.Command for the migrate command.
func MigrateCmd(session *app.Session) clihelp.Command {
	return clihelp.Command{
		Name:        "migrate",
		Description: "Copy configuration and credentials to a remote machine via SSH/SCP.",
		UsageLine:   "mail_cli migrate [user@]host",
		Parameters: []clihelp.Param{
			{Name: "[user@]host", Description: "Remote destination machine address (e.g. server.local or user@server.local)."},
		},
		Examples: []clihelp.Example{
			{Line: "mail_cli migrate myserver.local"},
			{Line: "mail_cli migrate user@myserver.local"},
		},
		Args: clihelp.ExactArgs(1),
		Run: func(ctx *clihelp.Context) error {
			return migrateRun(session, ctx.Args[0])
		},
	}
}

func migrateRun(session *app.Session, target string) error {
	if os.Geteuid() == 0 {
		return fmt.Errorf("migrate: refusing to run as root")
	}

	userPart, host := parseTarget(target)
	if userPart == "" {
		current, err := user.Current()
		if err != nil {
			return fmt.Errorf("migrate: cannot determine current user: %w", err)
		}
		if current.Username == "root" {
			return fmt.Errorf("migrate: cannot use root as default user; specify user@host instead")
		}
		userPart = current.Username
	}
	sshTarget := fmt.Sprintf("%s@%s", userPart, host)

	if _, err := exec.LookPath("ssh"); err != nil {
		return fmt.Errorf("migrate: ssh not found in PATH")
	}
	if _, err := exec.LookPath("scp"); err != nil {
		return fmt.Errorf("migrate: scp not found in PATH")
	}

	configDir := session.Config.ConfigDir
	var files []string

	always := []string{"config.json"}
	for _, name := range always {
		path := filepath.Join(configDir, name)
		if _, err := os.Stat(path); err == nil {
			files = append(files, path)
		}
	}

	patterns := []string{"credentials.json", "token.json", "token_*.json"}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(configDir, pattern))
		if err != nil {
			return fmt.Errorf("migrate: failed to glob %s: %w", pattern, err)
		}
		files = append(files, matches...)
	}

	tokensDir := filepath.Join(configDir, "tokens")
	if entries, err := os.ReadDir(tokensDir); err == nil {
		for _, entry := range entries {
			files = append(files, filepath.Join(tokensDir, entry.Name()))
		}
	}

	if len(files) == 0 {
		return fmt.Errorf("migrate: no files found to copy in %s", configDir)
	}

	sshCmd := exec.Command("ssh", sshTarget, "mkdir", "-p", filepath.Join(".config", app.AppName))
	sshCmd.Stderr = os.Stderr
	if err := sshCmd.Run(); err != nil {
		return fmt.Errorf("migrate: failed to create remote config directory: %w", err)
	}

	for _, src := range files {
		rel, _ := filepath.Rel(configDir, src)
		dst := fmt.Sprintf("%s:%s/%s", sshTarget, filepath.Join(".config", app.AppName), rel)

		scpCmd := exec.Command("scp", src, dst)
		scpCmd.Stderr = os.Stderr
		if err := scpCmd.Run(); err != nil {
			return fmt.Errorf("migrate: failed to copy %s: %w", rel, err)
		}
		fmt.Printf("  ✓ %s\n", rel)
	}

	fmt.Println()
	fmt.Println("Migration complete. Run the following on the remote machine to verify:")
	fmt.Printf("  %s test\n", app.AppName)

	return nil
}

func parseTarget(raw string) (string, string) {
	if strings.Contains(raw, "@") {
		parts := strings.SplitN(raw, "@", 2)
		return parts[0], parts[1]
	}
	return "", raw
}
