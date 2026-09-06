package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
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

func runPlayerCommand(cfg Config, cli CLIOptions) error {
	podcastsDir := cfg.PodcastsDir
	if podcastsDir == "" {
		podcastsDir = "."
	}

	subcmd := cli.PlayerSubcmd
	args := cli.Args
	if subcmd == "" && len(args) > 0 {
		switch strings.ToLower(args[0]) {
		case "play", "stop", "pause", "status", "daemon":
			subcmd = strings.ToLower(args[0])
			args = args[1:]
		default:
			subcmd = "play"
		}
	} else if subcmd == "" {
		subcmd = "status"
	}

	switch subcmd {
	case "play":
		return handlePlayerPlay(podcastsDir, args)
	case "stop":
		return handlePlayerStop()
	case "pause":
		return handlePlayerPause()
	case "status":
		return handlePlayerStatus()
	case "daemon":
		return handlePlayerDaemon(args)
	default:
		return fmt.Errorf("unknown player action %q (use play, stop, pause, or status)", subcmd)
	}
}

func handlePlayerPlay(podcastsDir string, args []string) error {
	if len(args) == 0 {
		if isPlayerSocketAlive() {
			if err := ResumePlayerSocket(); err == nil {
				fmt.Println("Playback resumed.")
				return nil
			}
		}
		return fmt.Errorf("missing episode identifier for player play")
	}

	target := args[0]
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

	if err := StartPlayerTrack(ep.Path, ep.Title, ep.PodcastTitle); err != nil {
		return fmt.Errorf("failed to start player: %w", err)
	}

	track := PlayerTrack{
		Title:   ep.Title,
		Podcast: ep.PodcastTitle,
		Path:    ep.Path,
	}
	globalPlayer.Current = &track
	globalPlayer.IsPlaying = true
	fmt.Printf("Started background playback (socket: %s)\n", PlayerSocketPath)
	return nil
}

func handlePlayerStop() error {
	if !isPlayerSocketAlive() {
		fmt.Println("Player is not running.")
		return nil
	}
	if err := StopPlayerSocket(); err != nil {
		return err
	}
	globalPlayer.Stop()
	fmt.Println("Playback stopped.")
	return nil
}

func handlePlayerPause() error {
	if !isPlayerSocketAlive() {
		fmt.Println("Player is not running.")
		return nil
	}
	paused, err := PausePlayerSocket()
	if err != nil {
		return err
	}
	if paused {
		fmt.Println("Playback paused.")
	} else {
		fmt.Println("Playback resumed.")
	}
	return nil
}

func handlePlayerStatus() error {
	st, err := QueryPlayerStatus()
	if err != nil || st == nil || !st.IsRunning {
		fmt.Println("No active playback session (player is stopped).")
		return nil
	}

	statusLabel := "Playing"
	if st.IsPaused {
		statusLabel = "Paused"
	}

	fmt.Printf("Playback Status:  %s\n", bold(statusLabel))
	if st.Title != "" {
		fmt.Printf("Track:            %s\n", boldCyan(st.Title))
	}
	pct := 0.0
	if st.Duration > 0 {
		pct = (st.Position / st.Duration) * 100
	}
	fmt.Printf("Position:         %s / %s (%.0f%%)\n", formatPlayerTime(st.Position), formatPlayerTime(st.Duration), pct)
	fmt.Printf("Socket:           %s\n", PlayerSocketPath)
	return nil
}

func handlePlayerDaemon(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing audio path for player daemon")
	}
	audioPath := args[0]
	title := ""
	podcast := ""
	for i := 1; i < len(args); i++ {
		if args[i] == "--title" && i+1 < len(args) {
			title = args[i+1]
			i++
		} else if args[i] == "--podcast" && i+1 < len(args) {
			podcast = args[i+1]
			i++
		}
	}
	return runPlayerDaemon(audioPath, title, podcast)
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
