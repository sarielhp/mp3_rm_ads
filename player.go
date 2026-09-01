package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
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
	mu             syncMutex
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

var playerSpawnEnabled = true

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

	if !playerSpawnEnabled {
		p.IsPlaying = true
		p.IsPaused = false
		p.startPlayTime = time.Now()
		p.startOffsetSec = startSec
		return
	}

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
