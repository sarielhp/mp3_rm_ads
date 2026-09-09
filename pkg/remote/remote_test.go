package remote

import (
	"path/filepath"
	"testing"

	"github.com/sariel/abs/pkg/types"
)

type mockTransport struct {
	execs     []string
	execOut   map[string]string
	uploads   map[string]string
	downloads map[string]string
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		execOut:   make(map[string]string),
		uploads:   make(map[string]string),
		downloads: make(map[string]string),
	}
}

func (m *mockTransport) Exec(host, cmd string) (string, error) {
	m.execs = append(m.execs, cmd)
	if out, ok := m.execOut[cmd]; ok {
		return out, nil
	}
	return "", nil
}

func (m *mockTransport) Upload(host, localSrc, remoteDst string) error {
	m.uploads[remoteDst] = localSrc
	return nil
}

func (m *mockTransport) Download(host, remoteSrc, localDst string) error {
	m.downloads[localDst] = remoteSrc
	return nil
}

func (m *mockTransport) RsyncTo(host, localSrc, remoteDst string) error {
	return m.Upload(host, localSrc, remoteDst)
}

func (m *mockTransport) RsyncFrom(host, remoteSrc, localDst string) error {
	return m.Download(host, remoteSrc, localDst)
}

func TestValidateBatchID(t *testing.T) {
	valid := []string{"batch-123", "batch_2026_abc", "simple"}
	invalid := []string{"", "../escape", "foo/bar", "with space", "bad\\slash"}

	for _, s := range valid {
		if !ValidateBatchID(s) {
			t.Errorf("expected %q to be valid batch ID", s)
		}
	}
	for _, s := range invalid {
		if ValidateBatchID(s) {
			t.Errorf("expected %q to be invalid batch ID", s)
		}
	}
}

func TestShellQuote(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"hello", "'hello'"},
		{"it's", "'it'\\''s'"},
		{"", "''"},
	}
	for _, tt := range tests {
		got := ShellQuote(tt.in)
		if got != tt.want {
			t.Errorf("ShellQuote(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestShellQuoteHomePath(t *testing.T) {
	if got := ShellQuoteHomePath("~"); got != "$HOME" {
		t.Errorf("ShellQuoteHomePath(~) = %q, want $HOME", got)
	}
	if got := ShellQuoteHomePath("~/abs_remote"); got != "$HOME/'abs_remote'" {
		t.Errorf("ShellQuoteHomePath(~/abs_remote) = %q, want $HOME/'abs_remote'", got)
	}
	if got := ShellQuoteHomePath("/tmp/foo"); got != "'/tmp/foo'" {
		t.Errorf("ShellQuoteHomePath(/tmp/foo) = %q, want '/tmp/foo'", got)
	}
}

func TestManifestOperations(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "manifest.json")

	manifest := &types.RemoteBatchManifest{
		BatchID:    "batch-001",
		TotalItems: 2,
		Items: []types.RemoteBatchJobItem{
			{ID: "item-1", AudioFileName: "audio1.mp3", Status: types.BatchStatusQueued},
			{ID: "item-2", AudioFileName: "audio2.mp3", Status: types.BatchStatusQueued},
		},
	}

	if err := SaveManifest(manifestPath, manifest); err != nil {
		t.Fatalf("SaveManifest failed: %v", err)
	}

	loaded, err := LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}
	if loaded.BatchID != "batch-001" || len(loaded.Items) != 2 {
		t.Errorf("unexpected loaded manifest: %+v", loaded)
	}

	UpdateManifestItem(loaded, "item-1", types.BatchStatusCompleted, "")
	UpdateManifestItem(loaded, "item-2", types.BatchStatusCompleted, "")

	if loaded.Status != types.BatchStatusCompleted || loaded.CompletedItems != 2 {
		t.Errorf("expected all items completed: %+v", loaded)
	}
}

func TestDoneManifest(t *testing.T) {
	tmpDir := t.TempDir()
	donePath := filepath.Join(tmpDir, "done.json")

	item := RemoteDoneItem{
		RelPath:             "Podcast/Ep1.mp3",
		Status:              types.StateReadyForCopyBack,
		CleanedDurationSec:  100,
		CutDurationSec:      20,
		OriginalDurationSec: 120,
	}

	if err := AddDoneEpisode(donePath, item); err != nil {
		t.Fatalf("AddDoneEpisode failed: %v", err)
	}

	m, err := LoadDoneManifest(donePath)
	if err != nil {
		t.Fatalf("LoadDoneManifest failed: %v", err)
	}
	if len(m.Episodes) != 1 {
		t.Fatalf("expected 1 episode in done manifest, got %d", len(m.Episodes))
	}

	if err := RemoveDoneEpisode(donePath, "Podcast/Ep1.mp3"); err != nil {
		t.Fatalf("RemoveDoneEpisode failed: %v", err)
	}
	m, _ = LoadDoneManifest(donePath)
	if len(m.Episodes) != 0 {
		t.Errorf("expected 0 episodes in done manifest after removal")
	}
}

func TestSafeRelUnder(t *testing.T) {
	baseDir := "/home/user/podcasts"
	tests := []struct {
		rel  string
		want bool
	}{
		{"Show/Ep1.mp3", true},
		{"../escape.mp3", false},
		{"/abs/path.mp3", false},
		{"", false},
	}
	for _, tt := range tests {
		_, ok := SafeRelUnder(baseDir, tt.rel)
		if ok != tt.want {
			t.Errorf("SafeRelUnder(%q, %q) ok = %v, want %v", baseDir, tt.rel, ok, tt.want)
		}
	}
}

func TestMockTransport(t *testing.T) {
	mock := newMockTransport()
	mock.execOut["echo 1"] = "1"

	if !IsRemoteHostReachable("remote-host", mock) {
		t.Errorf("expected remote host to be reachable with mock")
	}
}
