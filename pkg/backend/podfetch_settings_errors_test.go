package backend

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPodFetchSettingsErrorsAreReturned(t *testing.T) {
	for _, response := range []string{"", "invalid", "null", "{}"} {
		t.Run(response, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "PUT" {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				_, _ = w.Write([]byte(response))
			}))
			defer server.Close()
			b := NewPodFetch(Config{Host: server.URL, MaxAttempts: 1})
			if err := b.UpdatePodcastSettings("123", false, false, 0); err == nil {
				t.Fatal("settings failure was silently accepted")
			}
		})
	}
}
