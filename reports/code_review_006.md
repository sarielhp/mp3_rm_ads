# Code Review 006

## 1. Remote Processing Subsystem
- **Remote Queue Ordering & 24h Policy Compliance**:
  - `remote_scan.go` (and the `push` mechanism) correctly uses `sortAudioFilesByDuration`, which internally relies on `sortAudioFilesByQueuePolicy` enforcing the 24h grouping policy (recent 24h first).
  - **Issue**: `remote_worker.go` re-sorts the manifest `Items` in `runBatchWorker` purely by priority and duration (ignoring the 24h rule). This overrides the correct queue ordering. The redundant sort in `remote_worker.go` should be removed so it simply processes the manifest in the order provided by the scanner.
  - **Issue**: `remote_queue_order_test.go` has a test `TestBatchWorkerSortsByPriorityAndDuration` that explicitly asserts the worker will sort items by duration. This test is enforcing the wrong behavior and needs to be updated.
- **SSH/SCP Resilience**:
  - `remote_batch.go` uploads files using `transport.Upload`. If an upload fails midway, it aborts the entire push instead of skipping the file or retrying. (No retry logic for SCP).
- **Batch Concurrency Safety**:
  - Concurrency is managed via file locks, but missing files or network interruptions can still result in broken states.

## 2. Episode Caching & Frequency Calculations
- **Timezone/Date Parsing Edge Cases**:
  - `feed_cache.go:parseFeedDate` uses `time.Parse` with `time.RFC1123` (`MST`). In Go, `time.Parse` defaults unrecognized timezones (like `PDT`, `EDT`, etc.) to UTC offset `+0000`. This silently misinterprets publication times (e.g., `12:00:00 PDT` becomes `12:00:00 UTC`), severely throwing off frequency calculations and missing the 24h window.
  - **Fix**: Pre-process the date string to replace common timezone abbreviations (PDT, PST, EDT, EST, etc.) with explicit numerical offsets (e.g. `-0700`, `-0800`) before running it through `time.Parse`.

## 3. Oversized Files to Split
The following files exceed the ~350 line limit or are approaching the hard limit:
- `save_cuts_test.go` (423 lines)
- `save_cuts_extra_test.go` (423 lines)
- `pm_frequency_test.go` (410 lines)
- `pm_server_exec.go` (383 lines)
- `profiles.go` (340 lines)
- `cli_remote_cmds.go` (335 lines)
- `cli_server_cmds.go` (377 lines)

## 4. AGENTS.md Compliance
- **No Comments In Code**: Verified. A deep scan using regex `//\s` and `/\*.*\*/` across the codebase confirmed that all Go files correctly omit comments in code, perfectly strictly complying with the self-documenting rule.

## Planned Fixes (High Priority)
1. **Fix `runBatchWorker` Sorting**: Remove the hardcoded `sort.SliceStable` block in `remote_worker.go` so it respects the 24h policy order populated in the manifest. Update `TestBatchWorkerSortsByPriorityAndDuration` to reflect this.
2. **Fix Timezone Parsing**: Update `parseFeedDate` in `feed_cache.go` to convert standard US timezone abbreviations into numerical offsets to guarantee correct UNIX timestamp parsing. Add test coverage for this edge case in `feed_cache_test.go`.
3. **Split Oversized Files**: Break down the >350 line files identified into smaller logical units.
