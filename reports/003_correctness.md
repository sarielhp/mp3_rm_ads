# Systems Code Review Report #003 (Lens: correctness)

- **Date**: 2026-09-06
- **Auditor**: Gemini 3.8 Flash (Tier 0 Workhorse) via `tools/audit`
- **Focus Lens**: `correctness`
- **Model Tier**: `TIER0`
- **Backend**: `gemini`
- **Scope**: `.`
- **Status**: Action Required

---

Auditing via Gemini Flash (bws run) [Profile: correctness]...
[SEVERITY]: Critical
[LOCATION]: [format.go:L289-307](file:///home/sariel/prog/26/podcasts/abs/format.go#L289-L307) (`buildMergedAndKeepIntervals`) and [format_intervals.go:L94-120](file:///home/sariel/prog/26/podcasts/abs/format_intervals.go#L94-L120) (`calculateKeepSegments`)
[ROOT CAUSE]: Invariant breakdown between audio cutting segments and metadata interval representation. In [`buildMergedAndKeepIntervals`](file:///home/sariel/prog/26/podcasts/abs/format.go#L289-L307), `newMerged` is produced by merging nearby ad intervals within `allowedGap` (up to 5.0s) via [`mergeBounds`](file:///home/sariel/prog/26/podcasts/abs/format_intervals.go#L132-L171). However, `keep` segments passed to ffmpeg and `keepIntervals` saved to metadata are computed from raw, unmerged `combined` ads via [`calculateKeepSegments`](file:///home/sariel/prog/26/podcasts/abs/format_intervals.go#L94-L120). Furthermore, raw `combined` is passed to [`buildCutEntries`](file:///home/sariel/prog/26/podcasts/abs/format.go#L271-L287) and [`mergeBounds`](file:///home/sariel/prog/26/podcasts/abs/format_intervals.go#L132-L171) without calling [`sanitizeAdSegments`](file:///home/sariel/prog/26/podcasts/abs/format_intervals.go#L7-L31), allowing NaN/Inf/out-of-bounds bounds to corrupt `MergedCutIntervals` and fail `json.Marshal` in [`saveCutsJSON`](file:///home/sariel/prog/26/podcasts/abs/format.go#L192-L248) (which discards marshal errors and writes a corrupted blank file).
[FAILURE TRACE]:
1. LLM identifies two ad segments with a short pause: `[100.0, 150.0]` and `[153.0, 200.0]` (gap = 3.0s, durations $\ge$ 30s).
2. [`mergeBounds`](file:///home/sariel/prog/26/podcasts/abs/format_intervals.go#L132-L171) merges them into `[100.0, 200.0]`. Metadata records `MergedCutIntervals: [{"start": 100, "end": 200}]` and `TotalCutDurationSec: 100.0`.
3. [`calculateKeepSegments`](file:///home/sariel/prog/26/podcasts/abs/format_intervals.go#L94-L120) runs against unmerged `combined` and generates keep slice `[150.0, 153.0]`.
4. ffmpeg splices `[0.0, 100.0]`, `[150.0, 153.0]`, and `[200.0, end]`, preserving a 3-second audio fragment in the audio file despite the cuts metadata declaring it removed.
5. Invariant $\sum \text{Keep} + \sum \text{Cut} == \text{TotalDuration}$ is violated, and physical audio diverges from the `.cuts.json` specification.
[REMEDIATION]:
Sanitize ad segments before building bounds, and calculate keep segments from the merged cut bounds rather than unmerged inputs:
```go
func buildMergedAndKeepIntervals(totalDuration float64, combined []AdSegment) ([]MergedCutInterval, [][2]float64, []KeepSegment) {
	combined = sanitizeAdSegments(combined, totalDuration)
	allBounds := make([][2]float64, 0, len(combined))
	for _, ad := range combined {
		allBounds = append(allBounds, [2]float64{ad.Start, ad.End})
	}
	sortBounds(allBounds)

	newMerged := mergeBounds(allBounds)
	formattedMerged := make([]MergedCutInterval, 0, len(newMerged))
	var mergedAds []AdSegment
	for _, b := range newMerged {
		formattedMerged = append(formattedMerged, MergedCutInterval{Start: roundFloat(b[0], 2), End: roundFloat(b[1], 2)})
		mergedAds = append(mergedAds, AdSegment{Start: b[0], End: b[1]})
	}

	keep := calculateKeepSegments(totalDuration, mergedAds)
	keepIntervals := make([]KeepSegment, 0, len(keep))
	for _, k := range keep {
		keepIntervals = append(keepIntervals, KeepSegment{Start: roundFloat(k[0], 2), End: roundFloat(k[1], 2)})
	}
	return formattedMerged, keep, keepIntervals
}
```

---

[SEVERITY]: Critical
[LOCATION]: [transcribe_chunks.go:L164-194](file:///home/sariel/prog/26/podcasts/abs/transcribe_chunks.go#L164-L194) (`adjustChunkSegment`)
[ROOT CAUSE]: Broken boundary condition checks for chunk overlap filtering. Lines 170-176 use an `if isFirst { ... } else if segStart >= cutEnd { ... }` construct:
When `isFirst` is true, trailing overlap segments (`segStart >= cutEnd`) are never dropped because the `else if` is skipped. They are clamped at line 184 (`seg.End = cutEnd`), while `seg.Start` remains $> cutEnd$.
When `!isFirst`, leading overlap segments (`segEnd <= cutStart`) are never dropped because `segEnd <= cutStart` is only inside the `if isFirst` branch. `seg.Start` is clamped to `cutStart` at line 179 while `seg.End` remains $< cutStart$.
Both cases produce inverted timestamps (`seg.Start > seg.End`).
[FAILURE TRACE]:
1. Audio file $>1200\text{s}$ is split into chunks with 30s overlap. Chunk 0 spans `[0, 1200]`, with audio extracted up to `1230s`.
2. Speech occurs in overlap region at `[1205, 1215]`.
3. For chunk 0 (`isFirst == true`), `segEnd <= cutStart` (1215 $\le$ 0) is false; `segStart >= cutEnd` is not evaluated.
4. Line 184 sets `seg.End = 1200.0`, leaving `seg.Start = 1205.0`.
5. Segment `[1205.0, 1200.0]` is emitted.
6. For chunk 1 (`isFirst == false`), speech at `[1180, 1195]` is evaluated. `segStart >= cutEnd` (1180 $\ge$ 2400) is false; `segEnd <= cutStart` (1195 $\le$ 1200) is never tested.
7. Line 179 sets `seg.Start = 1200.0`, leaving `seg.End = 1195.0`.
8. Both inverted segments enter [`assembleTranscriptionResult`](file:///home/sariel/prog/26/podcasts/abs/transcribe_chunks.go#L196-L216) and [`convertJSONToSRT`](file:///home/sariel/prog/26/podcasts/abs/output.go#L30-L71), generating invalid subtitles (`00:20:05,000 --> 00:20:00,000`) and corrupting LLM prompts.
[REMEDIATION]:
Filter out segments strictly outside the chunk bounds for all chunks, and reject any segment where end precedes or equals start:
```go
func adjustChunkSegment(seg TranscriptionSegment, ch chunkInfo, isFirst, isLast bool) (TranscriptionSegment, bool) {
	cutStart := ch.actualStart
	cutEnd := ch.actualEnd
	segStart := seg.Start + ch.extractStart
	segEnd := seg.End + ch.extractStart

	if segEnd <= cutStart || segStart >= cutEnd {
		return seg, false
	}

	if segStart < cutStart {
		seg.Start = cutStart
	} else {
		seg.Start = segStart
	}
	if segEnd > cutEnd {
		seg.End = cutEnd
	} else {
		seg.End = segEnd
	}

	if seg.End <= seg.Start {
		return seg, false
	}

	for i := range seg.Words {
		seg.Words[i].Start += ch.extractStart
		seg.Words[i].End += ch.extractStart
	}
	return seg, true
}
```

---

[SEVERITY]: Critical
[LOCATION]: [pkg/backend/abs_duration.go:L17-53](file:///home/sariel/prog/26/podcasts/abs/pkg/backend/abs_duration.go#L17-L53) (`GetMP3DiskDurationNative`, `GetMP3DiskDuration`) and [pkg/backend/abs_rescan.go:L203-220](file:///home/sariel/prog/26/podcasts/abs/pkg/backend/abs_rescan.go#L203-L220) (`identifyEpisodesToUpdate`, `applyRescanUpdates`)
[ROOT CAUSE]: Premature decode abort treated as full stream duration. [`GetMP3DiskDurationNative`](file:///home/sariel/prog/26/podcasts/abs/pkg/backend/abs_duration.go#L17-L40) breaks on any `d.Decode` error (ID3 padding, corrupt frames, stream abort) without checking `err == io.EOF`. It returns the duration accumulated prior to the error. Because `dur > 0`, [`GetMP3DiskDuration`](file:///home/sariel/prog/26/podcasts/abs/pkg/backend/abs_duration.go#L42-L81) accepts this partial duration, bypassing `ffprobe` and `mediainfo` fallbacks. In [`abs_rescan.go`](file:///home/sariel/prog/26/podcasts/abs/pkg/backend/abs_rescan.go#L203), `diskDuration < dbDuration - 0.5` triggers, and [`applyRescanUpdates`](file:///home/sariel/prog/26/podcasts/abs/pkg/backend/abs_rescan.go#L213-L220) permanently overwrites the database duration with the truncated value.
[FAILURE TRACE]:
1. A 3600-second episode MP3 contains a corrupt frame or non-audio ID3v1 metadata block at 300 seconds.
2. [`GetMP3DiskDurationNative`](file:///home/sariel/prog/26/podcasts/abs/pkg/backend/abs_duration.go#L17-L40) encounters decode error at minute 5, exits loop without checking `io.EOF`, and returns `300.0`.
3. [`GetMP3DiskDuration`](file:///home/sariel/prog/26/podcasts/abs/pkg/backend/abs_duration.go#L42-L81) returns `300.0`.
4. `abs rescan` checks `diskDuration (300.0) < dbDuration (3600.0) - 0.5`, marks the episode as recut, and commits `300.0` to the Audiobookshelf SQLite database.
[REMEDIATION]:
Require clean `io.EOF` completion before trusting native MP3 duration; fallback to `ffprobe` on any decoding error:
```go
func GetMP3DiskDurationNative(path string) float64 {
	file, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer file.Close()

	d := mp3.NewDecoder(file)
	var duration float64
	var frame mp3.Frame
	var skipped int

	for {
		if err := d.Decode(&frame, &skipped); err != nil {
			if err == io.EOF {
				return duration
			}
			return 0
		}
		duration += frame.Duration().Seconds()
	}
}
```

---

[SEVERITY]: Critical
[LOCATION]: [quarantine.go:L64-119](file:///home/sariel/prog/26/podcasts/abs/quarantine.go#L64-L119) (`quarantineAbandonedDuplicates`)
[ROOT CAUSE]: Data loss of active episode files due to unverified disk existence of the replacement file. In [`quarantineAbandonedDuplicates`](file:///home/sariel/prog/26/podcasts/abs/quarantine.go#L64-L119), `trackedByNormalized` is populated solely from Audiobookshelf metadata (`ep.Title + ".mp3"` or `AudioFile.Metadata.Filename`). At line 100, if `trackedFn != fn`, it assumes `fn` is an abandoned duplicate without verifying that `trackedFn` actually exists on disk. If `trackedFn` is only in server metadata or was not downloaded, `fn` (the sole existing copy of the audio and metadata) is moved to `.bak`.
[FAILURE TRACE]:
1. File on disk is named `Episode 01.mp3`.
2. Audiobookshelf server metadata contains `Title: "Episode 01 (9f8e7d)"` with no `AudioFile.Metadata.Filename` populated, mapping `trackedByNormalized["episode 01"]` to `"Episode 01 (9f8e7d).mp3"`.
3. `trackedFiles["Episode 01.mp3"]` is false.
4. Line 100 triggers: `trackedFn ("Episode 01 (9f8e7d).mp3") != fn ("Episode 01.mp3")`.
5. `quarantineFile("Episode 01.mp3")` renames the episode to `.bak`.
6. `"Episode 01 (9f8e7d).mp3"` never existed on disk; the podcast directory is left with 0 active audio files for the episode.
[REMEDIATION]:
Verify that the replacement `trackedFn` exists on disk before quarantining any supposed duplicate:
```go
		norm := normalizeEpisodeTitle(fn)
		if trackedFn, exists := trackedByNormalized[norm]; exists && trackedFn != fn {
			trackedPath := filepath.Join(podDir, trackedFn)
			if fi, err := os.Stat(trackedPath); err != nil || fi.IsDir() || fi.Size() == 0 {
				continue
			}
			base := strings.TrimSuffix(mp3, ".mp3")

			_ = quarantineFile(mp3)
			_ = quarantineFile(base + ".cuts.json")
			_ = quarantineFile(base + ".transcript.json")
			_ = quarantineFile(mp3 + ".precut")
			_ = quarantineFile(base + ".srt")
			_ = quarantineFile(base + ".txt")

			quarantined = append(quarantined, fn)
		}
```

---

[SEVERITY]: Major
[LOCATION]: [pipeline.go:L98-114](file:///home/sariel/prog/26/podcasts/abs/pipeline.go#L98-L114) (`executeRecutAudio`)
[ROOT CAUSE]: Missing rollback and permanent deletion/displacement of canonical file on partial failure. In [`executeRecutAudio`](file:///home/sariel/prog/26/podcasts/abs/pipeline.go#L78-L116), `safeMove(mainMP3File, precutFile)` moves the original file to `.precut` before installing the new file. If the subsequent `safeMove(tempOutputFile, outputFile)` fails (permission denied, disk full, cross-device error), `executeRecutAudio` returns immediately with no rollback: `mainMP3File` is missing from the directory, `outputFile` does not exist, and `workDir` remains uncollected.
[FAILURE TRACE]:
1. User runs `abs recut episode.mp3`.
2. Audio cutting completes in `.work/episode.mp3.tmp.mp3`.
3. `safeMove("episode.mp3", "episode.mp3.precut")` succeeds.
4. `safeMove(".work/episode.mp3.tmp.mp3", "episode.mp3")` fails due to filesystem error or permission denial.
5. Error is printed to `os.Stderr` and function returns.
6. `episode.mp3` no longer exists in the podcast folder.
[REMEDIATION]:
Hard-link or copy original to `.precut` before moving, and roll back `.precut` if installing the cut audio fails:
```go
	preserved := false
	if sourceAudioFile == mainMP3File && fileExists(mainMP3File) {
		checkPrecutSymlink(precutFile)
		if err := os.Link(mainMP3File, precutFile); err != nil {
			if cpErr := copyFileErr(mainMP3File, precutFile); cpErr != nil {
				fmt.Fprintf(os.Stderr, "Error: could not preserve the original: %v\n", cpErr)
				return
			}
		}
		preserved = true
		if !cli.Quiet {
			fmt.Printf("Original file preserved at: '%s'\n", precutFile)
		}
	}

	if mvErr := safeMove(tempOutputFile, outputFile); mvErr != nil {
		if preserved {
			_ = os.Remove(precutFile)
		}
		fmt.Fprintf(os.Stderr, "Error: could not install the recut audio: %v\n", mvErr)
		return
	}
	os.RemoveAll(workDir)
```

---

[SEVERITY]: Major
[LOCATION]: [download_queue.go:L117-127](file:///home/sariel/prog/26/podcasts/abs/download_queue.go#L117-L127) (`isItemInDownloadQueue`) and [download_queue.go:L167-187](file:///home/sariel/prog/26/podcasts/abs/download_queue.go#L167-L187) (`EnqueueDownload`)
[ROOT CAUSE]: State machine terminal lock preventing retry of failed jobs. In [`finalizeDownloadQueueItem`](file:///home/sariel/prog/26/podcasts/abs/download_queue.go#L264-L288), failed downloads are transitioned to `Status = "failed"`. The background worker in [`claimDownloadQueueItem`](file:///home/sariel/prog/26/podcasts/abs/download_queue.go#L239-L262) only processes `Status == "queued"`. When a user attempts to retry or re-enqueue the episode, [`isItemInDownloadQueue`](file:///home/sariel/prog/26/podcasts/abs/download_queue.go#L117-L127) only skips `Status == "completed"`, treating `Status == "failed"` as already queued and rejecting the request with `"already_queued"`.
[FAILURE TRACE]:
1. Network drops during episode download; [`executeQueueItemDownload`](file:///home/sariel/prog/26/podcasts/abs/download_queue.go#L323-L346) fails.
2. Item status becomes `"failed"`.
3. Worker skips the item because status is not `"queued"`.
4. User clicks download again in TUI or runs download command.
5. [`EnqueueDownload`](file:///home/sariel/prog/26/podcasts/abs/download_queue.go#L167-L187) calls [`isItemInDownloadQueue`](file:///home/sariel/prog/26/podcasts/abs/download_queue.go#L117-L127), which matches the failed item and returns `true`.
6. Enqueue fails with `"already_queued"`; episode is permanently blocked from being downloaded.
[REMEDIATION]:
Allow re-enqueuing or replacing failed queue items in [`isItemInDownloadQueue`](file:///home/sariel/prog/26/podcasts/abs/download_queue.go#L117-L127):
```go
func isItemInDownloadQueue(item DownloadQueueItem, q *DownloadQueuePersist) bool {
	for i, existing := range q.Items {
		if existing.Status == "completed" {
			continue
		}
		if matchEpisodeDeduplication(item.GUID, item.EnclosureURL, item.EpisodeTitle, existing.GUID, existing.EnclosureURL, existing.EpisodeTitle) {
			if existing.Status == "failed" {
				q.Items[i].Status = "queued"
				q.Items[i].Error = ""
				q.Items[i].AddedAt = time.Now().UTC()
				_ = saveDownloadQueue(q)
				return true
			}
			return true
		}
	}
	return false
}
```

---

[SEVERITY]: Major
[LOCATION]: [pkg/backend/podfetch_db.go:L273-287](file:///home/sariel/prog/26/podcasts/abs/pkg/backend/podfetch_db.go#L273-L287) (`updatePodFetchDurationDB`)
[ROOT CAUSE]: Database-wide metadata corruption via unscoped fuzzy matching. In [`updatePodFetchDurationDB`](file:///home/sariel/prog/26/podcasts/abs/pkg/backend/podfetch_db.go#L273-L287), the query `UPDATE podcast_episodes SET total_time = ? WHERE local_url LIKE ? OR local_url = ? OR name = ? OR name LIKE ?` includes `OR name LIKE "%" + noExt + "%"` without scoping by `podcast_id`. If `noExt` is a short or generic title (e.g., `"01"`, `"1"`, `"Intro"`, `"Trailer"`, `"Episode 1"`), the query updates `total_time` across unrelated episodes throughout the entire SQLite database.
[FAILURE TRACE]:
1. File `/podcasts/ShowA/01.mp3` (duration 600s) has duration synchronized.
2. `noExt` is `"01"`.
3. `UPDATE podcast_episodes SET total_time = 600 WHERE ... OR name LIKE '%01%'` executes.
4. Every episode in the database containing `"01"` in its title (e.g., `"Episode 01"`, `"010"`, `"2024-01-15"`) has its duration overwritten to 600 seconds.
[REMEDIATION]:
Restrict the update to exact matches or exact file path matches, eliminating the broad `name LIKE '%...%'` clause:
```go
func updatePodFetchDurationDB(dbPath, filePath string, duration float64) error {
	verifyPodfetchNotDisabled("updatePodFetchDurationDB")
	db, err := sql.Open("sqlite3", dbPath+"?_busy_timeout=5000")
	if err != nil {
		return err
	}
	defer db.Close()

	base := filepath.Base(filePath)
	likePattern := "%" + base

	_, err = db.Exec("UPDATE podcast_episodes SET total_time = ? WHERE local_url = ? OR local_url LIKE ?", int(duration), filePath, likePattern)
	return err
}
```
