# Security Remediation Summary #002

- **Date**: 2026-09-06
- **Auditor & Remediation Agent**: Antigravity / Gemini 3.8 Flash
- **Focus Area**: Security Audit Remediation
- **Status**: Complete & Verified (Quality Gate Passed)
- **Total Issues Found & Resolved**: 7 issues (3 Critical, 3 Major, 1 Moderate)

---

### Issue Remediations

1. **Issue 1 (Critical) — Remote Command Injection in `remote_cancel.go`**
   - *Description*: Unquoted interpolation of CLI `batchID` parameter into `pkill` command string allowed remote arbitrary command execution via SSH login shell.
   - *Mitigation*: Enforced strict alphanumeric/dash/underscore format validation via `validateBatchID` and safely single-quoted the pattern with `shellQuote`.

2. **Issue 2 (Critical) — Remote Command Execution via `%q` Double Quotes**
   - *Description*: Remote shell command formatting used Go's `%q` format verb which creates POSIX double quotes that evaluate `$()` and backtick command substitutions in filenames.
   - *Mitigation*: Replaced `%q` with `shellQuoteHomePath` across remote `cat` and `rm -f` commands in `remote_status.go`, `remote_batch.go`, and `remote_clear.go`.

3. **Issue 3 (Critical) — Path Traversal & Remote Command Injection via `batchID`**
   - *Description*: Unchecked `batchID` in `abs_pull`, `abs_cancel`, and `abs_status` allowed `os.TempDir()` path traversal leading to arbitrary local directory deletion and unquoted `rm -rf` command injection.
   - *Mitigation*: Implemented `validateBatchID` to reject directory traversals and shell metacharacters, and safely quoted remote staging paths with `shellQuoteHomePath` and `shellQuote`.

4. **Issue 4 (Major) — Shell Argument Injection in Remote SSH FFmpeg Execution**
   - *Description*: Unquoted input/output audio paths and `%q`-quoted complex filter strings in `cutAudioFFmpegWithHost` permitted argument splitting and shell command injection over SSH.
   - *Mitigation*: Wrapped remote audio input/output filepaths and filter complex arguments in single quotes via `shellQuote` in `buildRemoteCutCleanupCmd` and `buildRemoteFFmpegCmd`.

5. **Issue 5 (Major) — Denial of Service via Unbounded `io.ReadAll`**
   - *Description*: Unbounded `io.ReadAll` calls on untrusted RSS feed responses and podcast cover images allowed infinite HTTP streams or oversized payloads to trigger process OOM crashes.
   - *Mitigation*: Applied `io.LimitReader` bounds of 32 MB for podcast RSS feeds (`feed_cache.go`, `abs_episodes.go`) and 10 MB for cover images (`podfetch_api.go`).

6. **Issue 6 (Major) — Credential Exposure in Error Messages via Query Parameters**
   - *Description*: Passing the Gemini API key as a URL query parameter (`?key=%s`) leaked secrets to logs and console output wrapped in `*url.Error` upon network failures.
   - *Mitigation*: Removed query parameters from Gemini Studio REST endpoints and securely supplied the API key via the `x-goog-api-key` HTTP header.

7. **Issue 7 (Moderate) — Path Traversal via `..` in Podcast Title Sanitizers**
   - *Description*: `sanitizePodcastName` and `sanitizePodcastTitle` replaced illegal characters with underscores but failed to guard against `..` segments, escaping podcast library root directories.
   - *Mitigation*: Updated `sanitizePodcastName` and `sanitizePodcastTitle` across `pkg/backend` and CLI helpers to reject `.`/`..` traversals and return safe fallback directory names.

---

### Regression Unit Tests Added / Updated

- `remote_quote_test.go`: Added `TestValidateBatchID`, `TestRunRemoteCancelRejectsHostileBatchID`, `TestRemoteQuotedCommandsAvoidShellInjection`, and `TestRemoteFFmpegCommandQuoting`.
- `feed_cache_test.go`: Added `TestFetchFeedDirectBoundsBody`.
- `pkg/backend/podfetch_test.go`: Added `TestPodFetchDownloadCoverBoundsOversizedImage`.
- `gemini_test.go`: Added `TestGeminiStudioNoKeyInURLError`.
- `pkg/backend/abs_scan_test.go`: Added `TestSanitizePodcastNameTraversal`.
- `pkg/backend/abs_opml_test.go`: Added `TestSanitizePodcastTitleTraversal`.
- `main_test.go`: Added `TestSanitizePodcastTitleTraversal`.
