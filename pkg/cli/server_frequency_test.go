package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"pod/pkg/backend"
	configPkg "pod/pkg/config"
	"pod/pkg/podcast"
)

func TestHandleServerFrequency_LocalDirectory(t *testing.T) {
	rootTmp := t.TempDir()

	hourlyDir := filepath.Join(rootTmp, "Hourly Podcast")
	_ = os.MkdirAll(hourlyDir, 0755)
	_ = configPkg.SavePodcastConfig(hourlyDir, configPkg.PodcastConfig{
		AdRemoval:      configPkg.AdRemovalAll,
		DownloadPolicy: configPkg.DownloadPolicyLatest,
		DownloadK:      3,
	})

	var hourlyEpisodes []podcast.CachedEpisodeSummary
	baseH := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 15; i++ {
		hourlyEpisodes = append(hourlyEpisodes, podcast.CachedEpisodeSummary{
			EpisodeFile: podcast.EpisodeFile{
				Title:       fmt.Sprintf("H %d", i),
				PublishedAt: baseH.Add(time.Duration(i) * time.Hour).UnixMilli(),
			},
		})
	}
	_ = podcast.SavePodcastCache(hourlyDir, &podcast.CachedPodcastIndex{
		PodcastName: "Hourly Podcast",
		PodcastDir:  hourlyDir,
		Episodes:    hourlyEpisodes,
	})

	dailyDir := filepath.Join(rootTmp, "Daily Podcast")
	_ = os.MkdirAll(dailyDir, 0755)
	_ = configPkg.SavePodcastConfig(dailyDir, configPkg.PodcastConfig{
		AdRemoval:      configPkg.AdRemovalLatest,
		DownloadPolicy: configPkg.DownloadPolicyLatestK,
		DownloadK:      5,
	})

	var dailyEpisodes []podcast.CachedEpisodeSummary
	baseD := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 15; i++ {
		dailyEpisodes = append(dailyEpisodes, podcast.CachedEpisodeSummary{
			EpisodeFile: podcast.EpisodeFile{
				Title:       fmt.Sprintf("D %d", i),
				PublishedAt: baseD.Add(time.Duration(i) * 24 * time.Hour).UnixMilli(),
			},
		})
	}
	_ = podcast.SavePodcastCache(dailyDir, &podcast.CachedPodcastIndex{
		PodcastName: "Daily Podcast",
		PodcastDir:  dailyDir,
		Episodes:    dailyEpisodes,
	})

	config := Config{PodcastsDir: rootTmp}
	var cli CLIOptions
	cli.ServerSubcmd = "disable-hourly"
	cli.DisableHourly = true
	cli.Quiet = true

	if err := handleServerFrequency(config, cli); err != nil {
		t.Fatalf("handleServerFrequency failed: %v", err)
	}

	hourlyCfg := configPkg.LoadPodcastConfig(hourlyDir, configPkg.DefaultPodcastConfig(nil))
	if hourlyCfg.Frequency == nil {
		t.Fatal("expected hourly frequency to be set")
	}
	if hourlyCfg.Frequency.Type != string(backend.CadenceHourly) {
		t.Errorf("expected cadence hourly, got %s", hourlyCfg.Frequency.Type)
	}
	if hourlyCfg.DownloadPolicy != configPkg.DownloadPolicyNone {
		t.Errorf("expected download_policy none, got %s", hourlyCfg.DownloadPolicy)
	}
	if hourlyCfg.AdRemoval != configPkg.AdRemovalNone {
		t.Errorf("expected ad_removal none, got %s", hourlyCfg.AdRemoval)
	}

	dailyCfg := configPkg.LoadPodcastConfig(dailyDir, configPkg.DefaultPodcastConfig(nil))
	if dailyCfg.Frequency == nil {
		t.Fatal("expected daily frequency to be set")
	}
	if dailyCfg.Frequency.Type != string(backend.CadenceDaily) {
		t.Errorf("expected cadence daily, got %s", dailyCfg.Frequency.Type)
	}
	if dailyCfg.DownloadPolicy != configPkg.DownloadPolicyLatestK {
		t.Errorf("expected download_policy latest_k unchanged, got %s", dailyCfg.DownloadPolicy)
	}
	if dailyCfg.AdRemoval != configPkg.AdRemovalLatest {
		t.Errorf("expected ad_removal latest unchanged, got %s", dailyCfg.AdRemoval)
	}
}

func TestHandleServerDisableHourly(t *testing.T) {
	rootTmp := t.TempDir()

	hourlyDir := filepath.Join(rootTmp, "News Hourly")
	_ = os.MkdirAll(hourlyDir, 0755)
	_ = configPkg.SavePodcastConfig(hourlyDir, configPkg.PodcastConfig{
		AdRemoval:      configPkg.AdRemovalAll,
		DownloadPolicy: configPkg.DownloadPolicyLatest,
	})

	var hourlyEpisodes []podcast.CachedEpisodeSummary
	baseH := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 12; i++ {
		hourlyEpisodes = append(hourlyEpisodes, podcast.CachedEpisodeSummary{
			EpisodeFile: podcast.EpisodeFile{
				Title:       fmt.Sprintf("News %d", i),
				PublishedAt: baseH.Add(time.Duration(i) * time.Hour).UnixMilli(),
			},
		})
	}
	_ = podcast.SavePodcastCache(hourlyDir, &podcast.CachedPodcastIndex{
		PodcastName: "News Hourly",
		PodcastDir:  hourlyDir,
		Episodes:    hourlyEpisodes,
	})

	config := Config{PodcastsDir: rootTmp}
	var cli CLIOptions
	cli.Quiet = true

	if err := handleServerDisableHourly(config, cli); err != nil {
		t.Fatalf("handleServerDisableHourly failed: %v", err)
	}

	cfg := configPkg.LoadPodcastConfig(hourlyDir, configPkg.DefaultPodcastConfig(nil))
	if cfg.DownloadPolicy != configPkg.DownloadPolicyNone || cfg.AdRemoval != configPkg.AdRemovalNone {
		t.Errorf("expected policy to be disabled, got dl=%s ad=%s", cfg.DownloadPolicy, cfg.AdRemoval)
	}
}

// This one still captures os.Stderr, and stays sequential because of it.
// Execute goes through clihelp, which writes its own diagnostics straight to
// os.Stderr; there is no writer to inject short of changing that dependency.
func TestExecuteBlockUnknownCommand(t *testing.T) {
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	code := Execute([]string{"block", "hourly"})

	_ = w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	out := buf.String()

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(out, `unknown command "block" for "pod"`) {
		t.Errorf("expected stderr to report unknown command, got: %q", out)
	}
}
