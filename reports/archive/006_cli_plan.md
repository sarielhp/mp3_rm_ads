# Remediation Plan: Systems Code Review #006 (Lens: CLI)

## Overview

This architectural plan outlines the remediation strategy for all 9 issues identified in `reports/006_cli.md`. Every change is scoped strictly to the reported defect, includes dedicated regression unit tests validating the mitigation, strictly follows Go sizing guidelines (<= 80 lines per function, decomposed in place, no file > 800 lines), preserves zero comments in code per `AGENTS.md`, and is validated via `./tools/check`.

---

## Defects and Detailed Action Plan

### 1. Issue 1 (Critical): Argument Slicing in `abs test kitty <image-file>`
- **Severity**: Critical
- **Affected Files**: `cli_parse.go`, `kitty_encode.go`, `main.go`
- **Root Cause**: `buildTestCommand.Run` binds `opts.Args = ctx.Args` directly. When invoking `abs test kitty sample.png`, `ctx.Args` is `["kitty", "sample.png"]`. `testKittyImage` in `kitty_encode.go` expects `args[0]` to be the file path and calls `os.Stat(args[0])`. Because `args[0]` is the subcommand verb `"kitty"`, `os.Stat("kitty")` fails with `ENOENT`.
- **Remediation**:
  1. In `cli_parse.go` (`buildTestCommand`): Slice off the leading `"kitty"` token when assigning `opts.Args` if `len(ctx.Args) > 0 && ctx.Args[0] == "kitty"`.
  2. In `kitty_encode.go` (`testKittyImage`): Defensively guard against any residual `"kitty"` subcommand token: if `len(args) > 0 && args[0] == "kitty" { args = args[1:] }`.
- **Regression Testing**:
  - Add unit test in `cli_test_kitty_test.go` verifying that invoking `abs test kitty <image>` strips the action verb and passes only `<image>` in `opts.Args`, while setting `opts.TestKitty = true`.

---

### 2. Issue 2 (Major): Validation of Positional Service Arguments in `resolveTestCommandArgs`
- **Severity**: Major
- **Affected Files**: `cli_parse.go`
- **Root Cause**: Unrecognized targets in `resolveTestCommandArgs` fall through the switch statement and trigger the unconstrained fallback `opts.TestWhisper = true`. An invalid target like `abs test unknown` or `abs test abs-server` silently initiates network and container probes against Whisper.
- **Remediation**:
  1. Change signature of `resolveTestCommandArgs(args []string, opts *CLIOptions)` to return `error`.
  2. If `args` is empty, set `opts.TestWhisper = true; return nil`.
  3. Validate valid targets: `"whisper"`, `"whisper-server"`, `"abs"`, and `"kitty"`.
  4. For `"abs"`, validate sub-targets: `"connect"`, `"map"`, `"download"`. Return error for unknown abs sub-targets (`"unknown abs test target %q (expected: connect, map, download)"`).
  5. For any unknown primary target, return an actionable error (`"unknown test target %q (valid targets: whisper, abs, kitty)"`).
  6. In `buildTestCommand.Run`, propagate any error returned by `resolveTestCommandArgs`.
- **Regression Testing**:
  - Add tests in `cli_test_validation_test.go` executing `abs test bogus`, `abs test abs invalid`, `abs test abs map`, `abs test abs download`, `abs test abs connect`, `abs test whisper`, `abs test kitty`, verifying that invalid targets return errors with expected messages and valid targets succeed.

---

### 3. Issue 3 (Major): Error Stream and Exit Code in `handleConfigGet`
- **Severity**: Major
- **Affected Files**: `config_cli.go`, `main.go`, `config_test_extra.go`
- **Root Cause**: When an unrecognized configuration key is queried via `abs config get <key>`, `handleConfigGet` prints `fmt.Printf("Unknown configuration key: '%s'\n", key)` to `os.Stdout` and returns void. `main.go` does not check for errors and exits with code 0.
- **Remediation**:
  1. Refactor `handleConfigGet(cfg Config, key string) error` in `config_cli.go`.
  2. Return `fmt.Errorf("unknown configuration key %q; run 'abs config show' to list keys", key)` on unrecognized keys instead of writing to `os.Stdout`.
  3. In `main.go` (`handleMainConfig`), check the returned error from `handleConfigGet`; print `"Error: %v\n"` to `os.Stderr` and call `os.Exit(1)`.
  4. Update `config_test_extra.go` to handle the error return value.
- **Regression Testing**:
  - Add test in `config_cli_test.go` verifying that `handleConfigGet` returns non-nil error with unknown keys and nil error with valid keys.

---

### 4. Issue 4 (Major): Exit Status and Error Message in `buildHelpCommand`
- **Severity**: Major
- **Affected Files**: `cli_parse.go`
- **Root Cause**: `ctx.App.RenderCommand` returns a boolean indicating whether the requested command path exists. When an unknown topic is passed, it writes nothing and returns `false`. `buildHelpCommand` discards this return value and unconditionally invokes `os.Exit(0)`.
- **Remediation**:
  1. Check the return value of `ctx.App.RenderCommand(...)`.
  2. If `false`, emit `fmt.Fprintf(os.Stderr, "Error: unknown command %q. Run 'abs help' for available commands.\n", ctx.Args[0])` and invoke `os.Exit(1)`.
- **Regression Testing**:
  - Add test in `cli_help_test.go` testing command lookup / help verification on valid and invalid command paths.

---

### 5. Issue 5 (Major): Conflicting Export Flags in `transcript` Command
- **Severity**: Major
- **Affected Files**: `cli_commands_parity.go`, `extra_cli_cmds.go`
- **Root Cause**: Three overlapping mechanisms specify the export format (`--export <format>`, `--txt`, `--srt`). When contradictory flags are passed (`abs transcript e12345 --export srt --txt`), `--txt` silently shadows `--export srt` without notice.
- **Remediation**:
  1. In `cli_commands_parity.go` (`buildTranscriptCommand`), mark legacy `--txt` and `--srt` options as hidden (`Hidden: true`) so only canonical `--export <format>` is advertised.
  2. In `extra_cli_cmds.go` (`runTranscriptCommand`), count how many format flags were provided (`cli.ExportFormat != ""`, `cli.ExportTXT`, `cli.ExportSRT`).
  3. If `fmtCount > 1`, return `fmt.Errorf("conflicting export format flags; use canonical '--export <format>'")`.
  4. Validate `cli.ExportFormat` when supplied (must be `"txt"` or `"srt"`).
- **Regression Testing**:
  - Add unit tests in `cli_transcript_export_test.go` testing conflicting flags (`--export srt --txt`, `--export txt --srt`, `--txt --srt`) return the expected conflict error, while single format specifications work properly.

---

### 6. Issue 6 (Moderate): Proliferation of Duplicate Subcommands and Backdoor Aliases
- **Severity**: Moderate
- **Affected Files**: `cli_config_cmds.go`, `queue_cmd.go`, `cli_remote_exec.go`, `cli_proc_cmds.go`, `global_policy_test.go`, `cli_config_cache_test.go`
- **Root Cause**:
  1. `abs config show` and `abs config list` are duplicate registrations with identical handlers.
  2. `queue_cmd.go` accepts unadvertised backdoor aliases: `ls`, `rm`, `del`, `delete`.
  3. `cli_remote_exec.go:16` accepts unregistered synonyms: `empty`, `purge`.
  4. `cli_proc_cmds.go:69` accepts `empty`.
  5. `buildConfigCacheSubcommand` advertises `[reset|clear]` instead of canonical `clear`.
- **Remediation**:
  1. In `cli_config_cmds.go`: remove the duplicate `list` subcommand from `buildConfigBasicSubcommands`.
  2. In `queue_cmd.go`: remove backdoor alias cases `ls`, `rm`, `del`, `delete`, keeping only `list`, `add`, `remove`, `clear`.
  3. In `cli_remote_exec.go`: change `case "clear", "empty", "purge":` to `case "clear":`.
  4. In `cli_proc_cmds.go`: change `if ctx.Args[0] == "clear" || ctx.Args[0] == "empty"` to `if ctx.Args[0] == "clear"`.
  5. In `cli_config_cmds.go` (`buildConfigCacheSubcommand`): update usage to `abs config cache [clear]` and reject `reset` in favor of canonical `clear`.
  6. Update test files (`global_policy_test.go`, `cli_config_cache_test.go`) to test canonical commands and verify removed aliases are rejected.
- **Regression Testing**:
  - Add regression tests in `cli_canonical_subcommands_test.go` validating that canonical subcommands (`config show`, `queue remove`, `proc clear`, `remote clear`, `config cache clear`) work as expected and undocumented aliases (`queue rm`, `remote purge`, `config list`) are rejected.

---

### 7. Issue 7 (Moderate): Subcommand Naming Inconsistency (`snake_case` vs `kebab-case`)
- **Severity**: Moderate
- **Affected Files**: `cli_server_cmds_extra.go`, `cli_server_cmds_extra2.go`, `pm_server_exec2.go`, `pm_frequency.go`
- **Root Cause**: `server` subcommands `get_info` and `disable_hourly` were registered using `snake_case`, whereas the CLI standard is `kebab-case`. Running `abs server get-info` or `abs server disable-hourly` failed.
- **Remediation**:
  1. In `cli_server_cmds_extra.go` (`buildServerGetInfoSubcommand`): Change `Name` to `"get-info"`, add `Aliases: []string{"get_info"}`, set `UsageLine` to `"abs server get-info [<k>] [options]"`, and set `opts.ServerSubcmd = "get-info"`.
  2. In `cli_server_cmds_extra2.go` (`buildServerDisableHourlySubcommand`): Change `Name` to `"disable-hourly"`, add `Aliases: []string{"disable_hourly"}`, set `UsageLine` to `"abs server disable-hourly [options]"`, and set `opts.ServerSubcmd = "disable-hourly"`.
  3. In `pm_server_exec2.go`: support both `"get-info"` and `"get_info"` in the switch statement, as well as `"disable-hourly"` and `"disable_hourly"`.
  4. In `pm_frequency.go`: match both `"disable_hourly"` and `"disable-hourly"`.
- **Regression Testing**:
  - Add unit tests in `cli_server_subcommands_test.go` verifying that both kebab-case names (`get-info`, `disable-hourly`) and backward-compatible aliases parse correctly.

---

### 8. Issue 8 (Moderate): Stream Inversion & Exit Code in `abs test abs map/download`
- **Severity**: Moderate
- **Affected Files**: `backend_cli.go`, `main.go`
- **Root Cause**: `absMapPodcasts` and `absDownloadAllData` print `ERROR: audiobookshelf is not configured.` to `stdout` and return void. `main.go` ignores failure states and terminates with exit code 0.
- **Remediation**:
  1. Refactor `absMapPodcasts(cfg Config, quiet bool) bool` to return `bool`. On configuration or backend errors, write to `os.Stderr` (`fmt.Fprintln(os.Stderr, "Error: ...")`) and return `false`. Return `true` on success.
  2. Refactor `absDownloadAllData(cfg Config, quiet bool) bool` to return `bool`. On errors, write to `os.Stderr` (`fmt.Fprintln(os.Stderr, "Error: ...")`) and return `false`. Return `true` on success.
  3. In `main.go` (`handleMainTest`): If `!absMapPodcasts(...)` or `!absDownloadAllData(...)`, call `os.Exit(1)`.
- **Regression Testing**:
  - Add unit tests in `backend_cli_test.go` verifying that `absMapPodcasts` and `absDownloadAllData` return `false` when unconfigured and emit error text to `os.Stderr`.

---

### 9. Issue 9 (Moderate): Information Density & Brevity Rules Compliance
- **Severity**: Moderate
- **Affected Files**: `cli_proc_cmds.go`, `cli_parse.go`, `cli_server_cmds_extra.go`, `cli_server_cmds_extra2.go`
- **Root Cause**:
  1. `abs proc -h` renders over 20 lines (up to 31+ lines) because 19 transcription options are displayed in local help.
  2. Several command and flag summaries exceed the 45-character limit (reaching up to 125 chars).
- **Remediation**:
  1. In `cli_server_cmds_extra2.go`:
     - `clean-orphans` description: `"Delete orphaned ABS podcast entries"` (35 chars).
  2. In `cli_server_cmds_extra.go`:
     - `get-info` description: `"Cache metadata for latest K episodes"` (36 chars).
     - `scan` description: `"Scan ABS library for podcasts & episodes"` (41 chars).
  3. In `cli_parse.go`:
     - `status` description: `"Show status overview of library & worker"` (41 chars).
     - `--dry-run` description: `"Preview actions without file changes"` (36 chars).
     - `--whisper-engine` description: `"Engine: local, docker, remote, gemini"` (39 chars).
     - `--whisper-model` description: `"Model name or alias (e.g. tiny.en, base)"` (42 chars).
  4. In `cli_proc_cmds.go`:
     - `proc` description: `"Process audio files or dirs for ad removal"` (43 chars).
     - `proc` parameter: `"Audio files (.mp3), JSONs, or directories"` (42 chars).
     - `collect` description: `"Pull completed batches from remote host"` (40 chars).
     - `clear` description: `"Stop remote workers and clear remote queue"` (43 chars).
  5. In `cli_parse.go` (`getTranscriptionOptions`):
     - Mark secondary options with `Hidden: true` (`--use-chunks`, `--extract-keywords`, `-t, --tminutes`, `--rffmpeg`, `--remote`, `--local`, `--no-collect`, `-n, --limit`, `-P, --priority`, `--whisper-engine`, `--whisper-model`), leaving 8 primary options visible so `abs proc -h` output is strictly <= 20 lines while preserving flag parsing.
- **Regression Testing**:
  - Add unit tests in `cli_brevity_test.go` verifying:
    - `abs proc -h` line count is <= 20 lines.
    - Updated command and option descriptions do not exceed 45 characters.

---

## Verification & Sizing Constraints

- Run `./tools/check` (`make check`) after changes to ensure:
  1. Code formatting (`gofmt -s -w .`) and config template generation pass.
  2. Go vet passes with 0 warnings.
  3. Staticcheck passes baseline comparison with 0 new issues.
  4. Sizing audit passes: each function <= 80 lines (hard limit), files <= 800 lines (warn), no file > 1100 lines (hard limit).
  5. All unit and regression tests pass with `-race` enabled.
  6. Binary builds cleanly (`./abs`).
