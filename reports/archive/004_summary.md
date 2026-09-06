# Post-Work Summary: Resilience Remediation (Review #004)

**Date**: 2026-09-06  
**Focus Area**: Resilience, Error Handling, Goroutine Safety & Observability  
**Scope**: 7 issues found in `reports/004_resilience.md` (4 Critical, 2 Major, 1 Moderate). All 7 issues were resolved and verified with dedicated regression unit tests.

---

### Issue Summaries & Mitigations

1. **Download Queue Worker Goroutine Panic & Wedging (`download_queue.go`)**
   - *Issue*: Background download worker goroutines lacked panic recovery, risking process crashes and permanently wedging `downloadWorkerRunning=true`.
   - *Mitigation*: Wrapped worker invocation in `TriggerDownloadQueueWorker` with a `defer recover()` boundary that logs the panic stack and guarantees `downloadWorkerRunning=false`.

2. **TUI Download Policy Goroutine Panic Safety (`tui_modal_download.go`)**
   - *Issue*: TUI event loop spawned bare `syncPolicyToBackend` goroutines without panic recovery, risking process termination and leaving the terminal corrupted in raw mode.
   - *Mitigation*: Added `defer recover()` boundaries with stack trace logging both around the goroutine spawn in `applyDownloadPolicyModal` and internally within `syncPolicyToBackend`.

3. **HTTP Response Body Leak in Cover Download (`pkg/backend/podfetch_api.go`)**
   - *Issue*: `resp.Body.Close()` was only deferred conditionally on HTTP 200, leaking sockets and file descriptors on non-200 responses.
   - *Mitigation*: Restructured `DownloadCover` to call `defer resp.Body.Close()` immediately when `err == nil` before checking `resp.StatusCode`.

4. **Unbounded Remote SSH/Rsync Execution (`remote_transport.go`)**
   - *Issue*: SSH and rsync commands executed synchronously without context deadlines or keepalives, allowing stalled remote hosts to freeze CLI and daemon loops indefinitely.
   - *Mitigation*: Built commands using `exec.CommandContext` with configurable timeouts and added `ServerAliveInterval=5` and `ServerAliveCountMax=3` options.

5. **Transient Feed Fetch Retries & Socket Leaks (`feed_cache.go` & `pkg/backend/podfetch_api.go`)**
   - *Issue*: Direct RSS feed fetches failed permanently on transient upstream errors (HTTP 429, 408, 5xx) without retry or backoff.
   - *Mitigation*: Implemented a 3-attempt retry loop with backoff and per-attempt body closure for transient HTTP status codes in `fetchFeedDirect` and `PodcastFeedEpisodes`.

6. **Silently Ignored Deletion Failures in Keep Policy (`pkg/backend/podfetch_ops.go` & `podfetch_api.go`)**
   - *Issue*: `ApplyKeepPolicy` and `DeletePodcastEpisode` swallowed deletion failures, falsely reporting retention success when deletions failed.
   - *Mitigation*: Fixed `DeletePodcastEpisode` to return API errors and updated `ApplyKeepPolicy` to accumulate deletion failures into an explicit returned error.

7. **Discarded Finalize and Worker Processing Errors (`download_queue.go`)**
   - *Issue*: Errors from `finalizeDownloadQueueItem` and `ProcessNextDownloadQueueItem` were discarded with `_`, masking queue state persistence failures.
   - *Mitigation*: Captured and wrapped `finalizeDownloadQueueItem` errors in `ProcessNextDownloadQueueItem`, and logged non-nil processing errors in `runDownloadQueueWorkerLoop`.
