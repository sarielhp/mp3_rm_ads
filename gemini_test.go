package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cloud.google.com/go/vertexai/genai"
)

func TestGeminiPromptContent(t *testing.T) {
	_ = t.TempDir()
	if len(geminiAdRemovalPrompt) == 0 {
		t.Fatal("expected geminiAdRemovalPrompt to be non-empty")
	}
	for _, expected := range []string{"advertisement", "music_interlude", "intro_outro", "cuts", "segments", "חסויות", "קודי קופון"} {
		if !strings.Contains(geminiAdRemovalPrompt, expected) {
			t.Errorf("expected prompt to contain %q", expected)
		}
	}
}

func TestParseGeminiJSONStringValid(t *testing.T) {
	_ = t.TempDir()
	rawJSON := `{
		"cuts": [
			{"start": 10.5, "end": 45.0, "type": "advertisement", "reason": "Sponsor Wolt"},
			{"start": 120.0, "end": 135.0, "type": "music_interlude", "reason": "Transition"}
		],
		"segments": [
			{"start": 0.0, "end": 10.5, "text": "Hello and welcome"},
			{"start": 45.0, "end": 60.0, "text": "ברוכים הבאים לפרק"}
		]
	}`

	payload, err := parseGeminiJSONString(rawJSON)
	if err != nil {
		t.Fatalf("unexpected error parsing valid JSON: %v", err)
	}
	if len(payload.Cuts) != 2 {
		t.Fatalf("expected 2 cuts, got %d", len(payload.Cuts))
	}
	if payload.Cuts[0].Start != 10.5 || payload.Cuts[0].Type != "advertisement" {
		t.Errorf("unexpected cut 0: %+v", payload.Cuts[0])
	}
	if len(payload.Segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(payload.Segments))
	}
	if payload.Segments[1].Text != "ברוכים הבאים לפרק" {
		t.Errorf("unexpected segment text: %s", payload.Segments[1].Text)
	}
}

func TestParseGeminiJSONStringMarkdownBlocks(t *testing.T) {
	_ = t.TempDir()
	rawJSON := "```json\n{\n  \"cuts\": [],\n  \"segments\": [{\"start\": 1.0, \"end\": 2.0, \"text\": \"test\"}]\n}\n```"
	payload, err := parseGeminiJSONString(rawJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(payload.Segments) != 1 || payload.Segments[0].Text != "test" {
		t.Errorf("unexpected payload: %+v", payload)
	}

	rawJSON2 := "```\n{\n  \"cuts\": [],\n  \"segments\": []\n}\n```"
	payload2, err := parseGeminiJSONString(rawJSON2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(payload2.Cuts) != 0 || len(payload2.Segments) != 0 {
		t.Errorf("unexpected payload: %+v", payload2)
	}
}

func TestParseGeminiJSONStringErrors(t *testing.T) {
	_ = t.TempDir()
	for _, invalid := range []string{"", "   \t  ", "{not-valid-json}", "random text"} {
		_, err := parseGeminiJSONString(invalid)
		if err == nil {
			t.Errorf("expected error for input %q, got nil", invalid)
		}
	}
}

func TestParseGeminiContentResponse(t *testing.T) {
	_ = t.TempDir()
	if _, err := parseGeminiContentResponse(nil); err == nil {
		t.Error("expected error for nil response")
	}

	emptyCandidates := &genai.GenerateContentResponse{}
	if _, err := parseGeminiContentResponse(emptyCandidates); err == nil {
		t.Error("expected error for empty candidates")
	}

	nilContent := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{{Content: nil}},
	}
	if _, err := parseGeminiContentResponse(nilContent); err == nil {
		t.Error("expected error for nil candidate content")
	}

	validResp := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content: &genai.Content{
					Parts: []genai.Part{
						genai.Text(`{"cuts":[{"start":1.0,"end":5.0,"type":"ad","reason":"sponsor"}],"segments":[{"start":5.0,"end":10.0,"text":"content"}]}`),
					},
				},
			},
		},
	}
	payload, err := parseGeminiContentResponse(validResp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(payload.Cuts) != 1 || payload.Cuts[0].Start != 1.0 {
		t.Errorf("unexpected payload cuts: %+v", payload.Cuts)
	}
}

func TestConvertGeminiToAbsTypes(t *testing.T) {
	_ = t.TempDir()
	tdEmpty, adsEmpty := convertGeminiToAbsTypes(nil)
	if tdEmpty == nil || len(adsEmpty) != 0 {
		t.Errorf("expected empty result for nil payload, got td=%v ads=%v", tdEmpty, adsEmpty)
	}

	payload := &geminiResponsePayload{
		Cuts: []geminiCutItem{
			{Start: 12.0, End: 30.5, Type: "advertisement", Reason: "Wolt plug"},
			{Start: 50.0, End: 60.0, Type: "music_interlude", Reason: ""},
			{Start: 70.0, End: 80.0, Type: "", Reason: "Generic ad"},
		},
		Segments: []geminiSegmentItem{
			{Start: 0.0, End: 12.0, Text: "Intro words."},
			{Start: 30.5, End: 50.0, Text: "Main discussion."},
		},
	}

	td, ads := convertGeminiToAbsTypes(payload)
	if len(td.Segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(td.Segments))
	}
	if td.Segments[0].Start != 0.0 || td.Segments[0].End != 12.0 || td.Segments[0].Text != "Intro words." {
		t.Errorf("unexpected segment 0: %+v", td.Segments[0])
	}
	if td.Text != "Intro words. Main discussion." {
		t.Errorf("unexpected full text: %q", td.Text)
	}

	if len(ads) != 3 {
		t.Fatalf("expected 3 ads, got %d", len(ads))
	}
	if ads[0].Reason != "[advertisement] Wolt plug" {
		t.Errorf("unexpected ad 0 reason: %q", ads[0].Reason)
	}
	if ads[1].Reason != "[music_interlude]" {
		t.Errorf("unexpected ad 1 reason: %q", ads[1].Reason)
	}
	if ads[2].Reason != "Generic ad" {
		t.Errorf("unexpected ad 2 reason: %q", ads[2].Reason)
	}
}

func TestIsGeminiEngine(t *testing.T) {
	_ = t.TempDir()
	cfg := Config{WhisperEngine: WhisperEngineLocal}
	cli := CLIOptions{WhisperEngine: "gemini"}
	if !isGeminiEngine(cfg, cli) {
		t.Error("expected true when cli.WhisperEngine is gemini")
	}

	cli2 := CLIOptions{}
	cfg2 := Config{
		ActiveWhisperID: 1,
		WhisperProfiles: []WhisperProfile{
			{ID: 1, Engine: WhisperEngineGemini},
		},
	}
	if !isGeminiEngine(cfg2, cli2) {
		t.Error("expected true when active whisper profile engine is gemini")
	}

	cfg3 := Config{
		ActiveWhisperID: 1,
		WhisperProfiles: []WhisperProfile{
			{ID: 1, Engine: WhisperEngineLocal},
		},
	}
	if isGeminiEngine(cfg3, cli2) {
		t.Error("expected false for local engine")
	}
}

func TestGeminiConfigDefaultsAndGetters(t *testing.T) {
	_ = t.TempDir()
	var emptyCfg Config
	if emptyCfg.GetGeminiProjectID() != "vm-on-cloud-sariel" {
		t.Errorf("unexpected default project id: %s", emptyCfg.GetGeminiProjectID())
	}
	if emptyCfg.GetGeminiStagingBucket() != "abs-audio-staging-sariel" {
		t.Errorf("unexpected default staging bucket: %s", emptyCfg.GetGeminiStagingBucket())
	}
	if emptyCfg.GetGeminiLocation() != "us-central1" {
		t.Errorf("unexpected default location: %s", emptyCfg.GetGeminiLocation())
	}

	customCfg := Config{
		GeminiProjectID:     "my-custom-project",
		GeminiStagingBucket: "gs://my-staging-bucket",
		GeminiLocation:      "europe-west1",
	}
	if customCfg.GetGeminiProjectID() != "my-custom-project" {
		t.Errorf("unexpected custom project id: %s", customCfg.GetGeminiProjectID())
	}
	if customCfg.GetGeminiStagingBucket() != "my-staging-bucket" {
		t.Errorf("unexpected trimmed bucket: %s", customCfg.GetGeminiStagingBucket())
	}
	if customCfg.GetGeminiLocation() != "europe-west1" {
		t.Errorf("unexpected custom location: %s", customCfg.GetGeminiLocation())
	}
}

func TestApplyGeminiEnvOverrides(t *testing.T) {
	_ = t.TempDir()
	origProject := os.Getenv("GEMINI_PROJECT_ID")
	origBucket := os.Getenv("GEMINI_STAGING_BUCKET")
	origLoc := os.Getenv("GEMINI_LOCATION")
	defer func() {
		os.Setenv("GEMINI_PROJECT_ID", origProject)
		os.Setenv("GEMINI_STAGING_BUCKET", origBucket)
		os.Setenv("GEMINI_LOCATION", origLoc)
	}()

	os.Setenv("GEMINI_PROJECT_ID", "env-project-123")
	os.Setenv("GEMINI_STAGING_BUCKET", "gs://env-bucket-456")
	os.Setenv("GEMINI_LOCATION", "me-west1")

	cfg := Config{}
	applyGeminiEnvOverrides(&cfg)

	if cfg.GeminiProjectID != "env-project-123" {
		t.Errorf("expected env project id, got %s", cfg.GeminiProjectID)
	}
	if cfg.GeminiStagingBucket != "gs://env-bucket-456" {
		t.Errorf("expected env bucket, got %s", cfg.GeminiStagingBucket)
	}
	if cfg.GeminiLocation != "me-west1" {
		t.Errorf("expected env location, got %s", cfg.GeminiLocation)
	}
}

func TestGeminiUploadAudioToGCSNonExistentFile(t *testing.T) {
	tempDir := t.TempDir()
	nonExistent := filepath.Join(tempDir, "missing.mp3")
	ctx := context.Background()
	_, err := uploadAudioToGCS(ctx, "test-bucket", nonExistent)
	if err == nil {
		t.Error("expected error for non-existent file, got nil")
	}
}

func TestGeminiDeleteGCSObjectEmptyPrefix(t *testing.T) {
	_ = t.TempDir()
	ctx := context.Background()
	deleteGCSObject(ctx, "test-bucket", "")
	deleteGCSObject(ctx, "test-bucket", "short")
}

func TestWhisperEngineBadgeAndSpecGemini(t *testing.T) {
	_ = t.TempDir()
	badge := whisperEngineBadge(WhisperEngineGemini)
	if badge != "[GEMINI]" {
		t.Errorf("expected [GEMINI], got %s", badge)
	}

	spec := "Gemini Fast|gemini|gemini-1.5-flash|60.0"
	p := parseWhisperProfileSpec(spec, 7)
	if p.ID != 7 || p.Engine != WhisperEngineGemini || p.Name != "Gemini Fast" {
		t.Errorf("unexpected parsed profile: %+v", p)
	}
}

func TestInferWhisperEngineURLGemini(t *testing.T) {
	_ = t.TempDir()
	eng := inferWhisperEngineFromURL("https://us-central1-aiplatform.googleapis.com/gemini")
	if eng != WhisperEngineGemini {
		t.Errorf("expected WhisperEngineGemini, got %s", eng)
	}
}
