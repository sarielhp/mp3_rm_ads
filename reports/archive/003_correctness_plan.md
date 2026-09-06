# Architectural Plan: Correctness Audit Remediation (#003)

- **Date**: 2026-09-06
- **Focus Area**: Correctness Audit
- **Report Reference**: `reports/003_correctness.md`
- **Quality Gate**: `tools/check`
- **Sizing Constraints**: Function length <= 80 lines, decompose in place, files <= 800 (warn) / 1100 (limit).

---

## Overview

This plan details the root cause analysis, architectural remediation, and regression unit testing strategy for all 7 issues identified in `reports/003_correctness.md`.

| ID | Severity | File(s) | Summary | Regression Test Target |
|---|---|---|---|---|
| 1 | Critical | `format.go`, `format_intervals.go` | Unsanitized input and cut/keep segment divergence | `save_cuts_test.go` |
| 2 | Critical | `transcribe_chunks.go` | Inverted timestamps in chunk overlap boundary conditions | `transcribe_chunks_test.go` |
| 3 | Critical | `pkg/backend/abs_duration.go` | Native MP3 decode error treated as EOF, truncating duration | `pkg/backend/abs_duration_test.go` |
| 4 | Critical | `quarantine.go` | Quarantining files without verifying existence of replacement on disk | `quarantine_test.go` |
| 5 | Major | `pipeline.go` | Original audio displaced/lost if recut audio install fails | `pipeline_test.go` |
| 6 | Major | `download_queue.go` | Failed downloads permanently locked from re-enqueuing | `download_queue_test.go` |
| 7 | Major | `pkg/backend/podfetch_db.go` | Unscoped fuzzy name matching corrupts unrelated episode durations | `pkg/backend/podfetch_db_test.go` |

---

## Detailed Issue Analysis & Remediation Strategy

### Issue 1: Invariant Breakdown Between Audio Cutting Segments & Cuts Metadata (Critical)

- **Locations**: `format.go:L289-307` (`buildMergedAndKeepIntervals`), `format_intervals.go:L7-31` (`sanitizeAdSegments`), `format.go:L192-248` (`saveCutsJSON`)
- **Root Cause**:
  1. `buildMergedAndKeepIntervals` computes `newMerged` by merging nearby ad intervals within `allowedGap` (via `mergeBounds`). However, `keep` segments (used by ffmpeg) and `keepIntervals` (saved in metadata) are computed from raw, unmerged `combined` ads via `calculateKeepSegments`. If ads are merged across a short gap, the gap audio was kept in ffmpeg segments while the metadata claimed it was cut, violating $\sum \text{Keep} + \sum \text{Cut} == \text{TotalDuration}$.
  2. Input ad segments were not sanitized before calling `buildCutEntries` and `mergeBounds`, allowing NaN/Inf or out-of-range timestamps to enter `CutsData`, causing `json.Marshal` failure or invalid intervals.
- **Architectural Fix**:
  1. In `saveCutsJSON`, sanitize `combined := sanitizeAdSegments(append(existingRaw, adSegments...), totalDuration)` prior to passing to `buildCutEntries` and `buildMergedAndKeepIntervals`.
  2. In `buildMergedAndKeepIntervals`, ensure `combined` is sanitized with `totalDuration`.
  3. Construct `mergedAds []AdSegment` from `newMerged` bounds (`[2]float64{b[0], b[1]}`), and pass `mergedAds` to `calculateKeepSegments(totalDuration, mergedAds)`. This guarantees that `keep` segments and `keepIntervals` strictly complement the merged cuts.
  4. In `saveCutsJSON`, check `jsonMarshalIndent` error before writing.
- **Regression Unit Test**:
  - Add test in `save_cuts_test.go`:
    - Test with two ad segments separated by a small gap (e.g. `[100.0, 150.0]` and `[153.0, 200.0]`, gap 3.0s < 5.0s, duration >= 30s) which merge into `[100.0, 200.0]`.
    - Assert that `keepSegments` does NOT include `[150.0, 153.0]`, and that keep segments complement the merged cuts exactly (`[0, 100]` and `[200, total]`).
    - Test with NaN/Inf and out-of-bounds segments to verify sanitization in `saveCutsJSON`.

---

### Issue 2: Broken Boundary Condition Checks for Chunk Overlap Filtering (Critical)

- **Locations**: `transcribe_chunks.go:L164-194` (`adjustChunkSegment`)
- **Root Cause**:
  - Lines 170-176 used `if isFirst { if segEnd <= cutStart { return seg, false } } else if segStart >= cutEnd { return seg, false }`.
  - For `isFirst == true`, `segStart >= cutEnd` was skipped, leaving segments starting beyond `cutEnd` to have `seg.End` clamped to `cutEnd` while `seg.Start > cutEnd`, producing `seg.Start > seg.End`.
  - For `isFirst == false`, `segEnd <= cutStart` was never checked, leaving segments ending before `cutStart` to have `seg.Start` clamped to `cutStart` while `seg.End < cutStart`, producing `seg.Start > seg.End`.
- **Architectural Fix**:
  - For all chunks (regardless of `isFirst`), filter out segments strictly outside the chunk bounds: `if segEnd <= cutStart || segStart >= cutEnd { return seg, false }`.
  - Clamp `seg.Start = max(segStart, cutStart)` and `seg.End = min(segEnd, cutEnd)`.
  - If after clamping `seg.End <= seg.Start`, return `seg, false`.
  - Adjust word timestamps only for valid kept segments.
- **Regression Unit Test**:
  - Add tests in `transcribe_chunks_test.go`:
    - Chunk with `isFirst = true`, segment in trailing overlap beyond `cutEnd` -> must be rejected (`keep == false`).
    - Chunk with `isFirst = false`, segment in leading overlap before `cutStart` -> must be rejected (`keep == false`).
    - Straddling segment properly clamped with valid `seg.Start < seg.End`.
    - Inverted timestamp assertion ensuring `seg.Start < seg.End` for any returned segment.

---

### Issue 3: Premature Decode Abort Treated as Full Stream Duration (Critical)

- **Locations**: `pkg/backend/abs_duration.go:L17-40` (`GetMP3DiskDurationNative`, `GetMP3DiskDuration`)
- **Root Cause**:
  - In `GetMP3DiskDurationNative`, any error returned by `d.Decode(&frame, &skipped)` broke out of the loop and returned whatever `duration` was accumulated so far.
  - Non-EOF decode errors (corrupt frames, trailing non-audio tags, stream aborts) caused partial duration to be returned.
  - Because `dur > 0`, `GetMP3DiskDuration` accepted the partial duration without invoking `ffprobe`/`mediainfo` fallbacks, triggering spurious recuts in `abs rescan`.
- **Architectural Fix**:
  - In `GetMP3DiskDurationNative`, check `if err != nil { if err == io.EOF { return duration } return 0 }`.
  - A non-EOF error must return `0`, signaling incomplete decode and allowing `GetMP3DiskDuration` to fall back to `ffprobe` / `mediainfo`.
- **Regression Unit Test**:
  - Add test in `pkg/backend/abs_duration_test.go`:
    - Create a truncated/corrupted MP3 stream (or file with invalid frames midway through).
    - Assert `GetMP3DiskDurationNative` returns `0` upon encountering decode errors before `io.EOF`.
    - Create a valid MP3 file or reader and verify normal completion returns the expected duration when reaching `io.EOF`.

---

### Issue 4: Data Loss of Active Episode Files via Unverified Replacement (Critical)

- **Locations**: `quarantine.go:L64-119` (`quarantineAbandonedDuplicates`)
- **Root Cause**:
  - `trackedByNormalized` is populated from server metadata (`ep.Title + ".mp3"` or `AudioFile.Metadata.Filename`).
  - If a file `fn` on disk matched normalized title of a tracked episode `trackedFn`, but `trackedFn != fn`, `quarantineAbandonedDuplicates` assumed `fn` was an abandoned duplicate without verifying whether `trackedFn` actually existed on disk.
  - If `trackedFn` only existed in server metadata or was not yet downloaded, `fn` was quarantined to `.bak`, leaving zero active copies on disk.
- **Architectural Fix**:
  - Before quarantining `mp3` / `fn`, check if the proposed replacement `trackedFn` exists on disk and is a non-empty regular file:
    ```go
    trackedPath := filepath.Join(podDir, trackedFn)
    if fi, err := os.Stat(trackedPath); err != nil || fi.IsDir() || fi.Size() == 0 {
        continue
    }
    ```
- **Regression Unit Test**:
  - Add test in `quarantine_test.go`:
    - Create directory with `Episode 01.mp3`.
    - Provide `trackedEpisodes` where title matches normalized form but mapped `trackedFn` is `"Episode 01 (9f8e7d).mp3"` which does NOT exist on disk.
    - Run `quarantineAbandonedDuplicates`.
    - Assert `Episode 01.mp3` is NOT quarantined and returns 0 quarantined items.
    - Create `"Episode 01 (9f8e7d).mp3"` on disk; rerun and assert `Episode 01.mp3` IS now quarantined.

---

### Issue 5: Missing Rollback and Permanent Deletion of Canonical Audio (Major)

- **Locations**: `pipeline.go:L78-116` (`executeRecutAudio`)
- **Root Cause**:
  - In `executeRecutAudio`, original `mainMP3File` was moved to `precutFile` via `safeMove`.
  - If the subsequent `safeMove(tempOutputFile, outputFile)` failed (e.g. disk full, permission denied), `executeRecutAudio` returned immediately without rolling back.
  - As a result, `mainMP3File` was missing from the directory, `outputFile` did not exist, and canonical audio was lost.
- **Architectural Fix**:
  - Preserve the original file safely before attempting installation: hard-link `mainMP3File` to `precutFile` via `os.Link(mainMP3File, precutFile)`.
  - If `os.Link` fails (e.g. cross-filesystem), copy `mainMP3File` to `precutFile` using `copyFileErr`.
  - `mainMP3File` remains in place until `safeMove(tempOutputFile, outputFile)` succeeds.
  - If installing the recut audio fails, remove `precutFile` to restore original state, log error, and abort.
- **Regression Unit Test**:
  - Add test in `pipeline_test.go` (or `main_test.go`):
    - Test recut audio file preservation and atomic failure rollback: simulate failure when moving cut output to final destination.
    - Verify that original file is intact and not lost.

---

### Issue 6: State Machine Terminal Lock Preventing Retry of Failed Downloads (Major)

- **Locations**: `download_queue.go:L117-127` (`isItemInDownloadQueue`), `download_queue.go:L167-187` (`EnqueueDownload`)
- **Root Cause**:
  - When a download failed, its status became `"failed"`.
  - `isItemInDownloadQueue` only skipped `"completed"`, treating `"failed"` items as still actively queued.
  - Calling `EnqueueDownload` returned `(false, "already_queued")`, permanently locking the failed item from retry.
- **Architectural Fix**:
  - In `isItemInDownloadQueue`: skip both `"completed"` and `"failed"`, so it accurately checks for active queued/downloading jobs.
  - In `EnqueueDownload`:
    - Check `if isItemInDownloadQueue(item, q) { return false, "already_queued" }`.
    - If found with status `"failed"`, reset `Status = "queued"`, clear `Error = ""`, update `AddedAt = time.Now().UTC()`, save queue, and return `(true, "queued")`.
    - If not found, append new item with status `"queued"`.
- **Regression Unit Test**:
  - Add test in `download_queue_test.go`:
    - Enqueue an item, simulate failure (`Status = "failed"`, `Error = "network error"`).
    - Verify `isItemInDownloadQueue` returns `false`.
    - Call `EnqueueDownload` again on the same item.
    - Assert `ok == true` and reason is `"queued"`.
    - Assert the queue item transitioned back to `Status == "queued"` and `Error == ""`.

---

### Issue 7: Database-Wide Metadata Corruption via Unscoped Fuzzy Matching (Major)

- **Locations**: `pkg/backend/podfetch_db.go:L273-287` (`updatePodFetchDurationDB`)
- **Root Cause**:
  - The SQL query executed `UPDATE podcast_episodes SET total_time = ? WHERE local_url LIKE ? OR local_url = ? OR name = ? OR name LIKE ?`.
  - When `noExt` is generic or short (e.g. `"01"`, `"Trailer"`, `"Intro"`), `OR name LIKE '%01%'` matched and updated every episode in the database containing that substring across all podcasts.
- **Architectural Fix**:
  - Restrict matching strictly to the file path:
    ```go
    base := filepath.Base(filePath)
    likePattern := "%" + base

    _, err = db.Exec("UPDATE podcast_episodes SET total_time = ? WHERE local_url = ? OR local_url LIKE ?", int(duration), filePath, likePattern)
    ```
  - This eliminates false positives against episode names across unrelated podcasts.
- **Regression Unit Test**:
  - Add test in `pkg/backend/podfetch_db_test.go`:
    - Populate SQLite DB with two episodes: Episode A (`local_url: "/podcasts/ShowA/01.mp3"`, `name: "01"`, `total_time: 100`) and Episode B (`local_url: "/podcasts/ShowB/episode01.mp3"`, `name: "Episode 01 - Interview"`, `total_time: 5000`).
    - Call `updatePodFetchDurationDB(dbPath, "/podcasts/ShowA/01.mp3", 600)`.
    - Assert Episode A's `total_time` is updated to 600.
    - Assert Episode B's `total_time` remains 5000 (unaffected).

---

## Verification & AGENTS.md Adherence

1. **Function Line Length**: Every modified and helper function must remain <= 80 lines.
2. **Quality Gate**: Run `./tools/check` to execute `gofmt`, `go vet`, `staticcheck`, line length audit, tests, and build.
3. **Summary**: Create `reports/003_summary.md` reflecting findings and remediation.
4. **Commit**: Final commit of all changes.
