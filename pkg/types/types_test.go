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
	if !cfg.IsSpeculativeTranscriptionEnabled() {
		t.Error("expected default SpeculativeTranscription to be true")
	}

	f := false
	tr := true
	cfg.GeminiAPIKeyEnabled = &f
	cfg.OpenRouterAPIKeyEnabled = &tr
	cfg.SpeculativeTranscription = &f

	if cfg.IsGeminiAPIKeyEnabled() {
		t.Error("expected GeminiAPIKeyEnabled to be false")
	}
	if !cfg.IsOpenRouterAPIKeyEnabled() {
		t.Error("expected OpenRouterAPIKeyEnabled to be true")
	}
	if cfg.IsSpeculativeTranscriptionEnabled() {
		t.Error("expected SpeculativeTranscription to be false")
	}
}
