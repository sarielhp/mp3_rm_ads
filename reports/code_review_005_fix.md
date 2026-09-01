# Code Review 005 Fixes

## 1. CLI Aliases Addressed (AGENTS.md Rule 11)
- **Top-level aliases removed**: Cleaned up `cli_parse.go` to remove duplicate top-level aliases (`new`, `scan`, `clean-orphans`, `opml`). These commands are now canonical subcommands of `abs server` in `cli_server_cmds.go`, establishing a single truth-source.
- **Implicit mapping disabled**: Modified `normalizeCLIArgs` in `cli_parse.go` to stop blindly auto-forwarding files ending in `.mp3` to the `proc` command and bare numbers to the `server download` command. These now require explicitly passing `abs proc` or `abs server download`.
- **Test suite updated**: Updated all unit tests in `cli_all_test.go`, `cli_policy_test.go`, and `misc_extra_test.go` that were relying on implicit aliases (e.g. replaced `abs scan` with `abs server scan`).

## 2. File Spiting (>400 lines)
Refactored several large files directly violating the 150-300 lines limit target:
- **`tui.go` (was 500 lines)**: Split into `tui.go` (291 lines), `tui_types.go`, and `tui_views.go`. Kept the main controller state machine in `tui.go` and isolated UI structural rendering into `tui_views.go`.
- **`transcribe.go` (was 436 lines)**: Split out `transcribeChunks` into `transcribe_chunks.go`, reducing the core file size to 210 lines.
- **`pm_download.go` (was 440 lines)**: Reorganized by extracting the monstrous `downloadPodcastEpisodes` implementation into `pm_download_episodes.go`.
- **`player.go` (was 420 lines)**: Isolated all sink management, volume changes, and physical device mappings into `player_sink.go`, shrinking the core logic back to 291 lines.
- **`tui_keys_test.go` (was 538 lines)**: Fragmented auxiliary keys navigation tests into `tui_keys_extra_test.go`.
- **`tui_test.go` (was 540 lines)**: Exerted the internal TUI unit tests into `tui_nav_test.go`, maintaining a compact core test profile.
*(Note: `cli_parse.go` was reduced to 387 lines organically by eliminating unused aliases, falling under the >400 threshold).*

## 3. Strict Code Comments Check (AGENTS.md)
Ensured strict adherence to the "no comments in code" rule. Verified through rigorous AST/regex checks there were absolutely no multi-line `/*` or inline `//` developer comments present anywhere in the codebase outside of essential compilation pragmas (such as `//go:embed VERSION`). No modifications were needed to enforce this.

## 4. Verification
Ran `make check` which fully executed:
- Standard Go build verification
- Gofmt linting
- Go Vet error-checking
- All unit tests under isolation
- Line Sizing Gate
Results: `Success: Quality Gate Passed`. All components compile, run, and pass strict code-quality rules reliably.
