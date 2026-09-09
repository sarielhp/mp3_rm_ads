package detect

import (
	"context"
	"errors"
	"testing"

	"github.com/sariel/abs/pkg/types"
)

func TestExtractJSONArray(t *testing.T) {
	raw := `Here are the ads:
[
  {"start": 10.5, "end": 25.0, "reason": "sponsor plug"},
  {"start": 50.0, "end": 75.2, "reason": "midroll"}
]
Hope this helps!`

	ads := ExtractJSONArray(raw)
	if len(ads) != 2 {
		t.Fatalf("expected 2 ads, got %d", len(ads))
	}
	if ads[0].Start != 10.5 || ads[0].End != 25.0 || ads[0].Reason != "sponsor plug" {
		t.Errorf("unexpected ad 0: %+v", ads[0])
	}
	if ads[1].Start != 50.0 || ads[1].End != 75.2 || ads[1].Reason != "midroll" {
		t.Errorf("unexpected ad 1: %+v", ads[1])
	}
}

func TestExtractJSONArrayEmpty(t *testing.T) {
	raw := `No ads found: []`
	ads := ExtractJSONArray(raw)
	if ads == nil || len(ads) != 0 {
		t.Errorf("expected empty slice, got %v", ads)
	}

	invalid := `No JSON here at all`
	if ads := ExtractJSONArray(invalid); ads != nil {
		t.Errorf("expected nil for invalid JSON, got %v", ads)
	}
}

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
		if got := ContainsHebrew(c.input); got != c.expected {
			t.Errorf("ContainsHebrew(%q) = %v, expected %v", c.input, got, c.expected)
		}
	}
}

func TestWhisperProfileSupportsLanguage(t *testing.T) {
	wpEn := types.WhisperProfile{Engine: types.WhisperEngineLocal, Model: "tiny.en", Languages: []string{"en"}}
	if !WhisperProfileSupportsLanguage(wpEn, "en") {
		t.Errorf("expected wpEn to support en")
	}
	if WhisperProfileSupportsLanguage(wpEn, "he") {
		t.Errorf("expected wpEn NOT to support he")
	}

	wpMulti := types.WhisperProfile{Engine: types.WhisperEngineDocker, Languages: []string{"en", "he"}}
	if !WhisperProfileSupportsLanguage(wpMulti, "he") || !WhisperProfileSupportsLanguage(wpMulti, "en") {
		t.Errorf("expected wpMulti to support both en and he")
	}
	if WhisperProfileSupportsLanguage(wpMulti, "fr") {
		t.Errorf("expected wpMulti NOT to support fr")
	}

	wpWildcard := types.WhisperProfile{Engine: types.WhisperEngineGemini, Languages: []string{"en", "he", "*"}}
	if !WhisperProfileSupportsLanguage(wpWildcard, "fr") {
		t.Errorf("expected wpWildcard to support fr via *")
	}
}

func TestResolveLocalWhisperProfileHebrewRouting(t *testing.T) {
	cfg := types.Config{
		ActiveWhisperID: 1,
		WhisperProfiles: []types.WhisperProfile{
			{
				ID:        1,
				Name:      "Local CLI",
				Engine:    types.WhisperEngineLocal,
				Model:     "tiny.en",
				Languages: []string{"en"},
			},
			{
				ID:        2,
				Name:      "Docker Daemon",
				Engine:    types.WhisperEngineDocker,
				URL:       "http://localhost:8088/inference",
				Languages: []string{"en", "he"},
			},
		},
	}

	wpEn := ResolveLocalWhisperProfile(cfg, false)
	if wpEn.ID != 1 {
		t.Errorf("expected profile 1 for English, got %d", wpEn.ID)
	}

	wpHe := ResolveLocalWhisperProfile(cfg, true)
	if wpHe.ID != 2 {
		t.Errorf("expected profile 2 (Docker) for Hebrew, got %d", wpHe.ID)
	}
}

func TestAwaitRaceResultsGeminiWins(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	geminiCh := make(chan GeminiRaceResult, 1)
	localCh := make(chan LocalRaceResult, 1)

	geminiCh <- GeminiRaceResult{
		TD:  &types.TranscriptionData{Text: "Gemini text"},
		Ads: []types.AdSegment{{Start: 10, End: 20, Reason: "Sponsor"}},
	}

	td, ads, geminiWon, err := AwaitRaceResults(ctx, cancel, geminiCh, localCh, true)
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

	geminiCh := make(chan GeminiRaceResult, 1)
	localCh := make(chan LocalRaceResult, 1)

	geminiCh <- GeminiRaceResult{Err: errors.New("503 UNAVAILABLE: High demand")}
	localCh <- LocalRaceResult{TD: &types.TranscriptionData{Text: "Local text"}}

	td, ads, geminiWon, err := AwaitRaceResults(ctx, cancel, geminiCh, localCh, true)
	if err != nil || geminiWon {
		t.Fatalf("expected local to win after Gemini 503: err=%v, geminiWon=%v", err, geminiWon)
	}
	if td.Text != "Local text" || len(ads) != 0 {
		t.Errorf("unexpected local winner payload: td=%+v, ads=%+v", td, ads)
	}
}
