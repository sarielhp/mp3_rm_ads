package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

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
	if len(p.Sinks) <= 1 {
		p.mu.Unlock()
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
	p.CurrentSpeaker = target.Description
	p.mu.Unlock()

	_ = exec.Command("pactl", "set-default-sink", target.Name).Run()
	_ = exec.Command("wpctl", "set-default", target.ID).Run()
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
