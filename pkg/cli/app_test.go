package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"pod/pkg/config"
	"pod/pkg/util"

	"github.com/sarielhp/clihelp"
)

func TestFindMP3Files(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	os.WriteFile(d+"/a.mp3", []byte("x"), 0644)
	os.WriteFile(d+"/a.txt", []byte("x"), 0644)
	os.MkdirAll(d+"/sub", 0755)
	os.WriteFile(d+"/sub/b.mp3", []byte("x"), 0644)
	files := util.FindMP3Files(d)
	if len(files) != 2 {
		t.Errorf("got %d files, want 2", len(files))
	}
}

func TestSafeMove(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	src, dst := d+"/s.txt", d+"/d.txt"
	os.WriteFile(src, []byte("x"), 0644)
	util.SafeMove(src, dst)
	if util.FileExists(src) || !util.FileExists(dst) {
		t.Error("safeMove failed")
	}
}

func TestCopyFile(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	src, dst := d+"/s.txt", d+"/d.txt"
	os.WriteFile(src, []byte("x"), 0644)
	util.CopyFileErr(src, dst)
	if !util.FileExists(dst) {
		t.Error("copyFile failed")
	}
}

func TestSelectProfile(t *testing.T) {
	t.Parallel()
	cfg := Config{ActiveProfileID: 2, Profiles: []LLMProfile{{ID: 1, Name: "One", Model: "m1"}, {ID: 2, Name: "Two", Model: "m2"}}}
	if p, _ := config.SelectLLMProfile(&cfg, ""); p.ID != 2 {
		t.Error("default profile")
	}
	if p, _ := config.SelectLLMProfile(&cfg, "1"); p.ID != 1 {
		t.Error("by id")
	}
	if p, _ := config.SelectLLMProfile(&cfg, "Two"); p.ID != 2 {
		t.Error("by name")
	}
}

func TestTopLevelCommandsUniqueFirstLetters(t *testing.T) {
	var action string
	var opts CLIOptions
	app := buildCLIApp(&action, &opts)

	seen := make(map[byte]string)
	for _, cmd := range app.Commands {
		if cmd.Name == "" {
			continue
		}
		first := cmd.Name[0]
		if existing, ok := seen[first]; ok {
			t.Errorf("command %q has conflicting first letter '%c' with command %q", cmd.Name, first, existing)
		}
		seen[first] = cmd.Name
	}
	if len(app.Commands) != 7 {
		t.Errorf("expected 7 canonical top-level commands, got %d", len(app.Commands))
	}
}

func checkUsageOutputNoRepeatedSections(t *testing.T, path []string, out string) {
	sections := []string{"Flags:", "Global Flags:", "Subcommands:", "Parameters:"}
	lines := strings.Split(out, "\n")
	for _, sec := range sections {
		cnt := 0
		for _, line := range lines {
			if strings.TrimSpace(line) == sec {
				cnt++
			}
		}
		if cnt > 1 {
			t.Errorf("command %v has repeated section %q (%d occurrences):\n%s", path, sec, cnt, out)
		}
	}
	flagLines := make(map[string]bool)
	inFlags := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "Flags:" || trimmed == "Global Flags:" {
			inFlags = true
			continue
		}
		if inFlags && (trimmed == "" || strings.HasSuffix(trimmed, ":")) {
			inFlags = false
		}
		if inFlags && strings.HasPrefix(trimmed, "-") {
			flagPart := strings.Fields(trimmed)[0]
			if flagLines[flagPart] {
				t.Errorf("command %v has repeated flag %q:\n%s", path, flagPart, out)
			}
			flagLines[flagPart] = true
		}
	}
}

func TestUsageHelpNoRepeatedText(t *testing.T) {
	var buf bytes.Buffer
	var action string
	var opts CLIOptions
	app := buildCLIApp(&action, &opts)

	for _, cmd := range app.Commands {
		buf.Reset()
		renderOpts := clihelp.Options{Writer: &buf, Width: 80}
		if !app.RenderCommand(renderOpts, cmd.Name) {
			t.Errorf("failed to render help for %s", cmd.Name)
			continue
		}
		checkUsageOutputNoRepeatedSections(t, []string{cmd.Name}, buf.String())

		for _, sub := range cmd.Subcommands {
			buf.Reset()
			if !app.RenderCommand(renderOpts, cmd.Name, sub.Name) {
				t.Errorf("failed to render help for %s %s", cmd.Name, sub.Name)
				continue
			}
			checkUsageOutputNoRepeatedSections(t, []string{cmd.Name, sub.Name}, buf.String())
		}
	}
}
