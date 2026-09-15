package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"pod/pkg/podcast"
	"reflect"
	"strings"
	"testing"
)

func TestResolveTranscriptAfterAudioRemoved(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir, paths := createTestPodcastWithEpisodes(t, root, "Show", []string{"episode"})
	id := podcast.EpisodeShortIDReadOnly(dir, podcast.GeneratePodcastShortID("Show"), paths[0])
	for _, suffix := range []string{".transcript.json", ".transcript.txt"} {
		if err := os.WriteFile(strings.TrimSuffix(paths[0], ".mp3")+suffix, []byte("retained transcript"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, archived := range []bool{false, true} {
		if archived {
			if err := os.Remove(paths[0]); err != nil {
				t.Fatal(err)
			}
		}
		before := queueTree(t, root)
		ep, err := resolveTranscriptEpisode(root, id)
		if err != nil || ep.Path != paths[0] {
			t.Fatalf("archived=%v: episode=%+v, err=%v", archived, ep, err)
		}
		if !reflect.DeepEqual(before, queueTree(t, root)) {
			t.Fatal("transcript lookup modified files")
		}
	}
}

func TestTextPagerStreamsCompleteTranscript(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pager")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf '%s\\n' \"$1\"\n/bin/cat\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PAGER", path+" option")
	text := strings.Repeat("Full transcript text.\n", 10000)
	var out, stderr bytes.Buffer
	if err := runTextPager(text, &out, &stderr); err != nil {
		t.Fatal(err)
	}
	if out.String() != "option\n"+text {
		t.Fatal("pager did not receive arguments and complete transcript")
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 7\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := runTextPager("text", &out, &stderr); err == nil {
		t.Fatal("pager failure was swallowed")
	}
}
