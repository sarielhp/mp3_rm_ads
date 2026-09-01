# Code Review Report 002

## Architecture
The ABS (Audiobookshelf) auto-cutting agent follows a clean pipeline architecture (fetch -> transcribe -> ad detect -> cut) with an event-based TUI (Tea-based). Dependencies on external tools like `ffmpeg` and `docker` are cleanly isolated into dedicated modules. A backend abstraction layer enables fetching episodes and tracking state from Audiobookshelf or Podfetch. 
While the architecture is robust, the pipeline initialization and batch loop inside `batch_proc.go` have become monolithic, violating the principle of single responsibility and risking context drift.

## File Size Limits
Several files are approaching or exceeding the strict 600-line hard limit defined in `AGENTS.md` (and ideally should be 150-300 lines). The most pressing are:
- `tui_screens_test.go` (~586 lines) - Should be split into `tui_screens_extra_test.go` or grouped logically by screen/feature (e.g. `tui_marquee_test.go`, `tui_viewer_test.go`).
- `batch_proc.go` (~542 lines) - Contains a massive `processAudioFilesBatch` function that handles everything from file discovery and CLI flag normalization to executing the core pipeline loop. Needs to be broken down into modular steps (file discovery, CLI setup, execution loop).
- `misc_test.go` (~532 lines) - Catch-all test file that needs to be decomposed into specific domain tests (e.g. `misc_docker_test.go`, `misc_abs_test.go`, `misc_extra_test.go`).

## Error Propagation
Errors are occasionally swallowed or written directly to `os.Stderr` rather than propagated up to the caller or handled gracefully.
- In `output.go`, json reading errors return an empty string and print to stderr instead of bubbling up errors.
- Extensive use of `fmt.Printf` / `fmt.Fprintf` in pipeline execution code (e.g. `pm_download.go`) conflates library logic with CLI presentation. This makes it difficult for the TUI to intercept errors.
- `fatalError()` is used deep within batch loop logic (e.g., in `batch_proc.go` lines 15, 202) instead of bubbling up an error for the caller to decide the exit strategy.

## Concurrency
The application makes reasonable use of goroutines for background polling (Docker logs, parallel LLM profiling, etc.).
- There is a violation of the `AGENTS.md` directive regarding synchronization primitives: the codebase directly uses `sync.Mutex`, `sync.RWMutex`, and `sync.WaitGroup` rather than the required custom `syncMu` / `syncMutex` / `syncWG`. The policy indicates a preference to avoid importing the `sync` package directly. 
- In `player.go`, `pm_server_exec.go`, and `download_queue.go`, direct goroutine spawning without timeout/context management risks goroutine leaks if external tools (`ffmpeg`, `ffprobe`) hang.

## Bugs / Edge Cases
- **Missing Sync Wrapper Abstraction:** The codebase doesn't actually define `syncMutex` or `syncWG`, but the documentation policy strictly mandates it.
- **Empty JSON unmarshal in Output Module:** `output.go` suppresses failures during `TranscriptionData` extraction which could result in a silent failure if the Whisper output JSON is truncated or invalid.

## AGENTS.md Compliance
- **No comments in code:** Adherence is currently 100%. A strict regex scan confirmed no inline comments (`// `) violating the documentation rule.
- **File Sizing:** As noted above, adherence is slipping for the largest files, which must be addressed immediately to ensure token efficiency and avoid diff-drift.
- **Single `main` package:** Compliant. All code resides in the main package with subdirectories limited to external tools.

## Next Steps (Fixes)
1. Split `batch_proc.go` into `batch_proc.go` and `batch_proc_loop.go` or similar logically decomposed files.
2. Split `tui_screens_test.go` and `misc_test.go`.
3. Introduce custom sync aliases (`syncMutex`, `syncWG`) in `types.go` and replace direct `sync.*` calls to adhere to `AGENTS.md`.
4. Refine `output.go` error handling.
