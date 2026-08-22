package sieve

import (
	"fmt"
	"os"
	"strings"

	"mail_cli/app"
	"mail_cli/cfg_acc"
	"mail_cli/cfg_g"
)

func ExportScript(config *cfg_g.Config, outputPath string) error {
	_, targetAcc, _, _, err := cfg_g.ResolveAccountFromConfig(config)
	if err != nil {
		return err
	}

	rules := targetAcc.Rules
	if len(rules) == 0 {
		return fmt.Errorf("no rules defined for account %s", targetAcc.Name)
	}

	sieveScript := generateSieveScript(rules)

	err = os.WriteFile(outputPath, []byte(sieveScript), 0600)
	if err != nil {
		return fmt.Errorf("failed to write Sieve script: %w", err)
	}

	fmt.Printf("%s Generated Sieve script for account %s with %d rule(s)\n", app.PrefixSuccess, targetAcc.Name, len(rules))
	fmt.Printf("%s Sieve script written to: %s\n", app.PrefixInfo, outputPath)
	fmt.Printf("%s Note: FastMail does not support JMAP server-side filters.\n", app.PrefixWarn)
	fmt.Printf("%s Upload this script to FastMail at: https://www.fastmail.com/help/objects/filters\n", app.PrefixWarn)
	fmt.Printf("%s (Settings > Filters > Manage Sieve Scripts)\n\n", app.PrefixWarn)

	fmt.Println("=== Sieve script content ===")
	fmt.Print(sieveScript)

	return nil
}

func generateSieveScript(rules []cfg_acc.Rule) string {
	var sb strings.Builder

	sb.WriteString("# mail_cli generated Sieve script\n")
	sb.WriteString("# Rules exported from local configuration\n")
	sb.WriteString("# Upload this to FastMail via: Settings > Filters > Manage Sieve Scripts\n")
	sb.WriteString("#\n")
	sb.WriteString("# See: https://www.fastmail.com/help/objects/filters.html\n\n")

	sb.WriteString("require [\"fileinto\"];\n\n")

	for i, rule := range rules {
		if rule.Sender == "" && rule.Subject == "" {
			continue
		}

		label := sanitizeForSieve(rule.Label)

		if rule.Subject != "" {
			subjPattern := sanitizeForSieve(rule.Subject) + "*"
			sb.WriteString(fmt.Sprintf("# Rule %d: subject:%s -> %s\n", i+1, rule.Subject, rule.Label))
			sb.WriteString(fmt.Sprintf("if header :matches \"Subject\" \"%s\"\n", subjPattern))
		} else {
			sender := sanitizeForSieve(rule.Sender)
			sb.WriteString(fmt.Sprintf("# Rule %d: %s -> %s\n", i+1, rule.Sender, rule.Label))
			if strings.HasPrefix(sender, "@") || strings.HasPrefix(sender, "*@") {
				pattern := "*" + strings.TrimPrefix(sender, "*")
				sb.WriteString(fmt.Sprintf("if address :matches \"from\" \"%s\"\n", pattern))
			} else {
				sb.WriteString(fmt.Sprintf("if address :is \"from\" \"%s\"\n", sender))
			}
		}
		sb.WriteString("{\n")
		sb.WriteString(fmt.Sprintf("    fileinto \"%s\";\n", label))
		sb.WriteString("    stop;\n")
		sb.WriteString("}\n\n")
	}

	return sb.String()
}

func sanitizeForSieve(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}
