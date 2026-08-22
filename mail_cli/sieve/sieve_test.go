package sieve

import (
	"mail_cli/cfg_acc"
	"strings"
	"testing"
)

func TestSanitizeForSieve(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no escaping needed", "test@example.com", "test@example.com"},
		{"backslash escaped", `test\"quote`, `test\\\"quote`},
		{"double quote escaped", `test"quote`, `test\"quote`},
		{"both escaped", `test\"both`, `test\\\"both`},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeForSieve(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeForSieve(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGenerateSieveScript(t *testing.T) {
	rules := []cfg_acc.Rule{
		{Sender: "spam@example.com", Label: "Spam"},
		{Sender: "news@example.com", Label: "News"},
		{Subject: "Alert", Label: "Alerts"},
	}
	script := generateSieveScript(rules)

	if !strings.Contains(script, "require [\"fileinto\"];") {
		t.Error("generateSieveScript should include fileinto require")
	}
	if !strings.Contains(script, "spam@example.com") {
		t.Error("generateSieveScript should include sender filter")
	}
	if !strings.Contains(script, "News") {
		t.Error("generateSieveScript should include News label")
	}
	if !strings.Contains(script, "header :matches \"Subject\" \"Alert*\"") {
		t.Error("generateSieveScript should include subject matches filter")
	}
	if !strings.Contains(script, "stop") {
		t.Error("generateSieveScript should include stop directive")
	}
}

func TestGenerateSieveScriptEmpty(t *testing.T) {
	script := generateSieveScript([]cfg_acc.Rule{})
	if !strings.Contains(script, "require") {
		t.Error("generateSieveScript should always include require even with empty rules")
	}
}
