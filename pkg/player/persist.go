package player

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/sariel/abs/pkg/types"
	"github.com/sariel/abs/pkg/util"
)

func GetPlayQueueFilePath() string {
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
	filePath := GetPlayQueueFilePath()
	data := types.PlayQueuePersist{
		Current:  p.Current,
		Queue:    p.Queue,
		Position: p.Position,
	}
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return
	}
	_ = util.WriteFileAtomic(filePath, bytes, 0600)
}

func (p *AudioPlayer) LoadQueueFromFile() {
	filePath := GetPlayQueueFilePath()
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		return
	}
	var data types.PlayQueuePersist
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

func (p *AudioPlayer) GetUnifiedQueue() []types.UnifiedQueueItem {
	p.mu.Lock()
	defer p.mu.Unlock()

	var items []types.UnifiedQueueItem
	if p.Current != nil {
		items = append(items, types.UnifiedQueueItem{
			Track:     *p.Current,
			IsCurrent: true,
			IsPlaying: p.IsPlaying,
			IsPaused:  p.IsPaused,
			Position:  p.Position,
			Duration:  p.Duration,
		})
	}
	for _, q := range p.Queue {
		items = append(items, types.UnifiedQueueItem{
			Track: q,
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

	var all []types.PlayerTrack
	if hasCurrent {
		all = append(all, *p.Current)
	}
	all = append(all, p.Queue...)

	item := all[from]
	all = append(all[:from], all[from+1:]...)
	all = append(all[:to], append([]types.PlayerTrack{item}, all[to:]...)...)

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
