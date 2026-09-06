# Systems Code Review #006 Remediation Summary (Lens: CLI)

## Summary of Round
A total of **9 issues** (1 Critical, 4 Major, 4 Moderate) were identified in `reports/006_cli.md` during the CLI audit. All 9 issues have been fully remediated and validated with dedicated regression unit tests in `cli_remediation_006_test.go`, complying strictly with `AGENTS.md` function sizing (<= 80 lines) and codebase conventions.

---

## Remediated Issues

1. **Issue 1 (Critical) - Argument Slicing in `abs test kitty <image-file>`**
   - *Description*: `abs test kitty <image-file>` passed `["kitty", "<image-file>"]` wholesale into `testKittyImage`, causing `os.Stat("kitty")` to fail with ENOENT.
   - *Mitigation*: Sliced off the `"kitty"` subcommand token in `buildTestCommand.Run` and defensively stripped residual tokens in `testKittyImage`.

2. **Issue 2 (Major) - Validation of Positional Service Arguments in `resolveTestCommandArgs`**
   - *Description*: `resolveTestCommandArgs` lacked validation of service targets, silently defaulting unknown service targets to HTTP probes against Whisper.
   - *Mitigation*: Refactored `resolveTestCommandArgs` to validate targets and sub-targets, returning actionable errors on unknown inputs.

3. **Issue 3 (Major) - Error Stream and Exit Code in `handleConfigGet`**
   - *Description*: `handleConfigGet` printed unknown key errors to stdout and returned status code 0, breaking automated scripting and status checks.
   - *Mitigation*: Refactored `handleConfigGet` to return an `error`, routing error output to stderr and terminating with exit code 1 in `main.go`.

4. **Issue 4 (Major) - Exit Status and Error Message in `buildHelpCommand`**
   - *Description*: `buildHelpCommand` discarded `RenderCommand`'s boolean result, producing blank output and exiting with status 0 on unknown commands.
   - *Mitigation*: Checked `RenderCommand` return value, printing an actionable error message to stderr and exiting with code 1 upon unknown commands.

5. **Issue 5 (Major) - Conflicting Export Flags in `transcript` Command**
   - *Description*: Three overlapping mechanisms (`--export <format>`, `--txt`, `--srt`) allowed conflicting flags where `--txt` silently shadowed `--export srt`.
   - *Mitigation*: Enforced mutual exclusivity in `runTranscriptCommand` and marked legacy `--txt` and `--srt` options hidden in favor of canonical `--export`.

6. **Issue 6 (Moderate) - Duplicate Subcommands and Backdoor Aliases**
   - *Description*: Duplicate subcommand `config list` and undocumented backdoor aliases (`ls`, `rm`, `del`, `delete`, `empty`, `purge`, `reset`) cluttered the interface.
   - *Mitigation*: Removed duplicate `config list`, eliminated unregistered backdoor aliases across queue, remote, proc, and cache commands, and enforced canonical names.

7. **Issue 7 (Moderate) - Subcommand Naming Inconsistency (`snake_case` vs `kebab-case`)**
   - *Description*: Server subcommands `get_info` and `disable_hourly` used snake_case instead of the CLI's standard kebab-case conventions.
   - *Mitigation*: Standardized primary command names to `get-info` and `disable-hourly` while retaining legacy snake_case variants as backward-compatible aliases.

8. **Issue 8 (Moderate) - Stream Inversion & Exit Code in `abs test abs map/download`**
   - *Description*: `absMapPodcasts` and `absDownloadAllData` printed configuration errors to stdout and returned void, exiting with code 0 on failure.
   - *Mitigation*: Refactored `absMapPodcasts` and `absDownloadAllData` to return `bool`, write errors to stderr, and propagate exit code 1 in `main.go`.

9. **Issue 9 (Moderate) - Information Density & Brevity Rules Compliance**
   - *Description*: `abs proc -h` rendered 31 lines due to 19 flattened options, and several command/option summaries exceeded 45 characters.
   - *Mitigation*: Shortened command and option summaries to <= 45 characters and marked secondary options hidden in `proc` help to achieve exactly 20 lines.
