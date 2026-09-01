# AGENTS.md — Guidelines for AI-assisted development (Go)

## Build & Quality

- **Go 1.26+** — single `main` package with files organized by concern
- Build: `ruby tools/build_local.rb` (or `make build`)
- Test: `go test -timeout 30s ./...` (all tests use `t.TempDir()` for isolation)
- Lint: `go vet ./...` then `staticcheck ./...`
- Format: `gofmt -s -w .` before committing

## Automation Tools (`tools/`)

| Script | Purpose |
|--------|---------|
| `tools/build_local.rb` | Build local `./abs` binary strictly within repo directory |
| `tools/check.rb` | Full quality gate: format → tidy → vet → staticcheck → test → build |
| `tools/format.sh` | Run `gofmt -s -w .` only |
| `tools/lint.rb` | Static analysis: `go vet` + `staticcheck` + `tools/audit_lines.rb` |
| `tools/audit_lines.rb` | Audit Go source file lengths against 150-300 target (600 limit) |
| `tools/outline_symbols.rb` | Index all Go types, structs, interfaces, and functions |
| `tools/show_symbol.rb <sym>` | Display single symbol code block with line numbers |
| `tools/suggest_split.rb` | Suggest logical file split boundaries for oversized files |
| `tools/generate_config_template.rb` | Generate `examples/config.json.template` |
| `tools/map.sh` | Print package structure, key types, and exported functions |
| `tools/version.sh` | Print current version from `VERSION` file |
| `tools/visual_audit.rb` | Live PTY visual audit: exercises and snapshots all 19 TUI screens/modes |
| `tools/verify_remote_queue.rb` | Live remote queue compliance audit against 24h & duration policy |
| `tools/bump-version.rb` | Bump version, git add/commit/push (silent, outputs "Success VERSION (commit+push)") |
| `tools/commit.rb <msg>` | Quality gate + stage + commit (silent, outputs "Success <msg>") |
| `tools/checkpoint.sh` | Auto micro-commit of all changes (saves work state) |

## Makefile

A `Makefile` at the project root delegates to all scripts:

| Target | Action |
|--------|--------|
| `make check` | Full quality gate |
| `make verify-queue` | Verify remote queue compliance (`tools/verify_remote_queue.rb`) |
| `make visual` | Run full live PTY visual audit across all 19 TUI screens (`tools/visual_audit.rb`) |
| `make lint` | Static analysis (vet + staticcheck + line audit) |
| `make audit` | Audit Go source file line lengths (`tools/audit_lines.rb`) |
| `make symbols` | Outline symbols (`tools/outline_symbols.rb ARGS="..."`) |
| `make suggest-split` | Suggest file splits (`tools/suggest_split.rb ARGS="..."`) |
| `make template` | Regenerate config template (`tools/generate_config_template.rb`) |
| `make test` | Run tests |
| `make build` | Build binary |
| `make format` | Format code |
| `make map` | Show architecture overview |
| `make version` | Show current version |
| `make bump` | Bump patch version |
| `make commit` | Quality gate + commit |
| `make push` | Alias for `make bump` |
| `make checkpoint` | Micro-commit all changes |
| `make ci` | Alias for `make check` |
| `make clean` | Remove binary |

### Workflow

```
# Standard development loop:
make commit ARGS="feat: add new feature"   # quality gate + commits (silent)
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
 - Config: `~/.config/abs/config.json`
 - Migration: Run `abs config migrate` to import settings from legacy `podcasts_manager` or `mp3_rm_ads` configs.
 - Environment Overrides: Supported env vars override config values:
   - `WHISPER_URL`
   - `ABS_URL` / `AUDIOBOOKSHELF_URL`
   - `ABS_USER` / `AUDIOBOOKSHELF_USER`
   - `ABS_PASS` / `AUDIOBOOKSHELF_PASS`
   - `PODCASTS_DIR`
   - `WHISPER_LANGUAGE`
   - `WHISPER_DOCKER_CONTAINER`

## Agent Development Rules

1. **Verification**: After modifying any Go file, run `make check` to verify.
2. **Error Resolution**: If `make check` fails, focus on fixing the first reported
   error before making additional changes.
3. **Exploration**: Run `make map` before introducing new types to inspect
   existing structs and interfaces.
4. **Checkpointing**: Run `make checkpoint` after passing checks to preserve
   working states during long sessions.
5. **Surgical Editing**: Never perform full-file rewrites (`replaceAll`) on existing
   files. Read the target section, locate the specific function/struct, and apply
   localized targeted diffs.
6. **Plan Before Build**: For multi-step refactoring or subtle bug fixes, always
   use `plan` mode / consult the `planner` subagent to formulate the exact steps
   before modifying code in `build` mode.
7. **File Length**: Keep source files between 150-300 lines. Split large structs
   across sibling files in the same package when exceeding 600 lines.
8. **Commit Messages**: Use conventional commits format:
   `feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `test:`
9. **Subdirectory Isolation**: Under no circumstances modify files outside this
   subdirectory. Find operations must be restricted to this subdirectory.
10. **Dependency Updates**: Use standard Go tooling to check for updates (`go list -m -u all`
    or `go list -m -u <pkg>`) and upgrade with `go get <pkg>@latest`.
11. **No CLI Aliases**: Avoid defining command or subcommand aliases in CLI apps (`clihelp`). Each command and subcommand must have a single canonical name to maintain clarity, prevent command-space collisions, and keep documentation consistent.
12. **Local Build Isolation**: All local Go builds must be executed using `tools/build_local.rb` (or `make build`). The build script must generate a local binary (`./abs`) within this repository directory and must NEVER write into or modify directories outside this repository.

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