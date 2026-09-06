package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type PodcastInfoJSON struct {
	ID                 string             `json:"id"`
	UUID               string             `json:"uuid,omitempty"`
	Title              string             `json:"title"`
	Directory          string             `json:"directory"`
	Author             string             `json:"author,omitempty"`
	FeedURL            string             `json:"feed_url,omitempty"`
	CoverPath          string             `json:"cover_path,omitempty"`
	Description        string             `json:"description,omitempty"`
	AutoDownload       bool               `json:"auto_download"`
	DownloadPolicy     string             `json:"download_policy"`
	DownloadK          int                `json:"download_k"`
	AutoCleanup        bool               `json:"auto_cleanup"`
	AutoCleanupDays    int                `json:"auto_cleanup_days"`
	AdRemoval          string             `json:"ad_removal"`
	SQLiteSync         string             `json:"sqlite_sync"`
	TotalEpisodes      int                `json:"total_episodes"`
	CleanEpisodes      int                `json:"clean_episodes"`
	TotalDurationSec   float64            `json:"total_duration_sec"`
	TotalDiskSizeBytes int64              `json:"total_disk_size_bytes"`
	RecentEpisodes     []RecentEpisodeDTO `json:"recent_episodes,omitempty"`
}

type RecentEpisodeDTO struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Date     string `json:"date"`
	Status   string `json:"status"`
	Duration string `json:"duration"`
}

type EpisodeInfoJSON struct {
	ID                  string          `json:"id"`
	PodcastID           string          `json:"podcast_id"`
	PodcastTitle        string          `json:"podcast_title"`
	Title               string          `json:"title"`
	PublishedDate       string          `json:"published_date"`
	AudioPath           string          `json:"audio_path"`
	FileSizeBytes       int64           `json:"file_size_bytes"`
	FileSizeFormatted   string          `json:"file_size_formatted"`
	Status              string          `json:"status"`
	OriginalDurationSec float64         `json:"original_duration_sec"`
	CleanDurationSec    float64         `json:"clean_duration_sec"`
	PercentReduction    float64         `json:"percent_reduction"`
	HasTranscript       bool            `json:"has_transcript"`
	TranscriptPath      string          `json:"transcript_path,omitempty"`
	TranscriptSegments  int             `json:"transcript_segments,omitempty"`
	Description         string          `json:"description,omitempty"`
	Cuts                []EpisodeCutDTO `json:"cuts,omitempty"`
}

type EpisodeCutDTO struct {
	StartFormatted string  `json:"start"`
	EndFormatted   string  `json:"end"`
	DurationSec    float64 `json:"duration_sec"`
	DurationStr    string  `json:"duration"`
	Reason         string  `json:"reason,omitempty"`
}

func runInfoCommand(cfg Config, cli CLIOptions) error {
	podcastsDir := cfg.PodcastsDir
	if podcastsDir == "" {
		podcastsDir = "."
	}

	if len(cli.Args) == 0 {
		return fmt.Errorf("missing target ID or query for info command")
	}

	target := cli.Args[0]
	resolved, err := resolveAnyID(podcastsDir, target)
	if err != nil {
		return err
	}

	if resolved.IsPodcast() {
		return inspectPodcastInfo(resolved.Podcast, cli)
	}

	if resolved.IsEpisode() {
		return inspectEpisodeInfo(resolved.Episode, cli)
	}

	return fmt.Errorf("could not inspect %q", target)
}

func inspectPodcastInfo(pod *ResolvedPodcast, cli CLIOptions) error {
	dto := buildPodcastInfoDTO(pod)

	if cli.JSON {
		data, err := json.MarshalIndent(dto, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	printPodcastInfoCard(dto)
	return nil
}

func collectPodcastStatsAndRecent(pod *ResolvedPodcast, mp3s []string) (int, float64, int64, []RecentEpisodeDTO) {
	cleanCount := 0
	var totalDur float64
	var totalSize int64

	type epTime struct {
		path string
		pt   time.Time
		fi   os.FileInfo
	}
	var epList []epTime

	for _, mp3 := range mp3s {
		fi, err := os.Stat(mp3)
		if err == nil {
			totalSize += fi.Size()
		}
		if isEpisodeClean(mp3) {
			cleanCount++
		}
		st := getOrCreateEpisodeStatus(mp3)
		od, _ := getEpisodeDurations(mp3, st)
		totalDur += od

		pt := getEpisodePublicationTime(mp3)
		if pt.IsZero() && fi != nil {
			pt = fi.ModTime()
		}
		epList = append(epList, epTime{path: mp3, pt: pt, fi: fi})
	}

	sort.Slice(epList, func(i, j int) bool {
		return epList[i].pt.After(epList[j].pt)
	})

	var recent []RecentEpisodeDTO
	limit := 5
	if limit > len(epList) {
		limit = len(epList)
	}
	for i := 0; i < limit; i++ {
		mp3 := epList[i].path
		epID := getOrSetEpisodeShortID(pod.Dir, pod.ShortID, mp3)
		st, _ := getEpisodeStatusLabel(mp3)
		od, _ := getEpisodeDurations(mp3, getOrCreateEpisodeStatus(mp3))
		recent = append(recent, RecentEpisodeDTO{
			ID:       epID,
			Title:    stripExt(filepath.Base(mp3)),
			Date:     epList[i].pt.Format("2006-01-02"),
			Status:   formatShortStatus(st),
			Duration: formatClock(od),
		})
	}
	return cleanCount, totalDur, totalSize, recent
}

func getPodcastMetadataFields(pod *ResolvedPodcast) (string, string, string, string, string) {
	cached, _ := loadPodcastCache(pod.Dir)
	author, feedURL, desc, uuid := "", "", "", pod.UUID
	coverPath := findCoverImageInDir(pod.Dir)

	if cached != nil {
		author = cached.Author
		feedURL = cached.FeedURL
		desc = cached.Description
		if cached.ABSItemID != "" {
			uuid = cached.ABSItemID
		}
		if coverPath == "" && cached.CoverPath != "" {
			coverPath = cached.CoverPath
		}
	}
	return author, feedURL, coverPath, desc, uuid
}

func checkSQLiteSyncStatus() string {
	cfgGlobal := loadConfig()
	dbPath := cfgGlobal.PodfetchDBPath
	if dbPath == "" {
		dbPath = "/media/dockers/podfetch/db/podcast.db"
	}
	if fi, err := os.Stat(dbPath); err == nil && !fi.IsDir() {
		return fmt.Sprintf("Synced (%s)", dbPath)
	}
	return "Not connected"
}

func buildPodcastInfoDTO(pod *ResolvedPodcast) PodcastInfoJSON {
	mp3s := findMP3Files(pod.Dir)
	cleanCount, totalDur, totalSize, recent := collectPodcastStatsAndRecent(pod, mp3s)
	author, feedURL, coverPath, desc, uuid := getPodcastMetadataFields(pod)
	sqliteSync := checkSQLiteSyncStatus()

	autoDl := pod.Config.IsAutoDownloadEnabled()
	autoCl := pod.Config.IsAutoCleanupEnabled()

	return PodcastInfoJSON{
		ID:                 pod.ShortID,
		UUID:               uuid,
		Title:              pod.Title,
		Directory:          pod.Dir,
		Author:             author,
		FeedURL:            feedURL,
		CoverPath:          coverPath,
		Description:        desc,
		AutoDownload:       autoDl,
		DownloadPolicy:     pod.Config.DownloadPolicy,
		DownloadK:          pod.Config.DownloadK,
		AutoCleanup:        autoCl,
		AutoCleanupDays:    pod.Config.AutoCleanupDays,
		AdRemoval:          pod.Config.AdRemoval,
		SQLiteSync:         sqliteSync,
		TotalEpisodes:      len(mp3s),
		CleanEpisodes:      cleanCount,
		TotalDurationSec:   totalDur,
		TotalDiskSizeBytes: totalSize,
		RecentEpisodes:     recent,
	}
}

func printPodcastInfoCard(info PodcastInfoJSON) {
	fmt.Printf("\n%s\n", strings.Repeat("=", 80))
	fmt.Printf("Podcast: %s [%s]\n", bold(info.Title), boldCyan(info.ID))
	fmt.Printf("%s\n", strings.Repeat("=", 80))
	fmt.Printf("  Short ID:         %s\n", boldCyan(info.ID))
	if info.UUID != "" {
		fmt.Printf("  UUID:             %s\n", info.UUID)
	}
	fmt.Printf("  Directory:        %s\n", info.Directory)
	if info.Author != "" {
		fmt.Printf("  Author:           %s\n", info.Author)
	}
	if info.FeedURL != "" {
		fmt.Printf("  Feed URL:         %s\n", info.FeedURL)
	}
	if info.CoverPath != "" {
		fmt.Printf("  Cover:            %s\n", info.CoverPath)
	}

	fmt.Println("\n  Policy & Sync:")
	fmt.Printf("    SQLite Sync:    %s\n", info.SQLiteSync)
	dlBadge := downloadPolicyBadge(info.DownloadPolicy, info.DownloadK)
	fmt.Printf("    Auto Download:  %v %s\n", info.AutoDownload, dlBadge)
	retStr := "Disabled"
	if info.AutoCleanupDays > 0 {
		retStr = fmt.Sprintf("%dd retention", info.AutoCleanupDays)
	}
	fmt.Printf("    Auto Cleanup:   %v (%s)\n", info.AutoCleanup, retStr)
	adBadge := adRemovalModeBadge(info.AdRemoval)
	fmt.Printf("    Ad Removal:     %s %s\n", adRemovalModeLabel(info.AdRemoval), adBadge)

	fmt.Println("\n  Library Stats:")
	cleanPct := 0.0
	if info.TotalEpisodes > 0 {
		cleanPct = float64(info.CleanEpisodes) / float64(info.TotalEpisodes) * 100
	}
	fmt.Printf("    Episodes:       %d total (%d clean, %.1f%% clean)\n", info.TotalEpisodes, info.CleanEpisodes, cleanPct)
	fmt.Printf("    Total Duration: %s\n", formatDurationHours(info.TotalDurationSec))
	fmt.Printf("    Disk Usage:     %s\n", formatDiskSize(info.TotalDiskSizeBytes))

	if info.Description != "" {
		fmt.Println("\n  Description:")
		fmt.Printf("    %s\n", truncate(strings.ReplaceAll(info.Description, "\n", " "), 120))
	}

	if len(info.RecentEpisodes) > 0 {
		fmt.Println("\n  Recent Episodes:")
		for _, ep := range info.RecentEpisodes {
			fmt.Printf("    %-6s  %-10s  [%-7s]  %-7s  %s\n",
				boldCyan(ep.ID), ep.Date, ep.Status, ep.Duration, truncate(ep.Title, 35))
		}
	}
	fmt.Printf("%s\n\n", strings.Repeat("=", 80))
}

func inspectEpisodeInfo(ep *ResolvedEpisode, cli CLIOptions) error {
	dto := buildEpisodeInfoDTO(ep)

	if cli.JSON {
		data, err := json.MarshalIndent(dto, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	printEpisodeInfoCard(dto, cli.ShowCuts)
	return nil
}

func getEpisodeTranscriptInfo(epPath string) (bool, string, int) {
	txPath := stripExt(epPath) + ".transcript.json"
	hasTx := false
	txSegments := 0
	if data, err := os.ReadFile(txPath); err == nil {
		hasTx = true
		var td TranscriptionData
		if json.Unmarshal(data, &td) == nil {
			txSegments = len(td.Segments)
		}
	}
	return hasTx, txPath, txSegments
}

func collectEpisodeCuts(epPath string, st *EpisodeStatusFile) []EpisodeCutDTO {
	var cuts []EpisodeCutDTO
	cutsFile := stripExt(epPath) + ".cuts.json"
	if data, err := os.ReadFile(cutsFile); err == nil {
		var cd CutsData
		if json.Unmarshal(data, &cd) == nil {
			for _, c := range cd.CutIntervals {
				cuts = append(cuts, EpisodeCutDTO{
					StartFormatted: c.StartFormatted,
					EndFormatted:   c.EndFormatted,
					DurationSec:    c.DurationSec,
					DurationStr:    formatClock(c.DurationSec),
					Reason:         c.Reason,
				})
			}
		}
	}

	if len(cuts) == 0 && st != nil && len(st.Ads) > 0 {
		for _, ad := range st.Ads {
			dur := ad.End - ad.Start
			cuts = append(cuts, EpisodeCutDTO{
				StartFormatted: formatClock(ad.Start),
				EndFormatted:   formatClock(ad.End),
				DurationSec:    dur,
				DurationStr:    formatClock(dur),
				Reason:         ad.Reason,
			})
		}
	}
	return cuts
}

func buildEpisodeInfoDTO(ep *ResolvedEpisode) EpisodeInfoJSON {
	fi, _ := os.Stat(ep.Path)
	var fileSize int64
	if fi != nil {
		fileSize = fi.Size()
	}

	st := getOrCreateEpisodeStatus(ep.Path)
	statusStr, _ := getEpisodeStatusLabel(ep.Path)
	origDur, cleanDur := getEpisodeDurations(ep.Path, st)

	pctReduction := 0.0
	if origDur > 0 && cleanDur > 0 && origDur > cleanDur {
		pctReduction = (origDur - cleanDur) / origDur * 100
	}

	pubTime := getEpisodePublicationTime(ep.Path)
	pubDateStr := "-"
	if !pubTime.IsZero() {
		pubDateStr = pubTime.Format("2006-01-02 15:04")
	}

	hasTx, txPath, txSegments := getEpisodeTranscriptInfo(ep.Path)

	desc := ""
	if det, _ := loadEpisodeDetails(ep.PodcastDir, ep.Filename); det != nil && det.Description != "" {
		desc = det.Description
	}

	cuts := collectEpisodeCuts(ep.Path, st)

	return EpisodeInfoJSON{
		ID:                  ep.ShortID,
		PodcastID:           ep.PodcastShortID,
		PodcastTitle:        ep.PodcastTitle,
		Title:               ep.Title,
		PublishedDate:       pubDateStr,
		AudioPath:           ep.Path,
		FileSizeBytes:       fileSize,
		FileSizeFormatted:   formatDiskSize(fileSize),
		Status:              statusStr,
		OriginalDurationSec: origDur,
		CleanDurationSec:    cleanDur,
		PercentReduction:    pctReduction,
		HasTranscript:       hasTx,
		TranscriptPath:      txPath,
		TranscriptSegments:  txSegments,
		Description:         desc,
		Cuts:                cuts,
	}
}

func printEpisodeInfoCard(info EpisodeInfoJSON, showCuts bool) {
	fmt.Printf("\n%s\n", strings.Repeat("=", 80))
	fmt.Printf("Episode: %s [%s]\n", bold(info.Title), boldCyan(info.ID))
	fmt.Printf("%s\n", strings.Repeat("=", 80))
	fmt.Printf("  Episode ID:       %s\n", boldCyan(info.ID))
	fmt.Printf("  Podcast:          %s [%s]\n", info.PodcastTitle, boldCyan(info.PodcastID))
	fmt.Printf("  Published Date:   %s\n", info.PublishedDate)
	fmt.Printf("  Audio Path:       %s\n", info.AudioPath)
	fmt.Printf("  File Size:        %s\n", info.FileSizeFormatted)
	fmt.Printf("  Status:           %s\n", bold(info.Status))

	fmt.Println("\n  Audio & Processing Stats:")
	fmt.Printf("    Original Dur:   %s (%.1fs)\n", formatClock(info.OriginalDurationSec), info.OriginalDurationSec)
	cleanStr := "-"
	if info.CleanDurationSec > 0 {
		cleanStr = fmt.Sprintf("%s (%.1fs)", formatClock(info.CleanDurationSec), info.CleanDurationSec)
	}
	fmt.Printf("    Cleaned Dur:    %s\n", cleanStr)
	if info.PercentReduction > 0 {
		diffSec := info.OriginalDurationSec - info.CleanDurationSec
		fmt.Printf("    Reduction:      -%s (-%.1f%%)\n", formatClock(diffSec), info.PercentReduction)
	}

	printEpisodeCutsAndTranscript(info, showCuts)
	fmt.Printf("%s\n\n", strings.Repeat("=", 80))
}

func printEpisodeCutsAndTranscript(info EpisodeInfoJSON, showCuts bool) {
	if len(info.Cuts) > 0 || showCuts {
		fmt.Printf("\n  Commercial Cuts (%d cuts detected):\n", len(info.Cuts))
		if len(info.Cuts) == 0 {
			fmt.Println("    No cuts detected.")
		} else {
			for idx, cut := range info.Cuts {
				reason := cut.Reason
				if reason == "" {
					reason = "Advertisement segment"
				}
				fmt.Printf("    [%d] %-8s - %-8s (%s) : %s\n",
					idx+1, cut.StartFormatted, cut.EndFormatted, cut.DurationStr, reason)
			}
		}
	}

	fmt.Println("\n  Transcript Info:")
	if info.HasTranscript {
		fmt.Printf("    Status:         Available (%d segments)\n", info.TranscriptSegments)
		fmt.Printf("    Path:           %s\n", info.TranscriptPath)
	} else {
		fmt.Println("    Status:         Not available")
	}

	if info.Description != "" {
		fmt.Println("\n  Show Notes / Description:")
		fmt.Printf("    %s\n", truncate(strings.ReplaceAll(info.Description, "\n", " "), 150))
	}
}

func formatDurationHours(totalSec float64) string {
	if totalSec <= 0 {
		return "0s"
	}
	h := int(totalSec) / 3600
	m := (int(totalSec) % 3600) / 60
	if h > 0 {
		return fmt.Sprintf("%dh %02dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

func formatDiskSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func findCoverImageInDir(dir string) string {
	coverNames := []string{
		"cover.jpg", "cover.jpeg", "cover.png", "cover.webp",
		"folder.jpg", "folder.jpeg", "folder.png",
	}
	for _, name := range coverNames {
		p := filepath.Join(dir, name)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}
	return ""
}
