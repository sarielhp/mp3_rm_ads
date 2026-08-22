package usage

import (
	"bytes"
	"strings"
	"testing"

	"github.com/acarl005/stripansi"
	"github.com/sarielhp/clihelp"
)

func TestUsage_GetApp(t *testing.T) {
	app := GetApp()
	if app == nil {
		t.Fatal("expected non-nil clihelp.App")
	}
	if app.Name != "mail_cli" {
		t.Errorf("expected app name 'mail_cli', got %q", app.Name)
	}
	if len(app.Commands) < 15 {
		t.Errorf("expected at least 15 commands, got %d", len(app.Commands))
	}
}

func TestUsage_RenderGlobal(t *testing.T) {
	var buf bytes.Buffer
	Render(clihelp.Options{Writer: &buf, Width: 80})
	output := buf.String()

	if !strings.Contains(output, "Usage of mail_cli:") {
		t.Errorf("expected global usage header, got: %s", output)
	}
	if !strings.Contains(output, "Commands:") {
		t.Errorf("expected Commands section, got: %s", output)
	}
	if !strings.Contains(output, "scan") || !strings.Contains(output, "spam") || !strings.Contains(output, "tui") {
		t.Errorf("expected key commands listed, got: %s", output)
	}
	if !strings.Contains(output, "github.com/sarielhp/gmail_cli") {
		t.Errorf("expected GitHub link in global usage, got: %s", output)
	}
}

func TestUsage_RenderCommand(t *testing.T) {
	tests := []struct {
		path        []string
		mustContain []string
	}{
		{
			path:        []string{"scan"},
			mustContain: []string{"Detailed Usage: scan", "Description:", "<lbl_prefix>", "-m, --move"},
		},
		{
			path:        []string{"rule", "add"},
			mustContain: []string{"rule add", "<email>", "<lbl>"},
		},
		{
			path:        []string{"cache", "prune"},
			mustContain: []string{"cache prune", "--wipe"},
		},
		{
			path:        []string{"labels", "rename"},
			mustContain: []string{"labels rename", "<old_name>", "<new_name>"},
		},
		{
			path:        []string{"spam"},
			mustContain: []string{"Detailed Usage: spam", "del", "learn"},
		},
		{
			path:        []string{"tui"},
			mustContain: []string{"tui", "terminal email browser"},
		},
	}

	for _, tt := range tests {
		var buf bytes.Buffer
		ok := Render(clihelp.Options{Writer: &buf, Width: 80}, tt.path...)
		if !ok {
			t.Errorf("expected Render to succeed for path: %v", tt.path)
		}
		output := buf.String()
		for _, exp := range tt.mustContain {
			if !strings.Contains(output, exp) {
				t.Errorf("path %v expected output to contain %q, got:\n%s", tt.path, exp, output)
			}
		}
	}
}

func TestUsage_RenderMarkdown(t *testing.T) {
	t.Setenv("CLIHELP_GEN", "1")
	tempDir := t.TempDir()
	changed, err := RenderMarkdown(clihelp.MarkdownOptions{Dir: tempDir})
	if err != nil {
		t.Fatalf("unexpected error rendering markdown: %v", err)
	}
	if !changed {
		t.Errorf("expected markdown to be generated on first run")
	}

	// Second run without CLIHELP_GEN should report no changes (hash caching)
	t.Setenv("CLIHELP_GEN", "")
	changed2, err2 := RenderMarkdown(clihelp.MarkdownOptions{Dir: tempDir})
	if err2 != nil {
		t.Fatalf("unexpected error on second markdown render: %v", err2)
	}
	if changed2 {
		t.Errorf("expected no changes on second run due to hash caching")
	}
}

func TestUsage_ReflowWidths(t *testing.T) {
	widths := []int{60, 100}
	paths := [][]string{{"scan"}, {"spam"}, {"rule", "add"}}

	for _, width := range widths {
		for _, path := range paths {
			var buf bytes.Buffer
			opts := clihelp.Options{Writer: &buf, Width: width}
			ok := Render(opts, path...)
			if !ok {
				t.Errorf("Render failed for width=%d path=%v", width, path)
				continue
			}
			output := buf.String()
			lines := strings.Split(output, "\n")
			for i, line := range lines {
				visible := stripansi.Strip(line)
				if len(visible) > width {
					t.Errorf("width=%d path=%v line %d exceeds %d cols: %d visible chars: %q",
						width, path, i+1, width, len(visible), visible)
				}
			}
		}
	}
}

func TestUsage_GlobalRendersAtWidths(t *testing.T) {
	widths := []int{60, 100}
	for _, width := range widths {
		var buf bytes.Buffer
		ok := Render(clihelp.Options{Writer: &buf, Width: width})
		if !ok {
			t.Errorf("Global Render failed for width=%d", width)
		}
		output := buf.String()
		if !strings.Contains(output, "Usage of mail_cli:") {
			t.Errorf("width=%d: expected global usage header", width)
		}
		if !strings.Contains(output, "Commands:") {
			t.Errorf("width=%d: expected Commands section", width)
		}
	}
}
