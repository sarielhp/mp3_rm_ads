package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"abs/pkg/config"
	"abs/pkg/pipeline"
	"abs/pkg/podcast"
	"abs/pkg/util"
)

type lsPodcastItem struct {
	ShortID        string `json:"id"`
	Title          string `json:"title"`
	EpisodeCount   int    `json:"episode_count"`
	CleanCount     int    `json:"clean_count"`
	DownloadPolicy string `json:"download_policy"`
	AdRemoval      string `json:"ad_removal"`
	Retention      string `json:"retention"`
	LastEpisode    string `json:"last_episode"`
}

type lsEpisodeItem struct {
	path           string
	podcastDir     string
	podcastTitle   string
	podcastShortID string
	episodeShortID string
	episodeName    string
	modTime        time.Time
	sizeBytes      int64
	origDuration   float64
	cleanDuration  float64
	hasTranscript  bool
	statusStr      string
	statusColor    string
}

type lsEpisodeJSON struct {
	ID                  string  `json:"id"`
	PodcastID           string  `json:"podcast_id"`
	PodcastTitle        string  `json:"podcast_title,omitempty"`
	Title               string  `json:"title"`
	Date                string  `json:"date"`
	OriginalDurationSec float64 `json:"original_duration_sec,omitempty"`
	CleanDurationSec    float64 `json:"clean_duration_sec,omitempty"`
	DurationSec         float64 `json:"duration_sec,omitempty"`
	Status              string  `json:"status"`
	HasTranscript       bool    `json:"has_transcript"`
	Path                string  `json:"path"`
}

func podcastExistsByIndexOrID(podcastsDir, query string) bool {
	res, err := podcast.ResolveAnyID(podcastsDir, query)
	return err == nil && res != nil
}

func listAllPodcasts(podcastsDir string, cli CLIOptions) error {
	entries := podcast.ScanPodcastDirs(podcastsDir)
	if len(entries) == 0 {
		if !cli.Quiet {
			fmt.Println("No podcasts found.")
		}
		return nil
	}

	items := collectPodcastListItems(entries)

	if cli.JSON {
		data, err := json.MarshalIndent(items, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	if cli.Quiet {
		for _, item := range items {
			fmt.Println(item.ShortID)
		}
		return nil
	}

	printPodcastsTable(items)
	return nil
}

func collectPodcastListItems(entries []podcast.PodcastDirEntry) []lsPodcastItem {
	var items []lsPodcastItem
	for _, p := range entries {
		mp3s := util.FindMP3Files(p.Dir)
		cfg := config.LoadPodcastConfig(p.Dir, config.PodcastConfig{})
		cleanCount := 0
		var newestTime time.Time

		for _, mp3 := range mp3s {
			if pipeline.IsEpisodeClean(mp3) {
				cleanCount++
			}
			pt := podcast.GetEpisodePublicationTime(mp3)
			if pt.After(newestTime) {
				newestTime = pt
			}
		}

		retention := "-"
		if cfg.AutoCleanupDays > 0 {
			retention = fmt.Sprintf("%dd", cfg.AutoCleanupDays)
		}

		lastEpStr := "-"
		if !newestTime.IsZero() {
			lastEpStr = newestTime.Format("2006-01-02")
		}

		items = append(items, lsPodcastItem{
			ShortID:        p.ShortID,
			Title:          p.Title,
			EpisodeCount:   len(mp3s),
			CleanCount:     cleanCount,
			DownloadPolicy: cfg.DownloadPolicy,
			AdRemoval:      cfg.AdRemoval,
			Retention:      retention,
			LastEpisode:    lastEpStr,
		})
	}
	return items
}

func printPodcastsTable(items []lsPodcastItem) {
	fmt.Printf("\nPodcasts in Library (%d total):\n", len(items))
	titleWidth := 22
	cols := podcastTableColumns(titleWidth)

	fmt.Println(renderTableTop(cols))
	fmt.Println(renderTableHeader(cols))
	fmt.Println(renderTableDivider(cols))

	for _, item := range items {
		cells := buildPodcastRowCells(item, titleWidth)
		fmt.Println(renderTableRow(cells, cols))
	}

	fmt.Println(renderTableBottom(cols))
	fmt.Println("  🎙️ Episodes   ✨ Clean   ⬇️ Download   ✂️ Ad Removal   ⏳ Retention   📅 Last Ep")
}

func listLatestEpisodes(podcastsDir string, limit int, cli CLIOptions) error {
	podEntries := podcast.ScanPodcastDirs(podcastsDir)
	podIDMap := make(map[string]string)
	podTitleMap := make(map[string]string)
	for _, p := range podEntries {
		podIDMap[p.Dir] = p.ShortID
		podTitleMap[p.Dir] = p.Title
	}

	allMp3s := util.FindMP3Files(podcastsDir)
	if len(allMp3s) == 0 {
		if !cli.Quiet {
			fmt.Println("No podcast audio files (.mp3) found.")
		}
		return nil
	}

	items := collectLatestEpisodeItems(allMp3s, podTitleMap, podIDMap)
	sort.Slice(items, func(i, j int) bool {
		return items[i].modTime.After(items[j].modTime)
	})

	if limit > len(items) {
		limit = len(items)
	}
	latest := items[:limit]

	if cli.JSON {
		return outputLatestEpisodesJSON(latest)
	}

	if cli.Quiet {
		for _, item := range latest {
			fmt.Println(item.path)
		}
		return nil
	}

	printLatestEpisodesTable(latest, limit)
	return nil
}

func outputLatestEpisodesJSON(items []lsEpisodeItem) error {
	var jsonList []lsEpisodeJSON
	for _, it := range items {
		jsonList = append(jsonList, lsEpisodeJSON{
			ID:            it.episodeShortID,
			PodcastID:     it.podcastShortID,
			PodcastTitle:  it.podcastTitle,
			Title:         it.episodeName,
			Date:          publicationDateTime(it.modTime),
			Status:        it.statusStr,
			DurationSec:   it.origDuration,
			HasTranscript: it.hasTranscript,
			Path:          it.path,
		})
	}
	data, err := json.MarshalIndent(jsonList, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func collectLatestEpisodeItems(allMp3s []string, podTitleMap, podIDMap map[string]string) []lsEpisodeItem {
	var items []lsEpisodeItem
	for _, mp3 := range allMp3s {
		fi, err := os.Stat(mp3)
		if err != nil {
			continue
		}
		podDir := filepath.Dir(mp3)
		epName := strings.TrimSuffix(filepath.Base(mp3), filepath.Ext(mp3))
		if strings.EqualFold(filepath.Base(mp3), "podcast.mp3") {
			epName = filepath.Base(podDir)
			podDir = filepath.Dir(podDir)
		}
		podTitle := podTitleMap[podDir]
		if podTitle == "" {
			podTitle = filepath.Base(podDir)
		}
		shortID := podIDMap[podDir]
		if shortID == "" {
			shortID = podcast.GeneratePodcastShortID(podTitle)
		}

		epShortID := podcast.GetOrSetEpisodeShortID(podDir, shortID, mp3)
		statusStr, statusColor := getEpisodeStatusLabel(mp3)
		st := pipeline.GetOrCreateEpisodeStatus(mp3)
		origDur, cleanDur := pipeline.EpisodeDurations(mp3, st)
		txPath := util.StripExt(mp3) + ".transcript.json"
		_, errTx := os.Stat(txPath)

		items = append(items, lsEpisodeItem{
			path:           mp3,
			podcastDir:     podDir,
			podcastTitle:   podTitle,
			podcastShortID: shortID,
			episodeShortID: epShortID,
			episodeName:    epName,
			modTime:        podcast.GetEpisodePublicationTime(mp3),
			sizeBytes:      fi.Size(),
			origDuration:   origDur,
			cleanDuration:  cleanDur,
			hasTranscript:  errTx == nil,
			statusStr:      statusStr,
			statusColor:    statusColor,
		})
	}
	return items
}

func printLatestEpisodesTable(latest []lsEpisodeItem, limit int) {
	fmt.Printf("\nLatest %d Episodes Across All Podcasts:\n", limit)
	podWidth := 16
	titleWidth := 24
	cols := latestEpisodeTableColumns(podWidth, titleWidth)

	fmt.Println(renderTableTop(cols))
	fmt.Println(renderTableHeader(cols))
	fmt.Println(renderTableDivider(cols))

	for _, item := range latest {
		cells := buildLatestEpisodeRowCells(item, podWidth, titleWidth)
		fmt.Println(renderTableRow(cells, cols))
	}

	fmt.Println(renderTableBottom(cols))
	fmt.Println()
}

func formatShortStatus(status string) string {
	switch status {
	case "Needs Ad Removal", "NeedsAd", "NeedAd", "NeedAdR":
		return "✂ NeedAdR"
	case "Queued Remote", "Queued":
		return "⏳ Queued"
	case "In Progress", "Active":
		return "⚡ Active"
	case "Clean":
		return "✓ Clean"
	default:
		return status
	}
}

func getEpisodeStatusLabel(mp3Path string) (string, string) {
	st := pipeline.GetOrCreateEpisodeStatus(mp3Path)
	if st.Status == StateDone || st.Status == StateCopiedBack || pipeline.IsEpisodeCompleted(mp3Path) {
		return "Clean", "green"
	}
	if st.Status == StateQueuedRemote {
		return "Queued Remote", "cyan"
	}
	if st.Status == StateTranscribingRemotely || st.Status == StateCuttingRemotely || st.Status == StateTranscribingLocally || st.Status == StateCuttingLocally {
		return "In Progress", "yellow"
	}
	return "NeedAdR", "yellow"
}
