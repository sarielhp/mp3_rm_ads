package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func findAudioFilesForRemote(paths []string, defaultDir string) []string {
	var targetPaths []string
	if len(paths) > 0 {
		targetPaths = paths
	} else if defaultDir != "" {
		targetPaths = []string{defaultDir}
	} else {
		return nil
	}

	var results []string
	seen := make(map[string]bool)

	for _, p := range targetPaths {
		fi, err := os.Stat(p)
		if err != nil {
			continue
		}
		if fi.IsDir() {
			if strings.HasSuffix(filepath.Clean(p), "-1") {
				continue
			}
			mp3s := findMP3Files(p)
			for _, m := range mp3s {
				absM, err := filepath.Abs(m)
				if err == nil && !seen[absM] && !strings.Contains(absM, "/.work/") && !strings.HasSuffix(absM, ".precut") {
					if strings.Contains(absM, "-1/") || filepath.Base(absM) == "podcast.mp3" {
						continue
					}
					seen[absM] = true
					results = append(results, absM)
				}
			}
		} else {
			if strings.HasSuffix(strings.ToLower(p), ".mp3") {
				absP, err := filepath.Abs(p)
				if err == nil && !seen[absP] {
					seen[absP] = true
					results = append(results, absP)
				}
			}
		}
	}
	return results
}

func sortAudioFilesByQueuePolicy(files []string, now time.Time) {
	if len(files) <= 1 {
		return
	}
	if now.IsZero() {
		now = time.Now()
	}
	durMap := make(map[string]float64, len(files))
	priMap := make(map[string]int, len(files))
	pubMap := make(map[string]time.Time, len(files))
	recentMap := make(map[string]bool, len(files))

	for _, f := range files {
		durMap[f] = getEpisodeDurationForQueue(f)
		priMap[f] = getEpisodePriorityForQueue(f)
		isRec, pt := isEpisodeRecent24h(f, now)
		pubMap[f] = pt
		recentMap[f] = isRec
	}

	sort.SliceStable(files, func(i, j int) bool {
		fi := files[i]
		fj := files[j]

		pi := priMap[fi]
		pj := priMap[fj]
		if pi != pj {
			return pi > pj
		}

		recI := recentMap[fi]
		recJ := recentMap[fj]
		if recI != recJ {
			return recI && !recJ
		}

		if recI {
			pubI := pubMap[fi]
			pubJ := pubMap[fj]
			if !pubI.Equal(pubJ) {
				return pubI.After(pubJ)
			}
			di := durMap[fi]
			dj := durMap[fj]
			if di != dj {
				return di < dj
			}
			return fi < fj
		}

		di := durMap[fi]
		dj := durMap[fj]
		if di != dj {
			return di < dj
		}
		pubI := pubMap[fi]
		pubJ := pubMap[fj]
		if !pubI.Equal(pubJ) {
			return pubI.After(pubJ)
		}
		return fi < fj
	})
}

func sortAudioFilesByDuration(files []string) {
	sortAudioFilesByQueuePolicy(files, time.Now())
}

func getEpisodeDurationForQueue(audioPath string) float64 {
	statPath := statusPathFor(audioPath)
	if st, err := loadEpisodeStatus(statPath); err == nil && st != nil {
		if st.Original.DurationSec > 0 {
			return st.Original.DurationSec
		}
	}
	dur := getAudioDuration(audioPath)
	if dur <= 0 {
		dur = getMP3DiskDuration(audioPath)
	}
	if dur <= 0 {
		if fi, err := os.Stat(audioPath); err == nil && fi.Size() > 0 {
			return float64(fi.Size()) / 16000.0
		}
	}
	return dur
}

func getEpisodePriorityForQueue(audioPath string) int {
	statPath := statusPathFor(audioPath)
	if st, err := loadEpisodeStatus(statPath); err == nil && st != nil {
		return st.Priority
	}
	return 0
}

func isEpisodeRecent24h(audioPath string, now time.Time) (bool, time.Time) {
	pt := getEpisodePublicationTime(audioPath)
	if pt.IsZero() {
		return false, pt
	}
	cutoff := now.Add(-24 * time.Hour)
	return pt.After(cutoff), pt
}
