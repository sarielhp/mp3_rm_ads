# Systems Remediation Plan #001

## Overview
This architectural remediation plan defines concrete implementation strategies and regression tests for all 10 issues identified in `reports/001_systems.md`. Every remediation adheres strictly to the project guidelines in `AGENTS.md` (Go stdlib only + existing flock, functions <= 80 lines, files within comfort limit, temp files in `.work/`, no code comments, `t.TempDir()` in tests).

---

## Issue Details, Architectural Strategy, and Test Plans

### Issue 1: Flawed PID-file Locking in `acquireWorkerLock`
- **Location**: `lock.go:76-95, 98-131`
- **Severity**: Critical
- **Root Cause**: PID-file based locking protocol deletes locks belonging to active workers (due to arbitrary 6-hour timeout) and suffers from an ABA deletion race where two competing processes inspect a lock and delete newly acquired locks.
- **Architectural Strategy**:
  - Replace the PID text file / `os.Remove` protocol with an OS advisory lock using `github.com/gofrs/flock` on `.worker.lock`.
  - The OS advisory lock is bound to the kernel file descriptor table and releases automatically when the process terminates, eliminating stale lock cleanup logic and races.
  - Support in-process re-entrancy via an in-memory reference table guarded by a `syncMutex`, allowing recursive invocation (e.g. `runRemoteWorker` calling `runRemoteScan`) within the same process.
  - Remove dead/stale PID inspection functions (`checkStaleWorkerLock`).
- **Regression Test Plan**:
  - Update and expand worker lock tests in `remote_sync_test.go` and `systems_review_fixes_test.go`:
    1. Lock acquisition on a clean directory succeeds.
    2. In-process re-entrant acquisition succeeds and returns an unlock function.
    3. Releasing nested locks leaves the underlying lock active until the outer lock is released.
    4. Cross-process / second flock attempt fails with "remote worker is already running".

---

### Issue 2: Fail-Open Lock Acquisition in `checkSkipOrLockAudioFile`
- **Location**: `batch_proc_file.go:137-149`
- **Severity**: Critical
- **Root Cause**: In `checkSkipOrLockAudioFile`, if `acquireFileLock(mainMP3File)` returns an error (`err != nil`), the error is logged to stderr but the function falls through and returns `fileLock (nil), true, false`. This allows destructive audio processing to proceed unlocked upon permission or filesystem errors.
- **Architectural Strategy**:
  - Update `checkSkipOrLockAudioFile` to fail closed: if `err != nil`, log warning (if not quiet) and immediately return `nil, false, false`.
- **Regression Test Plan**:
  - Add unit test `TestCheckSkipOrLockAudioFile_LockErrorFailsClosed`:
    - Point `checkSkipOrLockAudioFile` to a file in an inaccessible/unwritable path (or simulate lock failure) and verify it returns `fileLock == nil`, `proceed == false`, `stopped == false`.

---

### Issue 3: Cross-Process Queue Claiming and Lease Invariant in `ProcessNextDownloadQueueItem`
- **Location**: `download_queue.go:234-309`
- **Severity**: Major
- **Root Cause**: `downloadQueueMutex` is only in-process; the shared persistent queue file `download_queue.json` can be read and claimed concurrently by multiple processes without an inter-process file lock. Transitions to `"downloading"` ignore write errors, and `reconcileStaleDownloadingItems` indiscriminately requeues any `"downloading"` item even if another process is actively downloading it.
- **Architectural Strategy**:
  - Add `OwnerPID int` to `DownloadQueueItem`.
  - Encapsulate queue claiming in `claimDownloadQueueItem() (DownloadQueueItem, bool, error)`:
    - Protect the read-modify-write operation with an inter-process advisory file lock (`acquireFileLock(getDownloadQueueFilePath())`).
    - Mark the selected item as `"downloading"` and record `OwnerPID = os.Getpid()`.
    - Durably save the queue and propagate any filesystem errors.
  - In `reconcileStaleDownloadingItems`, only requeue items if `OwnerPID <= 0` or `!isProcessAlive(OwnerPID)`.
- **Regression Test Plan**:
  - Add unit test `TestDownloadQueue_CrossProcessClaiming`:
    - Verify that claiming sets `OwnerPID`, writes durably, and reconciles only dead PIDs while preserving active owner leases.

---

### Issue 4: Lost-Wakeup Race in `TriggerDownloadQueueWorker`
- **Location**: `download_queue.go:312-335`
- **Severity**: Major
- **Root Cause**: If a trigger arrives while `downloadWorkerRunning` is true, the trigger is dropped. If the active worker has already finished checking the queue and is in the middle of terminating, newly enqueued items remain unhandled indefinitely.
- **Architectural Strategy**:
  - Introduce `downloadWorkerPending bool` guarded by `downloadWorkerMu`.
  - When `TriggerDownloadQueueWorker` is called while `downloadWorkerRunning` is true, set `downloadWorkerPending = true`.
  - In the worker goroutine loop, before clearing `downloadWorkerRunning` and exiting, check if `downloadWorkerPending` is true: if so, clear `downloadWorkerPending` and continue the processing loop.
- **Regression Test Plan**:
  - Add unit test `TestDownloadQueue_WorkerPendingWakeup`:
    - Enqueue an item while simulating worker termination and ensure the pending trigger processes the subsequent item.

---

### Issue 5: Potential Nil Dereference in `ProcessNextDownloadQueueItem`
- **Location**: `download_queue.go:254-291`
- **Severity**: Major
- **Root Cause**: If `client` is nil (e.g. ABS disabled or credentials missing) and `item.PodcastID` is nonempty, `ProcessNextDownloadQueueItem` enters the non-empty branch and dereferences `client.DownloadEpisodes`, causing a panic.
- **Architectural Strategy**:
  - Replace unchecked dereference with a guarded switch:
    - `case client == nil`: set `dlErr = fmt.Errorf("download client is unavailable")`
    - `case podcastID == ""`: set `dlErr = fmt.Errorf("podcast ID not found for %q", item.PodcastTitle)`
    - `default`: call `client.DownloadEpisodes(...)`
  - Record the failure status on the queue item and return the error cleanly.
- **Regression Test Plan**:
  - Add unit test `TestProcessNextDownloadQueueItem_NilClientSafe`:
    - Enqueue an item with a predefined `PodcastID`, invoke `ProcessNextDownloadQueueItem(nil)`, and verify it returns an error without panicking and transitions the queue item to `"failed"`.

---

### Issue 6: Redundant Direct Download Execution in TUI Enqueue Paths
- **Location**: `tui_batch.go:127-147, 199-226`
- **Severity**: Major
- **Root Cause**: The TUI directly calls `absCli.DownloadEpisodes(...)` and subsequently triggers `TriggerDownloadQueueWorker(absCli)`, which also executes `DownloadEpisodes` for the same item, causing duplicate download submissions.
- **Architectural Strategy**:
  - In `enqueueDownloadFromEpisodeList`: remove the direct call to `absCli.DownloadEpisodes`. The function enqueues the item and triggers the background worker.
  - In `batchQueueDownload`: remove the `toDownload` collection and direct `absCli.DownloadEpisodes` call. It enqueues the batch items and triggers the worker.
- **Regression Test Plan**:
  - Add unit tests verifying that both single and batch enqueue operations delegate execution solely to the download queue worker without issuing duplicate calls.

---

### Issue 7: Non-Transactional Original Preservation in `installCutAudioAndPreserveOriginal`
- **Location**: `batch_proc_file.go:283-299`
- **Severity**: Major
- **Root Cause**: Moving `mainMP3File` to `precutFile` before installing `tempOutputFile` leaves the canonical audio file missing if the subsequent move fails or the process crashes.
- **Architectural Strategy**:
  - Preserve `mainMP3File` to `precutFile` using hard link `os.Link(mainMP3File, precutFile)` (falling back to `copyFileErr` if cross-device or unsupported) WITHOUT removing `mainMP3File`.
  - Atomically move `tempOutputFile` to `outputFile` via `safeMove`.
  - If the move fails, remove `precutFile` to cleanly rollback.
- **Regression Test Plan**:
  - Add unit test `TestInstallCutAudio_AtomicPreserveRollback`:
    - Test successful installation verifies both files exist.
    - Test failure to move temp output keeps `mainMP3File` intact.

---

### Issue 8: Premature Metadata Commit in `handleNoAdsDetected`
- **Location**: `batch_proc_file.go:218-232`
- **Severity**: Major
- **Root Cause**: `handleNoAdsDetected` writes cuts JSON and sets status to `StateDone` before copying the audio file to `outputFile`. Copy errors are discarded and non-atomic, risking truncated or missing output while marking the item completed.
- **Architectural Strategy**:
  - Implement `installNoAdsOutput(source, output string) error` that stages the copy via `.work/` and atomically moves it to `outputFile`.
  - Only update `saveCutsJSON` and set `StateDone` after `installNoAdsOutput` succeeds.
  - Return early with error logging if installation fails.
- **Regression Test Plan**:
  - Add unit test `TestHandleNoAdsDetected_InstallOutputFailureDoesNotCommit`:
    - Verify that when the output path is unwritable, `StateDone` is not written.

---

### Issue 9: Premature Local State Transition in `pushSingleAudioFile`
- **Location**: `remote_batch.go:111-136`
- **Severity**: Major
- **Root Cause**: `pushSingleAudioFile` marks local state as `StateQueuedRemote` before attempting upload. If upload fails, local state is left stranded in `StateQueuedRemote`, preventing subsequent reprocessing.
- **Architectural Strategy**:
  - Prepare the staging status file in `.work/` (in compliance with Temp File Policy).
  - Execute audio and status uploads first.
  - Only upon successful upload of both files, transition the local episode status to `StateQueuedRemote` and persist it.
- **Regression Test Plan**:
  - Add unit test `TestPushSingleAudioFile_UploadFailureRollback`:
    - Mock failing transport and verify local episode status is not changed to `StateQueuedRemote`.

---

### Issue 10: Unlocked `queue.json` Updates in CLI and TUI
- **Location**: `queue_cmd.go:205-220, 223-250, 278-297`; `tui_data_queue.go:52-60`
- **Severity**: Moderate
- **Root Cause**: Updates to `queue.json` use unlocked read-modify-write patterns and suppress save errors, causing lost updates between concurrent operations.
- **Architectural Strategy**:
  - Introduce `updateQueue(dir string, mutate func([]string) []string) error` in `queue_cmd.go`:
    - Acquires advisory file lock on `queue.json.lock`.
    - Reads existing entries, applies `mutate`, formats JSON, and writes atomically.
  - Update `addEpisodeToQueueFile`, `removeEpisodeFromQueueFile`, `addPodcastEpisodesToQueue`, and `saveQueue` to use `updateQueue`.
- **Regression Test Plan**:
  - Add unit test `TestUpdateQueue_ConcurrentTransactions`:
    - Concurrently add and remove distinct episodes and verify that no updates are lost.

---

## Verification & Execution Order
1. Implement Issue 1 (`lock.go`) + tests.
2. Implement Issue 2 (`batch_proc_file.go`) + tests.
3. Implement Issues 3, 4, 5 (`download_queue.go`) + tests.
4. Implement Issue 6 (`tui_batch.go`) + tests.
5. Implement Issues 7, 8 (`batch_proc_file.go`) + tests.
6. Implement Issue 9 (`remote_batch.go`) + tests.
7. Implement Issue 10 (`queue_cmd.go`, `tui_data_queue.go`) + tests.
8. Run `./tools/check` to verify all linters, audits (function line count <= 80), staticcheck, and tests pass.
9. Generate `reports/001_summary.md`.
10. Commit all changes cleanly.
