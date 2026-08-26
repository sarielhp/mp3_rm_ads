package main

import (
	"encoding/json"
	"os"
	"testing"
)

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
