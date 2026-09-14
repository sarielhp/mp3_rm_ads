package player

import (
	"fmt"
	"strings"

	"pod/pkg/types"
)

func (p *AudioPlayer) RenderProgressBar(width int) string {
	if width < 10 {
		width = 10
	}
	p.UpdatePosition()
	p.mu.Lock()
	pos := p.Position
	dur := p.Duration
	p.mu.Unlock()

	cur := FormatPlayerTime(pos)
	tot := FormatPlayerTime(dur)

	timeText := fmt.Sprintf(" %s / %s", cur, tot)
	barWidth := width - len(timeText) - 2
	if barWidth < 5 {
		barWidth = 5
	}

	progress := 0.0
	if dur > 0 {
		progress = pos / dur
		if progress > 1.0 {
			progress = 1.0
		}
	}

	filled := int(progress * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	empty := barWidth - filled

	bar := "[" + strings.Repeat("=", filled)
	if empty > 0 {
		bar += ">" + strings.Repeat("─", empty-1)
	}
	bar += "]" + timeText
	return bar
}

func (p *AudioPlayer) RenderVolumeBar(width int) string {
	if width < 8 {
		width = 8
	}
	p.mu.Lock()
	pct := p.Volume
	muted := p.Muted
	p.mu.Unlock()

	if pct < 0 {
		pct = 0
	}
	if pct > 150 {
		pct = 150
	}
	barWidth := 10
	filled := int(float64(pct) / 100.0 * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	empty := barWidth - filled
	if empty < 0 {
		empty = 0
	}

	status := fmt.Sprintf("%d%%", pct)
	if muted {
		status = "MUTED"
	}
	return fmt.Sprintf("[%s%s] %s", strings.Repeat("█", filled), strings.Repeat("░", empty), status)
}

func (p *AudioPlayer) View() types.PlayerView {
	p.UpdatePosition()
	p.mu.Lock()
	defer p.mu.Unlock()

	v := types.PlayerView{
		IsPlaying:      p.IsPlaying,
		IsPaused:       p.IsPaused,
		Position:       p.Position,
		Duration:       p.Duration,
		Volume:         p.Volume,
		Muted:          p.Muted,
		CurrentSpeaker: p.CurrentSpeaker,
		LastError:      p.LastError,
		Queue:          append([]types.PlayerTrack(nil), p.Queue...),
	}
	if p.Current != nil {
		v.Has = true
		v.Title = p.Current.Title
		v.Podcast = p.Current.Podcast
		v.Path = p.Current.Path
	} else if !IsAudioSpawnDisabled() && IsPlayerSocketAlive() {
		if st, err := QueryPlayerStatus(); err == nil && st != nil && st.IsRunning {
			v.Has = true
			v.Title = st.Title
			v.Position = st.Position
			v.Duration = st.Duration
			v.IsPlaying = true
			v.IsPaused = st.IsPaused
		}
	}
	return v
}
