package adremoval

import (
	"abs/pkg/audio"
	"abs/pkg/format"
	"abs/pkg/pipeline"
	"abs/pkg/util"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type transcriptAuditItem struct {
	audioPath      string
	transcriptPath string
	statusPath     string
	cutsPath       string
	audioDur       float64
	textChars      int
	speechDur      float64
	coverageRatio  float64
	isCorrupted    bool
	isSuspicious   bool
	suspiciousMsg  string
	adFailed       bool
}

func RunTranscriptAudit(config Config, cli CLIOptions) {
	targets := cli.Args
	if len(targets) == 0 {
		if config.PodcastsDir == "" {
			fmt.Fprintf(os.Stderr, "Error: podcasts_dir not configured and no target paths provided.\n")
			return
		}
		targets = []string{config.PodcastsDir}
	}

	minRatio := 0.15
	if cli.AuditMinRatioStr != "" {
		if v, err := strconv.ParseFloat(cli.AuditMinRatioStr, 64); err == nil && v > 0 {
			minRatio = v
		}
	}
	minChars := cli.AuditMinChars
	if minChars <= 0 {
		minChars = 50
	}

	audioFiles := collectAudioFilesForAudit(targets)
	if len(audioFiles) == 0 {
		if !cli.Quiet {
			fmt.Println("No audio files found to audit.")
		}
		return
	}

	if !cli.Quiet {
		fmt.Printf("Auditing transcripts across %d audio file(s)...\n", len(audioFiles))
	}

	scanned := 0
	suspiciousCount := 0
	adFailedCount := 0

	for _, audioPath := range audioFiles {
		item := inspectEpisodeTranscript(audioPath, minRatio, minChars)
		if item == nil {
			continue
		}
		scanned++
		if item.isSuspicious {
			suspiciousCount++
			reportAndHealSuspicious(item, cli.DryRun, cli.Quiet)
		} else if item.adFailed {
			adFailedCount++
			reportAndHealFailedAd(item, cli.DryRun, cli.Quiet)
		} else if cli.Verbose && !cli.Quiet {
			fmt.Printf("  [OK] %s (%.0fs, %d chars, %.1f%% coverage)\n",
				auditDisplayName(item.audioPath), item.audioDur, item.textChars, item.coverageRatio*100)
		}
	}

	printAuditSummary(scanned, suspiciousCount, adFailedCount, cli.DryRun, cli.Quiet)
}

func auditDisplayName(p string) string {
	dir := filepath.Base(filepath.Dir(p))
	base := filepath.Base(p)
	if dir != "" && dir != "." && dir != "/" && base == "podcast.mp3" {
		return dir
	}
	return base
}

func inspectEpisodeTranscript(audioPath string, minRatio float64, minChars int) *transcriptAuditItem {
	base := util.StripExt(audioPath)
	transPath := base + ".transcript.json"
	statPath := pipeline.StatusPathFor(audioPath)
	cutsPath := base + ".cuts.json"

	if !util.FileExists(transPath) && !util.FileExists(statPath) {
		return nil
	}

	item := &transcriptAuditItem{
		audioPath:      audioPath,
		transcriptPath: transPath,
		statusPath:     statPath,
		cutsPath:       cutsPath,
	}

	st, _ := pipeline.LoadEpisodeStatus(statPath)
	if st != nil && st.Original.DurationSec > 0 {
		item.audioDur = st.Original.DurationSec
	} else {
		item.audioDur = audio.GetAudioDuration(audioPath)
	}
	if st != nil && st.AdDetectionSuccessful != nil && !*st.AdDetectionSuccessful {
		item.adFailed = true
	}

	if !util.FileExists(transPath) {
		if item.adFailed {
			return item
		}
		return nil
	}

	data, err := os.ReadFile(transPath)
	if err != nil {
		item.isSuspicious = true
		item.suspiciousMsg = "unreadable transcript file"
		return item
	}

	var td TranscriptionData
	if err := json.Unmarshal(data, &td); err != nil {
		item.isSuspicious = true
		item.isCorrupted = true
		item.suspiciousMsg = "corrupted JSON"
		return item
	}

	var rawMap map[string]interface{}
	if json.Unmarshal(data, &rawMap) == nil {
		if val, ok := rawMap["ad_detection_successful"].(bool); ok && !val {
			item.adFailed = true
		}
	}

	evaluateTranscriptMetrics(item, td, minRatio, minChars)
	return item
}

func evaluateTranscriptMetrics(item *transcriptAuditItem, td TranscriptionData, minRatio float64, minChars int) {
	text := strings.TrimSpace(td.Text)
	item.textChars = len([]rune(text))

	totalSpeech := 0.0
	for _, seg := range td.Segments {
		totalSpeech += (seg.End - seg.Start)
	}
	item.speechDur = totalSpeech

	if item.audioDur > 0 {
		item.coverageRatio = item.speechDur / item.audioDur
	}

	if item.audioDur > 60 {
		if item.textChars < minChars {
			item.isSuspicious = true
			item.suspiciousMsg = fmt.Sprintf("near empty text (%d chars < %d min)", item.textChars, minChars)
		} else if len(td.Segments) == 0 {
			item.isSuspicious = true
			item.suspiciousMsg = "no transcription segments found"
		} else if item.audioDur > 120 && item.coverageRatio < minRatio {
			item.isSuspicious = true
			item.suspiciousMsg = fmt.Sprintf("speech coverage %.1f%% below %.1f%% threshold", item.coverageRatio*100, minRatio*100)
		}
	}
}

func reportAndHealSuspicious(item *transcriptAuditItem, dryRun, quiet bool) {
	if !quiet {
		fmt.Printf("  [SUSPICIOUS] %s\n", auditDisplayName(item.audioPath))
		fmt.Printf("      - Audio length:  %s (%.1fs)\n", format.FormatTime(item.audioDur), item.audioDur)
		fmt.Printf("      - Issue:         %s\n", item.suspiciousMsg)
		if dryRun {
			fmt.Println("      - Action:        [DRY RUN] Would delete transcript and set status to NeedAdR")
		} else {
			fmt.Println("      - Action:        Deleted transcript & cuts, marked as NeedAdR")
		}
	}
	if !dryRun {
		if util.FileExists(item.transcriptPath) {
			_ = os.Remove(item.transcriptPath)
		}
		if util.FileExists(item.cutsPath) {
			_ = os.Remove(item.cutsPath)
		}
		_ = pipeline.UpdateEpisodeStatus(item.audioPath, func(st *EpisodeStatusFile) {
			st.Status = StateNeedsAdR
			st.Cleaned = EpisodeAudioMeta{}
			st.Ads = nil
			st.AdDetectionSuccessful = nil
			st.AdDetectionStatus = ""
		})
	}
}

func reportAndHealFailedAd(item *transcriptAuditItem, dryRun, quiet bool) {
	if !quiet {
		fmt.Printf("  [FAILED AD DETECTION] %s\n", auditDisplayName(item.audioPath))
		if dryRun {
			fmt.Println("      - Action:        [DRY RUN] Would mark as NeedAdR for ad detection retry")
		} else {
			fmt.Println("      - Action:        Marked as NeedAdR for ad detection retry")
		}
	}
	if !dryRun {
		_ = pipeline.UpdateEpisodeStatus(item.audioPath, func(st *EpisodeStatusFile) {
			st.Status = StateNeedsAdR
		})
	}
}

func printAuditSummary(scanned, suspicious, adFailed int, dryRun, quiet bool) {
	if quiet {
		return
	}
	fmt.Printf("\n%s\n", util.RepeatStr("-", 50))
	fmt.Println("TRANSCRIPT AUDIT SUMMARY:")
	fmt.Printf("  - Scanned episodes:        %d\n", scanned)
	fmt.Printf("  - Suspicious transcripts:  %d\n", suspicious)
	fmt.Printf("  - Failed ad detections:    %d\n", adFailed)
	if dryRun {
		fmt.Println("  (Dry-run mode: no files were modified or deleted)")
	}
	fmt.Printf("%s\n\n", util.RepeatStr("-", 50))
}

func collectAudioFilesForAudit(targets []string) []string {
	var files []string
	for _, target := range targets {
		fi, err := os.Stat(target)
		if err != nil {
			continue
		}
		if fi.IsDir() {
			files = append(files, util.FindMP3Files(target)...)
		} else if strings.HasSuffix(strings.ToLower(target), ".mp3") {
			files = append(files, target)
		}
	}
	return files
}
