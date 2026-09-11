package pipeline

import (
	"context"
	"errors"
	"testing"

	"abs/pkg/types"
)

func TestAwaitRaceResultsGeminiWins(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	geminiCh := make(chan GeminiRaceResult, 1)
	localCh := make(chan LocalRaceResult, 1)

	geminiCh <- GeminiRaceResult{
		TD:  &types.TranscriptionData{Text: "Gemini text"},
		Ads: []types.AdSegment{{Start: 10, End: 20, Reason: "Sponsor"}},
	}

	td, ads, geminiWon, err := AwaitRaceResults(ctx, cancel, geminiCh, localCh, true)
	if err != nil || !geminiWon {
		t.Fatalf("expected Gemini to win: err=%v, geminiWon=%v", err, geminiWon)
	}
	if td.Text != "Gemini text" || len(ads) != 1 {
		t.Errorf("unexpected race payload: td=%+v, ads=%+v", td, ads)
	}
}

func TestAwaitRaceResultsGemini503FallbackToLocal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	geminiCh := make(chan GeminiRaceResult, 1)
	localCh := make(chan LocalRaceResult, 1)

	geminiCh <- GeminiRaceResult{Err: errors.New("503 UNAVAILABLE: High demand")}
	localCh <- LocalRaceResult{TD: &types.TranscriptionData{Text: "Local text"}}

	td, ads, geminiWon, err := AwaitRaceResults(ctx, cancel, geminiCh, localCh, true)
	if err != nil || geminiWon {
		t.Fatalf("expected local to win after Gemini 503: err=%v, geminiWon=%v", err, geminiWon)
	}
	if td.Text != "Local text" || len(ads) != 0 {
		t.Errorf("unexpected local winner payload: td=%+v, ads=%+v", td, ads)
	}
}
