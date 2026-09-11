package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLLMProfileProbe(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		status        int
		wantError     bool
	}{
		{"ads", `[{"start":10,"end":20,"reason":"sponsor"}]`, 200, false},
		{"empty array", `[]`, 200, false},
		{"invalid JSON", `not JSON`, 200, true},
		{"invalid timestamps", `[{"start":20,"end":10}]`, 200, true},
		{"bad key", `Unauthorized`, 401, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.Method != "POST" || r.Header.Get("Authorization") != "Bearer test-key" {
					t.Error("incorrect API method or authentication")
				}
				var request struct {
					Model    string `json:"model"`
					Messages []struct {
						Content string `json:"content"`
					} `json:"messages"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
				}
				if request.Model != "test-model" || len(request.Messages) != 2 || !strings.Contains(request.Messages[1].Content, "sponsored") {
					t.Errorf("not an ad-detection request: %+v", request)
				}
				w.WriteHeader(tc.status)
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": tc.content}}}})
			}))
			defer server.Close()
			profile := LLMProfile{ID: 3, URL: server.URL, Model: "test-model", APIKey: "test-key"}
			if err := probeLLMProfile(Config{}, profile); (err != nil) != tc.wantError {
				t.Fatalf("error=%v, wantError=%v", err, tc.wantError)
			}
			if requests != 1 {
				t.Fatalf("requests=%d", requests)
			}
		})
	}
}

func TestLLMTestRejectsInvalidConfiguration(t *testing.T) {
	for _, id := range []string{"bad", "0", "-1", "3"} {
		if err := testLLMProfile(Config{}, id); err == nil {
			t.Fatalf("accepted missing/invalid profile %q", id)
		}
	}
	if err := probeLLMProfile(Config{}, LLMProfile{Model: "test"}); err == nil {
		t.Fatal("empty URL must not pass without an API call")
	}
	disabled := false
	cfg := Config{}
	cfg.OpenRouterAPIKeyEnabled = &disabled
	if err := probeLLMProfile(cfg, LLMProfile{Type: "openrouter", URL: "http://127.0.0.1:1", Model: "test"}); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("disabled API should fail before network request: %v", err)
	}
}
