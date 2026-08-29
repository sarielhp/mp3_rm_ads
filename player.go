package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type PlayerTrack struct {
	Title    string
	Podcast  string
	Path     string
	Duration float64
}

type AudioSink struct {
	ID          string
	Name        string
	Description string
	IsDefault   bool
}

type AudioPlayer struct {
	mu             sync.Mutex
	Current        *PlayerTrack
	Queue          []PlayerTrack
	IsPlaying      bool
	IsPaused       bool
	Position       float64
	Duration       float64
	cmd            *exec.Cmd
	startPlayTime  time.Time
	startOffsetSec float64
	Volume         int
	Muted          bool
	CurrentSpeaker string
	Sinks          []AudioSink
	isStarting     bool
}

var globalPlayer = &AudioPlayer{
	Volume: 70,
}

func init() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
	go func() {
		<-c
		globalPlayer.Stop()
		os.Exit(0)
	}()
	globalPlayer.LoadQueueFromFile()
}

func (p *AudioPlayer) EnqueueAndPlay(track PlayerTrack) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Current != nil && p.Current.Path == track.Path {
		return false
	}

	for _, q := range p.Queue {
		if q.Path == track.Path {
			return false
		}
	}

	if p.IsPlaying && p.Current != nil {
		p.Queue = append(p.Queue, track)
		p.saveQueueLocked()
		return true
	}

	p.playTrackLocked(track)
	return true
}

func (p *AudioPlayer) PlayTrack(track PlayerTrack) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.playTrackLocked(track)
}

func (p *AudioPlayer) playTrackLocked(track PlayerTrack) {
	p.stopLocked()
	p.Current = &track
	p.Duration = track.Duration
	if p.Duration <= 0 {
		p.Duration = getAudioDuration(track.Path)
	}
	p.Position = 0
	p.startOffsetSec = 0
	p.saveQueueLocked()
	p.startProcessLocked(0)
}

func (p *AudioPlayer) startProcessLocked(startSec float64) {
	if p.Current == nil || p.Current.Path == "" || p.isStarting {
		return
	}
	p.isStarting = true
	defer func() {
		p.isStarting = false
	}()

	p.killProcessLocked()

	args := []string{"-nodisp", "-autoexit", "-loglevel", "quiet"}
	if startSec > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.2f", startSec))
	}
	args = append(args, p.Current.Path)

	cmd := exec.Command("ffplay", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		mpgArgs := []string{"-q"}
		if startSec > 0 {
			mpgArgs = append(mpgArgs, "-k", fmt.Sprintf("%d", int(startSec*38.28)))
		}
		mpgArgs = append(mpgArgs, p.Current.Path)
		cmd = exec.Command("mpg123", mpgArgs...)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		_ = cmd.Start()
	}

	p.cmd = cmd
	p.IsPlaying = true
	p.IsPaused = false
	p.startPlayTime = time.Now()
	p.startOffsetSec = startSec

	go func(targetCmd *exec.Cmd) {
		if targetCmd == nil {
			return
		}
		_ = targetCmd.Wait()
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.cmd == targetCmd {
			p.cmd = nil
			p.nextLocked()
		}
	}(cmd)
}

func (p *AudioPlayer) killProcessLocked() {
	if p.cmd != nil && p.cmd.Process != nil {
		if pgid, err := syscall.Getpgid(p.cmd.Process.Pid); err == nil {
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		} else {
			_ = p.cmd.Process.Kill()
		}
		p.cmd = nil
	}
}

func (p *AudioPlayer) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopLocked()
}

func (p *AudioPlayer) stopLocked() {
	p.killProcessLocked()
	p.IsPlaying = false
	p.IsPaused = false
	p.Position = 0
	p.Current = nil
}

func (p *AudioPlayer) TogglePause() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.IsPlaying || p.Current == nil {
		return
	}

	p.updatePositionLocked()

	if p.IsPaused {
		p.startProcessLocked(p.Position)
	} else {
		p.IsPaused = true
		if p.cmd != nil && p.cmd.Process != nil {
			_ = p.cmd.Process.Signal(syscall.SIGSTOP)
		}
	}
}

func (p *AudioPlayer) Seek(deltaSec float64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Current == nil {
		return
	}
	p.updatePositionLocked()
	newPos := p.Position + deltaSec
	if newPos < 0 {
		newPos = 0
	}
	if p.Duration > 0 && newPos > p.Duration {
		newPos = p.Duration
	}
	p.Position = newPos
	if !p.IsPaused {
		p.startProcessLocked(newPos)
	}
}

func (p *AudioPlayer) Next() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.nextLocked()
}

func (p *AudioPlayer) nextLocked() {
	if len(p.Queue) > 0 {
		next := p.Queue[0]
		p.Queue = p.Queue[1:]
		p.playTrackLocked(next)
	} else {
		p.stopLocked()
	}
}

func (p *AudioPlayer) ClearQueue() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Queue = nil
	p.saveQueueLocked()
}

func (p *AudioPlayer) MoveTrack(from, to int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if from < 0 || from >= len(p.Queue) || to < 0 || to >= len(p.Queue) || from == to {
		return
	}
	track := p.Queue[from]
	p.Queue = append(p.Queue[:from], p.Queue[from+1:]...)
	p.Queue = append(p.Queue[:to], append([]PlayerTrack{track}, p.Queue[to:]...)...)
	p.saveQueueLocked()
}

func (p *AudioPlayer) RemoveTrack(idx int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if idx < 0 || idx >= len(p.Queue) {
		return
	}
	p.Queue = append(p.Queue[:idx], p.Queue[idx+1:]...)
	p.saveQueueLocked()
}

func (p *AudioPlayer) PlayQueueIndex(idx int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if idx < 0 || idx >= len(p.Queue) {
		return
	}
	track := p.Queue[idx]
	p.Queue = append(p.Queue[:idx], p.Queue[idx+1:]...)
	p.playTrackLocked(track)
}

func (p *AudioPlayer) UpdatePosition() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.updatePositionLocked()
}

func (p *AudioPlayer) updatePositionLocked() {
	if p.IsPlaying && !p.IsPaused {
		elapsed := time.Since(p.startPlayTime).Seconds()
		p.Position = p.startOffsetSec + elapsed
		if p.Duration > 0 && p.Position > p.Duration {
			p.Position = p.Duration
		}
	}
}

func (p *AudioPlayer) VolumeUp() {
	_ = exec.Command("pactl", "set-sink-volume", "@DEFAULT_SINK@", "+5%").Run()
	_ = exec.Command("wpctl", "set-volume", "@DEFAULT_AUDIO_SINK@", "5%+").Run()
	p.RefreshAudioInfo()
}

func (p *AudioPlayer) VolumeDown() {
	_ = exec.Command("pactl", "set-sink-volume", "@DEFAULT_SINK@", "-5%").Run()
	_ = exec.Command("wpctl", "set-volume", "@DEFAULT_AUDIO_SINK@", "5%-").Run()
	p.RefreshAudioInfo()
}

func (p *AudioPlayer) ToggleMute() {
	_ = exec.Command("pactl", "set-sink-mute", "@DEFAULT_SINK@", "toggle").Run()
	_ = exec.Command("wpctl", "set-mute", "@DEFAULT_AUDIO_SINK@", "toggle").Run()
	p.RefreshAudioInfo()
}

func (p *AudioPlayer) RefreshAudioInfo() {
	var vol int
	var hasVol bool
	out, err := exec.Command("pactl", "get-sink-volume", "@DEFAULT_SINK@").Output()
	if err == nil {
		str := string(out)
		if idx := strings.Index(str, "%"); idx != -1 {
			start := idx - 1
			for start >= 0 && str[start] >= '0' && str[start] <= '9' {
				start--
			}
			if num, err := strconv.Atoi(str[start+1 : idx]); err == nil {
				vol = num
				hasVol = true
			}
		}
	}

	var muted bool
	var hasMuted bool
	muteOut, err := exec.Command("pactl", "get-sink-mute", "@DEFAULT_SINK@").Output()
	if err == nil {
		muted = strings.Contains(strings.ToLower(string(muteOut)), "yes")
		hasMuted = true
	}

	if hasVol || hasMuted {
		p.mu.Lock()
		defer p.mu.Unlock()
		if hasVol {
			p.Volume = vol
		}
		if hasMuted {
			p.Muted = muted
		}
	}

	p.RefreshSinks()
}

func (p *AudioPlayer) RefreshSinks() {
	out, err := exec.Command("pactl", "list", "sinks", "short").Output()
	if err != nil {
		return
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var sinks []AudioSink
	for _, l := range lines {
		parts := strings.Fields(l)
		if len(parts) >= 2 {
			id := parts[0]
			name := parts[1]
			desc := name
			if strings.Contains(name, "usb") || strings.Contains(name, "Scarlett") || strings.Contains(name, "Focusrite") {
				desc = "Focusrite USB Audio"
			} else if strings.Contains(name, "hdmi") {
				desc = "HDMI / Monitor Audio"
			} else if strings.Contains(name, "analog") {
				desc = "Built-in Speakers / Headphones"
			}
			sinks = append(sinks, AudioSink{
				ID:          id,
				Name:        name,
				Description: desc,
			})
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Sinks = sinks
	if len(sinks) > 0 && p.CurrentSpeaker == "" {
		p.CurrentSpeaker = sinks[0].Description
	}
}

func (p *AudioPlayer) CycleSpeaker() {
	p.RefreshSinks()
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.Sinks) <= 1 {
		return
	}
	curIdx := 0
	for i, s := range p.Sinks {
		if s.Description == p.CurrentSpeaker || s.Name == p.CurrentSpeaker {
			curIdx = i
			break
		}
	}
	nextIdx := (curIdx + 1) % len(p.Sinks)
	target := p.Sinks[nextIdx]
	_ = exec.Command("pactl", "set-default-sink", target.Name).Run()
	_ = exec.Command("wpctl", "set-default", target.ID).Run()
	p.CurrentSpeaker = target.Description
}

func (p *AudioPlayer) RenderProgressBar(width int) string {
	if width < 10 {
		width = 10
	}
	p.UpdatePosition()
	p.mu.Lock()
	pos := p.Position
	dur := p.Duration
	p.mu.Unlock()

	cur := formatPlayerTime(pos)
	tot := formatPlayerTime(dur)

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

func formatPlayerTime(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	totalSec := int(seconds)
	h := totalSec / 3600
	m := (totalSec % 3600) / 60
	s := totalSec % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}
