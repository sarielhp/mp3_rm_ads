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
