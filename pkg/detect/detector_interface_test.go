package detect

import (
	"context"
	"errors"
	"testing"

	"pod/pkg/types"
)

type mockAdDetector struct {
	calls int
	segs  [][]types.AdSegment
	errs  []error
}

func (m *mockAdDetector) DetectAds(ctx context.Context, text string) ([]types.AdSegment, error) {
	idx := m.calls
	m.calls++
	var res []types.AdSegment
	if idx < len(m.segs) {
		res = m.segs[idx]
	}
	var err error
	if idx < len(m.errs) {
		err = m.errs[idx]
	}
	return res, err
}

func TestConfirmingDetectorRecoversAdsOnRetry(t *testing.T) {
	t.Parallel()
	mock := &mockAdDetector{
		segs: [][]types.AdSegment{
			{},                                   // initial empty
			{{Start: 10, End: 20, Reason: "Ad"}}, // recovered on retry
		},
	}
	cd := NewConfirmingDetector(mock, 2)
	ads, err := cd.DetectAds(context.Background(), "transcript")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ads) != 1 || ads[0].Start != 10 {
		t.Fatalf("expected recovered ad segment, got %+v", ads)
	}
	if mock.calls != 2 {
		t.Errorf("expected 2 calls, got %d", mock.calls)
	}
}

func TestConfirmingDetectorFailsOnRetryError(t *testing.T) {
	t.Parallel()
	mock := &mockAdDetector{
		segs: [][]types.AdSegment{{}, {}},
		errs: []error{nil, errors.New("rate limited")},
	}
	cd := NewConfirmingDetector(mock, 2)
	_, err := cd.DetectAds(context.Background(), "transcript")
	if err == nil {
		t.Fatal("expected error when confirmation fails, got nil")
	}
}

func TestConfirmingDetectorBelievesConfirmedEmpty(t *testing.T) {
	t.Parallel()
	mock := &mockAdDetector{
		segs: [][]types.AdSegment{{}, {}, {}},
	}
	cd := NewConfirmingDetector(mock, 2)
	ads, err := cd.DetectAds(context.Background(), "transcript")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ads) != 0 {
		t.Fatalf("expected empty ads, got %+v", ads)
	}
	if mock.calls != 3 {
		t.Errorf("expected 3 calls (initial + 2 confirmations), got %d", mock.calls)
	}
}
