package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sarielhp/clihelp"
	"pod/pkg/backend"
	"pod/pkg/config"
	"pod/pkg/pipeline"
	"pod/pkg/podcast"
	"pod/pkg/util"
)

func buildServerAddSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "add",
		Description: "Add a podcast subscription by RSS feed URL",
		UsageLine:   "pod server add <feed-url> [title]",
		Parameters: []clihelp.Param{
			{Name: "<feed-url>", Description: "Upstream podcast RSS feed URL"},
			{Name: "[title]", Description: "Optional title for podcast"},
		},
		Args: clihelp.RangeArgs(1, 2),
		Run: func(ctx *clihelp.Context) error {
			*action = "server"
			opts.ServerSubcmd = "add"
			opts.SyncSubcmd = "add"
			opts.Args = ctx.Args
			return nil
		},
	}
}

func buildServerRemoveSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "remove",
		Description: "Remove a podcast subscription by ID or title",
		UsageLine:   "pod server remove <id-or-title>",
		Parameters: []clihelp.Param{
			{Name: "<id-or-title>", Description: "Podcast ID, folder, or title"},
		},
		Args: clihelp.ExactArgs(1),
		Run: func(ctx *clihelp.Context) error {
			*action = "server"
			opts.ServerSubcmd = "remove"
			opts.SyncSubcmd = "remove"
			opts.Args = ctx.Args
			return nil
		},
	}
}

func buildServerFeedSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "feed",
		Description: "Regenerate feed.xml for local podcast(s)",
		UsageLine:   "pod server feed [id-or-title]",
		Parameters: []clihelp.Param{
			{Name: "[id-or-title]", Description: "Optional podcast ID or title to regenerate"},
		},
		Args: clihelp.RangeArgs(0, 1),
		Run: func(ctx *clihelp.Context) error {
			*action = "server"
			opts.ServerSubcmd = "feed"
			opts.SyncSubcmd = "feed"
			opts.Args = ctx.Args
			return nil
		},
	}
}

func buildServerImportSubcommand(opts *CLIOptions, action *string) clihelp.Command {
	return clihelp.Command{
		Name:        "import",
		Description: "Import subscriptions from OPML file or backend into local store",
		UsageLine:   "pod server import [file]",
		Parameters: []clihelp.Param{
			{Name: "[file]", Description: "Optional OPML file to import (defaults to importing from backend)"},
		},
		Args: clihelp.RangeArgs(0, 1),
		Run: func(ctx *clihelp.Context) error {
			*action = "server"
			opts.ServerSubcmd = "import"
			opts.SyncSubcmd = "import"
			opts.Args = ctx.Args
			return nil
		},
	}
}

func handleServerAdd(cfg Config, cli CLIOptions) error {
	if len(cli.Args) == 0 {
		return fmt.Errorf("feed URL is required")
	}
	feedURL := strings.TrimSpace(cli.Args[0])
	title := ""
	if len(cli.Args) > 1 {
		title = strings.TrimSpace(cli.Args[1])
	}

	var eps []backend.FeedEpisode
	if title == "" {
		if !cli.Quiet {
			fmt.Printf("Inspecting feed: %s\n", feedURL)
		}
		fetchedEps, _, _, _, err := podcast.FetchFeedDirect(feedURL, "", "")
		if err == nil && len(fetchedEps) > 0 {
			eps = fetchedEps
		}
	}

	store, err := podcast.NewSubscriptionStore(config.SubscriptionsFilePath(&cfg))
	if err != nil {
		return fmt.Errorf("open subscriptions store: %w", err)
	}

	imgURL := ""
	if entry := podcast.DefaultFeedCache().Get(feedURL); entry != nil {
		imgURL = entry.ImageURL
	}
	sub := podcast.Subscription{Title: title, FeedURL: feedURL, ImageURL: imgURL}
	if err := store.Add(sub); err != nil {
		return fmt.Errorf("add subscription: %w", err)
	}
	if err := store.Save(); err != nil {
		return fmt.Errorf("save subscriptions: %w", err)
	}

	added := store.Get(feedURL)
	if !cli.Quiet && added != nil {
		fmt.Printf("Added podcast subscription: [%s] %s\n", util.BoldCyan(added.ID), added.Title)
		podDir := filepath.Join(cfg.PodcastsDir, added.Folder)
		if cfg.ServerBaseURL != "" {
			_ = podcast.WritePodcastFeedXML(podDir, *added, cfg.ServerBaseURL, eps)
			fmt.Printf("Local RSS feed available at: %s/%s/feed.xml\n", strings.TrimRight(cfg.ServerBaseURL, "/"), added.Folder)
		}
	}
	return nil
}

func handleServerRemove(cfg Config, cli CLIOptions) error {
	if len(cli.Args) == 0 {
		return fmt.Errorf("subscription ID or title is required")
	}
	query := cli.Args[0]
	store, err := podcast.NewSubscriptionStore(config.SubscriptionsFilePath(&cfg))
	if err != nil {
		return fmt.Errorf("open subscriptions store: %w", err)
	}

	removed, err := store.Remove(query)
	if err != nil {
		return err
	}
	if !removed {
		return fmt.Errorf("no subscription matching %q found", query)
	}
	if err := store.Save(); err != nil {
		return fmt.Errorf("save subscriptions: %w", err)
	}

	if !cli.Quiet {
		fmt.Printf("Removed subscription: %s\n", query)
	}
	return nil
}

func handleServerFeed(cfg Config, cli CLIOptions) error {
	store, err := podcast.NewSubscriptionStore(config.SubscriptionsFilePath(&cfg))
	if err != nil {
		return fmt.Errorf("open subscriptions store: %w", err)
	}
	subs := store.List()
	if len(subs) == 0 {
		return fmt.Errorf("no subscriptions found in %s", store.FilePath())
	}

	target := ""
	if len(cli.Args) > 0 {
		target = cli.Args[0]
	}

	for _, sub := range subs {
		if target != "" && !strings.EqualFold(sub.ID, target) && !strings.Contains(strings.ToLower(sub.Title), strings.ToLower(target)) {
			continue
		}
		podDir := filepath.Join(cfg.PodcastsDir, sub.Folder)
		if err := podcast.WritePodcastFeedXML(podDir, sub, cfg.ServerBaseURL, nil); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to write feed for %s: %v\n", sub.Title, err)
			continue
		}
		if sub.ImageURL == "" {
			if entry := podcast.DefaultFeedCache().Get(sub.FeedURL); entry != nil && entry.ImageURL != "" {
				sub.ImageURL = entry.ImageURL
				_ = store.Add(sub)
				_ = store.Save()
			}
		}
		eps := podcast.CollectLocalEpisodes(podDir, nil)
		fmt.Printf("Updated feed: %s (%d episodes)\n", filepath.Join(podDir, "feed.xml"), len(eps))
	}
	if target == "" && cfg.PodcastsDir != "" {
		if err := podcast.WriteCatalogWebpage(cfg.PodcastsDir, subs, cfg.ServerBaseURL); err == nil {
			fmt.Printf("Updated catalog webpage: %s\n", filepath.Join(cfg.PodcastsDir, "index.html"))
		}
	}
	return nil
}

func handleServerImport(cfg Config, cli CLIOptions) error {
	store, err := podcast.NewSubscriptionStore(config.SubscriptionsFilePath(&cfg))
	if err != nil {
		return fmt.Errorf("open subscriptions store: %w", err)
	}
	if len(cli.Args) > 0 {
		data, err := os.ReadFile(cli.Args[0])
		if err != nil {
			return fmt.Errorf("read OPML file: %w", err)
		}
		n, err := store.ImportFromOPML(data)
		if err != nil {
			return fmt.Errorf("import OPML: %w", err)
		}
		if !cli.Quiet {
			fmt.Printf("Imported %d new subscription(s) from %s\n", n, cli.Args[0])
		}
		return nil
	}

	reader, err := backend.ReaderFromAppConfig(&cfg, cli.Quiet)
	if err != nil {
		return fmt.Errorf("backend not available for import: %w", err)
	}
	n, err := store.ImportFromBackend(reader)
	if err != nil {
		return fmt.Errorf("backend import failed: %w", err)
	}
	if !cli.Quiet {
		fmt.Printf("Imported %d new subscription(s) from backend into %s\n", n, store.FilePath())
	}
	return nil
}

func renderSubscriptionList(subs []podcast.Subscription, podcastsDir string, verbose bool) error {
	fmt.Printf("%-8s %-32s %-6s %-20s %s\n", "ID", "TITLE", "EPS", "FOLDER", "FEED URL")
	fmt.Println(strings.Repeat("-", 95))
	for _, s := range subs {
		title := util.TruncateDisplayName(s.Title, 30)
		folder := util.TruncateDisplayName(s.Folder, 18)
		epCount := 0
		if podcastsDir != "" {
			podDir := filepath.Join(podcastsDir, s.Folder)
			epCount = len(util.FindMP3Files(podDir))
		}
		feedURL := s.FeedURL
		if !verbose && len([]rune(feedURL)) > 35 {
			feedURL = util.Truncate(feedURL, 35)
		}
		fmt.Printf("%-8s %s %-6d %s %s\n", s.ID, util.PadRight(title, 32), epCount, util.PadRight(folder, 20), feedURL)
	}
	fmt.Printf("\nTotal: %d subscription(s)\n", len(subs))
	return nil
}

type subDownloadPlan struct {
	sub        podcast.Subscription
	podDir     string
	feedEps    []backend.FeedEpisode
	toDownload []backend.FeedEpisode
	err        error
}

func runSubscriptionDirectDownloads(store *podcast.SubscriptionStore, cfg Config, cli CLIOptions) error {
	subs := store.List()
	if len(subs) == 0 {
		return fmt.Errorf("no subscriptions found in %s", store.FilePath())
	}
	targets := resolveSubTargets(subs, cli)
	if len(targets) == 0 {
		if !cli.Quiet {
			fmt.Println("No matching podcast subscriptions found.")
		}
		return nil
	}

	plans := planSubDownloads(targets, cfg, cli)
	return executeSubDownloads(plans, store, cfg, cli)
}

func resolveSubTargets(subs []podcast.Subscription, cli CLIOptions) []podcast.Subscription {
	target := cli.Podcast
	if target == "" && len(cli.Args) > 0 {
		target = cli.Args[0]
	}
	var targets []podcast.Subscription
	for _, sub := range subs {
		if sub.Disabled {
			continue
		}
		if target != "" && !strings.EqualFold(sub.ID, target) && !strings.Contains(strings.ToLower(sub.Title), strings.ToLower(target)) {
			continue
		}
		targets = append(targets, sub)
	}
	return targets
}

func planSubDownloads(targets []podcast.Subscription, cfg Config, cli CLIOptions) []subDownloadPlan {
	start := time.Now()
	plans := make([]subDownloadPlan, len(targets))
	workers := podcast.FeedCheckWorkers(cli.FeedJobs, len(targets))

	jobs := make(chan int)
	var wg util.WaitGroup
	var mu util.Mutex
	done := 0

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				plans[i] = planOneSubDownload(targets[i], cfg, cli)
				mu.Lock()
				done++
				if !cli.Quiet {
					fmt.Printf("\rChecking feeds for new episodes (%d/%d)...\x1b[K", done, len(targets))
					os.Stdout.Sync()
				}
				mu.Unlock()
			}
		}()
	}

	for i := range targets {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	if !cli.Quiet {
		fmt.Print("\r\x1b[K")
	}
	reportSubDownloadPlans(plans, time.Since(start), cli)
	return plans
}

func planOneSubDownload(sub podcast.Subscription, cfg Config, cli CLIOptions) subDownloadPlan {
	podDir := filepath.Join(cfg.PodcastsDir, sub.Folder)
	plan := subDownloadPlan{
		sub:    sub,
		podDir: podDir,
	}
	if strings.TrimSpace(sub.FeedURL) == "" {
		plan.err = fmt.Errorf("no RSS feed URL configured")
		return plan
	}

	feedEps, _, _, _, err := podcast.FetchFeedDirect(sub.FeedURL, "", "")
	if err != nil {
		plan.err = fmt.Errorf("fetch feed %s: %w", sub.FeedURL, err)
		return plan
	}
	plan.feedEps = feedEps
	if len(feedEps) > 0 {
		plan.toDownload = selectSubEpisodesToDownload(podDir, feedEps, sub, cli, cfg)
	}
	return plan
}

func reportSubDownloadPlans(plans []subDownloadPlan, elapsed time.Duration, cli CLIOptions) {
	if cli.Quiet {
		return
	}
	selected, episodes, failed := 0, 0, 0
	for i := range plans {
		if plans[i].err != nil {
			failed++
		}
		if len(plans[i].toDownload) > 0 {
			selected++
			episodes += len(plans[i].toDownload)
		}
	}
	fmt.Printf("Checked %d feed(s) in %.1fs: %d episode(s) to download across %d podcast(s)",
		len(plans), elapsed.Seconds(), episodes, selected)
	if failed > 0 {
		fmt.Printf(", %d unreadable", failed)
	}
	fmt.Println(".")

	for i := range plans {
		pTitle := util.DisplayName(plans[i].sub.Title)
		switch {
		case plans[i].err != nil:
			fmt.Printf("  ! %s: %v\n", pTitle, plans[i].err)
		case cli.Verbose && len(plans[i].toDownload) == 0:
			fmt.Printf("  - %s: up to date\n", pTitle)
		}
	}
}

func executeSubDownloads(plans []subDownloadPlan, store *podcast.SubscriptionStore, cfg Config, cli CLIOptions) error {
	if cli.DryRun {
		printDryRunPlans(plans)
		return nil
	}

	downloader := podcast.NewDownloader()
	totalDownloaded, podcastsDownloaded := 0, 0
	for _, plan := range plans {
		if plan.err != nil {
			continue
		}
		if len(plan.toDownload) > 0 {
			n, err := executePodcastSubDownloads(downloader, plan, cfg, cli)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed downloading %s: %v\n", plan.sub.Title, err)
			} else {
				totalDownloaded += n
				podcastsDownloaded++
			}
		} else {
			_ = podcast.WritePodcastFeedXML(plan.podDir, plan.sub, cfg.ServerBaseURL, plan.feedEps)
		}
		updateSubscriptionCover(&plan, store)
	}

	if cfg.PodcastsDir != "" && store != nil {
		_ = podcast.WriteCatalogWebpage(cfg.PodcastsDir, store.List(), cfg.ServerBaseURL)
	}

	if !cli.Quiet && totalDownloaded > 0 {
		fmt.Printf("Downloaded %d episode(s) across %d podcast(s).\n", totalDownloaded, podcastsDownloaded)
	}
	return nil
}

func updateSubscriptionCover(plan *subDownloadPlan, store *podcast.SubscriptionStore) {
	if plan.sub.ImageURL == "" && store != nil {
		if entry := podcast.DefaultFeedCache().Get(plan.sub.FeedURL); entry != nil && entry.ImageURL != "" {
			plan.sub.ImageURL = entry.ImageURL
			_ = store.Add(plan.sub)
			_ = store.Save()
		}
	}
}

func printDryRunPlans(plans []subDownloadPlan) {
	for _, plan := range plans {
		if len(plan.toDownload) == 0 {
			continue
		}
		fmt.Printf("\n=== Podcast: %s ===\n", util.BoldCyan(plan.sub.Title))
		fmt.Printf("Found %d episode(s) to download:\n", len(plan.toDownload))
		for idx, ep := range plan.toDownload {
			fmt.Printf("  %d. %s\n", idx+1, ep.Title)
		}
	}
}

func executePodcastSubDownloads(downloader *podcast.Downloader, plan subDownloadPlan, cfg Config, cli CLIOptions) (int, error) {
	if err := os.MkdirAll(plan.podDir, 0755); err != nil {
		return 0, err
	}
	if !cli.Quiet {
		fmt.Printf("\n=== Podcast: %s ===\n", util.BoldCyan(plan.sub.Title))
		fmt.Printf("Found %d episode(s) to download:\n", len(plan.toDownload))
		for idx, ep := range plan.toDownload {
			fmt.Printf("  %d. %s\n", idx+1, ep.Title)
		}
	}
	shouldQueue := shouldQueueEpisode(plan.podDir, plan.sub, cfg)
	downloaded := 0
	for _, ep := range plan.toDownload {
		if err := executeSingleEpisodeDownload(downloader, plan.podDir, ep, cli.Quiet, shouldQueue); err != nil {
			fmt.Fprintf(os.Stderr, "    Download error for %q: %v\n", ep.Title, err)
		} else {
			downloaded++
		}
	}
	_ = podcast.WritePodcastFeedXML(plan.podDir, plan.sub, cfg.ServerBaseURL, plan.feedEps)
	return downloaded, nil
}

func selectSubEpisodesToDownload(podDir string, feedEps []backend.FeedEpisode, sub podcast.Subscription, cli CLIOptions, cfg Config) []backend.FeedEpisode {
	existingFiles := make(map[string]bool)
	for _, f := range util.FindMP3Files(podDir) {
		name := strings.ToLower(strings.TrimSuffix(filepath.Base(f), ".mp3"))
		existingFiles[name] = true
		stripped := strings.ToLower(podcast.StripEpisodeFilenamePrefix(name))
		existingFiles[stripped] = true
	}

	podCfg := config.LoadPodcastConfig(podDir, config.DefaultPodcastConfig(&cfg))
	if sub.DownloadPolicy != "" {
		podCfg.DownloadPolicy = sub.DownloadPolicy
		autoDl := config.NormalizeDownloadPolicy(sub.DownloadPolicy) != config.DownloadPolicyNone
		podCfg.AutoDownload = &autoDl
	}
	if sub.DownloadK > 0 {
		podCfg.DownloadK = sub.DownloadK
	}

	isDownloaded := func(ep backend.FeedEpisode) bool {
		safeStem := strings.ToLower(podcast.SanitizeTitle(ep.Title))
		rawStem := strings.ToLower(strings.TrimSpace(ep.Title))
		pubMs := podcast.GetPubMS(ep)
		var pubTime time.Time
		if pubMs > 0 {
			pubTime = time.UnixMilli(pubMs).UTC()
		}
		formatted := strings.ToLower(strings.TrimSuffix(podcast.FormatEpisodeFilename(pubTime, ep.Episode, ep.Title), ".mp3"))
		return existingFiles[formatted] || existingFiles[safeStem] || existingFiles[rawStem]
	}

	sortedCatalog := make([]backend.FeedEpisode, len(feedEps))
	copy(sortedCatalog, feedEps)
	sort.Slice(sortedCatalog, func(i, j int) bool {
		return podcast.GetPubMS(sortedCatalog[i]) < podcast.GetPubMS(sortedCatalog[j])
	})

	if cli.DownloadAll {
		eps, _ := podcast.SelectEpisodesByDownloadPolicy(sortedCatalog, isDownloaded, config.DownloadPolicyAll, 0, false)
		if cli.CountGiven && cli.Count > 0 && len(eps) > cli.Count {
			eps = eps[:cli.Count]
		}
		return eps
	}

	if cli.CountGiven && cli.Count > 0 {
		eps, _ := podcast.SelectEpisodesByDownloadPolicy(sortedCatalog, isDownloaded, config.DownloadPolicyAll, 0, false)
		if len(eps) > cli.Count {
			eps = eps[:cli.Count]
		}
		return eps
	}

	if podCfg.Favorite || config.NormalizeDownloadPolicy(podCfg.DownloadPolicy) == config.DownloadPolicyNew {
		eps, _ := podcast.SelectNewEpisodes(sortedCatalog, nil, isDownloaded, podCfg.FavoriteSince)
		return eps
	}

	if !podCfg.IsAutoDownloadEnabled() {
		return nil
	}

	policy := config.NormalizeDownloadPolicy(podCfg.DownloadPolicy)
	k := podCfg.DownloadK
	if k <= 0 {
		k = cfg.DefaultDownloadK
	}
	eps, _ := podcast.SelectEpisodesByDownloadPolicy(sortedCatalog, isDownloaded, policy, k, false)
	return eps
}

func shouldQueueEpisode(podDir string, sub podcast.Subscription, cfg Config) bool {
	podCfg := config.LoadPodcastConfig(podDir, config.DefaultPodcastConfig(&cfg))
	if podCfg.Favorite {
		return true
	}
	adPolicy := sub.AdRemoval
	if adPolicy == "" {
		adPolicy = podCfg.AdRemoval
	}
	if adPolicy == "" {
		adPolicy = cfg.DefaultAdRemoval
	}
	return config.NormalizeAdRemovalMode(adPolicy) != config.AdRemovalNone
}

func executeSingleEpisodeDownload(d *podcast.Downloader, podDir string, ep backend.FeedEpisode, quiet bool, shouldQueue bool) error {
	encURL := ep.EnclosureURL
	if ep.Enclosure != nil && ep.Enclosure.URL != "" {
		encURL = ep.Enclosure.URL
	}
	if encURL == "" {
		return fmt.Errorf("no enclosure URL found")
	}

	pubMs := podcast.GetPubMS(ep)
	var pubTime time.Time
	if pubMs > 0 {
		pubTime = time.UnixMilli(pubMs).UTC()
	}
	fn := podcast.FormatEpisodeFilename(pubTime, ep.Episode, ep.Title)
	destPath := filepath.Join(podDir, fn)

	if !quiet {
		fmt.Printf("  Downloading: %s\n", ep.Title)
	}

	if err := d.DownloadEpisode(context.Background(), encURL, destPath, quiet); err != nil {
		return err
	}

	st := pipeline.GetOrCreateEpisodeStatus(destPath)
	pubMS := podcast.GetPubMS(ep)
	if pubMS > 0 {
		st.PublishedAt = time.UnixMilli(pubMS).UTC().Format(time.RFC3339)
		st.PublicationSource = "feed"
		_ = pipeline.SaveEpisodeStatus(pipeline.StatusPathFor(destPath), st)
	}

	if shouldQueue {
		pipeline.AddToQueue(podDir, fn)
	}
	return nil
}
