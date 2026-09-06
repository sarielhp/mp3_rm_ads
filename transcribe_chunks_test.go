package main

import (
	"testing"
)

func TestAdjustChunkSegment_FirstChunkTrailingOverlap(t *testing.T) {
	ch := chunkInfo{
		actualStart:  0,
		actualEnd:    1200,
		extractStart: 0,
		extractEnd:   1230,
	}
	seg := TranscriptionSegment{
		Start: 1205,
		End:   1215,
		Text:  "Hello from overlap",
	}

	adj, ok := adjustChunkSegment(seg, ch, true, false)
	if ok {
		t.Fatalf("expected segment outside actualEnd to be dropped, got ok=true, seg: %+v", adj)
	}
}

func TestAdjustChunkSegment_NonFirstChunkLeadingOverlap(t *testing.T) {
	ch := chunkInfo{
		actualStart:  1200,
		actualEnd:    2400,
		extractStart: 1170,
		extractEnd:   2430,
	}
	seg := TranscriptionSegment{
		Start: 10,
		End:   25,
		Text:  "Hello from leading overlap",
	}

	adj, ok := adjustChunkSegment(seg, ch, false, false)
	if ok {
		t.Fatalf("expected segment before actualStart to be dropped, got ok=true, seg: %+v", adj)
	}
}

func TestAdjustChunkSegment_StraddlingClamped(t *testing.T) {
	ch := chunkInfo{
		actualStart:  1200,
		actualEnd:    2400,
		extractStart: 1170,
		extractEnd:   2430,
	}
	seg := TranscriptionSegment{
		Start: 20,
		End:   50,
		Text:  "Straddling segment",
		Words: []TranscriptionWord{
			{Start: 20, End: 30, Word: "Straddling"},
			{Start: 30, End: 50, Word: "segment"},
		},
	}

	adj, ok := adjustChunkSegment(seg, ch, false, false)
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
