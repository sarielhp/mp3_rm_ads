package types

import (
	"encoding/json"
	"testing"
)

func TestAdSegmentJSON(t *testing.T) {
	seg := AdSegment{
		Start:  10.5,
		End:    30.2,
		Reason: "sponsor plug",
	}
	data, err := json.Marshal(seg)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var parsed AdSegment
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if parsed.Start != 10.5 || parsed.End != 30.2 || parsed.Reason != "sponsor plug" {
		t.Errorf("Unexpected unmarshaled segment: %+v", parsed)
	}
}

func TestConfigFlags(t *testing.T) {
	var cfg Config
	if !cfg.IsGeminiAPIKeyEnabled() {
		t.Error("expected default GeminiAPIKeyEnabled to be true")
	}
	if cfg.IsOpenRouterAPIKeyEnabled() {
		t.Error("expected default OpenRouterAPIKeyEnabled to be false")
	}
	if cfg.IsSpeculativeTranscriptionEnabled() {
		t.Error("expected default SpeculativeTranscription to be false")
	}
	defaultSvcs := cfg.GetCompetingServices()
	if len(defaultSvcs) != 2 || defaultSvcs[0] != "gemini" || defaultSvcs[1] != "whisper" {
		t.Errorf("unexpected default competing services: %v", defaultSvcs)
	}

	f := false
	tr := true
	cfg.GeminiAPIKeyEnabled = &f
	cfg.OpenRouterAPIKeyEnabled = &tr
	cfg.SpeculativeTranscription = &tr
	cfg.CompetingServices = []string{"local", "docker"}

	if cfg.IsGeminiAPIKeyEnabled() {
		t.Error("expected GeminiAPIKeyEnabled to be false")
	}
	if !cfg.IsOpenRouterAPIKeyEnabled() {
		t.Error("expected OpenRouterAPIKeyEnabled to be true")
	}
	if !cfg.IsSpeculativeTranscriptionEnabled() {
		t.Error("expected SpeculativeTranscription to be true")
	}
	svcs := cfg.GetCompetingServices()
	if len(svcs) != 2 || svcs[0] != "local" || svcs[1] != "docker" {
		t.Errorf("unexpected competing services: %v", svcs)
	}

	cfg.SpeculativeTranscription = &f
	if cfg.IsSpeculativeTranscriptionEnabled() {
		t.Error("expected SpeculativeTranscription to be false when explicitly disabled")
	}
}

func TestEpisodeStatusFileFavorite(t *testing.T) {
	st := EpisodeStatusFile{}
	if st.IsFavorite() {
		t.Errorf("expected initially not favorite")
	}

	st.SetFavorite(true)
	if !st.IsFavorite() || !st.Favorite {
		t.Errorf("expected favorite to be true")
	}

	data, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var parsed EpisodeStatusFile
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if !parsed.IsFavorite() || !parsed.Favorite {
		t.Errorf("expected parsed status file to be favorite")
	}

	jsonUS := []byte(`{"media_file":"test.mp3","favorite":true}`)
	var parsedUS EpisodeStatusFile
	if err := json.Unmarshal(jsonUS, &parsedUS); err != nil {
		t.Fatalf("Unmarshal US failed: %v", err)
	}
	if !parsedUS.IsFavorite() || !parsedUS.Favorite {
		t.Errorf("expected favorite:true in JSON to set favorite")
	}

	st.SetFavorite(false)
	if st.IsFavorite() || st.Favorite {
		t.Errorf("expected SetFavorite(false) to clear favorite")
	}
}
