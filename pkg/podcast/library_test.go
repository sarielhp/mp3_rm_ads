package podcast

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"pod/pkg/progress"
)

func TestOpenProvidesOwnedState(t *testing.T) {
	lib := Open(Config{PodcastsDir: "/lib"}, nil, nil)

	if lib.FeedCache() == nil {
		t.Error("Library has no feed cache")
	}
	if lib.Queue() == nil {
		t.Error("Library has no download queue")
	}
	if lib.Progress() == nil {
		t.Error("a nil Reporter should become Discard, not nil")
	}
	if lib.Config().PodcastsDir != "/lib" {
		t.Errorf("config not carried: %+v", lib.Config())
	}
}

func TestOpenKeepsReporter(t *testing.T) {
	lines := &progress.Lines{}
	if got := Open(Config{}, nil, lines).Progress(); got != progress.Reporter(lines) {
		t.Error("Open replaced the caller's Reporter")
	}
}

// The feed cache and queue each serialise one file through their own mutex, so
// every Library over the default paths must share them; two mutexes over one
// file would not serialise anything.
func TestLibrariesShareTheDefaultFileBackedState(t *testing.T) {
	a := Open(Config{PodcastsDir: "/a"}, nil, nil)
	b := Open(Config{PodcastsDir: "/b"}, nil, nil)

	if a.Queue() != b.Queue() {
		t.Error("two Libraries hold different download queues over the same file")
	}
	if a.FeedCache() != b.FeedCache() {
		t.Error("two Libraries hold different feed caches over the same file")
	}
}

func TestLibraryResolvesAgainstItsOwnRoot(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "Some_Show")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ep.mp3"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	lib := Open(Config{PodcastsDir: root}, nil, nil)
	pods := lib.Podcasts()
	if len(pods) != 1 || pods[0].FolderName != "Some_Show" {
		t.Fatalf("Podcasts() = %+v", pods)
	}

	res, err := lib.Resolve("Some_Show")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res == nil {
		t.Fatal("Resolve returned nothing")
	}
}

// Config is the boundary this step exists to draw. If a field creeps in, the
// library has started to care about something that is not its business.
func TestConfigStaysThreeFields(t *testing.T) {
	got := reflect.TypeOf(Config{})
	want := []string{"PodcastsDir", "SubscriptionsFile", "ServerBaseURL"}
	if got.NumField() != len(want) {
		t.Fatalf("Config has %d fields, want %d — see review/002.md Step 4", got.NumField(), len(want))
	}
	for i, name := range want {
		if got.Field(i).Name != name {
			t.Errorf("field %d = %q, want %q", i, got.Field(i).Name, name)
		}
	}
}

// The application config carries transcription profiles, LLM keys and the
// terminal UI's colours. The podcast library has no business seeing any of it.
func TestLibraryDoesNotReadTheApplicationConfig(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			code, _, _ := strings.Cut(line, "//")
			if strings.Contains(code, "types.Config") {
				t.Errorf("%s:%d reads the application config; take what you need on podcast.Config instead:\n\t%s",
					name, i+1, strings.TrimSpace(line))
			}
		}
	}
}
