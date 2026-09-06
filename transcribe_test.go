package main

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildWhisperMultipartBody_File(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "sample.mp3")
	fileData := []byte("fake-mp3-audio-bytes")
	if err := os.WriteFile(filePath, fileData, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	reader, contentType, err := buildWhisperMultipartBody(filePath, "test prompt", "en", nil)
	if err != nil {
		t.Fatalf("buildWhisperMultipartBody failed: %v", err)
	}
	defer reader.Close()

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "multipart/form-data" {
		t.Fatalf("unexpected content type %q: %v", contentType, err)
	}

	mr := multipart.NewReader(reader, params["boundary"])
	foundFile := false
	fields := make(map[string]string)

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("reading multipart part failed: %v", err)
		}
		name := part.FormName()
		if name == "file" {
			foundFile = true
			if part.FileName() != "sample.mp3" {
				t.Errorf("expected filename sample.mp3, got %s", part.FileName())
			}
			partBytes, _ := io.ReadAll(part)
			if !bytes.Equal(partBytes, fileData) {
				t.Errorf("expected file bytes %v, got %v", fileData, partBytes)
			}
		} else {
			val, _ := io.ReadAll(part)
			fields[name] = string(val)
		}
	}

	if !foundFile {
		t.Errorf("file part not found in multipart body")
	}
	if fields["response_format"] != "verbose_json" {
		t.Errorf("unexpected response_format: %s", fields["response_format"])
	}
	if fields["language"] != "en" {
		t.Errorf("unexpected language: %s", fields["language"])
	}
	if fields["prompt"] != "test prompt" {
		t.Errorf("unexpected prompt: %s", fields["prompt"])
	}
}

func TestBuildWhisperMultipartBody_PCM(t *testing.T) {
	pcmData := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	reader, contentType, err := buildWhisperMultipartBody("/tmp/virtual.wav", "", "auto", pcmData)
	if err != nil {
		t.Fatalf("buildWhisperMultipartBody failed: %v", err)
	}
	defer reader.Close()

	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("failed parsing media type: %v", err)
	}

	mr := multipart.NewReader(reader, params["boundary"])
	foundFile := false

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("reading part failed: %v", err)
		}
		if part.FormName() == "file" {
			foundFile = true
			content, _ := io.ReadAll(part)
			if len(content) != 44+len(pcmData) {
				t.Fatalf("expected 44-byte header + 8 bytes PCM, got %d bytes", len(content))
			}
			if !bytes.Equal(content[44:], pcmData) {
				t.Errorf("PCM data mismatch")
			}
		}
	}

	if !foundFile {
		t.Errorf("file part not found")
	}
}

func TestReadLimitedBody_Main(t *testing.T) {
	raw := []byte("short message")
	res, err := readLimitedBody(bytes.NewReader(raw), 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(res) != "short message" {
		t.Errorf("expected 'short message', got %s", string(res))
	}

	_, err = readLimitedBody(bytes.NewReader(raw), 5)
	if err == nil {
		t.Fatalf("expected error exceeding 5 bytes limit")
	}
}

func TestSortSegmentsComprehensive(t *testing.T) {
	segs := []TranscriptionSegment{
		{Start: 10.0, End: 12.0, Text: "third"},
		{Start: 1.0, End: 3.0, Text: "first"},
		{Start: 5.0, End: 8.0, Text: "second"},
		{Start: 5.0, End: 7.0, Text: "second-b"},
	}

	sortSegments(segs)

	if segs[0].Start != 1.0 || segs[1].Start != 5.0 || segs[2].Start != 5.0 || segs[3].Start != 10.0 {
		t.Errorf("sortSegments produced incorrect order: %+v", segs)
	}

	empty := []TranscriptionSegment{}
	sortSegments(empty)
	if len(empty) != 0 {
		t.Errorf("empty slice sort failed")
	}

	large := make([]TranscriptionSegment, 1000)
	for i := range large {
		large[i] = TranscriptionSegment{Start: float64(1000 - i), Text: "test"}
	}
	sortSegments(large)
	for i := 0; i < len(large)-1; i++ {
		if large[i].Start > large[i+1].Start {
			t.Fatalf("large sort out of order at %d", i)
		}
	}
}

func TestJoinSegmentText(t *testing.T) {
	if got := joinSegmentText(nil); got != "" {
		t.Errorf("expected empty string for nil, got %q", got)
	}

	segs := []TranscriptionSegment{
		{Text: "Hello"},
		{Text: "world"},
		{Text: "from"},
		{Text: "Go"},
	}
	expected := "Hello world from Go"
	if got := joinSegmentText(segs); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestMergeSegmentsOptimized(t *testing.T) {
	if got := mergeSegments(nil); got != nil {
		t.Errorf("expected nil for nil input")
	}

	segs := []TranscriptionSegment{
		{Start: 0.0, End: 3.0, Text: "Hello"},
		{Start: 2.0, End: 5.0, Text: "world"},
		{Start: 10.0, End: 12.0, Text: "separate"},
	}

	merged := mergeSegments(segs)
	if len(merged) != 2 {
		t.Fatalf("expected 2 merged segments, got %d", len(merged))
	}
	if merged[0].Start != 0.0 || merged[0].End != 5.0 || merged[0].Text != "Hello world" {
		t.Errorf("unexpected merged[0]: %+v", merged[0])
	}
	if merged[1].Start != 10.0 || merged[1].End != 12.0 || merged[1].Text != "separate" {
		t.Errorf("unexpected merged[1]: %+v", merged[1])
	}
}
