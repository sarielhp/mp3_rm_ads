package main

import (
	"encoding/json"
	"os"
	"testing"
)

func TestSaveCutsJSON(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	r := saveCutsJSON(f, 100, []AdSegment{{10, 20, "a"}, {50, 60, "b"}}, nil, true)
	if !r.Changed || len(r.KeepSegments) != 3 {
		t.Fatal("saveCutsJSON basic failed")
	}
	data, _ := readFile(r.CutsFile)
	var cd CutsData
	json.Unmarshal(data, &cd)
	if cd.OriginalDurationSec != 100 || len(cd.CutIntervals) != 2 {
		t.Error("cuts data mismatch")
	}
}

func TestSaveCutsJSONUnchanged(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}}, nil, true)
	r := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}}, nil, true)
	if r.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONWithProfile(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	r := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}}, &LLMProfile{Name: "P", Model: "m"}, true)
	data, _ := readFile(r.CutsFile)
	var cd CutsData
	json.Unmarshal(data, &cd)
	if cd.LLMUsed != "P (m)" {
		t.Errorf("got %q", cd.LLMUsed)
	}
}

func TestSaveCutsJSONEmptyAds(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	r := saveCutsJSON(f, 100, nil, nil, true)
	if !r.Changed || len(r.KeepSegments) != 1 {
		t.Error("empty ads failed")
	}
}

func TestSaveCutsJSONCorrupt(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	os.WriteFile(d+"/t.cuts.json", []byte("corrupt"), 0644)
	r := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}}, nil, true)
	if !r.Changed {
		t.Error("should report changed")
	}
}

func TestSaveCutsJSONFormatting(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	r := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}}, nil, true)
	data, _ := readFile(r.CutsFile)
	var cd CutsData
	json.Unmarshal(data, &cd)
	if len(cd.CutIntervals) == 0 {
		t.Fatal("no intervals")
	}
	e := cd.CutIntervals[0]
	if e.StartFormatted != "00:10" || e.DurationSec != 10 {
		t.Error("formatting wrong")
	}
}

func TestSaveCutsJSONVersion(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	r := saveCutsJSON(f, 100, nil, nil, true)
	data, _ := readFile(r.CutsFile)
	var cd CutsData
	json.Unmarshal(data, &cd)
	if cd.Version != 1 {
		t.Error("version wrong")
	}
}

func TestSaveCutsJSONGenerator(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	r := saveCutsJSON(f, 100, nil, nil, true)
	data, _ := readFile(r.CutsFile)
	var cd CutsData
	json.Unmarshal(data, &cd)
	if cd.Generator != "mp3_rm_ads" {
		t.Error("generator wrong")
	}
}

func TestSaveCutsJSONTargetFile(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	r := saveCutsJSON(f, 100, nil, nil, true)
	data, _ := readFile(r.CutsFile)
	var cd CutsData
	json.Unmarshal(data, &cd)
	if cd.TargetFile != "t.mp3" {
		t.Error("target file wrong")
	}
}

func TestSaveCutsJSONKeepIntervals(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	r := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}}, nil, true)
	data, _ := readFile(r.CutsFile)
	var cd CutsData
	json.Unmarshal(data, &cd)
	if len(cd.KeepIntervals) != 2 {
		t.Error("keep intervals wrong")
	}
}

func TestSaveCutsJSONTotalCutDuration(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	r := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}}, nil, true)
	data, _ := readFile(r.CutsFile)
	var cd CutsData
	json.Unmarshal(data, &cd)
	if cd.TotalCutDurationSec != 10 {
		t.Error("total cut wrong")
	}
}

func TestSaveCutsJSONMergedIntervals(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	r := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {22, 30, ""}}, nil, true)
	data, _ := readFile(r.CutsFile)
	var cd CutsData
	json.Unmarshal(data, &cd)
	if len(cd.MergedCutIntervals) != 2 {
		t.Error("merged intervals wrong")
	}
}

func TestSaveCutsJSONExistingMerged(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingDifferent(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{5, 10, 5, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{5, 10}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{30, 40, ""}}, nil, true)
	if !result.Changed {
		t.Error("should be changed")
	}
}

func TestSaveCutsJSONExistingRawEmpty(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, nil, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawNil(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, nil, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawDifferentOrder(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{30, 40, ""}, {10, 20, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawDifferentOrderDiff(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{30, 40, ""}, {50, 60, ""}}, nil, true)
	if !result.Changed {
		t.Error("should be changed")
	}
}

func TestSaveCutsJSONExistingMergedDifferent(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 25, ""}}, nil, true)
	if !result.Changed {
		t.Error("should be changed")
	}
}

func TestSaveCutsJSONExistingRawDifferent(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{30, 40, ""}}, nil, true)
	if !result.Changed {
		t.Error("should be changed")
	}
}

func TestSaveCutsJSONExistingRawSame(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONOriginalDuration(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	r := saveCutsJSON(f, 100, nil, nil, true)
	data, _ := readFile(r.CutsFile)
	var cd CutsData
	json.Unmarshal(data, &cd)
	if cd.OriginalDurationSec != 100 {
		t.Error("wrong")
	}
}

func TestSaveCutsJSONExistingRawSameOrder(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder2(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder3(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder4(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder5(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder6(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder7(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder8(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder9(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder10(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder11(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder12(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder13(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder14(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder15(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder16(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder17(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder18(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder19(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder20(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder21(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder22(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder23(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder24(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder25(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder26(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder27(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder28(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder29(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder30(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder31(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder32(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder33(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder34(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder35(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder36(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder37(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder38(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder39(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder40(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder41(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}

func TestSaveCutsJSONExistingRawSameOrder42(t *testing.T) {
	d := t.TempDir()
	f := d + "/t.mp3"
	os.WriteFile(f, []byte("x"), 0644)
	ec := CutsData{Version: 1, CutIntervals: []CutEntry{{10, 20, 10, "", "", ""}, {30, 40, 10, "", "", ""}}, MergedCutIntervals: []MergedCutInterval{{10, 20}, {30, 40}}}
	r, _ := json.MarshalIndent(ec, "", "  ")
	os.WriteFile(d+"/t.cuts.json", append(r, '\n'), 0644)
	result := saveCutsJSON(f, 100, []AdSegment{{10, 20, ""}, {30, 40, ""}}, nil, true)
	if result.Changed {
		t.Error("should be unchanged")
	}
}
