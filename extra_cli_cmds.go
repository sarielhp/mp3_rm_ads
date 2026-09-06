package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func runFetchCommand(cfg Config, cli CLIOptions) error {
	podcastsDir := cfg.PodcastsDir
	if podcastsDir == "" {
		podcastsDir = "."
	}

	if len(cli.Args) > 0 {
		target := cli.Args[0]
		res, err := resolveAnyID(podcastsDir, target)
		if err != nil {
			return err
		}
		if !res.IsPodcast() {
			return fmt.Errorf("target %q is not a podcast", target)
		}
		return fetchSinglePodcastFeedCLI(res.Podcast)
	}

	entries := scanPodcastDirs(podcastsDir)
	if len(entries) == 0 {
		fmt.Println("No podcasts found to fetch.")
		return nil
	}

	fmt.Printf("Fetching RSS feeds for %d podcast(s)...\n", len(entries))
	for _, p := range entries {
		res, err := resolveAnyID(podcastsDir, p.shortID)
		if err == nil && res.IsPodcast() {
			_ = fetchSinglePodcastFeedCLI(res.Podcast)
		}
	}
	return nil
}

func fetchSinglePodcastFeedCLI(pod *ResolvedPodcast) error {
	feedURL := ""
	if cached, _ := loadPodcastCache(pod.Dir); cached != nil && cached.FeedURL != "" {
		feedURL = cached.FeedURL
	}
	if feedURL == "" {
		fmt.Printf("Skipping %s [%s]: No RSS feed URL found\n", pod.Title, pod.ShortID)
		return nil
	}

	eps, _, _, _, err := fetchFeedDirect(feedURL, "", "")
	if err != nil {
		fmt.Printf("Failed to fetch %s [%s]: %v\n", pod.Title, pod.ShortID, err)
		return err
	}

	fmt.Printf("✓ %s [%s]: Fetched feed (%d episodes listed)\n",
		bold(pod.Title), boldCyan(pod.ShortID), len(eps))
	return nil
}

func runPlayCommand(cfg Config, cli CLIOptions) error {
	podcastsDir := cfg.PodcastsDir
	if podcastsDir == "" {
		podcastsDir = "."
	}

	if len(cli.Args) == 0 {
		return fmt.Errorf("missing episode identifier for play command")
	}

	target := cli.Args[0]
	res, err := resolveAnyID(podcastsDir, target)
	if err != nil {
		return err
	}

	if !res.IsEpisode() {
		return fmt.Errorf("identifier %q is a podcast; please specify an episode ID to play", target)
	}

	ep := res.Episode
	fmt.Printf("Playing: %s [%s]\n", bold(ep.Title), boldCyan(ep.ShortID))
	fmt.Printf("Audio file: %s\n", ep.Path)

	for _, playerBin := range []string{"mpv", "ffplay", "mplayer"} {
		if path, err := exec.LookPath(playerBin); err == nil {
			cmd := exec.Command(path, ep.Path)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}
	}

	track := PlayerTrack{
		Title:   ep.Title,
		Podcast: ep.PodcastTitle,
		Path:    ep.Path,
	}
	globalPlayer.PlayTrack(track)
	fmt.Println("Playback started in background. Press Ctrl+C to stop.")
	for globalPlayer.View().IsPlaying {
		time.Sleep(500 * time.Millisecond)
	}
	return nil
}

func runTranscriptCommand(cfg Config, cli CLIOptions) error {
	podcastsDir := cfg.PodcastsDir
	if podcastsDir == "" {
		podcastsDir = "."
	}

	if len(cli.Args) == 0 {
		return fmt.Errorf("missing episode identifier for transcript command")
	}

	target := cli.Args[0]
	res, err := resolveAnyID(podcastsDir, target)
	if err != nil {
		return err
	}

	if !res.IsEpisode() {
		return fmt.Errorf("identifier %q is a podcast; please specify an episode ID to view transcript", target)
	}

	ep := res.Episode
	jsonPath := stripExt(ep.Path) + ".transcript.json"
	if _, err := os.Stat(jsonPath); err != nil {
		return fmt.Errorf("transcript file not found for episode [%s]: %s", ep.ShortID, jsonPath)
	}

	fmtCount := 0
	if cli.ExportFormat != "" {
		fmtCount++
	}
	if cli.ExportTXT {
		fmtCount++
	}
	if cli.ExportSRT {
		fmtCount++
	}
	if fmtCount > 1 {
		return fmt.Errorf("conflicting export format flags; use canonical '--export <format>'")
	}

	format := strings.ToLower(cli.ExportFormat)
	if cli.ExportTXT {
		format = "txt"
	} else if cli.ExportSRT {
		format = "srt"
	}

	if format != "" && format != "txt" && format != "srt" {
		return fmt.Errorf("invalid export format %q; expected 'txt' or 'srt'", format)
	}

	if format == "txt" {
		out := convertJSONToTXT(jsonPath, nil, 0, cli.Output, cli.Quiet)
		if !cli.Quiet {
			fmt.Printf("Exported TXT: %s\n", out)
		}
		return nil
	}

	if format == "srt" {
		out := convertJSONToSRT(jsonPath, nil, cli.Output, cli.Quiet)
		if !cli.Quiet {
			fmt.Printf("Exported SRT: %s\n", out)
		}
		return nil
	}

	return printTranscriptText(jsonPath)
}

func printTranscriptText(jsonPath string) error {
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return err
	}

	var td TranscriptionData
	if err := json.Unmarshal(data, &td); err == nil && len(td.Segments) > 0 {
		for _, seg := range td.Segments {
			timeStr := fmt.Sprintf("[%s -> %s]", formatSRTTime(seg.Start), formatSRTTime(seg.End))
			fmt.Printf("%s %s\n", boldCyan(timeStr), strings.TrimSpace(seg.Text))
		}
		return nil
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err == nil {
		if text, ok := raw["text"].(string); ok && text != "" {
			fmt.Println(text)
			return nil
		}
	}

	fmt.Println(string(data))
	return nil
}
