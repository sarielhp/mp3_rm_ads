package transcribe

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"os"
	"path/filepath"
	"testing"

	"github.com/sariel/abs/pkg/types"
)

func TestBuildWavHeader(t *testing.T) {
	dataSize := 32000
	header := BuildWavHeader(dataSize)

	if len(header) != 44 {
		t.Fatalf("expected header length 44, got %d", len(header))
	}
	if string(header[0:4]) != "RIFF" {
		t.Errorf("expected RIFF, got %s", string(header[0:4]))
	}
	if string(header[8:12]) != "WAVE" {
		t.Errorf("expected WAVE, got %s", string(header[8:12]))
	}
	if string(header[12:16]) != "fmt " {
		t.Errorf("expected fmt , got %s", string(header[12:16]))
	}
	if string(header[36:40]) != "data" {
		t.Errorf("expected data, got %s", string(header[36:40]))
	}
}

func TestMatchProgress(t *testing.T) {
	h, m, s, ok := matchProgressHMS("whisper.cpp: processing audio (01:23:45)")
	if !ok || h != 1 || m != 23 || s != 45 {
		t.Errorf("matchProgressHMS failed: got (%d, %d, %d, %v)", h, m, s, ok)
	}

	m, s, ok = matchProgressMS("processing audio (12:34)")
	if !ok || m != 12 || s != 34 {
		t.Errorf("matchProgressMS failed: got (%d, %d, %v)", m, s, ok)
	}

	pct, ok := matchProgressPercent("progress_common: 42.5%")
	if !ok || pct != 42.5 {
		t.Errorf("matchProgressPercent failed: got (%f, %v)", pct, ok)
	}
}

func TestBuildWhisperMultipartBody_File(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "sample.mp3")
	fileData := []byte("fake-mp3-audio-bytes")
	if err := os.WriteFile(filePath, fileData, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	reader, contentType, err := BuildWhisperMultipartBody(filePath, "test prompt", "en", nil)
	if err != nil {
		t.Fatalf("BuildWhisperMultipartBody failed: %v", err)
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
	reader, contentType, err := BuildWhisperMultipartBody("/tmp/virtual.wav", "", "auto", pcmData)
	if err != nil {
		t.Fatalf("BuildWhisperMultipartBody failed: %v", err)
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

func TestReadLimitedBody(t *testing.T) {
	raw := []byte("short message")
	res, err := ReadLimitedBody(bytes.NewReader(raw), 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(res) != "short message" {
		t.Errorf("expected 'short message', got %s", string(res))
	}

	_, err = ReadLimitedBody(bytes.NewReader(raw), 5)
	if err == nil {
		t.Fatalf("expected error exceeding 5 bytes limit")
	}
}

func TestSortSegments(t *testing.T) {
	segs := []types.TranscriptionSegment{
		{Start: 10.0, End: 12.0, Text: "third"},
		{Start: 1.0, End: 3.0, Text: "first"},
		{Start: 5.0, End: 8.0, Text: "second"},
		{Start: 5.0, End: 7.0, Text: "second-b"},
	}

	SortSegments(segs)

	if segs[0].Start != 1.0 || segs[1].Start != 5.0 || segs[2].Start != 5.0 || segs[3].Start != 10.0 {
		t.Errorf("SortSegments produced incorrect order: %+v", segs)
	}
}

func TestJoinSegmentText(t *testing.T) {
	if got := JoinSegmentText(nil); got != "" {
		t.Errorf("expected empty string for nil, got %q", got)
	}

	segs := []types.TranscriptionSegment{
		{Text: "Hello"},
		{Text: "world"},
		{Text: "from"},
		{Text: "Go"},
	}
	expected := "Hello world from Go"
	if got := JoinSegmentText(segs); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestMergeSegments(t *testing.T) {
	if got := MergeSegments(nil); got != nil {
		t.Errorf("expected nil for nil input")
	}

	segs := []types.TranscriptionSegment{
		{Start: 0.0, End: 3.0, Text: "Hello"},
		{Start: 2.0, End: 5.0, Text: "world"},
		{Start: 10.0, End: 12.0, Text: "separate"},
	}

	merged := MergeSegments(segs)
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

func TestAdjustChunkSegment(t *testing.T) {
	ch := ChunkInfo{
		ActualStart:  1200,
		ActualEnd:    2400,
		ExtractStart: 1170,
		ExtractEnd:   2430,
	}
	seg := types.TranscriptionSegment{
		Start: 20,
		End:   50,
		Text:  "Straddling segment",
		Words: []types.TranscriptionWord{
			{Start: 20, End: 30, Word: "Straddling"},
			{Start: 30, End: 50, Word: "segment"},
		},
	}

	adj, ok := AdjustChunkSegment(seg, ch, false, false)
	if !ok {
		t.Fatalf("expected straddling segment to be kept")
	}
	if adj.Start != 1200.0 || adj.End != 1220.0 {
		t.Fatalf("expected clamped [1200, 1220], got [%v, %v]", adj.Start, adj.End)
	}
	if adj.Start >= adj.End {
		t.Fatalf("inverted timestamps: start %v >= end %v", adj.Start, adj.End)
	}
	if adj.Words[0].Start != 1190.0 || adj.Words[1].End != 1220.0 {
		t.Fatalf("unexpected word timestamps: %+v", adj.Words)
	}
}
