package cfg_g

import (
	"testing"
)

func TestParseAccountLabelSpec(t *testing.T) {
	tests := []struct {
		input     string
		wantName  string
		wantLabel string
		wantErr   bool
	}{
		{"1:sort/mailing_list/wuna", "", "", true},
		{"2:wuna", "", "", true},
		{"10:keep/2026/08/wuna", "", "", true},
		{"wuna", "", "wuna", false},
		{"keep/2026/08/wuna", "", "keep/2026/08/wuna", false},
		{"0:wuna", "", "0:wuna", false},
		{"-1:wuna", "", "-1:wuna", false},
		{"1:", "", "", true},
		{"", "", "", false},
		{"GMail:inbox", "GMail", "inbox", false},
		{"FastMail:sort/news", "FastMail", "sort/news", false},
		{"Work-GMail:Spam", "Work-GMail", "Spam", false},
	}

	for _, tt := range tests {
		spec, err := ParseAccountLabelSpec(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseAccountLabelSpec(%q) expected error but got %+v", tt.input, spec)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseAccountLabelSpec(%q) unexpected error: %v", tt.input, err)
			continue
		}
		if spec.AccountName != tt.wantName || spec.Label != tt.wantLabel {
			t.Errorf("ParseAccountLabelSpec(%q) = {Name: %q, Label: %q}, want {Name: %q, Label: %q}",
				tt.input, spec.AccountName, spec.Label, tt.wantName, tt.wantLabel)
		}
	}
}
