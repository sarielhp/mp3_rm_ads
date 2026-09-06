# Systems Code Review Report #004 (Lens: resilience)

- **Date**: 2026-09-06
- **Auditor**: Claude / Codex (Tier 1 Standard) via `tools/audit`
- **Focus Lens**: `resilience`
- **Model Tier**: `STANDARD`
- **Backend**: `auto`
- **Scope**: `.`
- **Status**: Action Required

---

Auditing via Claude CLI (sonnet, tier: standard) [Profile: resilience]...
# Code Audit: Network Resilience, Error Handling & Observability

---

[SEVERITY]: Critical
[LOCATION]: `./download_queue.go` — `TriggerDownloadQueueWorker` / `runDownloadQueueWorkerLoop`
```go
downloadWorkerRunning = true
downloadWorkerMu.Unlock()
go runDownloadQueueWorkerLoop(client)
```
[ROOT CAUSE]: The background download worker goroutine has no `recover()` boundary. If `ProcessNextDownloadQueueItem` (or anything in `executeQueueItemDownload` → `client.DownloadEpisodes` → JSON/network code) panics — e.g. a nil `client`, a malformed `EpisodeObj`, or an unexpected type assertion deep in the backend — the panic is unrecovered and crashes the **entire process**, not just the download subsystem. Worse: `downloadWorkerRunning = false` is only reset at the normal-exit path at the bottom of the loop. A panic skips that reset entirely, meaning if the panic were ever recovered by an outer layer (e.g. test harness), `downloadWorkerRunning` would remain `true` forever, permanently wedging all future `TriggerDownloadQueueWorker` calls into `downloadWorkerPending = true` with no goroutine ever running again.
[FAILURE TRACE]: User enqueues a download → `TriggerDownloadQueueWorker` spawns goroutine → `client.DownloadEpisodes` hits an unexpected nil pointer or a backend type assertion failure → goroutine panics → entire `abs` CLI process terminates mid-operation (in a TUI session this kills the whole interactive UI), and the download queue file may be left with an item stuck in `"downloading"` status referencing a dead PID that later gets reconciled, but only on next process start.
[REMEDIATION]:
```go
go func() {
    defer func() {
        if r := recover(); r != nil {
            log.Printf("panic in download worker: %v\n%s", r, debug.Stack())
        }
        downloadWorkerMu.Lock()
        downloadWorkerRunning = false
        downloadWorkerMu.Unlock()
    }()
    runDownloadQueueWorkerLoop(client)
}()
```

---

[SEVERITY]: Critical
[LOCATION]: `./tui_modal_download.go` — `applyDownloadPolicyModal`
```go
go syncPolicyToBackend(pod, m.policyAutoDownload, m.policyAutoCleanup, pod.config.AutoCleanupDays)
```
[ROOT CAUSE]: This spawns a bare goroutine from the Bubble Tea TUI event loop with no panic boundary. `syncPolicyToBackend` calls `getBackend` → `backend.New(...)` → `UpdatePodcastSettings`, all of which can panic (e.g. nil DB handle, backend interface type assertion). Since the TUI has put the terminal into raw/alt-screen mode, an unrecovered panic here doesn't just kill the process — it typically leaves the user's terminal in a corrupted raw-mode state with no visible stack trace (stdout is being redrawn by the TUI), producing a very poor and hard-to-diagnose failure for the operator.
[FAILURE TRACE]: User opens the download-policy modal, applies settings for a podcast whose `pod.config.ID` construction path hits an edge case in the backend (e.g. sqlite DB is locked/corrupted causing a nil dereference in `updatePodFetchSettingsDB`'s deeper helpers) → goroutine panics → whole TUI process dies → user's terminal remains in raw mode until they blindly type `reset`.
[REMEDIATION]:
```go
go func() {
    defer func() {
        if r := recover(); r != nil {
            log.Printf("panic syncing policy to backend: %v\n%s", r, debug.Stack())
        }
    }()
    syncPolicyToBackend(pod, m.policyAutoDownload, m.policyAutoCleanup, pod.config.AutoCleanupDays)
}()
```

---

[SEVERITY]: Critical
[LOCATION]: `./pkg/backend/podfetch_api.go` — `DownloadCover`
```go
client := &http.Client{Timeout: 60 * time.Second}
resp, err := client.Get(imgURL)
if err == nil && resp.StatusCode == http.StatusOK {
    defer resp.Body.Close()
    ...
}
```
[ROOT CAUSE]: `resp.Body.Close()` is only deferred inside the `StatusCode == http.StatusOK` branch. If the request succeeds (`err == nil`) but returns any non-200 status (301/302/403/404/500 — extremely common for stale cover-image URLs), `resp.Body` is never closed. Each leaked body keeps the underlying TCP connection un-released from the client's connection pool until the GC finalizer eventually reclaims it (not guaranteed timely), exhausting file descriptors/sockets under load.
[FAILURE TRACE]: Run `abs remote scan` / library scan across a podcast library where a non-trivial fraction of `ImageURL`s are dead links returning 404 (common for podcasts with rotated CDN URLs) → each `DownloadCover` call leaks one socket/fd → repeated scans or large libraries (hundreds of podcasts) exhaust the process's open-file limit, causing subsequent unrelated `os.Open`/`net.Dial` calls to fail with `EMFILE`.
[REMEDIATION]:
```go
resp, err := client.Get(imgURL)
if err == nil {
    defer resp.Body.Close()
    if resp.StatusCode == http.StatusOK {
        ...
    }
}
```

---

[SEVERITY]: Critical
[LOCATION]: `./remote_transport.go` — `DefaultSSHTransport.Exec`, `RsyncTo`, `RsyncFrom`
```go
func (t *DefaultSSHTransport) Exec(host string, cmd string) (string, error) {
	c := exec.Command("ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", host, cmd)
	...
	err := c.Run()
```
[ROOT CAUSE]: `ConnectTimeout=5` only bounds the initial TCP/SSH handshake. Once the SSH session is established, there is no deadline on command execution — `exec.Command` is never built with `exec.CommandContext` nor given a `time.AfterFunc(...).Process.Kill()` watchdog. Every remote operation in the codebase (`status`, `scan`, `worker`, `push`, `pull`, `cancel`, `stop`, `clear`) funnels through this transport and blocks synchronously on `c.Run()`. A hung remote command (stalled Docker daemon, deadlocked `grep` on an NFS-mounted work dir, a half-open TCP connection after a network blip that the OS hasn't yet timed out) hangs the entire CLI invocation indefinitely with zero user feedback and no way to recover short of `kill -9` from another terminal. This is worse for `runRemoteWorkerLoop` (`abs remote worker --daemon`), which is meant to run unattended for hours: a single hung `Exec` call permanently freezes the daemon loop.
[FAILURE TRACE]: `abs remote status host` is issued → `fetchRemoteVersionAndWorkerStatus`/`checkAndRecoverRemoteWorker` calls `transport.Exec` against a host whose network briefly black-holes packets after the SSH session negotiates → TCP keepalive/timeout not configured on the ssh invocation → `c.Run()` blocks for the OS default TCP retransmission timeout (can be 15+ minutes or effectively forever without `ServerAliveInterval`) → CLI appears completely frozen.
[REMEDIATION]:
```go
func (t *DefaultSSHTransport) Exec(host string, cmd string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), t.getTimeout())
	defer cancel()
	c := exec.CommandContext(ctx, "ssh",
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=5",
		"-o", "ServerAliveInterval=5",
		"-o", "ServerAliveCountMax=3",
		host, cmd)
	...
	if ctx.Err() == context.DeadlineExceeded {
		return outStr, fmt.Errorf("ssh command on %s timed out after %s", host, t.getTimeout())
	}
```
Apply the same `CommandContext` treatment to `RsyncTo`/`RsyncFrom`.

---

[SEVERITY]: Major
[LOCATION]: `./feed_cache.go` — `fetchFeedDirect`; `./pkg/backend/podfetch_api.go` — `PodcastFeedEpisodes` (direct-fetch fallback)
```go
resp, err := client.Do(req)
...
if resp.StatusCode != http.StatusOK {
    return nil, "", "", false, fmt.Errorf("feed returned HTTP %d", resp.StatusCode)
}
```
[ROOT CAUSE]: Both RSS-fetch paths use an `http.Client` with only a blanket timeout — there is no retry/backoff for transient upstream conditions (`429 Too Many Requests`, `503 Service Unavailable`, `502/504`) despite `pkg/backend/podfetch.go`'s `Request` method already implementing exactly this pattern for its own API calls. Feed hosts (Cloudflare-fronted CDNs, podcast hosting platforms) routinely return `429`/`503` under transient load; a single failed attempt here permanently skips the podcast's feed refresh for that invocation instead of retrying with jitnovered backoff.
[FAILURE TRACE]: `abs remote scan` iterates dozens of podcasts sequentially; the feed CDN rate-limits after N rapid requests and returns `429` for episode N+1 → `fetchFeedDirect` returns an error on first attempt → caller (`fetchFeedEpisodesForPodcast`, `PodcastFeedEpisodes`) logs a failure and moves on, permanently missing new episodes for that podcast until the next manual run — with no automatic recovery even though the rate limit would have cleared in a second.
[REMEDIATION]:
```go
func fetchFeedDirect(feedURL, cachedETag, cachedLastMod string) ([]FeedEpisode, string, string, bool, error) {
    ...
    const maxAttempts = 3
    var lastErr error
    for attempt := 1; attempt <= maxAttempts; attempt++ {
        resp, err := client.Do(req)
        if err != nil {
            lastErr = err
        } else {
            defer resp.Body.Close()
            if resp.StatusCode == 429 || resp.StatusCode >= 500 {
                lastErr = fmt.Errorf("feed returned HTTP %d", resp.StatusCode)
            } else {
                // handle success/304 as before
                return ..., nil
            }
        }
        if attempt < maxAttempts {
            jitter := time.Duration(rand.Intn(500)) * time.Millisecond
            time.Sleep(time.Duration(attempt)*time.Second + jitter)
        }
    }
    return nil, "", "", false, lastErr
}
```

---

[SEVERITY]: Major
[LOCATION]: `./pkg/backend/podfetch_ops.go` — `ApplyKeepPolicy`
```go
for _, ep := range episodesToDelete {
    if !dryRun {
        if err := c.DeletePodcastEpisode(podcastID, ep.ID); err == nil {
            deletedCount++
        }
    }
}
```
[ROOT CAUSE]: The error from `DeletePodcastEpisode` is checked only to conditionally increment a counter — it is never logged, wrapped, or surfaced. A retention/keep-policy is a destructive-adjacent, correctness-critical operation (its entire purpose is enforcing "keep only N episodes"); silently ignoring failures means the policy silently fails to enforce its invariant with zero operator visibility. Compounding this, the caller `applyKeepPolicy` in `backend_client.go` further reduces `(deletedCount, error)` down to a single `bool`, discarding even the count.
[FAILURE TRACE]: Disk quota or permission issue on the podcast host causes every `DeletePodcastEpisode` call to fail (e.g., 403 from backend API) → `deletedCount` stays 0, function returns `(0, nil)` → caller reports success ("Applied keep policy") → episodes silently accumulate unbounded despite `keep: 3` config, exhausting disk over weeks with no error ever logged.
[REMEDIATION]:
```go
var errs []string
for _, ep := range episodesToDelete {
    if dryRun {
        continue
    }
    if err := c.DeletePodcastEpisode(podcastID, ep.ID); err != nil {
        errs = append(errs, fmt.Sprintf("%s: %v", ep.ID, err))
        continue
    }
    deletedCount++
}
if len(errs) > 0 {
    return deletedCount, fmt.Errorf("keep policy: %d deletion(s) failed: %s", len(errs), strings.Join(errs, "; "))
}
return deletedCount, nil
```

---

[SEVERITY]: Moderate
[LOCATION]: `./download_queue.go` — `ProcessNextDownloadQueueItem` / `runDownloadQueueWorkerLoop`
```go
dlErr := executeQueueItemDownload(client, podcastID, item)
_ = finalizeDownloadQueueItem(item.ID, dlErr)   // (A) discards finalize error
return true, dlErr
...
processed, _ := ProcessNextDownloadQueueItem(client)  // (B) discards processing error
```
[ROOT CAUSE]: Two layers of error discarding compound: (A) if `finalizeDownloadQueueItem` itself fails (e.g. `saveDownloadQueue` fails due to a full disk or a concurrent lock timeout), the queue item remains permanently in `"downloading"` status with a live `OwnerPID` (the current process), so `reconcileStaleDownloadingItems` will never reclaim it (`isProcessAlive(pid)` is true) — the item is stuck until process restart. (B) `runDownloadQueueWorkerLoop` then also drops the returned error from (A)/(B), so there is no logging path at all — this failure mode is completely invisible to both the user and any log stream.
[FAILURE TRACE]: Disk fills up momentarily while `saveDownloadQueue` writes the "downloading→completed" transition → `writeFileAtomic` fails → `finalizeDownloadQueueItem` returns an error → discarded → item is permanently wedged as "downloading" with the current (still-alive) PID → worker loop moves on silently, no operator signal that a queue entry is now un-claimable.
[REMEDIATION]:
```go
processed, err := ProcessNextDownloadQueueItem(client)
if err != nil {
    log.Printf("download queue: processing error: %v", err)
}
```
and in `ProcessNextDownloadQueueItem`:
```go
if fErr := finalizeDownloadQueueItem(item.ID, dlErr); fErr != nil {
    return true, fmt.Errorf("download error=%v, finalize error=%w", dlErr, fErr)
}
```
