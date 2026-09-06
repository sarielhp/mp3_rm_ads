package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

type PodcastPolicyResult struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	AutoDownload    bool   `json:"auto_download"`
	DownloadPolicy  string `json:"download_policy"`
	DownloadK       int    `json:"download_k"`
	AutoCleanup     bool   `json:"auto_cleanup"`
	AutoCleanupDays int    `json:"auto_cleanup_days"`
	AdRemoval       string `json:"ad_removal"`
	BackendSync     string `json:"backend_sync"`
}

func runPolicyCommand(cfg Config, cli CLIOptions) error {
	podcastsDir := cfg.PodcastsDir
	if podcastsDir == "" {
		podcastsDir = "."
	}

	if len(cli.Args) == 0 {
		return fmt.Errorf("missing podcast identifier for policy command")
	}

	target := cli.Args[0]
	resolved, err := resolveAnyID(podcastsDir, target)
	if err != nil {
		return err
	}

	if !resolved.IsPodcast() {
		return fmt.Errorf("identifier %q resolved to an episode, expected a podcast", target)
	}

	pod := resolved.Podcast
	hasUpdates := checkHasPolicyUpdates(cli)

	if !hasUpdates {
		return displayPodcastPolicy(pod, cli)
	}

	return updatePodcastPolicy(pod, cli)
}

func checkHasPolicyUpdates(cli CLIOptions) bool {
	return cli.AutoDownloadStr != "" ||
		cli.DownloadPolicy != "" ||
		cli.DownloadK > 0 ||
		cli.AutoCleanupStr != "" ||
		cli.CleanupDays > 0 ||
		cli.AdRemovalMode != ""
}

func displayPodcastPolicy(pod *ResolvedPodcast, cli CLIOptions) error {
	cfgGlobal := loadConfig()
	syncStatus := getBackendSyncInfo(pod, cfgGlobal)

	res := PodcastPolicyResult{
		ID:              pod.ShortID,
		Title:           pod.Title,
		AutoDownload:    pod.Config.IsAutoDownloadEnabled(),
		DownloadPolicy:  pod.Config.DownloadPolicy,
		DownloadK:       pod.Config.DownloadK,
		AutoCleanup:     pod.Config.IsAutoCleanupEnabled(),
		AutoCleanupDays: pod.Config.AutoCleanupDays,
		AdRemoval:       pod.Config.AdRemoval,
		BackendSync:     syncStatus,
	}

	if cli.JSON {
		data, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	printPodcastPolicyDetails(res)
	return nil
}

func printPodcastPolicyDetails(res PodcastPolicyResult) {
	fmt.Printf("\nPolicy for %s [%s]:\n", bold(res.Title), boldCyan(res.ID))
	fmt.Printf("%s\n", strings.Repeat("=", 65))
	dlBadge := downloadPolicyBadge(res.DownloadPolicy, res.DownloadK)
	fmt.Printf("  Auto Download:    %-5v %s\n", res.AutoDownload, dlBadge)
	retStr := "Disabled"
	if res.AutoCleanupDays > 0 {
		retStr = fmt.Sprintf("%d days retention", res.AutoCleanupDays)
	}
	fmt.Printf("  Auto Cleanup:     %-5v (%s)\n", res.AutoCleanup, retStr)
	adBadge := adRemovalModeBadge(res.AdRemoval)
	fmt.Printf("  Ad Removal:       %-8s %s\n", res.AdRemoval, adBadge)
	fmt.Printf("  Backend Sync:     %s\n", res.BackendSync)
	fmt.Printf("%s\n\n", strings.Repeat("=", 65))
}

func updatePodcastPolicy(pod *ResolvedPodcast, cli CLIOptions) error {
	applyPolicyOptionChanges(&pod.Config, cli)

	if err := savePodcastConfig(pod.Dir, pod.Config); err != nil {
		return fmt.Errorf("failed to save podcast config: %w", err)
	}

	autoDl := pod.Config.IsAutoDownloadEnabled()
	autoCl := pod.Config.IsAutoCleanupEnabled()
	syncMsg := syncPolicyWithBackend(pod, autoDl, autoCl, pod.Config.AutoCleanupDays)

	res := PodcastPolicyResult{
		ID:              pod.ShortID,
		Title:           pod.Title,
		AutoDownload:    autoDl,
		DownloadPolicy:  pod.Config.DownloadPolicy,
		DownloadK:       pod.Config.DownloadK,
		AutoCleanup:     autoCl,
		AutoCleanupDays: pod.Config.AutoCleanupDays,
		AdRemoval:       pod.Config.AdRemoval,
		BackendSync:     syncMsg,
	}

	if cli.JSON {
		data, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("Policy updated for %s [%s]: DL=%v (%s), Cleanup=%v (%dd), Ads=%s (%s)\n",
		bold(pod.Title), boldCyan(pod.ShortID), autoDl, pod.Config.DownloadPolicy, autoCl, pod.Config.AutoCleanupDays, pod.Config.AdRemoval, syncMsg)
	return nil
}

func applyPolicyOptionChanges(cfg *PodcastConfig, cli CLIOptions) {
	if cli.AutoDownloadStr != "" {
		cfg.SetAutoDownload(parseBoolString(cli.AutoDownloadStr))
	}
	if cli.DownloadPolicy != "" {
		cfg.DownloadPolicy = normalizeDownloadPolicy(cli.DownloadPolicy)
	}
	if cli.DownloadK > 0 {
		cfg.DownloadK = cli.DownloadK
	}
	if cli.AutoCleanupStr != "" {
		cfg.SetAutoCleanup(parseBoolString(cli.AutoCleanupStr))
	}
	if cli.CleanupDays > 0 {
		cfg.AutoCleanupDays = cli.CleanupDays
		autoCl := true
		cfg.AutoCleanup = &autoCl
	}
	if cli.AdRemovalMode != "" {
		cfg.AdRemoval = normalizeAdRemovalMode(cli.AdRemovalMode)
	}
}

func parseBoolString(s string) bool {
	v := strings.ToLower(strings.TrimSpace(s))
	if v == "true" || v == "1" || v == "yes" || v == "on" || v == "enable" || v == "enabled" {
		return true
	}
	return false
}

func getBackendSyncInfo(pod *ResolvedPodcast, cfg Config) string {
	b, err := getBackend(cfg, true)
	if err != nil || b == nil {
		return "Backend not connected"
	}
	return fmt.Sprintf("Connected to %s", b.Name())
}

func syncPolicyWithBackend(pod *ResolvedPodcast, autoDownload, autoCleanup bool, autoCleanupDays int) string {
	cfg := loadConfig()
	b, err := getBackend(cfg, true)
	if err != nil || b == nil {
		return "Local only (no backend)"
	}

	targetID := pod.UUID
	if targetID == "" {
		targetID = pod.ShortID
	}
	if targetID == "" {
		targetID = filepath.Base(pod.Dir)
	}

	err = b.UpdatePodcastSettings(targetID, autoDownload, autoCleanup, autoCleanupDays)
	if err != nil {
		return fmt.Sprintf("Sync error: %v", err)
	}
	return fmt.Sprintf("Synced to %s", b.Name())
}
