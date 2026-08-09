# AGENTS.md — Guidelines for AI-assisted development (Go)

## Build & Quality

- **Go 1.26+** — single `main` package with files organized by concern
- Build: `go build -o mp3_rm_ads .`
- Test: `go test ./...` (all tests use `t.TempDir()` for isolation)
- Lint: `go vet ./...` then `staticcheck ./...`
- Format: `gofmt -s -w .` before committing

## Temp File Policy

All temporary/intermediate files **must** be written to a `.work/` subdirectory
alongside the source audio file. The `verifyTempFile()` function enforces this
at runtime — any temp file path outside `.work/` causes an immediate abort.

### Rules

1. **WAV cache file** → `.work/<input>.wav` (single 16kHz mono WAV converted from original)
2. **Truncated previews** → `.work/*.truncated.wav`
3. **FFmpeg cut output** → `.work/*.tmp.mp3`
4. **Final output files** (transcript, SRT, TXT, cuts, ad-free MP3) → original
   directory (these are moved from `.work/` after processing)
5. **Cleanup** — `.work/` is removed after successful processing and on failure

### Adding new temp files

Always use `workDirFor(path)` to compute the `.work/` path, then call
`verifyTempFile(path)` before writing via ffmpeg or any other tool.

## Code Style

- Go 1.26+, no external dependencies beyond stdlib
- No comments in code (keep it self-documenting)
- `os/exec` for external commands (ffmpeg, ffprobe, docker)
- Custom `syncMu` / `syncMutex` / `syncWG` for thread safety (no sync package)
- All errors are returned; `os.Exit(1)` only in `main()` and fatal helpers

## External Dependencies

- `ffmpeg` — audio splitting, cutting, truncation
- `docker` — whisper container log polling (optional, local only)
- `whisper.cpp` server — HTTP API for transcription
- `libmp3lame` — MP3 encoding (via ffmpeg)

## Key Architecture

- Transcription: HTTP POST to whisper server with `verbose_json` response format
- Progress: Docker log polling via `docker logs --tail N`
- Ad detection: LLM API (Ollama, OpenRouter, etc.)
- Audio cutting: ffmpeg filter_complex with concat
- Config: `~/.config/mp3_rm_ads/config.json`

## Test Suite

- All tests in `main_test.go` (single package)
- Use `t.TempDir()` for temp files — never write to real filesystem paths
- Key test categories:
  - Time formatting (`formatTime`, `formatClock`, `formatSRTTime`)
  - Interval merging (`mergeIntervals`, `mergeBounds`, `calculateKeepSegments`)
  - JSON/transcript conversion (`saveCutsJSON`, `convertJSONToSRT`, `convertJSONToTXT`)
  - Config management (`saveConfig`, `loadConfig`, `ensureConfigExists`)
  - Docker helpers (`fetchDockerLogs`, `pollWhisperDockerProgress`)
  - LLM interaction (`detectAdsLLM`, `extractKeywordsLLM`, `extractJSONArray`)
  - Audio processing (`buildWavHeader`, `cutAudioFFmpeg`)
  - Utility functions (`stripExt`, `filepathBase`, `splitLines`, `toLower`)
- Run `go test -v -timeout 30s ./...` to verify all tests