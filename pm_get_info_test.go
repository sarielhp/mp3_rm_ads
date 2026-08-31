package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCLIServerGetInfoParsing(t *testing.T) {
	var action string
	var opts CLIOptions
	app := buildCLIApp(&action, &opts)

	if err := app.Execute([]string{"server", "get_info"}); err != nil {
		t.Fatalf("failed to parse 'server get_info': %v", err)
	}
	if action != "server" || opts.ServerSubcmd != "get_info" || opts.Count != 100 {
		t.Errorf("expected action=server, subcmd=get_info, count=100, got action=%s, subcmd=%s, count=%d",
			action, opts.ServerSubcmd, opts.Count)
	}

	action = ""
	opts = CLIOptions{}
	app = buildCLIApp(&action, &opts)
	if err := app.Execute([]string{"server", "get_info", "50", "-p", "Dan Snow"}); err != nil {
		t.Fatalf("failed to parse 'server get_info 50': %v", err)
	}
	if action != "server" || opts.ServerSubcmd != "get_info" || opts.Count != 50 || opts.Podcast != "Dan Snow" {
		t.Errorf("expected action=server, subcmd=get_info, count=50, podcast='Dan Snow', got action=%s, subcmd=%s, count=%d, podcast=%s",
			action, opts.ServerSubcmd, opts.Count, opts.Podcast)
	}
}

func TestServerGetInfoExecution(t *testing.T) {
	tempDir := t.TempDir()
	podDir := filepath.Join(tempDir, "Test_Show")
	_ = os.MkdirAll(podDir, 0755)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/libraries":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"libraries": []map[string]interface{}{
					{"id": "lib-1", "mediaType": "podcast"},
				},
			})
		case r.URL.Path == "/api/libraries/lib-1/items":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"results": []map[string]interface{}{
					{
						"id": "item-1",
						"media": map[string]interface{}{
							"metadata": map[string]interface{}{
								"title":   "Test Show",
								"feedUrl": "https://example.com/feed.xml",
							},
						},
					},
				},
			})
		case r.URL.Path == "/api/items/item-1":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "item-1",
				"media": map[string]interface{}{
					"metadata": map[string]interface{}{
						"title":   "Test Show",
						"feedUrl": "https://example.com/feed.xml",
					},
				},
			})
		case r.URL.Path == "/api/podcasts/feed":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"podcast": map[string]interface{}{
					"episodes": []map[string]interface{}{
						{
							"title":       "Episode 100",
							"pubDate":     "Mon, 31 Aug 2026 12:00:00 GMT",
							"description": "Latest episode description",
							"guid":        "guid-100",
						},
						{
							"title":       "Episode 99",
							"pubDate":     "Sun, 30 Aug 2026 12:00:00 GMT",
							"description": "Previous episode description",
							"guid":        "guid-99",
						},
					},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cfg := Config{
		AudiobookshelfURL:   server.URL,
		AudiobookshelfToken: "mock-token",
		PodcastsDir:         tempDir,
	}

	cli := CLIOptions{
		ServerSubcmd: "get_info",
		Count:        50,
		Quiet:        true,
	}

	handleServerGetInfo(cfg, cli)

	cachedEps := globalFeedCache.Get("https://example.com/feed.xml")
	if cachedEps == nil || len(cachedEps.Episodes) != 2 {
		t.Fatalf("expected 2 episodes cached in globalFeedCache, got: %+v", cachedEps)
	}
	if cachedEps.Episodes[0].Title != "Episode 100" {
		t.Errorf("expected newest episode 'Episode 100', got: %s", cachedEps.Episodes[0].Title)
	}
}
