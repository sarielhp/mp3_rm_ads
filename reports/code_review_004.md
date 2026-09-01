# Code Review Report 004

## 1. Error Propagation & Silent Failures
During the review of `audio.go`, `output.go`, and `ads.go`, several instances of error swallowing and silent failures were identified:

### `audio.go`
- **`getAudioDuration`**: Ignores `filepath.Abs` and `cmd.Output()` errors, returning `0.0` silently.
- **`extractID3Tags`**: Ignores errors from `filepath.Abs` and `cmd.Output()`, returning `nil` silently.
- **`cutAudioFFmpegWithHost`**: Swallows `scp` and `ffmpeg` execution errors, simply recursing to run without the host.
- **`convertToWAV` / `truncateAudio`**: Returns a boolean from `cmd.Run() == nil` but completely drops the underlying error context and `stderr` output (which is crucial when `ffmpeg` fails).

### `output.go`
- **`copyFile`**: Ignores `readFile(src)` errors and simply returns without copying or logging the failure.
- **`findMP3Files`**: Swallows `os.ReadDir(dir)` errors by returning an empty slice without logging.
- **`convertJSONToSRT` / `convertJSONToTXT`**: Swallows parsing errors internally if `data == nil`, though it does print to `os.Stderr`.

### `ads.go`
- **`detectAdsLLM`**: Prints a warning but swallows the actual error context for the caller by returning `nil`.
- **`extractJSONArray`**: Swallows `json.Unmarshal` errors and returns `nil`.
- **`extractKeywordsLLM`**: Similar to `detectAdsLLM`, it prints to `stdout` but returns an empty string without returning an error.

## 2. Unit Test Coverage on Error Paths
The test suite primarily covers happy-path scenarios. Missing test coverage for error paths includes:
- **LLM / Network Failures**: Simulating timeouts, 500 errors, or invalid JSON responses in LLM integrations.
- **File System Errors**: Permissions issues, missing source files for `copyFile`, symlink issues in `checkPrecutSymlink`.
- **Dependency Failures**: Situations where `ffmpeg` or `ffprobe` fail to execute or return invalid data.

## 3. Large File Splitting
Several files are approaching or exceeding the 450-line mark and should be split into smaller, more focused modules as per `AGENTS.md` (target 150-300 lines):
- **`format.go` (494 lines)**: Group formatting (time/clock), JSON/I/O helpers, and interval logic into separate files (e.g., `format_time.go`, `format_interval.go`, `format_io.go`).
- **`player.go` (493 lines)**: Separate the UI/rendering logic (e.g., `RenderProgressBar`, `RenderVolumeBar`) into `player_ui.go`.
- **`transcribe.go` (474 lines)**: Move the WAV header building and chunks logic into `transcribe_wav.go` and `transcribe_chunks.go`.
- **`tui_transcript_view.go` (523 lines)**: Split out the data loading and export logic from the view rendering (e.g., `tui_transcript_export.go`).
- **`remote_batch_test.go` (518 lines)**: Move the mock transport and its methods into `remote_batch_mock_test.go`.

## 4. AGENTS.md Compliance
- **No Comments in Code**: Found occasional comments in some files that violate the strict "no comments in code (keep it self-documenting)" rule from `AGENTS.md`. These need to be removed.
- **Temp File Policy**: Need to ensure all operations in `audio.go` strictly adhere to the `.work/` directory policy (currently, remote ffmpeg execution uses `/tmp/`, which violates the policy).

## 5. Action Plan
1. Fix error swallowing in `audio.go`, `output.go`, and `ads.go`.
2. Refactor `copyFile`, `findMP3Files`, and LLM functions to return `error` or handle failures explicitly without silent drops.
3. Remove any existing comments in these files to comply with `AGENTS.md`.
4. Perform file splits on `format.go`, `player.go`, `transcribe.go`, `tui_transcript_view.go`, and `remote_batch_test.go` to get them under 300 lines.
5. Fix remote host `/tmp/` usage in `audio.go` to use the `.work/` directory mechanism.
