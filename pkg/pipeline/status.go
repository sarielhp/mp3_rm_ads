package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"abs/pkg/audio"
	"abs/pkg/types"
	"abs/pkg/util"
)

func StatusPathFor(audioPath string) string {
	return audioPath + ".json"
}

func LoadEpisodeStatus(path string) (*types.EpisodeStatusFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var st types.EpisodeStatusFile
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("invalid episode status json in %s: %w", path, err)
	}
	return &st, nil
}

func SaveEpisodeStatus(path string, st *types.EpisodeStatusFile) error {
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
	return util.WriteFileAtomic(path, append(data, '\n'), 0644)
}

func GetOrCreateEpisodeStatus(audioPath string) *types.EpisodeStatusFile {
	statPath := StatusPathFor(audioPath)
	if st, err := LoadEpisodeStatus(statPath); err == nil && st != nil {
		return st
	}
	now := time.Now().UTC().Format(time.RFC3339)
	fname := filepath.Base(audioPath)
	var sz int64
	if fi, err := os.Stat(audioPath); err == nil {
		sz = fi.Size()
	}
	dur := audio.GetAudioDuration(audioPath)
	pubStr := ""
	if fi, err := os.Stat(audioPath); err == nil {
		pubStr = fi.ModTime().UTC().Format(time.RFC3339)
	}
	st := &types.EpisodeStatusFile{
		Version:     1,
		MediaFile:   fname,
		Status:      types.StateDownloaded,
		CreatedAt:   now,
		UpdatedAt:   now,
		PublishedAt: pubStr,
		Original: types.EpisodeAudioMeta{
			Filename:    fname,
			DurationSec: dur,
			SizeBytes:   sz,
		},
	}

	PopulatePrecutOrCutsMeta(st, audioPath, fname, dur, sz)
	PopulateAdsFromCutsFile(st, util.StripExt(audioPath)+".cuts.json")

	_ = SaveEpisodeStatus(statPath, st)
	return st
}

func PopulatePrecutOrCutsMeta(st *types.EpisodeStatusFile, audioPath, fname string, dur float64, sz int64) {
	base := util.StripExt(audioPath)
	cutsFile := base + ".cuts.json"
	transcriptFile := base + ".transcript.json"
	precutFile := audioPath + ".precut"

	if util.FileExists(precutFile) {
		var origSz int64
		if fi, err := os.Stat(precutFile); err == nil {
			origSz = fi.Size()
		}
		origDur := audio.GetAudioDuration(precutFile)
		st.Status = types.StateDone
		st.Original = types.EpisodeAudioMeta{
			Filename:    filepath.Base(precutFile),
			DurationSec: origDur,
			SizeBytes:   origSz,
		}
		st.Cleaned = types.EpisodeAudioMeta{
			Filename:      fname,
			DurationSec:   dur,
			SizeBytes:     sz,
			AdDurationSec: origDur - dur,
		}
	} else if util.FileExists(cutsFile) && util.FileExists(transcriptFile) {
		data, err := os.ReadFile(cutsFile)
		var cd types.CutsData
		if err == nil && json.Unmarshal(data, &cd) == nil && len(cd.CutIntervals) == 0 {
			st.Status = types.StateDone
			st.Cleaned = types.EpisodeAudioMeta{
				Filename:    fname,
				DurationSec: dur,
				SizeBytes:   sz,
			}
		}
	}
}

func PopulateAdsFromCutsFile(st *types.EpisodeStatusFile, cutsFile string) {
	if !util.FileExists(cutsFile) {
		return
	}
	data, err := os.ReadFile(cutsFile)
	if err != nil {
		return
	}
	var cd types.CutsData
	if json.Unmarshal(data, &cd) != nil || len(cd.CutIntervals) == 0 {
		return
	}
	for _, c := range cd.CutIntervals {
		st.Ads = append(st.Ads, types.EpisodeAdCut{
			Start:  c.StartSec,
			End:    c.EndSec,
			Reason: c.Reason,
		})
	}
}

func UpdateEpisodeStatus(audioPath string, mutate func(*types.EpisodeStatusFile)) error {
	statPath := StatusPathFor(audioPath)
	st := GetOrCreateEpisodeStatus(audioPath)
	mutate(st)
	if err := SaveEpisodeStatus(statPath, st); err != nil {
		return fmt.Errorf("could not record status for %s: %w", audioPath, err)
	}
	return nil
}

func IsEpisodeCompleted(audioPath string) bool {
	statPath := StatusPathFor(audioPath)
	st, err := LoadEpisodeStatus(statPath)
	if err == nil && st != nil {
		if st.AdDetectionSuccessful != nil && !*st.AdDetectionSuccessful {
			return false
		}
		if st.Status == types.StateNeedsAdR || st.Status == types.StateFailed {
			return false
		}
		if st.Status == types.StateDone || st.Status == types.StateCopiedBack || st.Status == types.StateArchived {
			return true
		}
	}
	base := util.StripExt(audioPath)
	cutsFile := base + ".cuts.json"
	transcriptFile := base + ".transcript.json"
	precutFile := audioPath + ".precut"
	if util.FileExists(cutsFile) && util.FileExists(transcriptFile) {
		if util.FileExists(precutFile) {
			return true
		}
		data, err := os.ReadFile(cutsFile)
		var cd types.CutsData
		if err == nil && json.Unmarshal(data, &cd) == nil && len(cd.CutIntervals) == 0 {
			return true
		}
	}
	return false
}

func IsEpisodeInRemoteFlight(audioPath string) bool {
	statPath := StatusPathFor(audioPath)
	st, err := LoadEpisodeStatus(statPath)
	if err == nil && st != nil {
		switch st.Status {
		case types.StateQueuedRemote, types.StateTranscribingRemotely, types.StateCuttingRemotely, types.StateReadyForCopyBack, types.StateAwaitingTranscription:
			return true
		}
	}
	return false
}

func ComputeRelativeMediaDir(baseDir, fullPath string) (string, error) {
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
