package cli

import (
	"abs/pkg/backend"
	"abs/pkg/config"
	"abs/pkg/podcast"
	"abs/pkg/util"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sarielhp/clihelp"
)

type PodcastPolicyResult struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	Favorite        bool   `json:"favorite"`
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

	if len(cli.Args) == 0 && !cli.PolicyAll {
		return fmt.Errorf("missing podcast identifier for policy command")
	}

	if err := parseShorthandNumberPolicy(&cli); err != nil {
		return err
	}

	target := ""
	if len(cli.Args) > 0 {
		target = cli.Args[0]
	}

	if strings.EqualFold(target, "default") {
		return handleDefaultPolicy(cli)
	}

	if strings.EqualFold(target, "all") || cli.PolicyAll {
		return handleAllPodcastsPolicy(cfg, podcastsDir, cli)
	}

	return handleSinglePodcastPolicy(cfg, podcastsDir, target, cli)
}

func handleDefaultPolicy(cli CLIOptions) error {
	globalCfg := loadConfig()
	applyDefaultPolicyChanges(&globalCfg, cli)
	if err := config.SaveConfig(&globalCfg); err != nil {
		return fmt.Errorf("failed to save global configuration: %w", err)
	}
	if cli.JSON {
		res := map[string]any{
			"default_download_policy": globalCfg.DefaultDownloadPolicy,
			"default_download_k":      globalCfg.DefaultDownloadK,
			"default_ad_removal":      globalCfg.DefaultAdRemoval,
		}
		data, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(data))
		return nil
	}
	fmt.Printf("Global default policy updated: download_policy=%s, download_k=%d, ad_removal=%s\n",
		globalCfg.DefaultDownloadPolicy, globalCfg.DefaultDownloadK, globalCfg.DefaultAdRemoval)
	return nil
}

func applyDefaultPolicyChanges(cfg *Config, cli CLIOptions) {
	if cli.AutoDownloadStr != "" {
		if parseBoolString(cli.AutoDownloadStr) {
			if cfg.DefaultDownloadPolicy == "" || cfg.DefaultDownloadPolicy == config.DownloadPolicyNone {
				cfg.DefaultDownloadPolicy = config.DownloadPolicyLatest
			}
		} else {
			cfg.DefaultDownloadPolicy = config.DownloadPolicyNone
		}
	}
	if cli.DownloadPolicy != "" {
		cfg.DefaultDownloadPolicy = config.NormalizeDownloadPolicy(cli.DownloadPolicy)
	}
	if cli.DownloadK > 0 {
		cfg.DefaultDownloadK = cli.DownloadK
	}
	if cli.AdRemovalMode != "" {
		cfg.DefaultAdRemoval = config.NormalizeAdRemovalMode(cli.AdRemovalMode)
	}
}

func handleAllPodcastsPolicy(cfg Config, podcastsDir string, cli CLIOptions) error {
	entries := podcast.ScanPodcastDirs(podcastsDir)
	if len(entries) == 0 {
		return fmt.Errorf("no podcasts found in %s", podcastsDir)
	}

	hasUpdates := checkHasPolicyUpdates(cli)
	if !hasUpdates {
		return displayAllPodcastsPolicy(entries, cli)
	}

	return updateAllPodcastsPolicy(cfg, entries, cli)
}

func updateAllPodcastsPolicy(cfg Config, entries []podcast.PodcastDirEntry, cli CLIOptions) error {
	updated := 0
	var sampleCfg config.PodcastConfig
	for _, entry := range entries {
		pCfg := config.LoadPodcastConfig(entry.Dir, config.DefaultPodcastConfig(&cfg))
		applyPolicyOptionChanges(&pCfg, cli)
		if err := config.SavePodcastConfig(entry.Dir, pCfg); err != nil {
			return fmt.Errorf("failed to save policy for %s: %w", entry.Title, err)
		}
		sampleCfg = pCfg
		updated++
	}

	defaultMsg := ""
	if cli.SetDefaultPolicy {
		globalCfg := loadConfig()
		applyDefaultPolicyChanges(&globalCfg, cli)
		if err := config.SaveConfig(&globalCfg); err != nil {
			return fmt.Errorf("failed to save global default configuration: %w", err)
		}
		defaultMsg = fmt.Sprintf(" (global default updated: %s)", globalCfg.DefaultDownloadPolicy)
	}

	if cli.JSON {
		res := map[string]any{
			"updated_count":   updated,
			"auto_download":   sampleCfg.IsAutoDownloadEnabled(),
			"download_policy": sampleCfg.DownloadPolicy,
			"ad_removal":      sampleCfg.AdRemoval,
			"set_default":     cli.SetDefaultPolicy,
		}
		data, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	dlBadge := config.DownloadPolicyBadge(sampleCfg.DownloadPolicy, sampleCfg.DownloadK)
	adBadge := config.AdRemovalModeBadge(sampleCfg.AdRemoval)
	fmt.Printf("Policy updated for %d podcast(s): AutoDownload=%v %s, AdRemoval=%s %s%s\n",
		updated, sampleCfg.IsAutoDownloadEnabled(), dlBadge, sampleCfg.AdRemoval, adBadge, defaultMsg)
	return nil
}

func displayAllPodcastsPolicy(entries []podcast.PodcastDirEntry, cli CLIOptions) error {
	var results []PodcastPolicyResult
	for _, entry := range entries {
		pCfg := config.LoadPodcastConfig(entry.Dir, config.PodcastConfig{})
		results = append(results, PodcastPolicyResult{
			ID:              entry.ShortID,
			Title:           entry.Title,
			AutoDownload:    pCfg.IsAutoDownloadEnabled(),
			DownloadPolicy:  pCfg.DownloadPolicy,
			DownloadK:       pCfg.DownloadK,
			AutoCleanup:     pCfg.IsAutoCleanupEnabled(),
			AutoCleanupDays: pCfg.AutoCleanupDays,
			AdRemoval:       pCfg.AdRemoval,
		})
	}

	if cli.JSON {
		data, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("\nPolicies for all podcasts (%d total):\n", len(results))
	fmt.Printf("%-8s  %-30s  %-15s  %-12s\n", "ID", "TITLE", "AUTO DOWNLOAD", "AD REMOVAL")
	fmt.Println(strings.Repeat("-", 72))
	for _, r := range results {
		dlBadge := config.DownloadPolicyBadge(r.DownloadPolicy, r.DownloadK)
		title := r.Title
		if len(title) > 30 {
			title = title[:27] + "..."
		}
		fmt.Printf("%-8s  %-30s  %-15s  %-12s\n", r.ID, title, dlBadge, r.AdRemoval)
	}
	fmt.Println()
	return nil
}

func handleSinglePodcastPolicy(cfg Config, podcastsDir, target string, cli CLIOptions) error {
	resolved, err := podcast.ResolveAnyID(podcastsDir, target)
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

	if err := updatePodcastPolicy(pod, cli); err != nil {
		return err
	}

	if cli.SetDefaultPolicy {
		globalCfg := loadConfig()
		applyDefaultPolicyChanges(&globalCfg, cli)
		if err := config.SaveConfig(&globalCfg); err != nil {
			return fmt.Errorf("failed to save global default configuration: %w", err)
		}
		fmt.Printf("Global default policy updated: default_download_policy=%s\n", globalCfg.DefaultDownloadPolicy)
	}

	return nil
}

func parseShorthandNumberPolicy(cli *CLIOptions) error {
	if len(cli.Args) <= 1 {
		return nil
	}
	k, err := strconv.Atoi(cli.Args[1])
	if err != nil || k <= 0 {
		return fmt.Errorf("invalid episode count %q: must be a positive integer", cli.Args[1])
	}
	if cli.AutoDownloadStr == "" {
		cli.AutoDownloadStr = "true"
	}
	if cli.DownloadPolicy == "" {
		cli.DownloadPolicy = DownloadPolicyLatestK
	}
	if cli.DownloadK <= 0 {
		cli.DownloadK = k
	}
	if cli.AdRemovalMode == "" {
		cli.AdRemovalMode = AdRemovalAll
	}
	return nil
}

func checkHasPolicyUpdates(cli CLIOptions) bool {
	return cli.AutoDownloadStr != "" ||
		cli.DownloadPolicy != "" ||
		cli.DownloadK > 0 ||
		cli.AutoCleanupStr != "" ||
		cli.CleanupDays > 0 ||
		cli.AdRemovalMode != "" ||
		cli.FavoriteStr != "" ||
		cli.SetDefaultPolicy
}

func displayPodcastPolicy(pod *ResolvedPodcast, cli CLIOptions) error {
	cfgGlobal := loadConfig()
	syncStatus := getBackendSyncInfo(pod, cfgGlobal)

	res := PodcastPolicyResult{
		ID:              pod.ShortID,
		Title:           pod.Title,
		Favorite:        pod.Config.Favorite,
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
	fmt.Printf("\nPolicy for %s [%s]:\n", util.Bold(res.Title), util.BoldCyan(res.ID))
	fmt.Printf("%s\n", strings.Repeat("=", 65))
	favStr := "No"
	if res.Favorite {
		favStr = util.BoldGreen("⭐ Yes")
	}
	fmt.Printf("  Favorite:         %s\n", favStr)
	dlBadge := config.DownloadPolicyBadge(res.DownloadPolicy, res.DownloadK)
	fmt.Printf("  Auto Download:    %-5v %s\n", res.AutoDownload, dlBadge)
	retStr := "Disabled"
	if res.AutoCleanupDays > 0 {
		retStr = fmt.Sprintf("%d days retention", res.AutoCleanupDays)
	}
	fmt.Printf("  Auto Cleanup:     %-5v (%s)\n", res.AutoCleanup, retStr)
	adBadge := config.AdRemovalModeBadge(res.AdRemoval)
	fmt.Printf("  Ad Removal:       %-8s %s\n", res.AdRemoval, adBadge)
	fmt.Printf("  Backend Sync:     %s\n", res.BackendSync)
	fmt.Printf("%s\n\n", strings.Repeat("=", 65))
}

func updatePodcastPolicy(pod *ResolvedPodcast, cli CLIOptions) error {
	applyPolicyOptionChanges(&pod.Config, cli)

	if err := config.SavePodcastConfig(pod.Dir, pod.Config); err != nil {
		return fmt.Errorf("failed to save podcast config: %w", err)
	}

	autoDl := pod.Config.IsAutoDownloadEnabled()
	autoCl := pod.Config.IsAutoCleanupEnabled()
	syncMsg := syncPolicyWithBackend(pod, autoDl, autoCl, pod.Config.AutoCleanupDays)

	res := PodcastPolicyResult{
		ID:              pod.ShortID,
		Title:           pod.Title,
		Favorite:        pod.Config.Favorite,
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

	favBadge := ""
	if pod.Config.Favorite {
		favBadge = " ⭐ [Favorite]"
	}
	fmt.Printf("Policy updated for %s [%s]%s: DL=%v (%s), Cleanup=%v (%dd), Ads=%s (%s)\n",
		util.Bold(pod.Title), util.BoldCyan(pod.ShortID), favBadge, autoDl, pod.Config.DownloadPolicy, autoCl, pod.Config.AutoCleanupDays, pod.Config.AdRemoval, syncMsg)
	return nil
}

func applyPolicyOptionChanges(cfg *PodcastConfig, cli CLIOptions) {
	if cli.FavoriteStr != "" {
		cfg.SetFavorite(parseBoolString(cli.FavoriteStr))
	}
	if cli.AutoDownloadStr != "" {
		cfg.SetAutoDownload(parseBoolString(cli.AutoDownloadStr))
	}
	if cli.DownloadPolicy != "" {
		cfg.DownloadPolicy = config.NormalizeDownloadPolicy(cli.DownloadPolicy)
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
		cfg.AdRemoval = config.NormalizeAdRemovalMode(cli.AdRemovalMode)
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
	b, err := backend.FromAppConfig(&cfg, true)
	if err != nil || b == nil {
		return "Backend not connected"
	}
	return fmt.Sprintf("Connected to %s", b.Name())
}

func syncPolicyWithBackend(pod *ResolvedPodcast, autoDownload, autoCleanup bool, autoCleanupDays int) string {
	cfg := loadConfig()
	b, err := backend.FromAppConfig(&cfg, true)
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

func buildServerPolicySubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "policy",
		Description: "View or update podcast download and AdR policy",
		UsageLine:   "abs server policy [<podcast-id>|all|default] [<number>] [options]",
		Parameters: []clihelp.Param{
			{Name: "[<podcast-id>|all|default]", Description: "Target podcast identifier, 'all' for all podcasts, or 'default' for global config"},
			{Name: "[<number>]", Description: "Shorthand: auto-download latest K episodes with ad-removal all"},
		},
		Args: clihelp.RangeArgs(0, 2),
		Options: []clihelp.Option{
			clihelp.Bool(&opts.PolicyAll, "--all", false, "Apply policy to all podcasts in library"),
			clihelp.Bool(&opts.SetDefaultPolicy, "--set-default", false, "Also update global default configuration for new podcasts"),
			clihelp.String(&opts.FavoriteStr, "--favorite <bool>", "", "Set as favorite (auto-downloads all new episodes and removes ads)"),
			clihelp.String(&opts.AutoDownloadStr, "--auto-download <bool>", "", "Enable automatic downloads (true/false)"),
			clihelp.String(&opts.DownloadPolicy, "--download-policy <mode>", "", "Policy mode ('none', 'latest', 'latest_k', 'all')"),
			clihelp.Int(&opts.DownloadK, "--download-k <num>", 0, "Number of latest episodes to download"),
			clihelp.String(&opts.AutoCleanupStr, "--auto-cleanup <bool>", "", "Enable automatic cleanup (true/false)"),
			clihelp.Int(&opts.CleanupDays, "--cleanup-days <days>", 0, "Retention window in days"),
			clihelp.String(&opts.AdRemovalMode, "--ad-removal <mode>", "", "Ad removal policy mode ('none', 'latest', 'all')"),
			clihelp.Bool(&opts.JSON, "--json", false, "Output results in JSON format"),
		},
		Examples: []clihelp.Example{
			{
				Line:        "abs server policy 42 1",
				Description: "Shorthand: auto-download latest 1 episode and remove all ads",
			},
			{
				Line:        "abs server policy all --auto-download false",
				Description: "Mark all podcasts as not auto-download",
			},
			{
				Line:        "abs server policy all --auto-download false --set-default",
				Description: "Disable auto-download for all podcasts and set global default",
			},
			{
				Line:        "abs server policy default --download-policy none",
				Description: "Set default download policy for new podcasts to none",
			},
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "server"
			opts.ServerSubcmd = "policy"
			opts.SyncSubcmd = "policy"
			opts.Args = ctx.Args
			return nil
		},
	}
}
