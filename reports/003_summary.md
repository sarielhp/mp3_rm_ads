# Correctness Audit Remediation Summary (#003)

- **Date**: 2026-09-06
- **Auditor / Remediator**: Antigravity (Correctness Audit)
- **Report Reference**: `reports/003_correctness.md`
- **Plan Reference**: `reports/003_correctness_plan.md`
- **Total Issues Found**: 7 (4 Critical, 3 Major)
- **Total Issues Resolved**: 7 (100% resolved with regression unit tests)

---

## Issue Details & Mitigations

1. **Ad Cuts & Keep Segments Invariant Breakdown** (Critical - `format.go`, `format_intervals.go`)
   - *Description*: Unsanitized ad segments and computing keep segments from unmerged inputs caused audio/cuts metadata divergence and JSON serialization failures.
   - *Mitigation*: Sanitized combined segments against total duration and derived keep intervals strictly from merged cut bounds.

2. **Chunk Overlap Inverted Timestamps** (Critical - `transcribe_chunks.go`)
   - *Description*: Chunk overlap boundary filtering used flawed `if/else if` logic causing inverted timestamps (`seg.Start > seg.End`).
   - *Mitigation*: Unified boundary filtering to drop segments outside chunk bounds and clamp timestamps safely with `seg.End > seg.Start` enforcement.

3. **MP3 Premature Decode Abort Duration Truncation** (Critical - `pkg/backend/abs_duration.go`)
   - *Description*: Native MP3 decoder aborted on non-EOF errors and returned premature durations, leading to false recut triggers and database truncation.
   - *Mitigation*: Required explicit `io.EOF` for valid decode durations, returning 0 on decode errors to trigger `ffprobe`/`mediainfo` fallbacks.

4. **Abandoned Duplicate Quarantine Data Loss** (Critical - `quarantine.go`)
   - *Description*: `quarantineAbandonedDuplicates` moved active episodes to `.bak` without checking if the replacement file actually existed on disk.
   - *Mitigation*: Added filesystem existence and non-zero size checks for tracked replacement files prior to quarantining candidate duplicates.

5. **Audio Recut Failure Missing Rollback** (Major - `pipeline.go`)
   - *Description*: `executeRecutAudio` moved the original file to `.precut` before installing the recut audio, losing the canonical file if installation failed.
   - *Mitigation*: Hard-linked or copied original to `.precut` while keeping source in place, and rolled back `.precut` on installation failure.

6. **Download Queue Terminal Lock on Failed Downloads** (Major - `download_queue.go`)
   - *Description*: Failed download queue items were permanently locked from being re-enqueued or retried due to terminal state matching.
   - *Mitigation*: Excluded failed items from active queue queries and reset status to `"queued"` when re-enqueuing previously failed items.

7. **PodFetch Database Unscoped Title Match Corruption** (Major - `pkg/backend/podfetch_db.go`)
   - *Description*: `updatePodFetchDurationDB` updated durations using broad substring matching on episode titles without scoping to podcast ID or local file.
   - *Mitigation*: Restricted the SQL update clause strictly to matching `local_url` by exact path or filename suffix.
