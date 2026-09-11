package transcribe

import (
	"context"
	"errors"
	"testing"

	"abs/pkg/types"
)

type mockTranscriber struct {
	td  *types.TranscriptionData
	err error
}

func (m *mockTranscriber) Transcribe(ctx context.Context, path string, opts Options) (*types.TranscriptionData, error) {
	return m.td, m.err
}

func TestFallbackTranscriberPrimarySucceeds(t *testing.T) {
	primary := &mockTranscriber{td: &types.TranscriptionData{Text: "Primary text"}}
	secondary := &mockTranscriber{td: &types.TranscriptionData{Text: "Secondary text"}}

	fallbackCalled := false
	fb := NewFallbackTranscriber(primary, secondary, func(err error) {
		fallbackCalled = true
	})

	res, err := fb.Transcribe(context.Background(), "test.mp3", Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Text != "Primary text" {
		t.Errorf("got %q, want Primary text", res.Text)
	}
	if fallbackCalled {
		t.Errorf("expected fallback NOT to be called")
	}
}

func TestFallbackTranscriberFallsBackOnPrimaryFailure(t *testing.T) {
	primary := &mockTranscriber{err: errors.New("primary connection failed")}
	secondary := &mockTranscriber{td: &types.TranscriptionData{Text: "Secondary text"}}

	fallbackCalled := false
	fb := NewFallbackTranscriber(primary, secondary, func(err error) {
		fallbackCalled = true
	})

	res, err := fb.Transcribe(context.Background(), "test.mp3", Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Text != "Secondary text" {
		t.Errorf("got %q, want Secondary text", res.Text)
	}
	if !fallbackCalled {
		t.Errorf("expected fallback callback to be invoked")
	}
}
