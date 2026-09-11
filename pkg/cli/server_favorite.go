package cli

import (
	"abs/pkg/config"
	"abs/pkg/podcast"
	"abs/pkg/util"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sarielhp/clihelp"
)

type FavoritePodcastResult struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Favorite       bool   `json:"favorite"`
	FavoriteSince  string `json:"favorite_since,omitempty"`
	DownloadPolicy string `json:"download_policy"`
	AdRemoval      string `json:"ad_removal"`
	AutoDownload   bool   `json:"auto_download"`
	BackendSync    string `json:"backend_sync,omitempty"`
}

func buildServerFavoriteSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "favorite",
		Description: "Set or list favorite podcasts (auto-downloads all new episodes and removes ads)",
		UsageLine:   "abs server favorite [<podcast-id>] [options]",
		Parameters: []clihelp.Param{
			{Name: "[<podcast-id>]", Description: "Target podcast identifier to mark/unmark as favorite (leave empty to list)"},
		},
		Args: clihelp.MaximumNArgs(2),
		Options: []clihelp.Option{
			clihelp.Bool(&opts.FavoriteOff, "--off", false, "Unmark podcast as favorite"),
			clihelp.Bool(&opts.PolicyAll, "--all", false, "Apply to all podcasts in library"),
			clihelp.Bool(&opts.JSON, "--json", false, "Output results in JSON format"),
		},
		Examples: []clihelp.Example{
			{
				Line:        "abs server favorite 'Huberman Lab'",
				Description: "Mark podcast as favorite (auto-downloads new episodes with ad removal)",
			},
			{
				Line:        "abs server favorite 42 --off",
				Description: "Remove podcast from favorites",
			},
			{
				Line:        "abs server favorite",
				Description: "List all currently favorited podcasts",
			},
		},
		Run: func(ctx *clihelp.Context) error {
			*action = "server"
			opts.ServerSubcmd = "favorite"
			opts.SyncSubcmd = "favorite"
			opts.Args = ctx.Args
			return nil
		},
	}
}

func handleServerFavorite(cfg Config, cli CLIOptions) error {
	podcastsDir := cfg.PodcastsDir
	if podcastsDir == "" {
		podcastsDir = "."
	}

	if len(cli.Args) == 0 && !cli.PolicyAll {
		return listFavoritePodcasts(podcastsDir, cli)
	}

	if len(cli.Args) > 0 && strings.EqualFold(cli.Args[0], "list") {
		return listFavoritePodcasts(podcastsDir, cli)
	}

	isOff := cli.FavoriteOff
	if len(cli.Args) > 1 {
		argLower := strings.ToLower(cli.Args[1])
		if argLower == "off" || argLower == "false" || argLower == "remove" {
			isOff = true
		}
	}

	if cli.PolicyAll || (len(cli.Args) > 0 && strings.EqualFold(cli.Args[0], "all")) {
		return setAllFavorites(cfg, podcastsDir, !isOff, cli)
	}

	target := cli.Args[0]
	return setSingleFavorite(cfg, podcastsDir, target, !isOff, cli)
}

func listFavoritePodcasts(podcastsDir string, cli CLIOptions) error {
	entries := podcast.ScanPodcastDirs(podcastsDir)
	var favorites []FavoritePodcastResult
	for _, entry := range entries {
		pCfg := config.LoadPodcastConfig(entry.Dir, config.PodcastConfig{})
		if pCfg.Favorite {
			sinceStr := ""
			if pCfg.FavoriteSince != nil {
				sinceStr = pCfg.FavoriteSince.Format("2006-01-02 15:04")
			}
			favorites = append(favorites, FavoritePodcastResult{
				ID:             entry.ShortID,
				Title:          entry.Title,
				Favorite:       true,
				FavoriteSince:  sinceStr,
				DownloadPolicy: pCfg.DownloadPolicy,
				AdRemoval:      pCfg.AdRemoval,
				AutoDownload:   pCfg.IsAutoDownloadEnabled(),
			})
		}
	}

	if cli.JSON {
		data, _ := json.MarshalIndent(favorites, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	if len(favorites) == 0 {
		fmt.Println("No favorite podcasts set. Use 'abs server favorite <podcast>' to favorite one.")
		return nil
	}

	fmt.Printf("\nFavorite Podcasts (%d total):\n", len(favorites))
	fmt.Printf("%-8s  %-32s  %-12s  %-12s  %-16s\n", "ID", "TITLE", "DOWNLOAD", "AD REMOVAL", "FAVORITE SINCE")
	fmt.Println(strings.Repeat("-", 84))
	for _, f := range favorites {
		title := f.Title
		if len(title) > 32 {
			title = title[:29] + "..."
		}
		dlBadge := config.DownloadPolicyBadge(f.DownloadPolicy, 0)
		adBadge := config.AdRemovalModeBadge(f.AdRemoval)
		fmt.Printf("%-8s  %-32s  %-12s  %-12s  %-16s\n", f.ID, title, dlBadge, adBadge, f.FavoriteSince)
	}
	fmt.Println()
	return nil
}

func setSingleFavorite(cfg Config, podcastsDir, target string, favorite bool, cli CLIOptions) error {
	resolved, err := podcast.ResolveAnyID(podcastsDir, target)
	if err != nil {
		return err
	}
	if !resolved.IsPodcast() {
		return fmt.Errorf("identifier %q resolved to an episode, expected a podcast", target)
	}

	pod := resolved.Podcast
	pod.Config.SetFavorite(favorite)
	if err := config.SavePodcastConfig(pod.Dir, pod.Config); err != nil {
		return fmt.Errorf("failed to save podcast config: %w", err)
	}

	syncMsg := syncPolicyWithBackend(pod, pod.Config.IsAutoDownloadEnabled(), pod.Config.IsAutoCleanupEnabled(), pod.Config.AutoCleanupDays)
	sinceStr := ""
	if pod.Config.FavoriteSince != nil {
		sinceStr = pod.Config.FavoriteSince.Format("2006-01-02 15:04")
	}

	res := FavoritePodcastResult{
		ID:             pod.ShortID,
		Title:          pod.Title,
		Favorite:       pod.Config.Favorite,
		FavoriteSince:  sinceStr,
		DownloadPolicy: pod.Config.DownloadPolicy,
		AdRemoval:      pod.Config.AdRemoval,
		AutoDownload:   pod.Config.IsAutoDownloadEnabled(),
		BackendSync:    syncMsg,
	}

	if cli.JSON {
		data, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	if favorite {
		fmt.Printf("⭐ Marked as favorite: %s [%s] (AutoDownload=true [DL: New], AdRemoval=all, %s)\n",
			util.Bold(pod.Title), util.BoldCyan(pod.ShortID), syncMsg)
	} else {
		fmt.Printf("Removed from favorites: %s [%s] (Policy=none, %s)\n",
			util.Bold(pod.Title), util.BoldCyan(pod.ShortID), syncMsg)
	}
	return nil
}

func setAllFavorites(cfg Config, podcastsDir string, favorite bool, cli CLIOptions) error {
	entries := podcast.ScanPodcastDirs(podcastsDir)
	if len(entries) == 0 {
		return fmt.Errorf("no podcasts found in %s", podcastsDir)
	}

	count := 0
	for _, entry := range entries {
		pCfg := config.LoadPodcastConfig(entry.Dir, config.DefaultPodcastConfig(&cfg))
		pCfg.SetFavorite(favorite)
		if err := config.SavePodcastConfig(entry.Dir, pCfg); err != nil {
			return fmt.Errorf("failed to save config for %s: %w", entry.Title, err)
		}
		count++
	}

	if cli.JSON {
		res := map[string]any{"updated_count": count, "favorite": favorite}
		data, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	action := "Marked"
	if !favorite {
		action = "Removed"
	}
	fmt.Printf("%s %d podcast(s) as favorite (AutoDownload=%v, AdRemoval=all, Policy=new)\n", action, count, favorite)
	return nil
}
