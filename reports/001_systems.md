# Systems Code Review Report #001 (Lens: systems)

- **Date**: 2026-09-05
- **Auditor**: Gemini 3.8 Flash (Tier 0 Workhorse) via `tools/audit`
- **Focus Lens**: `systems`
- **Model Tier**: `TIER0`
- **Backend**: `gemini`
- **Scope**: `.`
- **Status**: Action Required

---

Auditing via Gemini Flash (bws run) [Profile: systems]...
[SEVERITY]: Critical
[LOCATION]: `lock.go:82-122` (`acquireWorkerLock`)
[ROOT CAUSE]: Time-of-check to time-of-use (TOCTOU) race condition and non-atomic file creation in lock acquisition. `acquireWorkerLock` attempts to enforce single-instance execution for background remote workers. It first checks for lock existence using `os.ReadFile(lockPath)`, and if not found, proceeds to write the current process PID using `os.WriteFile(lockPath, []byte(content), 0644)`. Because `os.WriteFile` opens the file with `os.O_WRONLY|os.O_CREATE|os.O_TRUNC` rather than `os.O_EXCL`, concurrent processes attempting to start a worker will both observe a missing lockfile and both successfully overwrite `.worker.lock`. Both processes receive an `unlock` callback with `nil` error, completely breaking mutual exclusion.
[FAILURE TRACE]:
1. Worker A and Worker B are triggered near-simultaneously (e.g., automated cron job and manual CLI invocation `abs remote scan`).
2. Both execute `os.ReadFile(lockPath)`, which fails with `os.ErrNotExist`.
3. Worker A writes its PID via `os.WriteFile`.
4. Worker B immediately overwrites `.worker.lock` with its own PID via `os.WriteFile`.
5. Both Worker A and Worker B return `(unlockFunc, nil)`, entering the worker processing loop concurrently on the same directory, corrupting manifests and causing duplicate/colliding ffmpeg operations.
[REMEDIATION]:
Use atomic file creation with `os.O_CREATE|os.O_EXCL` so only one process can successfully create and hold the lock file:
```go
func acquireWorkerLock(resolvedDir string) (func(), error) {
	lockPath := filepath.Join(resolvedDir, ".worker.lock")
	content := fmt.Sprintf("%d\n%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339))

	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if !os.IsExist(err) {
			return nil, fmt.Errorf("failed to create worker lockfile %s: %w", lockPath, err)
		}
		// Existing lock inspection and stale lock recovery logic...
	} else {
		_, _ = f.WriteString(content)
		_ = f.Close()
	}

	return func() {
		_ = os.Remove(lockPath)
	}, nil
}
```

---

[SEVERITY]: Major
[LOCATION]: `tui_data_abs.go:103-113` (`buildABSEpisodeMap`) and `quarantine.go:72-74` (`quarantineAbandonedDuplicates`)
[ROOT CAUSE]: Nil pointer dereference of optional `ep.AudioFile.Metadata`. In `backend.Episode`, `AudioFile` is a `*PodcastAudioFile`, within which `Metadata` is an optional pointer (`*AudioFileMetadata` tagged with `json:"metadata,omitempty"`). When Audiobookshelf returns episode items with audio stream objects that lack metadata, `ep.AudioFile != nil` evaluates to true, but `ep.AudioFile.Metadata` is `nil`. Calling `ep.AudioFile.Metadata.Filename` or `ep.AudioFile.Metadata.RelPath` immediately panics at runtime with `runtime error: invalid memory address or nil pointer dereference`.
[FAILURE TRACE]:
1. Audiobookshelf returns episode entries where the audio track is present but `metadata` is null/omitted.
2. TUI startup loads podcasts and calls `buildABSEpisodeMap(episodes)`.
3. `ep.AudioFile != nil` evaluates to true, but `ep.AudioFile.Metadata` is `nil`.
4. Execution reaches `ep.AudioFile.Metadata.Filename`.
5. The application panics and crashes during initial view rendering.
[REMEDIATION]:
Check `ep.AudioFile.Metadata != nil` prior to field access (consistent with checks in `abs_rescan.go` and `backend_cli.go`):
```go
// tui_data_abs.go
if ep.AudioFile != nil && ep.AudioFile.Metadata != nil {
	if ep.AudioFile.Metadata.Filename != "" {
		episodeMap[ep.AudioFile.Metadata.Filename] = ep
		episodeMap[normalizeEpisodeTitle(ep.AudioFile.Metadata.Filename)] = ep
	}
	if ep.AudioFile.Metadata.RelPath != "" {
		cleanRel := filepath.Base(ep.AudioFile.Metadata.RelPath)
		episodeMap[cleanRel] = ep
		episodeMap[normalizeEpisodeTitle(cleanRel)] = ep
	}
}

// quarantine.go
if ep.AudioFile != nil && ep.AudioFile.Metadata != nil && ep.AudioFile.Metadata.Filename != "" {
	fn = ep.AudioFile.Metadata.Filename
}
```

---

[SEVERITY]: Major
[LOCATION]: `tui_search.go:271-285` (`prevTranscriptMatch`)
[ROOT CAUSE]: Out-of-bounds slice indexing panic caused by stale index offset after result set shrinkage. `prevTranscriptMatch()` decrements `m.transcriptMatchIdx` and wraps negative indices to `len(matches) - 1`. However, if a user navigates forward in a large match set (e.g., `m.transcriptMatchIdx = 5` out of 10 matches) and subsequently types an additional query character such that fewer matches are found (e.g., 2 matches), `m.transcriptMatchIdx` remains 5. When `prevTranscriptMatch()` is invoked, it decrements 5 to 4, checks `4 < 0` (false), and directly evaluates `matches[4]` against a slice of length 2, causing an out-of-range panic.
[FAILURE TRACE]:
1. User enters transcript search mode and inputs a prefix yielding 10 matches.
2. User presses `n` multiple times to cycle to match index 5 (`m.transcriptMatchIdx = 5`).
3. User adds a character to narrow the search; `matchingTranscriptIndices()` now yields 2 matches.
4. User presses `N` (Shift+N or Shift+Tab) to navigate to the previous match.
5. `m.transcriptMatchIdx--` decrements to 4.
6. `matches[m.transcriptMatchIdx]` accesses `matches[4]` on a slice with length 2.
7. Runtime panic: `index out of range [4] with length 2`.
[REMEDIATION]:
Clamp and wrap `m.transcriptMatchIdx` against `len(matches)` before indexing:
```go
func (m *tuiModel) prevTranscriptMatch() {
	matches := m.matchingTranscriptIndices()
	if len(matches) == 0 {
		if m.searchQuery != "" {
			m.showPopup(fmt.Sprintf("No matches for %q", m.searchQuery))
		}
		return
	}
	if m.transcriptMatchIdx <= 0 || m.transcriptMatchIdx >= len(matches) {
		m.transcriptMatchIdx = len(matches) - 1
	} else {
		m.transcriptMatchIdx--
	}
	m.scrollToTranscriptLine(matches[m.transcriptMatchIdx])
	m.showPopup(fmt.Sprintf("Match %d of %d", m.transcriptMatchIdx+1, len(matches)))
}
```

---

[SEVERITY]: Major
[LOCATION]: `pkg/backend/abs_keep.go:31-33` and `pkg/backend/podfetch_ops.go:214-216` (`ApplyKeepPolicy`)
[ROOT CAUSE]: Slice bounds out-of-range panic on negative `keep` count. In `ApplyKeepPolicy`, the deletion count is computed as `toDeleteCount := len(sortedDownloaded) - keep` inside the condition `if len(sortedDownloaded) > keep`. If `keep < 0` (such as from negative CLI input `--keep -1` or corrupted configuration), `len(sortedDownloaded) > keep` evaluates to true (e.g., 3 > -1). The calculation `3 - (-1)` produces `4`, which exceeds `len(sortedDownloaded)`. Slicing `sortedDownloaded[:toDeleteCount]` causes a panic: `slice bounds out of range [:4] with capacity 3`.
[FAILURE TRACE]:
1. User runs `abs scan --keep -1` or a negative keep policy is loaded.
2. `ApplyKeepPolicy` is invoked on a podcast with downloaded episodes.
3. `len(sortedDownloaded) > keep` evaluates to true.
4. `toDeleteCount := len(sortedDownloaded) - keep` evaluates to a value strictly greater than `len(sortedDownloaded)`.
5. Slice indexing `sortedDownloaded[:toDeleteCount]` panics.
[REMEDIATION]:
Validate `keep` and clamp `toDeleteCount` to the slice length:
```go
if keep < 0 {
	return 0, fmt.Errorf("keep count cannot be negative: %d", keep)
}
if len(sortedDownloaded) > keep {
	toDeleteCount := len(sortedDownloaded) - keep
	if toDeleteCount > len(sortedDownloaded) {
		toDeleteCount = len(sortedDownloaded)
	}
	episodesToDelete := sortedDownloaded[:toDeleteCount]
```

---

[SEVERITY]: Major
[LOCATION]: `remote_manifest.go:63-71` (`saveDoneManifest`) and `episode_status.go:92-99` (`saveEpisodeStatus`)
[ROOT CAUSE]: Missing file synchronization (`fsync`) before atomic rename causing state corruption across crashes. Both functions write manifest data to a temporary file via `os.WriteFile(tmpPath, data, 0644)` and immediately call `os.Rename(tmpPath, path)`. Because `os.WriteFile` does not call `f.Sync()` before closing the file descriptor, data resides exclusively in the OS page cache. Under standard filesystem semantics (ext4/XFS), directory entry updates from `rename` can be committed to disk journal before dirty data pages are flushed. Following an ungraceful shutdown, system crash, or VM reset, the destination file is observed as zero-length (0 bytes) or corrupted. Subsequent loads via `loadDoneManifest` or `loadEpisodeStatus` fail with JSON parse errors, permanently corrupting remote sync state.
[FAILURE TRACE]:
1. Worker completes processing an episode and writes `done.json` or `<episode>.json`.
2. `os.WriteFile` writes to cache and closes; `os.Rename` updates directory structure.
3. Power loss or host reset occurs before page cache is flushed to disk.
4. Machine boots; `done.json` or status file has size 0 on disk.
5. Next run attempts `json.Unmarshal` on `done.json`, throwing `unexpected end of JSON input` and aborting all subsequent pulls.
[REMEDIATION]:
Delegate persistence to `writeFileAtomic` (defined in `format.go`), which guarantees `f.Sync()` prior to `os.Rename`:
```go
func saveDoneManifest(path string, m *RemoteDoneManifest) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory for done manifest %s: %w", dir, err)
	}
	m.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if m.Episodes == nil {
		m.Episodes = make(map[string]RemoteDoneItem)
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal done manifest: %w", err)
	}
	return writeFileAtomic(path, append(data, '\n'), 0644)
}
```

---

[SEVERITY]: Major
[LOCATION]: `lock.go:32-35` and `57-60` (`acquireFileLock` and `acquireFileLockWithTimeout`)
[ROOT CAUSE]: Inadvertent lock release under POSIX `fcntl` semantics due to second file descriptor close. `acquireFileLock` and `acquireFileLockWithTimeout` obtain an advisory lock on `lockPath` using `flock.New(lockPath)`. Immediately after acquiring the lock, they open `lockPath` a second time using `os.OpenFile(lockPath, os.O_WRONLY|os.O_TRUNC, 0644)` to write the PID, and call `f.Close()`. Under POSIX semantics, closing ANY open file descriptor associated with an inode releases ALL locks held on that inode by the entire process (`fcntl` / `F_SETLK`). On network filesystems (NFS/CIFS) or systems where `flock` translates to `fcntl`, `f.Close()` immediately and silently releases the lock that `fl` just acquired, leaving the critical section completely unprotected.
[FAILURE TRACE]:
1. Process A calls `acquireFileLock` on a shared or NFS-mounted directory.
2. `fl.TryLock()` successfully acquires the lock.
3. Lines 32-35 open `lockPath` as a second descriptor `f` and invoke `f.Close()`.
4. The kernel drops the POSIX advisory lock associated with the inode for Process A.
5. Process B calls `acquireFileLock` concurrently and successfully acquires the lock.
6. Both processes execute their critical sections concurrently.
[REMEDIATION]:
Do not open and close a secondary file descriptor to record PID; rely on the existing descriptor held by `flock`:
```go
func acquireFileLock(targetPath string) (*fileLockWrapper, error) {
	lockPath := targetPath + ".lock"
	fl := flock.New(lockPath)

	locked, err := fl.TryLock()
	if err != nil {
		return nil, fmt.Errorf("failed to acquire lock on %s: %w", lockPath, err)
	}
	if !locked {
		return nil, nil
	}

	return &fileLockWrapper{fl: fl, lockPath: lockPath}, nil
}
```

---

[SEVERITY]: Moderate
[LOCATION]: `player_sink.go:105-124` (`AudioPlayer.CycleSpeaker`)
[ROOT CAUSE]: Lock contention held across long external process executions (`exec.Command(...).Run()`). `AudioPlayer.CycleSpeaker()` acquires `p.mu.Lock()` and retains it while executing external commands `pactl set-default-sink` and `wpctl set-default`. The mutex `p.mu` is shared with the Bubble Tea TUI render loop (`View()`, `RenderProgressBar()`, `RenderVolumeBar()`, `UpdatePosition()`), which queries player state on every keypress and tick. If PulseAudio or PipeWire is slow or unresponsive, `p.mu` remains held for seconds, blocking the TUI event loop and causing terminal UI freezes.
[FAILURE TRACE]:
1. User presses the hotkey to cycle audio output devices.
2. `CycleSpeaker()` acquires `p.mu.Lock()`.
3. `pactl` or `wpctl` hangs or encounters latency talking to the audio daemon.
4. Meanwhile, Bubble Tea triggers a view refresh and calls `globalPlayer.View()`, blocking on `p.mu.Lock()`.
5. The TUI becomes completely unresponsive until the external command terminates.
[REMEDIATION]:
Determine the target speaker and update player state under the lock, but execute the external CLI commands outside the mutex:
```go
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
```

---

[SEVERITY]: Moderate
[LOCATION]: `config.go:40-51` (`ensureConfigExists`)
[ROOT CAUSE]: Non-atomic file write with overly permissive file permissions (`0644`) for sensitive credentials. `ensureConfigExists()` creates the default configuration file containing API keys and Audiobookshelf credentials using `os.WriteFile(configPath(), append(data, '\n'), 0644)`. This operation is non-atomic (susceptible to torn writes or zero-byte truncation on crash) and sets file mode to world-readable (`0644`), exposing API keys and credentials to other local users, in direct contrast to `saveConfig` which enforces `0600` via `writeFileAtomic`.
[FAILURE TRACE]:
1. Application is run for the first time and calls `ensureConfigExists()`.
2. `config.json` is created with `-rw-r--r--` (`0644`).
3. Credentials or API keys are populated.
4. Any unprivileged local user on the multi-user system can read `~/.config/abs/config.json`.
[REMEDIATION]:
Use `writeFileAtomic` with `0600` permissions:
```go
data, _ := json.MarshalIndent(cfg, "", "  ")
_ = writeFileAtomic(configPath(), append(data, '\n'), 0600)
```
