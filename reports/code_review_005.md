# Code Review 005

## 1. TUI Subsystem State Management & Terminal Resize Handling
The TUI system uses `tea.WindowSizeMsg` to capture terminal dimensions (`m.width`, `m.height`) and scale viewport boundaries (`m.vp.Width`, `m.vp.Height`). While functioning correctly, the `tui.go` file contains the root `tuiModel` which has bloated significantly (exceeding 500 lines). The state structure manages screens, index pointers, scroll pointers, and modal states all in one monolithic structure. It needs to be split to meet the 150-300 lines target per file.

## 2. CLI Commands Parsing & Validation
In `cli_parse.go` and `cli_server_cmds.go`, there are multiple commands acting as aliases. Specifically, the top-level commands `new`, `scan`, `clean-orphans`, and `opml` act as aliases for `server scan --episodes-only`, `server scan`, `server clean-orphans`, and `server opml` respectively. 
This explicitly violates AGENTS.md Rule 11 ("No CLI Aliases: Avoid defining command or subcommand aliases in CLI apps... Each command and subcommand must have a single canonical name").
Additionally, the function `normalizeCLIArgs` performs implicit alias resolution based on parameter types (e.g., rewriting a standalone number to `server download <number>` and defaulting to `proc` for `.mp3` files). These implicit aliases should be removed for strict compliance.

## 3. Splitting Remaining Files >400 Lines
The following files exceed the 400-line warning threshold (well above the 150-300 lines target in AGENTS.md) and require logical splitting:
- `tui.go` (500 lines)
- `cli_parse.go` (456 lines)
- `transcribe.go` (436 lines)
- `pm_download.go` (440 lines)
- `tui_keys_test.go` (538 lines)
- `tui_test.go` (540 lines)
- `player.go` (420 lines)
- `save_cuts_test.go` (423 lines)
- `save_cuts_extra_test.go` (423 lines)
- `remote_batch.go` (417 lines)
- `remote_status.go` (415 lines)

*(Note: `pm_server_exec.go` is currently under 400 lines (383 lines) but will be evaluated for modularity).*

## 4. Strict AGENTS.md Compliance
Code scanning verified that there are no standard comments (`/* ... */` or `//`) in the codebase, apart from compiler pragmas like `//go:embed`. This is in perfect compliance with the "no comments in code" rule (keep it self-documenting). 

## Fix Plan
1. **CLI Aliases Fix**: Remove the top-level aliases (`new`, `scan`, `clean-orphans`, `opml`) from `cli_parse.go`, ensuring they only exist under `server`. Streamline `normalizeCLIArgs` or completely remove implicit argument-based command mapping.
2. **File Splitting**: Use `ruby tools/suggest_split.rb` and manual refactoring to extract logical units from files over 400 lines into sibling files (e.g. `tui_model.go`, `cli_args.go`, `transcribe_util.go`, etc.).
3. **Verification**: Run `make check` to ensure the Go build, formatting, vetting, static analysis, and tests all pass after the fixes.
