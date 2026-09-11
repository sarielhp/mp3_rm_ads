package detect

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"abs/pkg/types"
)

func TestExtractJSONArray(t *testing.T) {
	raw := `Here are the ads:
[
  {"start": 10.5, "end": 25.0, "reason": "sponsor plug"},
  {"start": 50.0, "end": 75.2, "reason": "midroll"}
]
Hope this helps!`

	ads, err := ExtractJSONArray(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ads) != 2 {
		t.Fatalf("expected 2 ads, got %d", len(ads))
	}
	if ads[0].Start != 10.5 || ads[0].End != 25.0 {
		t.Errorf("unexpected ad 0: %+v", ads[0])
	}
	if ads[1].Reason != "midroll" {
		t.Errorf("unexpected ad 1 reason: %s", ads[1].Reason)
	}
}

func TestExtractJSONArrayEmpty(t *testing.T) {
	empty := `[]`
	ads, err := ExtractJSONArray(empty)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ads) != 0 {
		t.Errorf("expected 0 ads, got %d", len(ads))
	}

	invalid := `No JSON here at all`
	if _, err := ExtractJSONArray(invalid); err == nil {
		t.Errorf("expected error for invalid JSON, got nil")
	}
}

// adStubServer answers each ad-detection call with the next canned body,
// repeating the last one once the list runs out.
func adStubServer(t *testing.T, bodies ...string) (*httptest.Server, *int32) {
	t.Helper()
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(atomic.AddInt32(&calls, 1)) - 1
		if n >= len(bodies) {
			n = len(bodies) - 1
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"choices":[{"message":{"role":"assistant","content":%q}}]}`, bodies[n])
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestDetectAdsRetriesEmptyAnswer(t *testing.T) {
	// A single empty answer is the observed flake: re-asking must recover the
	// ads rather than marking the episode clean.
	found := `[{"start": 0.0, "end": 36.6, "reason": "Preroll promo"}]`
	srv, calls := adStubServer(t, "[]", found)
	profile := types.LLMProfile{Name: "stub", Type: "openrouter", URL: srv.URL, Model: "stub"}

	segs, err := DetectAdsLLMTimeout("transcript", profile, "key", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(segs) != 1 || segs[0].End != 36.6 {
		t.Fatalf("expected the retry's ads to win, got %+v", segs)
	}
	if got := atomic.LoadInt32(calls); got != 2 {
		t.Errorf("expected 2 calls (initial + one retry), got %d", got)
	}
}

func TestDetectAdsAcceptsConfirmedEmptyAnswer(t *testing.T) {
	// A genuinely ad-free episode still returns empty, after confirmation.
	srv, calls := adStubServer(t, "[]")
	profile := types.LLMProfile{Name: "stub", Type: "openrouter", URL: srv.URL, Model: "stub"}

	segs, err := DetectAdsLLMTimeout("transcript", profile, "key", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(segs) != 0 {
		t.Fatalf("expected no ads, got %+v", segs)
	}
	if want := int32(1 + EmptyResultConfirmations); atomic.LoadInt32(calls) != want {
		t.Errorf("expected %d calls before believing empty, got %d", want, atomic.LoadInt32(calls))
	}
}

func TestDetectAdsDoesNotRetryWhenAdsFound(t *testing.T) {
	// A non-empty answer is taken at face value: no extra calls, no cost.
	srv, calls := adStubServer(t, `[{"start":1,"end":2,"reason":"ad"}]`)
	profile := types.LLMProfile{Name: "stub", Type: "openrouter", URL: srv.URL, Model: "stub"}

	segs, err := DetectAdsLLMTimeout("transcript", profile, "key", 5*time.Second)
	if err != nil || len(segs) != 1 {
		t.Fatalf("expected one segment, got %+v (err=%v)", segs, err)
	}
	if got := atomic.LoadInt32(calls); got != 1 {
		t.Errorf("expected a single call, got %d", got)
	}
}

func TestDetectAdsFailsWhenConfirmationErrors(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"choices":[{"message":{"role":"assistant","content":"[]"}}]}`)
			return
		}
		http.Error(w, "server unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	profile := types.LLMProfile{Name: "stub", Type: "openrouter", URL: srv.URL, Model: "stub"}
	_, err := DetectAdsLLMTimeout("transcript", profile, "key", 5*time.Second)
	if err == nil {
		t.Fatalf("expected error when confirmation fails, got nil")
	}
}
