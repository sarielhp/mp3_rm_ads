package backend

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPodFetchLoginAndTestConnection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer my-api-key" || r.Header.Get("x-api-key") == "my-api-key" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{"id": 1, "name": "Test Show"},
			})
			return
		}
		u, p, ok := r.BasicAuth()
		if ok && u == "admin" && p == "secret" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{"id": 1, "name": "Test Show"},
			})
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	beAPIKey := NewPodFetch(Config{
		Host:   srv.URL,
		APIKey: "my-api-key",
	})
	tok, err := beAPIKey.Login()
	if err != nil || tok != "my-api-key" {
		t.Fatalf("Login with API key failed: %v", err)
	}
	ok, err := beAPIKey.TestConnection(true)
	if !ok || err != nil {
		t.Fatalf("TestConnection with API key failed: %v", err)
	}

	beBasic := NewPodFetch(Config{
		Host: srv.URL,
		User: "admin",
		Pass: "secret",
	})
	tokBasic, err := beBasic.Login()
	if err != nil || tokBasic != "admin" {
		t.Fatalf("Login with Basic auth failed: %v", err)
	}
	okBasic, err := beBasic.TestConnection(true)
	if !okBasic || err != nil {
		t.Fatalf("TestConnection with Basic auth failed: %v", err)
	}

	beFail := NewPodFetch(Config{
		Host: srv.URL,
		User: "admin",
		Pass: "wrong",
	})
	okFail, _ := beFail.TestConnection(true)
	if okFail {
		t.Errorf("expected TestConnection to fail with invalid credentials")
	}
}

func setupMockPodFetchServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/podcasts" && r.Method == "GET":
			_ = json.NewEncoder(w).Encode([]podFetchItemDTO{
				{ID: 1, Name: "Tech Show", Directory: "tech_show", RSSFeed: "https://example.com/tech.xml", ImageURL: "https://example.com/tech.jpg", Summary: "Tech summary", Author: "Host 1"},
			})
		case r.URL.Path == "/api/v1/podcasts/1" && r.Method == "GET":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id": 1, "name": "Tech Show", "directory": "tech_show", "rssfeed": "https://example.com/tech.xml", "image_url": "https://example.com/tech.jpg", "summary": "Tech summary", "author": "Host 1",
				"episodes": []podFetchEpisodeDTO{
					{ID: 101, PodcastID: 1, EpisodeID: "ep-101", Name: "Episode 101", URL: "https://example.com/ep101.mp3", DateOfRecording: "2026-08-30", TotalTime: 3600, LocalURL: "ep101.mp3", Description: "Ep 101 description", Status: "D"},
				},
			})
		case r.URL.Path == "/api/v1/podcasts" && r.Method == "POST":
			_ = json.NewEncoder(w).Encode(podFetchItemDTO{ID: 2, Name: "New Show", Directory: "new_show", RSSFeed: "https://example.com/new.xml"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestPodFetchLibrariesAndPodcasts(t *testing.T) {
	srv := setupMockPodFetchServer()
	defer srv.Close()

	be := NewPodFetch(Config{Host: srv.URL, APIKey: "key-123"})

	libs, err := be.PodcastLibraries()
	if err != nil || len(libs) == 0 || libs[0].MediaType != "podcast" {
		t.Fatalf("PodcastLibraries failed: %v, libs: %+v", err, libs)
	}

	podcasts, err := be.Podcasts()
	if err != nil || len(podcasts) != 1 || podcasts[0].ID != "1" || podcasts[0].Media.Metadata.Title != "Tech Show" {
		t.Fatalf("Podcasts failed: %v, list: %+v", err, podcasts)
	}
	if len(podcasts[0].Media.Episodes) != 1 || podcasts[0].Media.Episodes[0].Title != "Episode 101" {
		t.Errorf("unexpected episodes in podcast: %+v", podcasts[0].Media.Episodes)
	}

	pod, err := be.GetPodcast("1")
	if err != nil || pod == nil || pod.Media.Metadata.Title != "Tech Show" {
		t.Fatalf("GetPodcast failed: %v", err)
	}

	created, err := be.CreatePodcast("lib", "folder", "/podcasts/new_show", "New Show", "https://example.com/new.xml")
	if err != nil || created == nil || created.ID != "2" {
		t.Fatalf("CreatePodcast failed: %v", err)
	}
}

func setupMockPodFetchEpisodesServer(downloadInvoked *bool, deletedEpisode *string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/podcasts/feed" && r.Method == "POST":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"episodes": []podFetchEpisodeDTO{
					{EpisodeID: "ep-1", Name: "Feed Ep 1", URL: "https://example.com/f1.mp3", DateOfRecording: "2026-08-30", TotalTime: 1800, Description: "Feed episode description"},
				},
			})
		case strings.HasSuffix(r.URL.Path, "/download"):
			*downloadInvoked = true
			w.WriteHeader(http.StatusOK)
		case strings.HasPrefix(r.URL.Path, "/api/v1/podcasts/1/episodes/"):
			*deletedEpisode = filepath.Base(r.URL.Path)
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/api/v1/podcasts/1/downloads":
			_ = json.NewEncoder(w).Encode([]podFetchEpisodeDTO{
				{ID: 99, Name: "Downloading Ep", EpisodeID: "ep-99", URL: "https://example.com/dl.mp3"},
			})
		case r.URL.Path == "/api/v1/podcasts/1/cover":
			w.Write([]byte("fake-cover-content"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestPodFetchEpisodesAndDownloads(t *testing.T) {
	var downloadInvoked bool
	var deletedEpisode string

	srv := setupMockPodFetchEpisodesServer(&downloadInvoked, &deletedEpisode)
	defer srv.Close()

	be := NewPodFetch(Config{Host: srv.URL, APIKey: "key-123"})

	eps, err := be.PodcastFeedEpisodes("https://example.com/feed.xml")
	if err != nil || len(eps) != 1 || eps[0].Title != "Feed Ep 1" {
		t.Fatalf("PodcastFeedEpisodes failed: %v, eps: %+v", err, eps)
	}

	if err := be.DownloadEpisodes("1", eps); err != nil || !downloadInvoked {
		t.Fatalf("DownloadEpisodes failed: %v, invoked: %v", err, downloadInvoked)
	}

	if err := be.DeletePodcastEpisode("1", "ep-55"); err != nil || deletedEpisode != "ep-55" {
		t.Fatalf("DeletePodcastEpisode failed: %v, deleted: %s", err, deletedEpisode)
	}

	dls, err := be.ActiveDownloads("1")
	if err != nil || len(dls) != 1 || dls[0].EpisodeDisplayTitle != "Downloading Ep" {
		t.Fatalf("ActiveDownloads failed: %v, dls: %+v", err, dls)
	}

	feedURL, err := be.OpenRSSFeed("1", srv.URL)
	if err != nil || !strings.Contains(feedURL, "/rss/1") {
		t.Errorf("OpenRSSFeed unexpected: %s, %v", feedURL, err)
	}

	tempDir := t.TempDir()
	coverPath := filepath.Join(tempDir, "cover.jpg")
	if err := be.DownloadCover("1", coverPath); err != nil {
		t.Fatalf("DownloadCover failed: %v", err)
	}
	data, err := os.ReadFile(coverPath)
	if err != nil || string(data) != "fake-cover-content" {
		t.Errorf("unexpected cover file data: %s", string(data))
	}
}
