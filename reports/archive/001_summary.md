# Systems Audit Remediation Summary #001

## Summary
The systems audit identified 10 issues across process locking, error propagation, queue concurrency, transactional file modifications, and database operations. All 10 issues were remediated and verified with dedicated regression unit tests, static analysis, function line limit audits, and full test suite passes with the race detector.

## Issues and Mitigations

1. **Flawed PID-file Locking (`lock.go`)**
   - *Issue*: Stale lock recovery based on 6-hour age thresholds and file deletion created races where active worker locks were deleted by competing processes.
   - *Mitigation*: Replaced PID-file unlinking with OS advisory locks (`flock`) tied to open file descriptors with thread-safe in-process re-entrancy tracking.

2. **Fail-Open Lock Acquisition (`batch_proc_file.go`)**
   - *Issue*: File locking errors only printed a warning and allowed audio processing to proceed unlocked with a nil file lock.
   - *Mitigation*: Updated `checkSkipOrLockAudioFile` to fail closed and immediately return `nil, false, false` whenever lock acquisition encounters an error.

3. **Cross-Process Queue Claiming & Indiscriminate Stale Requeue (`download_queue.go`)**
   - *Issue*: Persistent download queue claiming lacked inter-process locking, discarded status save errors, and blindly requeued items actively being downloaded by live processes.
   - *Mitigation*: Serialized queue claiming under an inter-process advisory lock with `OwnerPID` leases, persisted status changes durably, and restricted stale requeuing to dead PIDs.

4. **Lost-Wakeup Race in Worker Trigger (`download_queue.go`)**
   - *Issue*: Download queue triggers arriving while a worker was in the process of shutting down were dropped, leaving new items stranded in the queue.
   - *Mitigation*: Introduced a `downloadWorkerPending` flag to capture triggers received while active and re-check for pending work before worker termination.

5. **Nil Pointer Dereference on Download Failure (`download_queue.go`)**
   - *Issue*: Non-empty `PodcastID` unconditionally called `client.DownloadEpisodes` even when `client` was nil, causing a panic in background goroutines.
   - *Mitigation*: Added guarded checks for nil `client` before invoking `DownloadEpisodes`, recording a structured failure error rather than crashing.

6. **Redundant Direct Download Execution in TUI (`tui_batch.go`)**
   - *Issue*: Both single and batch TUI enqueue paths directly called `DownloadEpisodes` and then immediately triggered the background worker which executed `DownloadEpisodes` again.
   - *Mitigation*: Removed direct `DownloadEpisodes` invocations from both TUI enqueue paths, making the download queue worker the single executor.

7. **Non-Transactional Audio Preservation (`batch_proc_file.go`)**
   - *Issue*: Moving the original audio file to `.precut` before installing the replacement risked leaving the canonical episode missing if subsequent file moves failed.
   - *Mitigation*: Preserved the original via hard link or copy without removing the canonical file, atomically replaced the canonical path via `safeMove`, and rolled back `.precut` on failure.

8. **Premature Completion Metadata Commit (`batch_proc_file.go`)**
   - *Issue*: `handleNoAdsDetected` committed cuts JSON and `StateDone` status before copying the audio file, discarding copy errors and risking incomplete output files.
   - *Mitigation*: Staged output copying via `.work/` atomically through `installNoAdsOutput` before updating cuts metadata or marking status as `StateDone`.

9. **Premature Local State Transition on Remote Push (`remote_batch.go`)**
   - *Issue*: `pushSingleAudioFile` saved local status as `StateQueuedRemote` prior to uploading audio and status files, stranding episodes locally if network uploads failed.
   - *Mitigation*: Staged status files in `.work/`, executed remote uploads first, and committed the local `StateQueuedRemote` transition only after both uploads succeeded.

10. **Unlocked Read-Modify-Write Queue Operations (`queue_cmd.go`, `tui_data_queue.go`, `pkg/backend/podfetch_db.go`)**
    - *Issue*: `queue.json` updates across CLI and TUI were unlocked read-modify-write sequences susceptible to lost updates, and PodFetch DB podcast deletion lacked transactional atomicity.
    - *Mitigation*: Implemented `updateQueue` with inter-process advisory locking and atomic replacement, and wrapped PodFetch episode and podcast database deletions within a database transaction.
