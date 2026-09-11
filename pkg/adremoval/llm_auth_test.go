package adremoval

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"abs/pkg/detect"
)

func TestAdDetectionUsesAuthFolderCredentials(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	auth := filepath.Join(home, ".config", "auth")
	if err := os.MkdirAll(auth, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(auth, "openrouter_api_key"), []byte("test-auth-folder-key\n"), 0600); err != nil {
		t.Fatal(err)
	}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Authorization") != "Bearer test-auth-folder-key" {
			t.Error("normal ad-detection request omitted auth-folder credentials")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"[]"}}]}`))
	}))
	defer server.Close()
	enabled := true
	cfg := Config{ActiveProfileID: 3, Profiles: []LLMProfile{{ID: 3, Name: "OpenRouter Test", Type: "openrouter", URL: server.URL, Model: "test-model"}}}
	cfg.OpenRouterAPIKeyEnabled = &enabled
	for _, query := range []string{"", "3", "OpenRouter Test", "test-model"} {
		profile := selectProfile(cfg, query)
		if _, err := detectAdsLLM("[0s -> 10s] A short discussion.", profile); err != nil {
			t.Fatalf("query %q: %v", query, err)
		}
		if cfg.Profiles[0].APIKey != "" {
			t.Fatal("runtime credential was written into configuration")
		}
	}
	// The stub always answers "[]", so each query also pays for the
	// empty-answer confirmations before that verdict is believed.
	if want := 4 * (1 + detect.EmptyResultConfirmations); requests != want {
		t.Fatalf("requests=%d, want %d", requests, want)
	}
	enabled = false
	if profile := selectProfile(cfg, "3"); profile.APIKey != "" {
		t.Fatal("disabled credentials were resolved")
	}
}
