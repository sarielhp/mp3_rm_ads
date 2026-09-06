# Systems Code Review Report #001 (Lens: systems)

- **Date**: 2026-09-06
- **Auditor**: Claude / Codex (Tier 1 Standard) via `tools/audit`
- **Focus Lens**: `systems`
- **Model Tier**: `STANDARD`
- **Backend**: `auto`
- **Scope**: `.`
- **Status**: Action Required

---

Auditing via Codex CLI (gpt-5.6-sol, tier: standard) [Profile: systems]...
[SEVERITY]: Critical  
[LOCATION]: `lock.go:76-95, 98-131` — `checkStaleWorkerLock` / `acquireWorkerLock`  
[ROOT CAUSE]: The PID-file locking protocol can remove a lock belonging to a live worker. Any lock older than six hours is declared stale even when its PID is alive. Stale-lock reclamation also has an ABA race: two contenders can inspect the same stale file, after which one deletes the newly created lock of the other.  
[FAILURE TRACE]:

1. Worker A legitimately runs longer than six hours, or an old stale lock exists.
2. Processes B and C both classify the lock as stale.
3. B removes it and creates a new lock containing B’s PID.
4. C executes its already-authorized `os.Remove(lockPath)`, deleting B’s lock, then creates its own.
5. Both B and C process the same remote queue concurrently; either unlock callback can subsequently delete the other owner’s lock.

[REMEDIATION]: Use an OS advisory lock whose ownership is tied to an open descriptor. Do not infer ownership from age or unlink locks during acquisition.

```go
func acquireWorkerLock(resolvedDir string) (func(), error) {
	lockPath := filepath.Join(resolvedDir, ".worker.lock")
	fl := flock.New(lockPath)

	locked, err := fl.TryLock()
	if err != nil {
		return nil, fmt.Errorf("acquire worker lock %s: %w", lockPath, err)
	}
	if !locked {
		return nil, fmt.Errorf("remote worker is already running")
	}

	return func() {
		_ = fl.Unlock()
	}, nil
}
```

---

[SEVERITY]: Critical  
[LOCATION]: `batch_proc_file.go:137-149` — `checkSkipOrLockAudioFile`  
[ROOT CAUSE]: Lock acquisition fails open. An actual locking error prints a warning but processing continues with a nil lock. This converts permission errors, filesystem failures, and unsupported locking into authorization for unprotected destructive audio processing.  
[FAILURE TRACE]:

1. Two processes target the same MP3.
2. Both encounter a lock error, such as an inaccessible lock path or filesystem I/O failure.
3. Both continue because only `fileLock == nil && err == nil` causes a skip.
4. Both transcribe/cut and race through moves of the original, `.precut`, output, status, and work directory.
5. One process can remove or overwrite files still in use by the other.

[REMEDIATION]:

```go
fileLock, err := acquireFileLock(mainMP3File)
if err != nil {
	if !cli.Quiet {
		fmt.Fprintf(os.Stderr, "Cannot safely process %s: %v\n", shortName, err)
	}
	return nil, false, false
}
if fileLock == nil {
	if !cli.Quiet {
		fmt.Printf("Skipping %q: already being processed\n", shortName)
	}
	return nil, false, false
}
```

---

[SEVERITY]: Major  
[LOCATION]: `download_queue.go:234-309` — `ProcessNextDownloadQueueItem`  
[ROOT CAUSE]: Queue claiming is neither cross-process exclusive nor durably validated. `downloadQueueMutex` protects only one process, while the persistent queue is shared. The transition to `"downloading"` is also executed with its error discarded. `reconcileStaleDownloadingItems` can requeue another live process’s active item because `"downloading"` carries no owner or lease.  
[FAILURE TRACE]:

1. Two application processes read the same queued item.
2. Each process holds only its own `downloadQueueMutex`.
3. Both select the item and write `"downloading"`.
4. Alternatively, one write fails due to disk exhaustion but that process still continues.
5. Both submit the same download, and their final whole-file saves overwrite one another’s state.

[REMEDIATION]: Serialize persistent read-modify-write operations with an inter-process lock, check persistence errors, and use an owner/lease rather than blindly requeuing every `"downloading"` item.

```go
func claimDownloadQueueItem() (DownloadQueueItem, bool, error) {
	lock, err := acquireFileLock(getDownloadQueueFilePath())
	if err != nil || lock == nil {
		return DownloadQueueItem{}, false, err
	}
	defer lock.Release()

	q := loadDownloadQueue()
	for i := range q.Items {
		if q.Items[i].Status != "queued" {
			continue
		}
		q.Items[i].Status = "downloading"
		if err := saveDownloadQueue(q); err != nil {
			return DownloadQueueItem{}, false, err
		}
		return q.Items[i], true, nil
	}
	return DownloadQueueItem{}, false, nil
}
```

---

[SEVERITY]: Major  
[LOCATION]: `download_queue.go:312-335` — `TriggerDownloadQueueWorker`  
[ROOT CAUSE]: Worker shutdown has a lost-wakeup race. A trigger observed while `downloadWorkerRunning` is still true is discarded, even if the worker has already determined the queue is empty and is committed to exiting.  
[FAILURE TRACE]:

1. Worker calls `ProcessNextDownloadQueueItem` and receives `processed == false`.
2. Another goroutine enqueues an item.
3. Its trigger sees `downloadWorkerRunning == true` and returns.
4. The existing worker exits and only then clears `downloadWorkerRunning`.
5. The new item remains queued indefinitely until another manual trigger occurs.

[REMEDIATION]: Record triggers received while a worker is active and consume that pending signal before exiting.

```go
var downloadWorkerPending bool

func TriggerDownloadQueueWorker(client *ABSClient) {
	downloadWorkerMu.Lock()
	if downloadWorkerRunning {
		downloadWorkerPending = true
		downloadWorkerMu.Unlock()
		return
	}
	downloadWorkerRunning = true
	downloadWorkerMu.Unlock()

	go func() {
		for {
			processed, _ := ProcessNextDownloadQueueItem(client)
			if processed {
				continue
			}

			downloadWorkerMu.Lock()
			if downloadWorkerPending {
				downloadWorkerPending = false
				downloadWorkerMu.Unlock()
				continue
			}
			downloadWorkerRunning = false
			downloadWorkerMu.Unlock()
			return
		}
	}()
}
```

---

[SEVERITY]: Major  
[LOCATION]: `download_queue.go:254-291` — `ProcessNextDownloadQueueItem`  
[ROOT CAUSE]: `client` can remain nil while `podcastID` is nonempty. The branch decision checks only `podcastID`, then unconditionally dereferences `client`.  
[FAILURE TRACE]:

1. A persisted queue item contains `PodcastID`.
2. Processing is triggered with `client == nil`.
3. Audiobookshelf is disabled, its URL is empty, or client construction is unavailable.
4. `podcastID == ""` is false.
5. Line 291 calls `client.DownloadEpisodes`, causing a nil-pointer panic in the background goroutine and terminating the process.

[REMEDIATION]:

```go
var dlErr error
switch {
case client == nil:
	dlErr = fmt.Errorf("download client is unavailable")
case podcastID == "":
	dlErr = fmt.Errorf("podcast ID not found for %q", item.PodcastTitle)
default:
	dlErr = client.DownloadEpisodes(podcastID, []FeedEpisode{feedEp})
}
```

---

[SEVERITY]: Major  
[LOCATION]: `tui_batch.go:127-147` and `tui_batch.go:199-226` — single and batch download enqueue paths  
[ROOT CAUSE]: Two components own the same side effect. The TUI directly calls `DownloadEpisodes`, then starts the queue worker, which calls `DownloadEpisodes` again for the same queued records.  
[FAILURE TRACE]:

1. A feed-only episode is successfully appended to the queue.
2. The TUI immediately submits it at lines 145 or 224.
3. The item remains `"queued"`.
4. The worker starts and claims that same item.
5. The worker submits an identical download request, potentially creating duplicate server jobs or competing file writes.

[REMEDIATION]: Make the queue worker the sole download executor.

```go
if queuedCount > 0 {
	m.showToast(
		fmt.Sprintf("Batch enqueued %d episode(s) for download", queuedCount),
		ToastSuccess,
	)
	TriggerDownloadQueueWorker(absCli)
}
```

Remove the direct `absCli.DownloadEpisodes(...)` calls from both enqueue paths.

---

[SEVERITY]: Major  
[LOCATION]: `batch_proc_file.go:283-299` — `installCutAudioAndPreserveOriginal`  
[ROOT CAUSE]: Installing cut audio is a non-transactional two-move operation. The original path is removed before the replacement is installed. A crash or second-move failure leaves the canonical episode path missing.  
[FAILURE TRACE]:

1. `mainMP3File` is moved to `precutFile`.
2. The process crashes, the filesystem fills, or `safeMove(tempOutputFile, outputFile)` fails.
3. `mainMP3File`/`outputFile` no longer exists.
4. The original survives only under the internal `.precut` name, leaving the media library broken.

[REMEDIATION]: Preserve the original without removing the canonical path, then atomically rename the completed output over it.

```go
if sourceAudioFile == mainMP3File && fileExists(mainMP3File) {
	checkPrecutSymlink(precutFile)
	if err := os.Link(mainMP3File, precutFile); err != nil {
		return false
	}
}

if err := os.Rename(tempOutputFile, outputFile); err != nil {
	_ = os.Remove(precutFile)
	return false
}
return true
```

A durable atomic-copy helper may replace `os.Link` where hard links are unsuitable.

---

[SEVERITY]: Major  
[LOCATION]: `batch_proc_file.go:218-232` — `handleNoAdsDetected`  
[ROOT CAUSE]: Completion metadata is committed before the output file is installed. The output copy is non-atomic and its error is discarded. Both the `"done"` status and zero-cut metadata can cause later runs to treat an absent or partially copied output as complete.  
[FAILURE TRACE]:

1. The LLM returns no ad segments.
2. Cuts metadata and `StateDone` are persisted.
3. `copyFile` opens/truncates the output.
4. Copying fails or the process crashes midway.
5. The episode is subsequently skipped as completed although its declared cleaned output is missing or truncated.

[REMEDIATION]: Install the output atomically and successfully before committing completion metadata.

```go
func installNoAdsOutput(source, output string) error {
	if source == output {
		return nil
	}
	workDir := workDirFor(output)
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return err
	}
	tmp := filepath.Join(workDir, filepath.Base(output)+".tmp")
	verifyTempFile(tmp)

	if err := copyFileErr(source, tmp); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return safeMove(tmp, output)
}

// Only after installNoAdsOutput succeeds:
saveCutsJSON(mainMP3File, totalDuration, nil, &selectedProfile, cli.Quiet)
return updateEpisodeStatus(mainMP3File, func(st *EpisodeStatusFile) {
	st.Status = StateDone
	st.Cleaned = EpisodeAudioMeta{
		Filename: filepath.Base(outputFile),
		DurationSec: totalDuration,
	}
	st.Ads = nil
})
```

---

[SEVERITY]: Major  
[LOCATION]: `remote_batch.go:111-136` — `pushSingleAudioFile`  
[ROOT CAUSE]: Local state transitions to `StateQueuedRemote` before either remote upload succeeds, and the status-save error is ignored. A failed transfer therefore leaves a false in-flight marker that normal filtering treats as authoritative.  
[FAILURE TRACE]:

1. Local status is saved as `queued_remote`.
2. Audio upload or remote status upload fails.
3. The function returns an error without rolling back local state.
4. A later batch run sees `isEpisodeInRemoteFlight(f)` and filters the episode out.
5. The episode remains stranded despite never having become a valid remote job.

[REMEDIATION]: Publish remote artifacts first and commit the local transition only after successful remote admission.

```go
if err := saveEpisodeStatus(tmpStatPath, &remoteStat); err != nil {
	return err
}
defer os.Remove(tmpStatPath)

if err := transport.Upload(targetHost, f, remoteDstFile); err != nil {
	return fmt.Errorf("upload audio: %w", err)
}
if err := transport.Upload(targetHost, tmpStatPath, remoteDstStatus); err != nil {
	return fmt.Errorf("upload status: %w", err)
}

localStat.Status = StateQueuedRemote
if err := saveEpisodeStatus(statusPathFor(f), localStat); err != nil {
	return fmt.Errorf("record remote admission: %w", err)
}
```

Ideally the remote files should also be uploaded under partial names and atomically published together.

---

[SEVERITY]: Major  
[LOCATION]: `pkg/backend/podfetch_db.go:218-228` — `deletePodFetchPodcastDB`  
[ROOT CAUSE]: Deleting a podcast and its episodes is one logical operation implemented as two independent autocommit statements. The first error is discarded, and no transaction protects against interruption or failure between statements.  
[FAILURE TRACE]:

1. The episode deletion succeeds.
2. The process crashes, SQLite returns `BUSY`, or the podcast deletion fails.
3. All episodes are permanently deleted while the podcast remains.
4. Alternatively, the ignored first deletion fails and the podcast row is removed, leaving orphaned episode rows.

[REMEDIATION]:

```go
tx, err := db.Begin()
if err != nil {
	return err
}
defer tx.Rollback()

if _, err := tx.Exec(
	"DELETE FROM podcast_episodes WHERE podcast_id = ?", podcastID,
); err != nil {
	return err
}
if _, err := tx.Exec(
	"DELETE FROM podcasts WHERE id = ? OR name = ? OR directory = ?",
	podcastID, podcastID, podcastID,
); err != nil {
	return err
}
return tx.Commit()
```

---

[SEVERITY]: Moderate  
[LOCATION]: `queue_cmd.go:205-220, 223-250, 278-297`; `tui_data_queue.go:52-60`  
[ROOT CAUSE]: `queue.json` updates are unlocked read-modify-write sequences. Atomic replacement prevents malformed JSON but does not prevent lost updates between the CLI, TUI, or multiple application instances. Save errors are also suppressed.  
[FAILURE TRACE]:

1. Process A and process B read the same queue.
2. A adds episode X while B removes episode Y.
3. A saves `[... X]`.
4. B saves its independently filtered snapshot afterward.
5. Episode X silently disappears, or Y reappears, despite both commands reporting success.

[REMEDIATION]: Put the complete read-modify-write transaction under a per-queue inter-process lock and return persistence errors.

```go
func updateQueue(dir string, mutate func([]string) []string) error {
	path := filepath.Join(dir, "queue.json")
	lock, err := acquireFileLock(path)
	if err != nil || lock == nil {
		return fmt.Errorf("queue is locked: %w", err)
	}
	defer lock.Release()

	var entries []string
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &entries); err != nil {
			return err
		}
	}
	entries = mutate(entries)
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data, 0644)
}
```
