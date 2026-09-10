package cli

import (
	"database/sql"
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

func inspectPodcastInfo(pod *ResolvedPodcast, cli CLIOptions) error {
	dto := buildPodcastInfoDTO(pod, cli.Count)

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

func collectPodcastStatsAndRecent(pod *ResolvedPodcast, mp3s []string, maxEpisodes int) (int, float64, int64, []RecentEpisodeDTO) {
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
		epList = append(epList, epTime{path: mp3, pt: pt, fi: fi})
	}

	sort.Slice(epList, func(i, j int) bool {
		return epList[i].pt.After(epList[j].pt)
	})

	var recent []RecentEpisodeDTO
	limit := 5
	if maxEpisodes > 0 {
		limit = maxEpisodes
	}
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
			Title:    episodeTitleFromPath(mp3),
			Date:     publicationDateTime(epList[i].pt),
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
	if desc == "" {
		desc = queryPodfetchPodcastSummary(pod.Dir, pod.Title)
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

func buildPodcastInfoDTO(pod *ResolvedPodcast, maxEpisodes int) PodcastInfoJSON {
	mp3s := findMP3Files(pod.Dir)
	cleanCount, totalDur, totalSize, recent := collectPodcastStatsAndRecent(pod, mp3s, maxEpisodes)
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

func formatPodcastInfo(info PodcastInfoJSON) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n%s\n", strings.Repeat("=", 80)))
	sb.WriteString(fmt.Sprintf("Podcast: %s [%s]\n", bold(displayName(info.Title)), boldCyan(info.ID)))
	sb.WriteString(fmt.Sprintf("%s\n", strings.Repeat("=", 80)))
	sb.WriteString(fmt.Sprintf("  Short ID:         %s\n", boldCyan(info.ID)))
	if info.UUID != "" {
		sb.WriteString(fmt.Sprintf("  UUID:             %s\n", info.UUID))
	}
	sb.WriteString(fmt.Sprintf("  Directory:        %s\n", info.Directory))
	if info.Author != "" {
		sb.WriteString(fmt.Sprintf("  Author:           %s\n", displayName(info.Author)))
	}
	if info.FeedURL != "" {
		sb.WriteString(fmt.Sprintf("  Feed URL:         %s\n", info.FeedURL))
	}
	if info.CoverPath != "" {
		sb.WriteString(fmt.Sprintf("  Cover:            %s\n", info.CoverPath))
	}

	sb.WriteString("\n  Policy & Sync:\n")
	sb.WriteString(fmt.Sprintf("    SQLite Sync:    %s\n", info.SQLiteSync))
	dlBadge := downloadPolicyBadge(info.DownloadPolicy, info.DownloadK)
	sb.WriteString(fmt.Sprintf("    Auto Download:  %v %s\n", info.AutoDownload, dlBadge))
	retStr := "Disabled"
	if info.AutoCleanupDays > 0 {
		retStr = fmt.Sprintf("%dd retention", info.AutoCleanupDays)
	}
	sb.WriteString(fmt.Sprintf("    Auto Cleanup:   %v (%s)\n", info.AutoCleanup, retStr))
	adBadge := adRemovalModeBadge(info.AdRemoval)
	sb.WriteString(fmt.Sprintf("    AdR Policy:     %s %s\n", adRemovalModeLabel(info.AdRemoval), adBadge))

	sb.WriteString("\n  Library Stats:\n")
	cleanPct := 0.0
	if info.TotalEpisodes > 0 {
		cleanPct = float64(info.CleanEpisodes) / float64(info.TotalEpisodes) * 100
	}
	sb.WriteString(fmt.Sprintf("    Episodes:       %d total (%d clean, %.1f%% clean)\n", info.TotalEpisodes, info.CleanEpisodes, cleanPct))
	sb.WriteString(fmt.Sprintf("    Total Duration: %s\n", formatDurationHours(info.TotalDurationSec)))
	sb.WriteString(fmt.Sprintf("    Disk Usage:     %s\n", formatDiskSize(info.TotalDiskSizeBytes)))

	if info.Description != "" {
		sb.WriteString("\n  Description:\n")
		formatted := cleanAndFormatNotes(info.Description, 4, 76)
		sb.WriteString(formatted + "\n")
	}

	if len(info.RecentEpisodes) > 0 {
		sb.WriteString("\n  Recent Episodes:\n")
		for _, ep := range info.RecentEpisodes {
			sb.WriteString(fmt.Sprintf("    %-6s  %-10s  [%-7s]  %-7s  %s\n",
				boldCyan(ep.ID), ep.Date, ep.Status, ep.Duration, truncate(displayName(ep.Title), 35)))
		}
	}
	sb.WriteString(fmt.Sprintf("%s\n\n", strings.Repeat("=", 80)))
	return sb.String()
}

func printPodcastInfoCard(info PodcastInfoJSON) {
	fmt.Print(formatPodcastInfo(info))
}

func queryPodfetchPodcastSummary(podDir, title string) string {
	cfgGlobal := loadConfig()
	dbPath := cfgGlobal.PodfetchDBPath
	if dbPath == "" {
		dbPath = "/media/dockers/podfetch/db/podcast.db"
	}
	if fi, err := os.Stat(dbPath); err != nil || fi.IsDir() {
		return ""
	}
	db, err := sql.Open("sqlite3", dbPath+"?_busy_timeout=3000")
	if err != nil {
		return ""
	}
	defer db.Close()

	folderName := filepath.Base(podDir)
	row := db.QueryRow("SELECT summary FROM podcasts WHERE directory_name = ? OR name = ? LIMIT 1", folderName, title)
	var summary sql.NullString
	if err := row.Scan(&summary); err == nil && summary.Valid {
		return strings.TrimSpace(summary.String)
	}
	return ""
}
