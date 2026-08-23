package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type PlayQueuePersist struct {
	Current  *PlayerTrack  `json:"current,omitempty"`
	Queue    []PlayerTrack `json:"queue"`
	Position float64       `json:"position,omitempty"`
}

type AdQueueItem struct {
	PodcastDir    string
	PodcastName   string
	Filename      string
	Title         string
	HasAdsRemoved bool
	PublishedAt   int64
	Duration      float64
}

func getPlayQueueFilePath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	dir := filepath.Join(configDir, "abs")
	_ = os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "play_queue.json")
}

func (p *AudioPlayer) SaveQueueToFile() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.saveQueueLocked()
}

func (p *AudioPlayer) saveQueueLocked() {
	filePath := getPlayQueueFilePath()
	data := PlayQueuePersist{
		Current:  p.Current,
		Queue:    p.Queue,
		Position: p.Position,
	}
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(filePath, bytes, 0644)
}

func (p *AudioPlayer) LoadQueueFromFile() {
	filePath := getPlayQueueFilePath()
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		return
	}
	var data PlayQueuePersist
	if err := json.Unmarshal(bytes, &data); err != nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.Queue = data.Queue
	if p.Current == nil && data.Current != nil {
		p.Current = data.Current
		p.Position = data.Position
		p.Duration = data.Current.Duration
	}
}

type UnifiedQueueItem struct {
	Track     PlayerTrack
	IsCurrent bool
	IsPlaying bool
	IsPaused  bool
	Position  float64
	Duration  float64
}

func (p *AudioPlayer) GetUnifiedQueue() []UnifiedQueueItem {
	p.mu.Lock()
	defer p.mu.Unlock()

	var items []UnifiedQueueItem
	if p.Current != nil {
		items = append(items, UnifiedQueueItem{
			Track:     *p.Current,
			IsCurrent: true,
			IsPlaying: p.IsPlaying,
			IsPaused:  p.IsPaused,
			Position:  p.Position,
			Duration:  p.Duration,
		})
	}
	for _, q := range p.Queue {
		items = append(items, UnifiedQueueItem{
			Track:    q,
			Duration: q.Duration,
		})
	}
	return items
}

func (p *AudioPlayer) MoveUnifiedItem(from, to int) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	hasCurrent := (p.Current != nil)
	total := len(p.Queue)
	if hasCurrent {
		total++
	}

	if from < 0 || from >= total || to < 0 || to >= total || from == to {
		return false
	}

	var all []PlayerTrack
	if hasCurrent {
		all = append(all, *p.Current)
	}
	all = append(all, p.Queue...)

	item := all[from]
	all = append(all[:from], all[from+1:]...)
	all = append(all[:to], append([]PlayerTrack{item}, all[to:]...)...)

	if hasCurrent {
		if from == 0 || to == 0 {
			p.Current = &all[0]
			p.Duration = all[0].Duration
			p.Queue = all[1:]
			p.Position = 0
			p.startOffsetSec = 0
			p.saveQueueLocked()
			p.startProcessLocked(0)
			return true
		}
		p.Queue = all[1:]
	} else {
		p.Queue = all
	}

	p.saveQueueLocked()
	return true
}

func (p *AudioPlayer) RemoveUnifiedItem(idx int) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	hasCurrent := (p.Current != nil)
	if hasCurrent {
		if idx == 0 {
			p.nextLocked()
			return true
		}
		queueIdx := idx - 1
		if queueIdx < 0 || queueIdx >= len(p.Queue) {
			return false
		}
		p.Queue = append(p.Queue[:queueIdx], p.Queue[queueIdx+1:]...)
		p.saveQueueLocked()
		return true
	}

	if idx < 0 || idx >= len(p.Queue) {
		return false
	}
	p.Queue = append(p.Queue[:idx], p.Queue[idx+1:]...)
	p.saveQueueLocked()
	return true
}

func getAllAdQueueItems(pods []tuiPodcast, q map[string][]string) []AdQueueItem {
	var list []AdQueueItem
	for _, pod := range pods {
		filenames := q[pod.dir]
		for _, fn := range filenames {
			item := AdQueueItem{
				PodcastDir:  pod.dir,
				PodcastName: pod.name,
				Filename:    fn,
				Title:       fn,
			}
			for _, ep := range pod.episodes {
				if ep.filename == fn {
					item.Title = ep.displayTitle()
					item.HasAdsRemoved = ep.hasAdsRemoved
					item.PublishedAt = ep.publishedAt
					item.Duration = ep.duration
					break
				}
			}
			list = append(list, item)
		}
	}
	return list
}

func removeAdQueueItem(item AdQueueItem, q map[string][]string, saveFn func(dir string, entries []string)) {
	entries := q[item.PodcastDir]
	var updated []string
	for _, fn := range entries {
		if fn != item.Filename {
			updated = append(updated, fn)
		}
	}
	q[item.PodcastDir] = updated
	if saveFn != nil {
		saveFn(item.PodcastDir, updated)
	}
}

func moveAdQueueItem(items []AdQueueItem, from, to int, q map[string][]string, saveFn func(dir string, entries []string)) {
	if from < 0 || from >= len(items) || to < 0 || to >= len(items) || from == to {
		return
	}
	// For each podcast, update its order based on the new relative order in items
	item := items[from]
	targetItem := items[to]
	if item.PodcastDir == targetItem.PodcastDir {
		entries := q[item.PodcastDir]
		fromIdx, toIdx := -1, -1
		for i, fn := range entries {
			if fn == item.Filename {
				fromIdx = i
			}
			if fn == targetItem.Filename {
				toIdx = i
			}
		}
		if fromIdx != -1 && toIdx != -1 {
			fn := entries[fromIdx]
			entries = append(entries[:fromIdx], entries[fromIdx+1:]...)
			entries = append(entries[:toIdx], append([]string{fn}, entries[toIdx:]...)...)
			q[item.PodcastDir] = entries
			if saveFn != nil {
				saveFn(item.PodcastDir, entries)
			}
		}
	}
}
