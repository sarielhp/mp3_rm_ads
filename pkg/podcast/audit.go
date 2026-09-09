package podcast

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sariel/abs/pkg/audio"
	"github.com/sariel/abs/pkg/format"
	"github.com/sariel/abs/pkg/pipeline"
	"github.com/sariel/abs/pkg/types"
	"github.com/sariel/abs/pkg/util"
)

type TranscriptAuditItem struct {
	AudioPath      string
	TranscriptPath string
	StatusPath     string
	CutsPath       string
	AudioDur       float64
	TextChars      int
	SpeechDur      float64
	CoverageRatio  float64
	IsCorrupted    bool
	IsSuspicious   bool
	SuspiciousMsg  string
	AdFailed       bool
}

func AuditDisplayName(p string) string {
	dir := filepath.Base(filepath.Dir(p))
	base := filepath.Base(p)
	if dir != "" && dir != "." && dir != "/" && base == "podcast.mp3" {
		return dir
	}
	return base
}

func InspectEpisodeTranscript(audioPath string, minRatio float64, minChars int) *TranscriptAuditItem {
	base := util.StripExt(audioPath)
	transPath := base + ".transcript.json"
	statPath := pipeline.StatusPathFor(audioPath)
	cutsPath := base + ".cuts.json"

	if !util.FileExists(transPath) && !util.FileExists(statPath) {
		return nil
	}

	item := &TranscriptAuditItem{
		AudioPath:      audioPath,
		TranscriptPath: transPath,
		StatusPath:     statPath,
		CutsPath:       cutsPath,
	}

	st, _ := pipeline.LoadEpisodeStatus(statPath)
	if st != nil && st.Original.DurationSec > 0 {
		item.AudioDur = st.Original.DurationSec
	} else {
		item.AudioDur = audio.GetAudioDuration(audioPath)
	}
	if st != nil && st.AdDetectionSuccessful != nil && !*st.AdDetectionSuccessful {
		item.AdFailed = true
	}

	if !util.FileExists(transPath) {
		if item.AdFailed {
			return item
		}
		return nil
	}

	data, err := os.ReadFile(transPath)
	if err != nil {
		item.IsSuspicious = true
		item.SuspiciousMsg = "unreadable transcript file"
		return item
	}

	var td types.TranscriptionData
	if err := json.Unmarshal(data, &td); err != nil {
		item.IsSuspicious = true
		item.IsCorrupted = true
		item.SuspiciousMsg = "corrupted JSON"
		return item
	}

	var rawMap map[string]interface{}
	if json.Unmarshal(data, &rawMap) == nil {
		if val, ok := rawMap["ad_detection_successful"].(bool); ok && !val {
			item.AdFailed = true
		}
	}

	evaluateTranscriptMetrics(item, td, minRatio, minChars)
	return item
}

func evaluateTranscriptMetrics(item *TranscriptAuditItem, td types.TranscriptionData, minRatio float64, minChars int) {
	text := strings.TrimSpace(td.Text)
	item.TextChars = len([]rune(text))

	totalSpeech := 0.0
	for _, seg := range td.Segments {
		totalSpeech += (seg.End - seg.Start)
	}
	item.SpeechDur = totalSpeech

	if item.AudioDur > 0 {
		item.CoverageRatio = item.SpeechDur / item.AudioDur
	}

	if item.AudioDur > 60 {
		if item.TextChars < minChars {
			item.IsSuspicious = true
			item.SuspiciousMsg = fmt.Sprintf("near empty text (%d chars < %d min)", item.TextChars, minChars)
		} else if len(td.Segments) == 0 {
			item.IsSuspicious = true
			item.SuspiciousMsg = "no transcription segments found"
		} else if item.AudioDur > 120 && item.CoverageRatio < minRatio {
			item.IsSuspicious = true
			item.SuspiciousMsg = fmt.Sprintf("speech coverage %.1f%% below %.1f%% threshold", item.CoverageRatio*100, minRatio*100)
		}
	}
}

func ReportAndHealSuspicious(item *TranscriptAuditItem, dryRun, quiet bool) {
	if !quiet {
		fmt.Printf("  [SUSPICIOUS] %s\n", AuditDisplayName(item.AudioPath))
		fmt.Printf("      - Audio length:  %s (%.1fs)\n", format.FormatTime(item.AudioDur), item.AudioDur)
		fmt.Printf("      - Issue:         %s\n", item.SuspiciousMsg)
		if dryRun {
			fmt.Println("      - Action:        [DRY RUN] Would delete transcript and set status to NeedAdR")
		} else {
			fmt.Println("      - Action:        Deleted transcript & cuts, marked as NeedAdR")
		}
	}
	if !dryRun {
		if util.FileExists(item.TranscriptPath) {
			_ = os.Remove(item.TranscriptPath)
		}
		if util.FileExists(item.CutsPath) {
			_ = os.Remove(item.CutsPath)
		}
		_ = pipeline.UpdateEpisodeStatus(item.AudioPath, func(st *types.EpisodeStatusFile) {
			st.Status = types.StateNeedsAdR
			st.Cleaned = types.EpisodeAudioMeta{}
			st.Ads = nil
			st.AdDetectionSuccessful = nil
			st.AdDetectionStatus = ""
		})
	}
}

func ReportAndHealFailedAd(item *TranscriptAuditItem, dryRun, quiet bool) {
	if !quiet {
		fmt.Printf("  [FAILED AD DETECTION] %s\n", AuditDisplayName(item.AudioPath))
		if dryRun {
			fmt.Println("      - Action:        [DRY RUN] Would mark as NeedAdR for ad detection retry")
		} else {
			fmt.Println("      - Action:        Marked as NeedAdR for ad detection retry")
		}
	}
	if !dryRun {
		_ = pipeline.UpdateEpisodeStatus(item.AudioPath, func(st *types.EpisodeStatusFile) {
			st.Status = types.StateNeedsAdR
		})
	}
}

func PrintAuditSummary(scanned, suspicious, adFailed int, dryRun, quiet bool) {
	if quiet {
		return
	}
	div := strings.Repeat("-", 50)
	fmt.Printf("\n%s\n", div)
	fmt.Println("TRANSCRIPT AUDIT SUMMARY:")
	fmt.Printf("  - Scanned episodes:        %d\n", scanned)
	fmt.Printf("  - Suspicious transcripts:  %d\n", suspicious)
	fmt.Printf("  - Failed ad detections:    %d\n", adFailed)
	if dryRun {
		fmt.Println("  (Dry-run mode: no files were modified or deleted)")
	}
	fmt.Printf("%s\n\n", div)
}

func CollectAudioFilesForAudit(targets []string) []string {
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
