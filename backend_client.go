package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sariel/abs/pkg/backend"
)

const (
	greenCheck = "\u2713"
	yellowQ    = "?"
)

func checkmark(ok bool) string {
	if ok {
		return greenCheck
	}
	return yellowQ
}

func stripHTML(s string) string {
	return backend.StripHTML(s)
}

func getBackend(cfg Config, quiet bool) (backend.Backend, error) {
	backendName := cfg.BackendType
	if backendName == "" {
		if cfg.PodfetchURL != "" || cfg.PodfetchDBPath != "" {
			if cfg.AudiobookshelfURL == "" && cfg.AudiobookshelfDBPath == "" {
				backendName = "podfetch"
			}
		}
	}
	if backendName == "" {
		backendName = "audiobookshelf"
	}
	switch strings.ToLower(backendName) {
	case "podfetch", "pod_fetch":
		bCfg := backend.Config{
			Host:        cfg.PodfetchURL,
			User:        cfg.PodfetchUser,
			Pass:        cfg.PodfetchPass,
			Token:       cfg.PodfetchAPIKey,
			APIKey:      cfg.PodfetchAPIKey,
			DBPath:      cfg.PodfetchDBPath,
			PodcastsDir: cfg.PodcastsDir,
			Quiet:       quiet,
		}
		return backend.New("podfetch", bCfg)
	default:
		bCfg := backend.Config{
			Host:        cfg.AudiobookshelfURL,
			User:        cfg.AudiobookshelfUser,
			Pass:        cfg.AudiobookshelfPass,
			Token:       cfg.AudiobookshelfToken,
			DBPath:      cfg.AudiobookshelfDBPath,
			PodcastsDir: cfg.PodcastsDir,
			Quiet:       quiet,
		}
		return backend.New("audiobookshelf", bCfg)
	}
}

func getABSClient(cfg Config, quiet bool) (*backend.AudiobookshelfBackend, error) {
	token := cfg.AudiobookshelfToken
	if token == "" {
		token = backend.GetTokenFromDB(cfg.AudiobookshelfDBPath)
	}
	if token == "" && cfg.AudiobookshelfURL != "" && cfg.AudiobookshelfUser != "" && cfg.AudiobookshelfPass != "" {
		token, _ = absLogin(cfg)
	}
	if token == "" && cfg.AudiobookshelfURL == "" {
		return nil, fmt.Errorf("Audiobookshelf API token not configured and could not be retrieved from DB or login")
	}
	client := backend.NewAudiobookshelf(backend.Config{
		Host:        cfg.AudiobookshelfURL,
		User:        cfg.AudiobookshelfUser,
		Pass:        cfg.AudiobookshelfPass,
		Token:       token,
		DBPath:      cfg.AudiobookshelfDBPath,
		PodcastsDir: cfg.PodcastsDir,
		Quiet:       quiet,
	})
	return client, nil
}

func NewABSClient(host, token string) *backend.AudiobookshelfBackend {
	return backend.NewAudiobookshelf(backend.Config{
		Host:  host,
		Token: token,
	})
}

func testAudiobookshelfServer(cfg Config, quiet bool) bool {
	b, err := getBackend(cfg, quiet)
	if err != nil {
		return false
	}
	ok, _ := b.TestConnection(quiet)
	return ok
}

func absLogin(cfg Config) (string, error) {
	b, err := getABSClient(cfg, true)
	if err != nil {
		return "", err
	}
	return b.Login()
}

func absGet(baseURL, token, endpoint string, v interface{}) error {
	client := backend.NewAudiobookshelf(backend.Config{
		Host:  baseURL,
		Token: token,
	})
	data, err := client.Request(endpoint, "GET", nil)
	if err != nil {
		return err
	}
	return unmarshalJSON(data, v)
}

func absScanPodcasts(cfg Config, quiet bool) {
	b, err := getBackend(cfg, quiet)
	if err != nil {
		return
	}
	_, _ = b.Scan(backend.ScanOptions{
		PodcastsDir: cfg.PodcastsDir,
		Quiet:       quiet,
	})
}

func syncAudiobookshelfDuration(cfg *Config, filePath string, duration float64) {
	if cfg == nil {
		return
	}
	if cfg.AudiobookshelfURL == "" && cfg.AudiobookshelfDBPath == "" && cfg.PodfetchURL == "" && cfg.PodfetchDBPath == "" {
		return
	}
	b, err := getBackend(*cfg, true)
	if err != nil {
		return
	}
	_ = b.SyncDuration(filePath, duration)
}

func resetPodcastDateCheck(client *backend.AudiobookshelfBackend, dbPath, itemID, title string) error {
	if client != nil {
		return client.ResetPodcastDateCheck(itemID, title)
	}
	if dbPath != "" {
		return backend.ResetPodcastDateCheckInDB(dbPath, itemID, title)
	}
	return nil
}

func resetPodcastDateCheckInDB(dbPath, itemID, title string) error {
	return backend.ResetPodcastDateCheckInDB(dbPath, itemID, title)
}

func buildOPMLXML(feeds []backend.OPMLFeed) ([]byte, error) {
	return backend.BuildOPMLXML(feeds)
}

func parseOPMLXML(data []byte) ([]backend.OPMLFeed, error) {
	return backend.ParseOPMLXML(data)
}

func exportOPML(config Config, opmlFile string, quiet, verbose bool) {
	if opmlFile == "" {
		showOPMLExportUsage()
		fatalError("%s\n", "Error: missing required <file> argument for 'abs opml export <file>'.")
	}
	b, err := getBackend(config, quiet)
	if err != nil {
		fatalError("%s\n", fmt.Sprintf("Error: %v", err))
	}
	data, err := b.ExportOPML(backend.OPMLExportOptions{Quiet: quiet, Verbose: verbose})
	if err != nil {
		fatalError("%s\n", fmt.Sprintf("Error generating OPML: %v", err))
	}
	if dir := filepath.Dir(opmlFile); dir != "." && dir != "" {
		_ = os.MkdirAll(dir, 0755)
	}
	if err := os.WriteFile(opmlFile, data, 0644); err != nil {
		fatalError("%s\n", fmt.Sprintf("Error writing OPML file: %v", err))
	}
	if !quiet {
		fmt.Printf("Successfully exported podcast RSS feed(s) to: %s\n", opmlFile)
	}
}

func importOPML(config Config, opmlFile string, quiet, verbose bool) {
	if opmlFile == "" {
		showOPMLImportUsage()
		fatalError("%s\n", "Error: missing required <file> argument for 'abs opml import <file>'.")
	}
	data, err := os.ReadFile(opmlFile)
	if err != nil {
		fatalError("%s\n", fmt.Sprintf("Error reading OPML file '%s': %v", opmlFile, err))
	}
	b, err := getBackend(config, quiet)
	if err != nil {
		fatalError("%s\n", fmt.Sprintf("Error connecting to backend: %v", err))
	}
	res, err := b.ImportOPML(data, backend.OPMLImportOptions{Quiet: quiet, Verbose: verbose})
	if err != nil {
		fatalError("%s\n", fmt.Sprintf("Error importing OPML: %v", err))
	}
	if !quiet {
		if res.SkippedSelfFeeds > 0 {
			fmt.Printf("\nOPML Import Summary: %d newly subscribed, %d already existed, %d Audiobookshelf self-feed(s) skipped (%d total in OPML).\n",
				res.Subscribed, res.AlreadyExisted, res.SkippedSelfFeeds, res.TotalFeeds)
		} else {
			fmt.Printf("\nOPML Import Summary: %d newly subscribed, %d already existed (%d total in OPML).\n",
				res.Subscribed, res.AlreadyExisted, res.TotalFeeds)
		}
	}
}

func fetchPodcastFeeds(client *backend.AudiobookshelfBackend, baseURL string, silent, verbose bool) ([]backend.OPMLFeed, error) {
	if client == nil {
		return nil, fmt.Errorf("backend client is nil")
	}
	return client.FetchPodcastFeeds(silent, verbose)
}

func rescanPodcastEpisodes(client *backend.AudiobookshelfBackend, item backend.Podcast, dryRun bool, db *sql.DB, podcastsDir string, verbose bool, quiet bool) (int, int) {
	if client == nil {
		return 0, 0
	}
	return client.RescanPodcastEpisodes(item, dryRun, db, podcastsDir, verbose, quiet)
}

func getMP3DiskDurationNative(path string) float64 {
	return backend.GetMP3DiskDurationNative(path)
}

func getMP3DiskDuration(path string) float64 {
	return backend.GetMP3DiskDuration(path)
}

func updateEpisodeInTx(tx *sql.Tx, episodeID string, diskDuration float64, hostPath string) bool {
	return backend.UpdateEpisodeInTx(tx, episodeID, diskDuration, hostPath)
}

func applyKeepPolicy(client *backend.AudiobookshelfBackend, itemID, podcastTitle string, keep int, dryRun bool, verbose bool, quiet bool) bool {
	if client == nil {
		return false
	}
	_, err := client.ApplyKeepPolicy(itemID, podcastTitle, keep, dryRun, verbose, quiet)
	return err == nil
}

func waitForActiveDownloads(client *backend.AudiobookshelfBackend, podcasts []backend.Podcast, quiet bool) {
	if client == nil {
		return
	}
	_ = client.WaitForActiveDownloads(podcasts, quiet, 300*time.Second)
}

func analyzePodcastFrequency(episodes []backend.FeedEpisode) backend.PodcastFrequencyInfo {
	return backend.AnalyzePodcastFrequency(episodes)
}

func unmarshalJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
