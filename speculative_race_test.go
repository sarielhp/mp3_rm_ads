package main

import (
	"context"
	"errors"
	"testing"
)

func TestContainsHebrew(t *testing.T) {
	cases := []struct {
		input    string
		expected bool
	}{
		{"Hello World", false},
		{"5-4 Podcast episode", false},
		{"חיות כיס", true},
		{"/var/podcasts/קוד פתוח/ep1.mp3", true},
		{"Episode 10: שיחה על כלכלה", true},
		{"", false},
	}
	for _, c := range cases {
		if got := containsHebrew(c.input); got != c.expected {
			t.Errorf("containsHebrew(%q) = %v, expected %v", c.input, got, c.expected)
		}
	}
}

func TestWhisperProfileSupportsLanguage(t *testing.T) {
	wpEn := WhisperProfile{Engine: WhisperEngineLocal, Model: "tiny.en", Languages: []string{"en"}}
	if !whisperProfileSupportsLanguage(wpEn, "en") {
		t.Errorf("expected wpEn to support en")
	}
	if whisperProfileSupportsLanguage(wpEn, "he") {
		t.Errorf("expected wpEn NOT to support he")
	}

	wpMulti := WhisperProfile{Engine: WhisperEngineDocker, Languages: []string{"en", "he"}}
	if !whisperProfileSupportsLanguage(wpMulti, "he") || !whisperProfileSupportsLanguage(wpMulti, "en") {
		t.Errorf("expected wpMulti to support both en and he")
	}
	if whisperProfileSupportsLanguage(wpMulti, "fr") {
		t.Errorf("expected wpMulti NOT to support fr")
	}

	wpWildcard := WhisperProfile{Engine: WhisperEngineGemini, Languages: []string{"en", "he", "*"}}
	if !whisperProfileSupportsLanguage(wpWildcard, "fr") {
		t.Errorf("expected wpWildcard to support fr via *")
	}
}

func TestResolveLocalWhisperProfileHebrewRouting(t *testing.T) {
	cfg := Config{
		ActiveWhisperID: 1,
		WhisperProfiles: []WhisperProfile{
			{
				ID:        1,
				Name:      "Local CLI",
				Engine:    WhisperEngineLocal,
				Model:     "tiny.en",
				Languages: []string{"en"},
			},
			{
				ID:        2,
				Name:      "Docker Daemon",
				Engine:    WhisperEngineDocker,
				URL:       "http://localhost:8088/inference",
				Languages: []string{"en", "he"},
			},
		},
	}

	wpEn := resolveLocalWhisperProfile(cfg, false)
	if wpEn.ID != 1 {
		t.Errorf("expected profile 1 for English, got %d", wpEn.ID)
	}

	wpHe := resolveLocalWhisperProfile(cfg, true)
	if wpHe.ID != 2 {
		t.Errorf("expected profile 2 (Docker) for Hebrew, got %d", wpHe.ID)
	}
}

func TestAwaitRaceResultsGeminiWins(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	geminiCh := make(chan geminiRaceResult, 1)
	localCh := make(chan localRaceResult, 1)

	geminiCh <- geminiRaceResult{
		td:  &TranscriptionData{Text: "Gemini text"},
		ads: []AdSegment{{Start: 10, End: 20, Reason: "Sponsor"}},
	}

	td, ads, geminiWon, err := awaitRaceResults(ctx, cancel, geminiCh, localCh, true)
	if err != nil || !geminiWon {
		t.Fatalf("expected Gemini to win: err=%v, geminiWon=%v", err, geminiWon)
	}
	if td.Text != "Gemini text" || len(ads) != 1 {
		t.Errorf("unexpected race payload: td=%+v, ads=%+v", td, ads)
	}
}

func TestAwaitRaceResultsGemini503FallbackToLocal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	geminiCh := make(chan geminiRaceResult, 1)
	localCh := make(chan localRaceResult, 1)

	geminiCh <- geminiRaceResult{err: errors.New("503 UNAVAILABLE: High demand")}
	localCh <- localRaceResult{td: &TranscriptionData{Text: "Local text"}}

	td, ads, geminiWon, err := awaitRaceResults(ctx, cancel, geminiCh, localCh, true)
	if err != nil || geminiWon {
		t.Fatalf("expected local to win after Gemini 503: err=%v, geminiWon=%v", err, geminiWon)
	}
	if td.Text != "Local text" || len(ads) != 0 {
		t.Errorf("unexpected local winner payload: td=%+v, ads=%+v", td, ads)
	}
}
