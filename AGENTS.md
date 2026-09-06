# AGENTS.md — Guidelines for AI-assisted development (Go)

## Build & Quality

- **Go 1.26+** — single `main` package with files organized by concern
- Build: `./tools/build_local` (or `make build`)
- Test: `go test -timeout 30s ./...` (all tests use `t.TempDir()` for isolation)
- Lint: `go vet ./...` then `staticcheck ./...`
- Format: `gofmt -s -w .` before committing

## Automation Tools (`tools/`)

| Script | Purpose |
|--------|---------|
| `tools/build_local` | Build local `./abs` binary strictly within repo directory |
| `tools/check` | Full quality gate: format → tidy → vet → staticcheck → test → build |
| `tools/format.sh` | Run `gofmt -s -w .` only |
| `tools/lint` | Static analysis: `go vet` + `staticcheck` (respecting baseline) + `tools/audit_lines` |
| `tools/audit_lines` | Audit function lengths (80-line limit) and file lengths (800 warn / 1100 limit) |
| `tools/outline_symbols` | Index all Go types, structs, interfaces, and functions |
| `tools/show_symbol <sym>` | Display single symbol code block with line numbers |
| `tools/suggest_split` | Suggest logical file split boundaries for oversized files |
| `tools/generate_config_template` | Generate `examples/config.json.template` |
| `tools/map.sh` | Print package structure, key types, and exported functions |
| `tools/version.sh` | Print current version from `VERSION` file |
| `tools/visual_audit` | Live PTY visual audit: exercises and snapshots all 19 TUI screens/modes |
| `tools/verify_remote_queue` | Live remote queue compliance audit against 24h & duration policy |
| `tools/bump` | Bump version, git add/commit/push (skips duplicate gate if .verified_head matches; silent, outputs "Success VERSION (commit+push)") |
| `tools/commit <msg>` | Quality gate + stage + commit + records .verified_head (silent, outputs "Success <msg>") |
| `tools/snapshot [msg]` | Fast WIP commit without gating (<0.1s, never pushes) |
| `tools/checkpoint.sh` | Auto micro-commit of all changes (delegates to `tools/snapshot`) |

## Makefile

A `Makefile` at the project root delegates to all scripts:

| Target | Action |
|--------|--------|
| `make check` | Full quality gate |
| `make verify-queue` | Verify remote queue compliance (`tools/verify_remote_queue`) |
| `make visual` | Run full live PTY visual audit across all 19 TUI screens (`tools/visual_audit`) |
| `make lint` | Static analysis (vet + staticcheck + line audit) |
| `make audit` | Audit Go source file line lengths (`tools/audit_lines`) |
| `make symbols` | Outline symbols (`tools/outline_symbols ARGS="..."`) |
| `make suggest-split` | Suggest file splits (`tools/suggest_split ARGS="..."`) |
| `make template` | Regenerate config template (`tools/generate_config_template`) |
| `make test` | Run tests |
| `make build` | Build binary |
| `make format` | Format code |
| `make map` | Show architecture overview |
| `make version` | Show current version |
| `make bump` | Bump patch version |
| `make commit` | Quality gate + commit |
| `make push` | Alias for `make bump` |
| `make snapshot` | Fast WIP micro-commit (`tools/snapshot ARGS="..."`) |
| `make snap` | Alias for `make snapshot` |
| `make checkpoint` | Micro-commit all changes (delegates to `make snapshot`) |
| `make ci` | Alias for `make check` |
| `make clean` | Remove binary |

### Workflow

#### Standard Development & Release Loop
```
make commit ARGS="feat: add new feature"   # quality gate + commits + records .verified_head (silent)
make bump                                   # deduplicates gate via .verified_head, bumps version, commits, pushes (silent)
```

#### Deduplicated Commit-to-Bump Pipeline
`tools/commit` executes `./tools/check`. Upon passing and committing, it records the committed HEAD SHA into `.verified_head`. When `tools/bump` (`make bump`) runs immediately afterward, it checks whether HEAD matches `.verified_head` and only `VERSION` (or nothing) is modified in the working tree. If verified, it skips the redundant full CI quality gate (`./tools/check --full`), saving ~60s of duplicate test runs, increments the patch version, commits, pushes, and removes `.verified_head`. If `.verified_head` is absent, mismatched, or other files are modified, it executes the full quality gate as usual before pushing.

#### Fast Inner-Loop Testing Advice
During active development, use lightweight commands to maximize iteration speed:
- **Instant Snapshots (<0.1s)**: Use `make snap` or `make snapshot [ARGS="..."]` (or `make checkpoint`) freely to checkpoint dirty working trees into fast WIP commits (`--no-verify`, never pushes).
- **Fast Static Analysis**: Run `make lint` for fast feedback (`go vet` + baseline-aware `staticcheck` + line audit). `tools/lint` respects `tools/staticcheck-baseline.txt` so it succeeds when there are 0 new findings rather than failing on pre-existing baselined findings, running in seconds.
- **Targeted Unit Tests**: Run `go test ./...` or `make test` rather than full race/vulnerability scans on every minor change.
- **Full Gate Verification**: Run `make check` before preparing commits to verify formatting, vet, linting, tests, and build.
- **Publishing**: Run `make commit ARGS="..."` followed by `make bump` for a fast, zero-redundancy gated release.

## Version Management

- Version is stored in `VERSION` file (semver: `major.minor.patch`)
- Current version: 0.1.3
- After every successful push, the patch version is automatically bumped by 0.0.1
- Run `make bump` to bump, commit, and push version in one silent step
- The version is not embedded in the Go binary (VERSION file is the source of truth)

## Sizing

Two limits, and they are not equally important. `tools/audit_lines` checks both.

### Functions — hard limit 80 lines

No function may exceed **80 lines**. This is the limit that protects correctness, so
when it conflicts with anything else, it wins.

Extracting a function is a *semantic* edit: the extracted piece needs a name,
parameters and return values, and the compiler checks every call site. A control-flow
statement that goes missing during an extraction becomes a build error — a lost
`return` fails to compile, and a lost `continue` has no loop left to sit in.

### Files — warn over 800 lines, hard limit 1100

Files should stay comfortably readable, but file length is a *comfort* metric, not a
correctness one. Keep functions under 80 lines and files land in the 300–700 range on
their own; the 800-line warning exists to catch the cases where they do not.

### Validation Tool — `tools/audit_lines`

Line lengths are verified using `tools/audit_lines` (run via `make audit` or as part of `make lint` / `make check`):
- **Function verification**: Measures line counts of Go function declarations (matching `func` in column 0 to its closing `}`). Flags any function exceeding **80 lines**.
- **File verification**: Scans all Go files (excluding `.work/` and vendor directories). Emits a warning if a file exceeds **800 lines** and reports a hard failure if it exceeds **1100 lines**.
- **Key Flags**:
  - `-f, --max-func LINES`: Configure function line limit (default: `80`).
  - `-s, --soft LINES`: Configure file soft warning threshold (default: `800`).
  - `-m, --max LINES`: Configure file hard limit (default: `1100`).
  - `--strict`: Return a non-zero exit code if any hard limit is exceeded.
  - `-q, --quiet`: Report only warnings and violations.
  - `--include-tests`: Apply function limits to `*_test.go` files.


### Never split a file through a function body

Splitting a file is a *physical* edit — a byte range is moved to a new file, and
nothing checks that the control flow survived. Two such splits in this repo's history
did real damage and both compiled cleanly and passed the gate:

- `c9b68f4` (code-review-002) dropped every `continue` from the batch loop, so a
  single failing episode silently reprocessed the whole batch.
- `1c49c7b` (code-review-010) replaced `return len(episodesToDownload)` with
  `return 0`, so downloads reported that nothing was fetched.

When a file grows past the warning threshold, **decompose its long functions in place**
and let the file stay long until the decomposition makes a genuine module boundary
obvious. A file that is only long because one function inside it is long is a function
problem, not a file problem. `tools/suggest_split` proposes file boundaries — treat
its output as a hypothesis, and never accept a boundary that falls inside a function.

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
- Functions must be at most 80 lines; files warn over 800 and are capped at 1100 (see **Sizing**)

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
| `tui.go` | Interactive TUI browser (podcast list → episodes → detail) | ~598 |

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

## Key Architecture & Configuration
 
 - Transcription: HTTP POST to whisper server with `verbose_json` response format
 - Progress: Docker log polling via `docker logs --tail N`
 - Ad detection: LLM API (Ollama, OpenRouter, etc.)
 - Audio cutting: ffmpeg filter_complex with concat
 - Backends (`pkg/backend/`):
   - **Audiobookshelf (ABS)**: Primary podcast server backend via REST API (`audiobookshelf_url`, `audiobookshelf_user`, `audiobookshelf_pass`, `audiobookshelf_token`, `audiobookshelf_sqlite_db_path`).
   - **PodFetch**: Full-featured alternative podcast backend (`pkg/backend/podfetch*.go`) via REST API and SQLite database (`podfetch_url`, `podfetch_user`, `podfetch_pass`, `podfetch_api_key`, `podfetch_db_path`). Selected via `backend_type: "podfetch"`.
 - Playback Architecture: Headless background playback daemon spawned via `abs player <play|stop|pause|status> [id]`, controlled through IPC socket `/tmp/abs_player.sock` with MPRIS D-Bus integration (supports mpv and cvlc fallback).
 - Terminology: Standardized on "AdR" (Ad Removal) and "NeedAdR" across CLI, tables, status badges, and TUI.
 - Config: `~/.config/abs/config.json`
 - Migration: Run `abs config migrate` to import settings from legacy `podcasts_manager` or `mp3_rm_ads` configs.
 - Environment Overrides: Supported env vars override config values:
   - `WHISPER_URL`
   - `ABS_URL` / `AUDIOBOOKSHELF_URL`
   - `ABS_USER` / `AUDIOBOOKSHELF_USER`
   - `ABS_PASS` / `AUDIOBOOKSHELF_PASS`
   - `PODFETCH_URL`
   - `PODFETCH_USER`
   - `PODFETCH_PASS`
   - `PODFETCH_DB_PATH`
   - `PODCASTS_DIR`
   - `WHISPER_LANGUAGE`
   - `WHISPER_DOCKER_CONTAINER`

## Agent Development Rules

1. **Verification**: After modifying any Go file, run `make check` to verify.
2. **Error Resolution**: If `make check` fails, focus on fixing the first reported
   error before making additional changes.
3. **Exploration**: Run `make map` before introducing new types to inspect
   existing structs and interfaces.
4. **Checkpointing & Snapshots**: Run `make snapshot` (or `make snap` / `make checkpoint`)
   to preserve working states in sub-0.1s without gating or pushing.
5. **Surgical Editing**: Never perform full-file rewrites (`replaceAll`) on existing
   files. Read the target section, locate the specific function/struct, and apply
   localized targeted diffs.
6. **Plan Before Build**: For multi-step refactoring or subtle bug fixes, always
   use `plan` mode / consult the `planner` subagent to formulate the exact steps
   before modifying code in `build` mode.
7. **Function Length**: No function over 80 lines — decompose it in place, into
   named helpers in the same file. Files warn at 800 lines and are capped at 1100.
   Never split a file through a function body: that edit is unchecked by the
   compiler and has silently dropped control flow here twice (see **Sizing**).
8. **Commit Messages**: Use conventional commits format:
   `feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `test:`
9. **Subdirectory Isolation**: Under no circumstances modify files outside this
   subdirectory. Find operations must be restricted to this subdirectory.
10. **Dependency Updates**: Use standard Go tooling to check for updates (`go list -m -u all`
    or `go list -m -u <pkg>`) and upgrade with `go get <pkg>@latest`.
11. **No CLI Aliases**: Avoid defining command or subcommand aliases in CLI apps (`clihelp`). Each command and subcommand must have a single canonical name to maintain clarity, prevent command-space collisions, and keep documentation consistent.
12. **Local Build Isolation**: All local Go builds must be executed using `./tools/build_local` (or `make build`). The build script must generate a local binary (`./abs`) within this repository directory and must NEVER write into or modify directories outside this repository.

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