# Code Review 003

## 1. LLM Parsing Robustness (`ads.go`, `profiles.go`)
- **`ads.go`**:
  - The custom JSON array parser (`extractJSONArray`) fails silently if the JSON is malformed. If `json.Unmarshal` fails, it returns `nil` with no context.
  - `detectAdsLLM` and `extractKeywordsLLM` swallow LLM errors and print directly to standard output (`fmt.Printf`) instead of returning the errors to the caller or standard error.
- **`profiles.go`**:
  - `selectProfile` uses `fmt.Sscanf(useLLM, "%d", &id)` to parse integers without checking for errors, defaulting silently on failure.

## 2. Audio Processing & FFmpeg Resilience (`audio.go`)
- **Silent Failures**: Functions like `getAudioDuration`, `extractID3Tags`, `cutAudioFFmpeg`, and `truncateAudio` swallow `os/exec` command errors. They return zero values (`0.0`, `nil`, or `false`) making it impossible for the caller to log *why* ffmpeg failed (e.g. missing codec, corrupted file, out of memory).
- **Fallback Logic**: `cutAudioFFmpegWithHost` gracefully falls back to local execution if remote SCP/SSH fails, but completely hides the network/ssh error.

## 3. API Backend Clients (`backend_client.go`, `backend_cli.go`)
- **Ignored Errors**: Operations like `absScanPodcasts`, `syncAudiobookshelfDuration`, and `waitForActiveDownloads` silently ignore errors from the backend client initialization and the operation itself.
- **CLI Logging**: `backend_cli.go` uses `fmt.Println` indiscriminately for error reporting instead of properly routing to `os.Stderr` or returning `error`.

## 4. File Sizing
The following files exceed the 450-line limit and must be split into sibling files:
- `kitty.go` (519 lines)
- `config.go` (517 lines)
- `tui_keys.go` (522 lines)
- `tui_list_view.go` (526 lines)
- `remote_status.go` (511 lines)

## 5. AGENTS.md Compliance
- **No Comments in Code**: Adherence is excellent. No rogue comments were found in the reviewed source files.
- **Temp File Policy**: Mostly compliant, but `audio.go`'s remote SSH execution uses `/tmp` instead of the mandated `.work/` directory for its intermediate transfers.
- **File Size**: Violates the recommended 150-300 lines limit for the files listed above.

## Prioritized Fixes
1. Split `kitty.go`, `config.go`, `tui_keys.go`, `tui_list_view.go`, and `remote_status.go` into smaller sibling files to strictly adhere to the <450 lines file sizing rule.
2. Run `make check` to ensure the splits do not break any dependencies or quality gates.
