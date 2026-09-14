package transcribe

import (
	"testing"

	"pod/pkg/types"
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
	origUsable := WhisperProfileUsable
	WhisperProfileUsable = func(wp types.WhisperProfile) bool { return true }
	t.Cleanup(func() { WhisperProfileUsable = origUsable })

	cfg := types.Config{
		WhisperConfig: types.WhisperConfig{
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
