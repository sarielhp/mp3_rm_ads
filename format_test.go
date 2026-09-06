package main

import (
	"encoding/json"
	"os"
	"testing"
)

func TestConvertJSONToSRT(t *testing.T) {
	d := t.TempDir()
	jf := d + "/t.json"
	data := TranscriptionData{Text: "hw", Segments: []TranscriptionSegment{{0, 5, "h", "", nil}, {5, 10, "w", "", nil}}}
	raw, _ := json.Marshal(data)
	os.WriteFile(jf, raw, 0644)
	srt := convertJSONToSRT(jf, &data, "", true)
	if srt == "" {
		t.Fatal("empty srt path")
	}
	content, _ := readFile(srt)
	if !containsStr(string(content), "00:00:00,000 --> 00:00:05,000") {
		t.Error("missing timestamp")
	}
}

func TestConvertJSONToTXT(t *testing.T) {
	d := t.TempDir()
	jf := d + "/t.json"
	data := TranscriptionData{Text: "hw", Segments: []TranscriptionSegment{{0, 5, "h", "", nil}}, Language: "en"}
	raw, _ := json.Marshal(data)
	os.WriteFile(jf, raw, 0644)
	txt := convertJSONToTXT(jf, &data, 10, "", true)
	if txt == "" {
		t.Fatal("empty txt path")
	}
	content, _ := readFile(txt)
	if !containsStr(string(content), "PODCAST TRANSCRIPTION") {
		t.Error("missing header")
	}
}

func TestFormatTranscript(t *testing.T) {
	d := &TranscriptionData{Text: "hw", Segments: []TranscriptionSegment{{0, 5, "h", "", nil}, {5, 10, "w", "", nil}}}
	r := formatTranscript(d, 10)
	if !containsStr(r, "[0.0s -> 5.0s] h") {
		t.Error("missing segment")
	}
}

func TestFormatTranscriptNoSegments(t *testing.T) {
	r := formatTranscript(&TranscriptionData{Text: "hw"}, 10)
	if !containsStr(r, "[0.0s -> 10.0s] hw") {
		t.Error("missing text")
	}
}

func TestSaveJSONTranscript(t *testing.T) {
	d := t.TempDir()
	saveJSONTranscript(d+"/t.mp3", &TranscriptionData{Text: "hw"}, d+"/t.json", true, nil)
	if !fileExists(d + "/t.json") {
		t.Error("transcript not saved")
	}
}

func TestSaveJSONTranscriptWithTags(t *testing.T) {
	d := t.TempDir()
	saveJSONTranscript(d+"/t.mp3", &TranscriptionData{}, d+"/t.json", true, map[string]string{"title": "T"})
	raw, _ := readFile(d + "/t.json")
	var m map[string]interface{}
	json.Unmarshal(raw, &m)
	if m["id3_title"] != "T" {
		t.Error("tag not saved")
	}
}

func TestProcessJSONFile(t *testing.T) {
	d := t.TempDir()
	jf := d + "/t.json"
	data := TranscriptionData{Text: "hw", Segments: []TranscriptionSegment{{0, 5, "h", "", nil}}}
	raw, _ := json.Marshal(data)
	os.WriteFile(jf, raw, 0644)
	processJSONFile(jf, CLIOptions{Quiet: true, ExportSRT: true, ExportTXT: true})
	if !fileExists(d+"/t.srt") || !fileExists(d+"/t.transcript.txt") {
		t.Error("exports not created")
	}
}

func TestConvertJSONToSRTFromFile(t *testing.T) {
	d := t.TempDir()
	jf := d + "/t.json"
	data := TranscriptionData{Text: "h", Segments: []TranscriptionSegment{{0, 5, "h", "", nil}}}
	r, _ := json.Marshal(data)
	os.WriteFile(jf, r, 0644)
	srt := convertJSONToSRT(jf, nil, "", true)
	if srt == "" {
		t.Error("empty srt path")
	}
}

func TestConvertJSONToTXTFromFile(t *testing.T) {
	d := t.TempDir()
	jf := d + "/t.json"
	data := TranscriptionData{Text: "h", Segments: []TranscriptionSegment{{0, 5, "h", "", nil}}}
	r, _ := json.Marshal(data)
	os.WriteFile(jf, r, 0644)
	txt := convertJSONToTXT(jf, nil, 10, "", true)
	if txt == "" {
		t.Error("empty txt path")
	}
}

func TestConvertJSONToSRTInvalid(t *testing.T) {
	d := t.TempDir()
	jf := d + "/t.json"
	os.WriteFile(jf, []byte("bad"), 0644)
	if srt := convertJSONToSRT(jf, nil, "", true); srt != "" {
		t.Error("should be empty")
	}
}

func TestConvertJSONToTXTInvalid(t *testing.T) {
	d := t.TempDir()
	jf := d + "/t.json"
	os.WriteFile(jf, []byte("bad"), 0644)
	if txt := convertJSONToTXT(jf, nil, 10, "", true); txt != "" {
		t.Error("should be empty")
	}
}

func TestConvertJSONToSRTReadError(t *testing.T) {
	if srt := convertJSONToSRT("/nonexistent", nil, "", true); srt != "" {
		t.Error("should be empty")
	}
}

func TestConvertJSONToTXTReadError(t *testing.T) {
	if txt := convertJSONToTXT("/nonexistent", nil, 0, "", true); txt != "" {
		t.Error("should be empty")
	}
}

func TestProcessJSONFileNoFile(t *testing.T) {
	processJSONFile("/nonexistent", CLIOptions{Quiet: true})
}

func TestDisplayNameBidi(t *testing.T) {
	if got := displayName("Podcast 123"); got != "Podcast 123" {
		t.Errorf("expected 'Podcast 123', got %q", got)
	}

	heb := "שלום"
	got := displayName(heb)
	if got != "םולש" {
		t.Errorf("expected 'םולש', got %q", got)
	}

	mixed := "Ep 1 - שלום עולם"
	gotMixed := displayName(mixed)
	if !containsStr(gotMixed, "Ep 1 - ") {
		t.Errorf("expected 'Ep 1 - ' in %q", gotMixed)
	}
}

func TestValidateTranscriptSanity_ReconstructFullText(t *testing.T) {
	data := &TranscriptionData{
		Text: "",
		Segments: []TranscriptionSegment{
			{Start: 0.0, End: 10.0, Text: "this is segment one with many words to satisfy sanity check"},
			{Start: 10.0, End: 20.0, Text: "this is segment two with more words to reach the threshold"},
		},
	}
	ok := validateTranscriptSanity(data, 20.0, true)
	if !ok {
		t.Errorf("expected validateTranscriptSanity to pass")
	}
}
