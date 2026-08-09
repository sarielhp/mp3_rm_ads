# AGENTS.md — Guidelines for AI-assisted development (Go)

## Build & Quality

- **Go 1.26+** — single `main` package with files organized by concern
- Build: `go build -o mp3_rm_ads .`
- Test: `go test -timeout 30s ./...` (all tests use `t.TempDir()` for isolation)
- Lint: `go vet ./...` then `staticcheck ./...`
- Format: `gofmt -s -w .` before committing

## Automation Scripts (`scripts/`)

| Script | Purpose |
|--------|---------|
| `scripts/check.sh` | Full quality gate: format → tidy → vet → staticcheck → test → build |
| `scripts/format.sh` | Run `gofmt -s -w .` only |
| `scripts/lint.sh` | Static analysis: `go vet` + `staticcheck` |
| `scripts/map.sh` | Print package structure, key types, and exported functions |
| `scripts/version.sh` | Print current version from `VERSION` file |
| `scripts/bump-version.sh` | Bump version, git add/commit/push (silent, outputs "Success VERSION (commit+push)") |
| `scripts/commit.sh <msg>` | Run quality gate, stage all, commit with message |
| `scripts/push.sh` | Push to origin, then bump version |
| `scripts/checkpoint.sh` | Auto micro-commit of all changes (saves work state) |
| `scripts/ci.sh` | Full CI pipeline (runs check.sh) |

## Makefile

A `Makefile` at the project root delegates to all scripts:

| Target | Action |
|--------|--------|
| `make check` | Full quality gate |
| `make lint` | Static analysis (vet + staticcheck) |
| `make test` | Run tests |
| `make build` | Build binary |
| `make format` | Format code |
| `make map` | Show architecture overview |
| `make version` | Show current version |
| `make bump` | Bump patch version |
| `make commit` | Quality gate + commit |
| `make push` | Git push |
| `make checkpoint` | Micro-commit all changes |
| `make ci` | CI pipeline |
| `make clean` | Remove binary |

### Workflow

```
# Standard development loop:
make commit msg="feat: add new feature"   # checks + commits
make push                                   # git push

# Manual version bump + push:
make bump                                   # bumps version, commits, pushes (silent)

# Quick checks without commit:
make check

# Explore architecture before making changes:
make map

# Save work state during long sessions:
make checkpoint

# Manual version bump (already covered above):
# make bump
```

## Version Management

- Version is stored in `VERSION` file (semver: `major.minor.patch`)
- Current version: 0.1.3
- After every successful push, the patch version is automatically bumped by 0.0.1
- Run `make bump` to bump, commit, and push version in one silent step
- The version is not embedded in the Go binary (VERSION file is the source of truth)

## File Sizing

- **Target**: 150–300 lines per file (~1.5k–3k tokens)
- **Trigger**: Split into logical units when exceeding 600 lines
- **Rationale**: Files under 300 lines keep AI agent context focused and reduce
  diff-drift during AST-based edits. Files over 600 lines cause measurable
  degradation in edit accuracy and token efficiency.

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
- Files should be at most 300 lines; split into logical units when exceeding 600 lines

## File Organization

| File | Purpose | Lines |
|------|---------|-------|
| `main.go` | Entry point, CLI parsing, usage | ~374 |
| `profiles.go` | LLM profile management, timing summaries | ~179 |
| `pipeline.go` | Main processing pipeline (transcribe → detect → cut) | ~297 |
| `output.go` | Output/export functions (SRT, TXT, JSON transcript) | ~169 |
| `opencode.go` | OpenCode config import | ~103 |
| `types.go` | Shared data structures | ~143 |
| `config.go` | Config loading/saving, profile cost, IP detection | ~274 |
| `transcribe.go` | Whisper HTTP API, chunked transcription, WAV builder | ~560 |
| `ads.go` | LLM ad detection, keyword extraction, JSON parsing | ~283 |
| `audio.go` | ffmpeg wrappers (cut, convert, truncate, probe) | ~118 |
| `format.go` | Time formatting, SRT/TXT conversion, interval merging | ~497 |
| `docker.go` | Docker container detection, log polling, progress | ~338 |

## Test Files

| File | Purpose | Tests |
|------|---------|-------|
| `main_test.go` | Core utility tests (format, merge, sort, etc.) | 86 |
| `config_test.go` | Config management tests | 11 |
| `format_test.go` | Format/export tests (SRT, TXT, JSON) | 14 |
| `save_cuts_test.go` | saveCutsJSON tests | 64 |
| `misc_test.go` | Edge case tests (Docker, LLM, error paths) | 41 |

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

## Agent Development Rules

1. **Verification**: After modifying any Go file, run `make check` to verify.
2. **Error Resolution**: If `make check` fails, focus on fixing the first reported
   error before making additional changes.
3. **Exploration**: Run `make map` before introducing new types to inspect
   existing structs and interfaces.
4. **Checkpointing**: Run `make checkpoint` after passing checks to preserve
   working states during long sessions.
5. **File Length**: Keep source files between 150-300 lines. Split large structs
   across sibling files in the same package when exceeding 600 lines.
6. **Commit Messages**: Use conventional commits format:
   `feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `test:`

## Test Suite

- All tests in `*_test.go` files (single package)
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