package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	debugLoggerMu sync.Mutex
	debugFile     *os.File
	debugEnabled  bool
	snapshotSeq   int
	ansiRegex     = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b_[^\x1b]*\x1b\\`)
)

func debugBaseDir() string {
	cacheHome := os.Getenv("XDG_CACHE_HOME")
	if cacheHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			cacheHome = filepath.Join(userTmpDir(), "cache")
		} else {
			cacheHome = filepath.Join(home, ".cache")
		}
	}
	dir := filepath.Join(cacheHome, "abs", "debug")
	_ = os.MkdirAll(dir, 0755)
	_ = os.MkdirAll(filepath.Join(dir, "snapshots"), 0755)
	return dir
}

func initDebugLogger(enabled bool) {
	debugLoggerMu.Lock()
	defer debugLoggerMu.Unlock()

	debugEnabled = enabled || os.Getenv("ABS_DEBUG") == "1" || os.Getenv("ABS_DEBUG") == "true"
	if !debugEnabled || debugFile != nil {
		return
	}

	logPath := filepath.Join(debugBaseDir(), "abs_debug.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		debugFile = f
		now := time.Now().Format("2006-01-02 15:04:05.000")
		_, _ = fmt.Fprintf(debugFile, "\n=== ABS TUI Debug Session Started at %s ===\n", now)
		_, _ = fmt.Fprintf(debugFile, "PID: %d | TERM: %s | KittyTerminal: %v\n", os.Getpid(), os.Getenv("TERM"), isKittyTerminal())
	}
}

func debugLog(format string, args ...interface{}) {
	debugLoggerMu.Lock()
	defer debugLoggerMu.Unlock()

	if !debugEnabled && os.Getenv("ABS_DEBUG") != "1" && os.Getenv("ABS_DEBUG") != "true" {
		return
	}

	if debugFile == nil {
		logPath := filepath.Join(debugBaseDir(), "abs_debug.log")
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return
		}
		debugFile = f
		debugEnabled = true
	}

	now := time.Now().Format("15:04:05.000")
	msg := fmt.Sprintf(format, args...)
	_, _ = fmt.Fprintf(debugFile, "[%s] %s\n", now, msg)
}

func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

func (m *tuiModel) screenName() string {
	switch m.screen {
	case screenPodcasts:
		return "screenPodcasts"
	case screenPodcastDetail:
		return "screenPodcastDetail"
	case screenEpisodeDetail:
		return "screenEpisodeDetail"
	case screenPlayer:
		return "screenPlayer"
	case screenPlayQueue:
		return "screenPlayQueue"
	case screenAdQueue:
		return "screenAdQueue"
	case screenTranscript:
		return "screenTranscript"
	case screenTimeline:
		return "screenTimeline"
	default:
		return fmt.Sprintf("screen(%d)", m.screen)
	}
}

func (m *tuiModel) takeSnapshot() string {
	debugLoggerMu.Lock()
	snapshotSeq++
	seq := snapshotSeq
	debugLoggerMu.Unlock()

	initDebugLogger(true)

	snapDir := filepath.Join(debugBaseDir(), "snapshots")
	_ = os.MkdirAll(snapDir, 0755)

	ts := time.Now().Format("20060102_150405")
	rawFile := filepath.Join(snapDir, fmt.Sprintf("snapshot_%03d_%s_raw.ansi", seq, ts))
	txtFile := filepath.Join(snapDir, fmt.Sprintf("snapshot_%03d_%s.txt", seq, ts))

	viewContent := m.View()

	_ = os.WriteFile(rawFile, []byte(viewContent), 0644)

	var header strings.Builder
	header.WriteString(fmt.Sprintf("=== Snapshot #%03d (%s) ===\n", seq, time.Now().Format("2006-01-02 15:04:05")))
	header.WriteString(fmt.Sprintf("Screen: %s | Dimensions: %dx%d\n", m.screenName(), m.width, m.height))
	if m.podIdx < len(m.podcasts) {
		header.WriteString(fmt.Sprintf("Selected Podcast: %s (idx: %d/%d)\n", m.podcasts[m.podIdx].name, m.podIdx+1, len(m.podcasts)))
		eps := m.filteredEpisodes()
		if m.epIdx < len(eps) {
			header.WriteString(fmt.Sprintf("Selected Episode: %s (idx: %d/%d)\n", eps[m.epIdx].filename, m.epIdx+1, len(eps)))
		}
	}
	if m.screen == screenTranscript {
		header.WriteString(fmt.Sprintf("Transcript Scroll: %d/%d | ViewMode: %d\n", m.transcriptScroll, len(m.transcriptItems), m.transcriptViewMode))
	}
	header.WriteString("------------------------------------------------------------------------\n\n")

	plainText := stripANSI(viewContent)
	fullPlain := header.String() + plainText

	_ = os.WriteFile(txtFile, []byte(fullPlain), 0644)

	debugLog("SNAPSHOT #%03d taken on %s -> %s", seq, m.screenName(), filepath.Base(txtFile))

	return txtFile
}
