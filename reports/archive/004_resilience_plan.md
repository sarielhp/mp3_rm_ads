# Architectural Remediation Plan: Resilience Audit (Review Report #004)

**Document**: `reports/004_resilience_plan.md`  
**Date**: 2026-09-06  
**Status**: In Progress  
**Lens**: Resilience, Error Handling, Goroutine Safety & Observability  

---

## 1. Overview & Objectives

This plan addresses all seven defects identified in `reports/004_resilience.md`:
1. **Critical [SEV-1]**: Background goroutine panic boundary & state reset in `download_queue.go`.
2. **Critical [SEV-1]**: Unhandled goroutine panic in TUI download policy modal (`tui_modal_download.go`).
3. **Critical [SEV-1]**: Socket & file-descriptor leak on non-200 responses in `pkg/backend/podfetch_api.go` (`DownloadCover`).
4. **Critical [SEV-1]**: Unbounded command execution & missing keepalives in `remote_transport.go` (`DefaultSSHTransport`).
5. **Major [SEV-2]**: Missing retry/backoff on transient upstream errors (429/5xx) in `feed_cache.go` and `pkg/backend/podfetch_api.go`.
6. **Major [SEV-2]**: Silently swallowed errors in `pkg/backend/podfetch_ops.go` (`ApplyKeepPolicy`).
7. **Moderate [SEV-3]**: Discarded errors in download queue worker finalization & processing loop (`download_queue.go`).

### Invariants & Quality Standards
- **Function Line Limit**: Every modified or newly introduced function must be $\le 80$ lines.
- **File Line Comfort**: All modified files must remain within the 300–700 line comfort range (hard warning at 800, limit at 1100).
- **Regression Testing**: Every defect remediated must have at least one dedicated regression test verifying the fix.
- **Verification**: All quality gates must pass via `./tools/check` (`make check`).

---

## 2. Detailed Technical Remediation

### Defect 1: Goroutine Panic Recovery & Worker Wedging (`download_queue.go`)
- **Problem**: `TriggerDownloadQueueWorker` spawns `go runDownloadQueueWorkerLoop(client)` without a `recover()` boundary. If `runDownloadQueueWorkerLoop` panics, the entire process terminates. If recovered upstream, `downloadWorkerRunning` is never reset to `false`, permanently wedging the worker.
- **Mitigation**:
  - Wrap the worker execution inside `TriggerDownloadQueueWorker` with a `defer` block that captures `recover()`, logs the panic and stack trace with `debug.Stack()`, and guarantees `downloadWorkerRunning = false` under `downloadWorkerMu.Lock()`.
  - Ensure `runDownloadQueueWorkerLoop` also safely coordinates with this exit lifecycle.
  - Introduce `testDownloadExecuteHook` to allow unit tests to inject panics into download execution deterministically.
- **Regression Test**:
  - `TestTriggerDownloadQueueWorker_PanicRecovery`: Injects a panic during queue item execution, triggers the worker, waits for worker exit via `WaitDownloadWorkerForTest()`, verifies `downloadWorkerRunning == false`, and verifies that a subsequent download queue trigger runs without being wedged.

---

### Defect 2: Panic Boundary for TUI Download Policy Goroutine (`tui_modal_download.go`)
- **Problem**: `applyDownloadPolicyModal` spawns `go syncPolicyToBackend(...)` without a panic boundary. Any panic in backend initialization, SQLite operations, or type assertions crashes the process while the terminal is in raw/alt-screen mode, leaving the user terminal corrupted.
- **Mitigation**:
  - Wrap the background goroutine invocation in `applyDownloadPolicyModal` with a `defer func() { if r := recover(); r != nil { ... } }()`.
  - Additionally, protect `syncPolicyToBackend` internally with a deferred panic recovery logging handler to guard any direct caller.
- **Regression Test**:
  - `TestSyncPolicyToBackend_PanicRecovery`: Validates that a panic within policy synchronization is safely trapped, logged, and does not crash the caller or background thread.

---

### Defect 3: HTTP Response Body Leak in Cover Download (`pkg/backend/podfetch_api.go`)
- **Problem**: In `DownloadCover`, `defer resp.Body.Close()` is located inside `if resp.StatusCode == http.StatusOK`. Any non-200 HTTP response (404, 403, 500) leaves `resp.Body` unclosed, leaking sockets and file descriptors.
- **Mitigation**:
  - Restructure the check so that immediately upon `if err == nil`, `defer resp.Body.Close()` is called before inspecting `resp.StatusCode`.
- **Regression Test**:
  - `TestPodFetchDownloadCover_Non200ClosesBody`: Spins up an `httptest.Server` returning HTTP 404 and HTTP 500, verifies `DownloadCover` handles non-200 codes gracefully, and checks that connections/bodies are properly closed.

---

### Defect 4: Context Timeout & Keepalive on Remote SSH Transport (`remote_transport.go`)
- **Problem**: `DefaultSSHTransport.Exec`, `RsyncTo`, and `RsyncFrom` call `exec.Command` synchronously with only initial connection timeout (`ConnectTimeout=5`). If a remote host stalls mid-command or network drops packets, the CLI/daemon hangs indefinitely.
- **Mitigation**:
  - Build commands with `exec.CommandContext(ctx, ...)` utilizing `t.getTimeout()`.
  - Add `-o ServerAliveInterval=5` and `-o ServerAliveCountMax=3` to SSH arguments across `Exec`, `RsyncTo`, and `RsyncFrom`.
  - Return clear, actionable timeout error messages when `ctx.Err() == context.DeadlineExceeded`.
- **Regression Test**:
  - `TestDefaultSSHTransport_ContextTimeout`: Sets `Timeout: 50 * time.Millisecond` against a hanging/delayed command or dummy unreachable host, verifying that execution terminates promptly with a context deadline/timeout error.

---

### Defect 5: Retry & Backoff on Transient Feed Fetch Errors (`feed_cache.go` & `pkg/backend/podfetch_api.go`)
- **Problem**: Direct RSS feed fetching (`fetchFeedDirect` in `feed_cache.go` and `PodcastFeedEpisodes` in `pkg/backend/podfetch_api.go`) fails permanently on transient rate limits (HTTP 429) or upstream server errors (5xx/408) with no retry or backoff.
- **Mitigation**:
  - Implement a retry loop (up to 3 attempts) for HTTP 429, 408, and $\ge 500$ responses.
  - Ensure intermediate `resp.Body` is explicitly closed before sleeping and retrying to avoid connection leaks across attempts.
  - Support configurable retry delay/backoff so production runs jittered backoff while unit tests run without unnecessary latency.
- **Regression Tests**:
  - `TestFetchFeedDirect_TransientRetrySuccess`: Verifies `fetchFeedDirect` retries after initial 429 / 503 responses and successfully parses feed when upstream recovers on attempt 2 or 3.
  - `TestPodcastFeedEpisodes_DirectFetchRetry`: Verifies that direct-fetch fallback in `PodcastFeedEpisodes` retries on transient errors and properly parses valid RSS on retry.

---

### Defect 6: Silently Swallowed Errors in Keep Policy (`pkg/backend/podfetch_ops.go`)
- **Problem**: In `PodFetchBackend.ApplyKeepPolicy`, the error from `DeletePodcastEpisode` is checked only to conditionally increment `deletedCount`. Deletion failures are silently ignored, returning `(0, nil)`, masking retention policy enforcement failures.
- **Mitigation**:
  - Accumulate deletion errors in a slice `errs []string`.
  - When deletion errors occur, return the partial or zero `deletedCount` along with an aggregated error: `fmt.Errorf("keep policy: %d deletion(s) failed: %s", len(errs), strings.Join(errs, "; "))`.
- **Regression Test**:
  - `TestPodFetchApplyKeepPolicy_ErrorSurfacing`: Tests that when `DeletePodcastEpisode` fails (e.g. mock backend error or invalid endpoint), `ApplyKeepPolicy` returns an explicit error indicating deletion failure rather than returning `nil`.

---

### Defect 7: Discarded Errors in Download Queue Worker Loop (`download_queue.go`)
- **Problem**:
  1. In `ProcessNextDownloadQueueItem`, `_ = finalizeDownloadQueueItem(item.ID, dlErr)` silently discards finalize errors.
  2. In `runDownloadQueueWorkerLoop`, `processed, _ := ProcessNextDownloadQueueItem(client)` discards the processing error, concealing critical queue state write failures.
- **Mitigation**:
  - In `ProcessNextDownloadQueueItem`, capture and propagate `finalizeDownloadQueueItem` error, wrapping both download error and finalize error if both exist.
  - In `runDownloadQueueWorkerLoop`, log any non-nil error returned by `ProcessNextDownloadQueueItem`.
- **Regression Test**:
  - `TestProcessNextDownloadQueueItem_FinalizeError`: Verifies that when `finalizeDownloadQueueItem` fails (e.g. read-only queue path or atomic write failure), `ProcessNextDownloadQueueItem` returns the wrapped error rather than discarding it.

---

## 3. Implementation Order & Verification Steps

1. **Phase 1: Download Queue Hardening (Defects 1 & 7)**
   - Update `download_queue.go`: panic recovery, state reset, finalize error wrapping, loop error logging.
   - Add regression tests to `download_queue_test.go`.
   - Run `go test -run TestDownloadQueue` and verify function line sizes.

2. **Phase 2: TUI Modal Goroutine Panic Safety (Defect 2)**
   - Update `tui_modal_download.go`: panic recovery in `applyDownloadPolicyModal` and `syncPolicyToBackend`.
   - Add regression tests to `tui_download_policy_test.go`.
   - Run `go test -run TestTUI`.

3. **Phase 3: Backend Socket Leak Fix (Defect 3)**
   - Update `pkg/backend/podfetch_api.go`: deferred `resp.Body.Close()` in `DownloadCover`.
   - Add regression tests in `pkg/backend/podfetch_cover_test.go`.
   - Run `go test ./pkg/backend/...`.

4. **Phase 4: Remote Transport Timeout & Keepalives (Defect 4)**
   - Update `remote_transport.go`: `CommandContext`, `ServerAliveInterval`, `ServerAliveCountMax`.
   - Add `remote_transport_test.go` with timeout regression tests.
   - Run `go test -run TestDefaultSSHTransport`.

5. **Phase 5: Transient Feed Fetch Retries (Defect 5)**
   - Update `feed_cache.go`: retry loop on 429/5xx, closing body per iteration.
   - Update `pkg/backend/podfetch_api.go`: retry loop on direct-fetch fallback.
   - Add regression tests in `feed_cache_test.go` and `pkg/backend/podfetch_api_test.go`.
   - Run `go test ./...`.

6. **Phase 6: Keep Policy Error Surfacing (Defect 6)**
   - Update `pkg/backend/podfetch_ops.go`: accumulate and return deletion errors in `ApplyKeepPolicy`.
   - Add regression tests in `pkg/backend/podfetch_ops_test.go`.
   - Run `go test ./pkg/backend/...`.

7. **Phase 7: Full Quality Gate, Summary & Commit**
   - Run `./tools/check` (`make check`) verifying format, tidy, vet, staticcheck, all unit tests, line sizing audit, and build.
   - Write summary to `reports/004_summary.md`.
   - Commit changes via `tools/commit`.
