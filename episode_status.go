package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type EpisodeState string

const (
	StateDownloaded            EpisodeState = "downloaded"
	StateQueuedRemote          EpisodeState = "queued_remote"
	StateAwaitingTranscription EpisodeState = "awaiting_transcription"
	StateTranscribingLocally   EpisodeState = "transcribing_locally"
	StateTranscribingRemotely  EpisodeState = "transcribing_remotely"
	StateCuttingLocally        EpisodeState = "cutting_locally"
	StateCuttingRemotely       EpisodeState = "cutting_remotely"
	StateReadyForCopyBack      EpisodeState = "ready_for_copy_back"
	StateCopiedBack            EpisodeState = "copied_back"
	StateDone                  EpisodeState = "done"
	StateArchived              EpisodeState = "archived"
	StateFailed                EpisodeState = "failed"
)

type EpisodeAudioMeta struct {
	Filename      string  `json:"filename,omitempty"`
	DurationSec   float64 `json:"duration_sec,omitempty"`
	SizeBytes     int64   `json:"size_bytes,omitempty"`
	AdDurationSec float64 `json:"ad_duration_sec,omitempty"`
}

type EpisodeAdCut struct {
	Start  float64 `json:"start"`
	End    float64 `json:"end"`
	Reason string  `json:"reason,omitempty"`
}

type EpisodeStatusFile struct {
	Version       int              `json:"version"`
	MediaFile     string           `json:"media_file"`
	Status        EpisodeState     `json:"status"`
	CurrentStep   string           `json:"current_step,omitempty"`
	StepStartedAt string           `json:"step_started_at,omitempty"`
	CreatedAt     string           `json:"created_at"`
	UpdatedAt     string           `json:"updated_at"`
	PublishedAt   string           `json:"published_at,omitempty"`
	WorkerHost    string           `json:"worker_host,omitempty"`
	Original      EpisodeAudioMeta `json:"original,omitempty"`
	Cleaned       EpisodeAudioMeta `json:"cleaned,omitempty"`
	Ads           []EpisodeAdCut   `json:"ads,omitempty"`
	LastError     string           `json:"last_error,omitempty"`
	Priority      int              `json:"priority,omitempty"`
}

func statusPathFor(audioPath string) string {
	return audioPath + ".json"
}

func loadEpisodeStatus(path string) (*EpisodeStatusFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var st EpisodeStatusFile
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("invalid episode status json in %s: %w", path, err)
	}
	return &st, nil
}

func saveEpisodeStatus(path string, st *EpisodeStatusFile) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	st.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if st.CreatedAt == "" {
		st.CreatedAt = st.UpdatedAt
	}
	if st.Version == 0 {
		st.Version = 1
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal episode status: %w", err)
	}
	data = append(data, '\n')
	tmpPath := fmt.Sprintf("%s.tmp.%d", path, time.Now().UnixNano())
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write tmp episode status %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to atomically rename episode status %s: %w", path, err)
	}
	return nil
}

func getOrCreateEpisodeStatus(audioPath string) *EpisodeStatusFile {
	statPath := statusPathFor(audioPath)
	if st, err := loadEpisodeStatus(statPath); err == nil && st != nil {
		return st
	}
	now := time.Now().UTC().Format(time.RFC3339)
	fname := filepath.Base(audioPath)
	var sz int64
	if fi, err := os.Stat(audioPath); err == nil {
		sz = fi.Size()
	}
	dur := getAudioDuration(audioPath)
	pubStr := ""
	if pt := getEpisodePublicationTime(audioPath); !pt.IsZero() {
		pubStr = pt.UTC().Format(time.RFC3339)
	}
	st := &EpisodeStatusFile{
		Version:     1,
		MediaFile:   fname,
		Status:      StateDownloaded,
		CreatedAt:   now,
		UpdatedAt:   now,
		PublishedAt: pubStr,
		Original: EpisodeAudioMeta{
			Filename:    fname,
			DurationSec: dur,
			SizeBytes:   sz,
		},
	}

	populatePrecutOrCutsMeta(st, audioPath, fname, dur, sz)
	populateAdsFromCutsFile(st, stripExt(audioPath)+".cuts.json")

	_ = saveEpisodeStatus(statPath, st)
	return st
}

func populatePrecutOrCutsMeta(st *EpisodeStatusFile, audioPath, fname string, dur float64, sz int64) {
	base := stripExt(audioPath)
	cutsFile := base + ".cuts.json"
	transcriptFile := base + ".transcript.json"
	precutFile := audioPath + ".precut"

	if fileExists(precutFile) {
		var origSz int64
		if fi, err := os.Stat(precutFile); err == nil {
			origSz = fi.Size()
		}
		origDur := getAudioDuration(precutFile)
		st.Status = StateDone
		st.Original = EpisodeAudioMeta{
			Filename:    filepath.Base(precutFile),
			DurationSec: origDur,
			SizeBytes:   origSz,
		}
		st.Cleaned = EpisodeAudioMeta{
			Filename:      fname,
			DurationSec:   dur,
			SizeBytes:     sz,
			AdDurationSec: origDur - dur,
		}
	} else if fileExists(cutsFile) && fileExists(transcriptFile) {
		data, err := os.ReadFile(cutsFile)
		var cd CutsData
		if err == nil && json.Unmarshal(data, &cd) == nil && len(cd.CutIntervals) == 0 {
			st.Status = StateDone
			st.Cleaned = EpisodeAudioMeta{
				Filename:    fname,
				DurationSec: dur,
				SizeBytes:   sz,
			}
		}
	}
}

func populateAdsFromCutsFile(st *EpisodeStatusFile, cutsFile string) {
	if !fileExists(cutsFile) {
		return
	}
	data, err := os.ReadFile(cutsFile)
	if err != nil {
		return
	}
	var cd CutsData
	if json.Unmarshal(data, &cd) != nil || len(cd.CutIntervals) == 0 {
		return
	}
	for _, c := range cd.CutIntervals {
		st.Ads = append(st.Ads, EpisodeAdCut{
			Start:  c.StartSec,
			End:    c.EndSec,
			Reason: c.Reason,
		})
	}
}

func updateEpisodeStatus(audioPath string, mutate func(*EpisodeStatusFile)) error {
	statPath := statusPathFor(audioPath)
	st := getOrCreateEpisodeStatus(audioPath)
	mutate(st)
	if err := saveEpisodeStatus(statPath, st); err != nil {
		return fmt.Errorf("could not record status for %s: %w", audioPath, err)
	}
	return nil
}

func isEpisodeCompleted(audioPath string) bool {
	statPath := statusPathFor(audioPath)
	st, err := loadEpisodeStatus(statPath)
	if err == nil && st != nil {
		if st.Status == StateDone || st.Status == StateCopiedBack || st.Status == StateArchived {
			return true
		}
	}
	base := stripExt(audioPath)
	cutsFile := base + ".cuts.json"
	transcriptFile := base + ".transcript.json"
	precutFile := audioPath + ".precut"
	if fileExists(cutsFile) && fileExists(transcriptFile) {
		if fileExists(precutFile) {
			return true
		}
		data, err := os.ReadFile(cutsFile)
		var cd CutsData
		if err == nil && json.Unmarshal(data, &cd) == nil && len(cd.CutIntervals) == 0 {
			return true
		}
	}
	return false
}

func isEpisodeInRemoteFlight(audioPath string) bool {
	statPath := statusPathFor(audioPath)
	st, err := loadEpisodeStatus(statPath)
	if err == nil && st != nil {
		switch st.Status {
		case StateQueuedRemote, StateTranscribingRemotely, StateCuttingRemotely, StateReadyForCopyBack, StateAwaitingTranscription:
			return true
		}
	}
	return false
}

func computeRelativeMediaDir(baseDir, fullPath string) (string, error) {
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", err
	}
	absTarget, err := filepath.Abs(fullPath)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil || strings.HasPrefix(rel, "..") {
		return filepath.Base(fullPath), nil
	}
	return rel, nil
}
