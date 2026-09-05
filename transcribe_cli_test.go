package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildWhisperCLIArgs(t *testing.T) {
	args := buildWhisperCLIArgs("audio.mp3", "model.bin", "out_base", "en", "prompt text", 4, 4, true)
	expectedFlags := []string{
		"-m", "model.bin",
		"-f", "audio.mp3",
		"-dev", "0",
		"-fa",
		"-p", "4",
		"-t", "4",
		"-oj",
		"-of", "out_base",
		"-bs", "1",
		"-bo", "1",
		"-nf",
		"-l", "en",
		"--prompt", "prompt text",
	}
	if len(args) != len(expectedFlags) {
		t.Fatalf("expected %d args, got %d: %v", len(expectedFlags), len(args), args)
	}
	for i, exp := range expectedFlags {
		if args[i] != exp {
			t.Errorf("arg[%d] expected %q, got %q", i, exp, args[i])
		}
	}
}

func TestBuildWhisperCLIArgsNotGreedy(t *testing.T) {
	args := buildWhisperCLIArgs("audio.mp3", "model.bin", "out_base", "", "", 0, 0, false)
	for _, arg := range args {
		if arg == "-bs" || arg == "-bo" || arg == "-nf" {
			t.Errorf("unexpected greedy flag %q in args: %v", arg, args)
		}
	}
}

func TestParseWhisperCLIJSON(t *testing.T) {
	sampleJSON := `{
		"result": {"language": "en"},
		"transcription": [
			{
				"timestamps": {"from": "00:00:00,000", "to": "00:00:02,500"},
				"offsets": {"from": 0, "to": 2500},
				"text": " Hello world",
				"tokens": [
					{"text": " Hello", "offsets": {"from": 0, "to": 1200}},
					{"text": " world", "offsets": {"from": 1200, "to": 2500}}
				]
			},
			{
				"timestamps": {"from": "00:00:02,500", "to": "00:00:05,000"},
				"offsets": {"from": 2500, "to": 5000},
				"text": " next segment",
				"tokens": []
			}
		]
	}`

	td, err := parseWhisperCLIJSON([]byte(sampleJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if td.Language != "en" {
		t.Errorf("expected language 'en', got %q", td.Language)
	}
	if len(td.Segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(td.Segments))
	}
	if td.Segments[0].Start != 0.0 || td.Segments[0].End != 2.5 {
		t.Errorf("segment 0 timing error: %v -> %v", td.Segments[0].Start, td.Segments[0].End)
	}
	if td.Segments[0].Text != "Hello world" {
		t.Errorf("expected 'Hello world', got %q", td.Segments[0].Text)
	}
	if len(td.Segments[0].Words) != 2 {
		t.Errorf("expected 2 words, got %d", len(td.Segments[0].Words))
	}
	if td.Segments[0].Words[0].Word != "Hello" || td.Segments[0].Words[0].Start != 0.0 || td.Segments[0].Words[0].End != 1.2 {
		t.Errorf("word 0 mismatch: %+v", td.Segments[0].Words[0])
	}
	if td.Text != "Hello world next segment" {
		t.Errorf("expected 'Hello world next segment', got %q", td.Text)
	}
}

func TestParseWhisperCLIJSONInvalid(t *testing.T) {
	_, err := parseWhisperCLIJSON([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid json, got nil")
	}
}

func TestParseWhisperTimestamp(t *testing.T) {
	sec := parseWhisperTimestamp("01:02:03,500")
	expected := 3600.0 + 120.0 + 3.0 + 0.5
	if sec != expected {
		t.Errorf("expected %v, got %v", expected, sec)
	}
	if zero := parseWhisperTimestamp("invalid"); zero != 0 {
		t.Errorf("expected 0, got %v", zero)
	}
}

func TestParseWhisperCLITimestampsFallback(t *testing.T) {
	sampleJSON := `{
		"result": {"language": "en"},
		"transcription": [
			{
				"timestamps": {"from": "00:01:00,000", "to": "00:01:10,500"},
				"offsets": {"from": 0, "to": 0},
				"text": "fallback text"
			}
		]
	}`
	td, err := parseWhisperCLIJSON([]byte(sampleJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(td.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(td.Segments))
	}
	if td.Segments[0].Start != 60.0 || td.Segments[0].End != 70.5 {
		t.Errorf("expected 60 -> 70.5, got %v -> %v", td.Segments[0].Start, td.Segments[0].End)
	}
}

func TestResolveWhisperModelPath(t *testing.T) {
	tmpDir := t.TempDir()
	fakeModel := filepath.Join(tmpDir, "ggml-tiny.en.bin")
	if err := os.WriteFile(fakeModel, []byte("fake model"), 0644); err != nil {
		t.Fatalf("failed to create fake model: %v", err)
	}

	p, err := resolveWhisperModelPath(fakeModel)
	if err != nil || p != fakeModel {
		t.Errorf("expected direct resolution %q, got %q (err: %v)", fakeModel, p, err)
	}

	extraWhisperModelDirs = []string{tmpDir}
	defer func() { extraWhisperModelDirs = nil }()

	resolved, err := resolveWhisperModelPath("tiny.en")
	if err != nil {
		t.Fatalf("failed to resolve tiny.en from search dirs: %v", err)
	}
	if resolved != fakeModel {
		t.Errorf("expected %q, got %q", fakeModel, resolved)
	}

	resolvedTiny, err := resolveWhisperModelPath("tiny")
	if err != nil {
		t.Fatalf("failed to resolve tiny alias: %v", err)
	}
	if resolvedTiny != fakeModel {
		t.Errorf("expected %q, got %q", fakeModel, resolvedTiny)
	}

	_, err = resolveWhisperModelPath("nonexistent_model_xyz")
	if err == nil {
		t.Error("expected error for nonexistent model, got nil")
	}
}

func TestRunWhisperCLITranscriptionMock(t *testing.T) {
	tmpDir := t.TempDir()
	modelFile := filepath.Join(tmpDir, "ggml-tiny.en.bin")
	_ = os.WriteFile(modelFile, []byte("fake model"), 0644)

	audioFile := filepath.Join(tmpDir, "episode.mp3")
	_ = os.WriteFile(audioFile, []byte("fake mp3"), 0644)

	mockBin := filepath.Join(tmpDir, "mock-whisper-cli")
	mockScript := `#!/usr/bin/env ruby
output_base = nil
ARGV.each_with_index do |arg, i|
  output_base = ARGV[i + 1] if arg == "-of"
end
if output_base
  json_data = {
    "result" => { "language" => "en" },
    "transcription" => [
      {
        "offsets" => { "from" => 0, "to" => 3000 },
        "text" => "mocked transcription text",
        "tokens" => []
      }
    ]
  }
  require "json"
  File.write("#{output_base}.json", JSON.generate(json_data))
end
exit 0
`
	if err := os.WriteFile(mockBin, []byte(mockScript), 0755); err != nil {
		t.Fatalf("failed to write mock script: %v", err)
	}

	profile := WhisperProfile{
		Engine:     WhisperEngineLocal,
		CliBinary:  mockBin,
		Model:      modelFile,
		Processors: 2,
		Threads:    2,
		Greedy:     true,
	}

	td, err := runWhisperCLITranscription(audioFile, profile, true, false, "", "en")
	if err != nil {
		t.Fatalf("runWhisperCLITranscription failed: %v", err)
	}
	if td.Text != "mocked transcription text" {
		t.Errorf("expected 'mocked transcription text', got %q", td.Text)
	}
	if len(td.Segments) != 1 || td.Segments[0].End != 3.0 {
		t.Errorf("segment data unexpected: %+v", td.Segments)
	}
}

func TestRunWhisperCLITranscriptionError(t *testing.T) {
	profile := WhisperProfile{
		Engine: WhisperEngineLocal,
		Model:  "/nonexistent/model.bin",
	}
	_, err := runWhisperCLITranscription("/tmp/dummy.mp3", profile, true, false, "", "")
	if err == nil {
		t.Error("expected error for missing model, got nil")
	}
}

func TestResolveWhisperCLIBinary(t *testing.T) {
	bin := resolveWhisperCLIBinary("sh")
	if bin == "" {
		t.Error("expected to find sh binary")
	}
	custom := resolveWhisperCLIBinary("nonexistent-whisper-binary-12345")
	if custom != "nonexistent-whisper-binary-12345" {
		t.Errorf("expected nonexistent binary name returned as fallback, got %q", custom)
	}
}
