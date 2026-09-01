package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func loadAllQueues(pods []tuiPodcast) map[string][]string {
	q := make(map[string][]string)
	for _, pod := range pods {
		data, err := os.ReadFile(filepath.Join(pod.dir, "queue.json"))
		if err != nil {
			q[pod.dir] = nil
			continue
		}
		var entries []string
		if err := json.Unmarshal(data, &entries); err != nil {
			q[pod.dir] = nil
			continue
		}

		adFreeMap := make(map[string]bool)
		for _, ep := range pod.episodes {
			if ep.hasAdsRemoved {
				adFreeMap[ep.filename] = true
			}
		}

		var filtered []string
		needsResave := false
		for _, e := range entries {
			if !strings.HasSuffix(strings.ToLower(e), ".mp3") {
				needsResave = true
				continue
			}
			if adFreeMap[e] {
				needsResave = true
				continue
			}
			filtered = append(filtered, e)
		}
		q[pod.dir] = filtered
		if needsResave {
			saveQueue(pod.dir, filtered)
		}
	}
	return q
}

func saveQueue(dir string, entries []string) {
	if entries == nil {
		entries = []string{}
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(filepath.Join(dir, "queue.json"), data, 0644)
}
