# Security Remediation Plan #002

**Document**: `reports/002_security_plan.md`  
**Reference**: `reports/002_security.md`  
**Focus Area**: Security Audit & Vulnerability Remediation  
**Status**: Ready for Implementation  

---

## 1. Overview & Architecture Strategy

This plan specifies the technical implementation details for remediating all 7 security findings identified in `reports/002_security.md`. All fixes adhere strictly to the project invariants:
- Every remediated issue must include at least one regression unit test.
- Changes are scoped strictly to the reported defects.
- Function length limit: <= 80 lines per function (decomposed in place where necessary).
- Code style: clean, self-documenting Go, no inline code comments, error return checks.
- Full verification through `./tools/check` (`make check`).

---

## 2. Issue Analysis & Detailed Mitigation Strategy

### Issue 1 & Issue 3: Remote Command Injection & Local Path Traversal via Unvalidated `batchID`
- **Locations**:
  - `remote_cancel.go:31-34, 47-48`
  - `remote_collect.go:235-236, 251-254`
  - `remote_status.go:292-293, 302`
- **Root Causes**:
  - Unsanitized CLI argument `batchID` interpolated into `pkill -f 'batch-worker.*%s'` string passed to SSH remote login shell.
  - Path traversal through `batchID` allows escaping `os.TempDir()` (e.g. `../../target`) which gets wiped via `os.RemoveAll(tempDir)`.
  - Directory entries from remote `staging` directory passed unquoted to `rm -rf %s/%s`.
- **Mitigation Strategy**:
  1. Define `validateBatchID(batchID string) bool` in `remote_transport.go`:
     - Batch IDs must not be empty.
     - Must only contain characters `[a-zA-Z0-9_-]`.
     - Must satisfy `filepath.Base(batchID) == batchID` and not contain path separators or `..`.
  2. In `runRemoteCancel`:
     - Reject any non-empty `batchID` that fails `validateBatchID(batchID)` with `fmt.Errorf("invalid batch ID format: %q", batchID)`.
     - Construct `killCmd` using `shellQuote`: `fmt.Sprintf("pkill -f %s 2>/dev/null || true", shellQuote("batch-worker.*"+batchID))`.
  3. In `remote_collect.go` (`pullStagedBatches`):
     - Skip entries where `!validateBatchID(batchID)`.
     - Shell-quote arguments in remote cleanup: `fmt.Sprintf("rm -rf %s/%s", shellQuoteHomePath(remoteStagingDir), shellQuote(batchID))`.
  4. In `remote_status.go` (`fetchRemoteStatusBatches`):
     - Skip entries where `!validateBatchID(bid)`.
- **Regression Unit Tests**:
  - Unit tests in `remote_quote_test.go` and `remote_batch_test.go` covering `validateBatchID` with hostile traversal strings (`../../etc`, `..`, `.`), shell injection payloads (`x'; touch /tmp/pwned; #'`), and valid batch IDs.
  - Test verifying `runRemoteCancel` rejects malicious `batchID` without calling transport commands or executing shell injections.

---

### Issue 2: Remote Shell Execution via Go `%q` Double-Quote Escaping
- **Locations**:
  - `remote_status.go:91-93`
  - `remote_batch.go:266-268`
  - `remote_clear.go:44-45`
- **Root Causes**:
  - Go's `%q` verb formats strings with double quotes (`"..."`). When passed to a remote shell (`ssh host "cat ..."`), double quotes do not prevent POSIX shell command substitutions (`$(command)` and `` `command` ``) or parameter expansions (`$VAR`).
- **Mitigation Strategy**:
  1. Replace `%q` with `shellQuoteHomePath(...)` (or `shellQuote(...)`) in remote shell command templates:
     - `remote_status.go`: `fmt.Sprintf("cat %s 2>/dev/null", shellQuoteHomePath(activePath))`
     - `remote_batch.go`: `fmt.Sprintf("cat %s", shellQuoteHomePath(activeJsonPath))`
     - `remote_clear.go`:
       ```go
       delCmd := fmt.Sprintf("rm -f %s %s %s %s",
           shellQuoteHomePath(audioPath),
           shellQuoteHomePath(statPath),
           shellQuoteHomePath(basePath+".cuts.json"),
           shellQuoteHomePath(basePath+".transcript.json"))
       ```
- **Regression Unit Tests**:
  - Unit tests in `remote_quote_test.go` demonstrating that paths containing `$(touch /tmp/pwned)`, `` `whoami` ``, and spaces are safely single-quoted by `shellQuoteHomePath` and pass through POSIX shell execution without running the subshell command.

---

### Issue 4: Shell Argument Injection in Remote SSH FFmpeg Execution
- **Locations**:
  - `audio.go:145-156`
- **Root Causes**:
  - `remIn` and `remOut` paths formatted directly into remote shell commands without quotes (`fmt.Sprintf("rm -f %s %s", remIn, remOut)`).
  - `filterComplex` quoted with `%q` (double quotes) which permits shell expansions if segment markers or expressions contain metacharacters.
- **Mitigation Strategy**:
  1. Use `shellQuote` for all remote arguments in `cutAudioFFmpegWithHost`:
     - Remote cleanup: `fmt.Sprintf("rm -f %s %s", shellQuote(remIn), shellQuote(remOut))`
     - Remote ffmpeg command:
       ```go
       fmt.Sprintf("ffmpeg -y -loglevel error -i %s -filter_complex %s -map '[aout]' -c:a libmp3lame -b:a 192k %s",
           shellQuote(remIn), shellQuote(filterComplex), shellQuote(remOut))
       ```
  2. Factor command generation into helpers `buildRemoteCutCleanupCmd` and `buildRemoteFFmpegCmd` to allow isolated unit testing.
- **Regression Unit Tests**:
  - Unit test verifying command strings produced by `buildRemoteFFmpegCmd` and `buildRemoteCutCleanupCmd` for hostile inputs with spaces, quotes, and metacharacters preserve literal strings under shell evaluation.

---

### Issue 5: Denial of Service via Unbounded `io.ReadAll` on External HTTP Streams
- **Locations**:
  - `feed_cache.go:270`
  - `pkg/backend/abs_episodes.go:62`
  - `pkg/backend/podfetch_api.go:278`
- **Root Causes**:
  - Calling `io.ReadAll(resp.Body)` without an upper bound permits infinite HTTP streams or maliciously huge bodies to exhaust memory.
- **Mitigation Strategy**:
  1. In `feed_cache.go`:
     - Use `io.LimitReader(resp.Body, 32*1024*1024)` (32 MB max RSS feed size).
  2. In `pkg/backend/abs_episodes.go`:
     - Use `io.LimitReader(resFeed.Body, 32*1024*1024)` (32 MB max RSS feed size).
  3. In `pkg/backend/podfetch_api.go`:
     - Use `io.LimitReader(resp.Body, 10*1024*1024)` (10 MB max image size).
- **Regression Unit Tests**:
  - Test in `feed_cache_test.go` with an HTTP test server returning a body exceeding 32 MB to verify the reader truncates or stops at 32 MB without OOM.
  - Test in `pkg/backend/podfetch_test.go` with an HTTP test server returning an oversized image body verifying it limits to 10 MB.

---

### Issue 6: Credential Exposure in Error Messages via URL Query Parameters
- **Locations**:
  - `gemini_studio.go:96-107, 132-133, 153`
- **Root Causes**:
  - Passing `?key=%s` in Gemini REST API URLs exposes the secret API key in `*url.Error` strings when network operations fail.
- **Mitigation Strategy**:
  1. Remove `?key=%s` from URL endpoints:
     - `https://generativelanguage.googleapis.com/upload/v1beta/files`
     - `https://generativelanguage.googleapis.com/v1beta/%s`
     - `https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent`
  2. Supply API key via header `req.Header.Set("x-goog-api-key", apiKey)`.
- **Regression Unit Tests**:
  - Unit test in `gemini_test.go` with mock HTTP handler verifying the `x-goog-api-key` header is sent, URL contains no query string parameters, and simulated transport errors do not contain the secret API key.

---

### Issue 7: Path Traversal in Directory Creation via `..` in Podcast Title Sanitizers
- **Locations**:
  - `pkg/backend/abs_scan.go:106-114`
  - `pkg/backend/abs_opml_import.go:112-122`
  - `backend_cli.go:200-210`
- **Root Causes**:
  - Sanitizers replace illegal path characters with `_` but fail to check if the result is `.` or `..` or starts with traversal patterns, leading to directory traversal outside library roots.
- **Mitigation Strategy**:
  1. In `pkg/backend/abs_scan.go` (`sanitizePodcastName`):
     - Replace illegal characters with `_`.
     - Trim whitespace.
     - If `cleaned == ".." || cleaned == "." || strings.Trim(cleaned, ".") == "" || strings.HasPrefix(cleaned, "../")`:
       return `"podcast_escaped"` (or `"unnamed_podcast"` if empty).
  2. In `pkg/backend/abs_opml_import.go` and `backend_cli.go` (`sanitizePodcastTitle`):
     - Replace illegal characters with `_`.
     - Trim whitespace.
     - If `title == "" || title == ".." || title == "." || strings.Trim(title, ".") == ""`:
       return `"Untitled Podcast"`.
- **Regression Unit Tests**:
  - Unit tests in `pkg/backend/abs_scan_test.go`, `pkg/backend/abs_opml_test.go`, and `main_test.go` checking `..`, `.`, `...`, `.. `, and empty titles to verify they cannot escape their parent directory when joined with a base path.

---

## 3. Implementation Order & Verification Sequence

1. **Step 1: Core Path & Shell Security (Issues 1, 2, 3)**
   - Add `validateBatchID` in `remote_transport.go`.
   - Update `remote_cancel.go`, `remote_collect.go`, `remote_status.go`, `remote_batch.go`, and `remote_clear.go`.
   - Add regression tests in `remote_quote_test.go` and `remote_batch_test.go`.
2. **Step 2: FFmpeg Remote Execution Quoting (Issue 4)**
   - Update `audio.go` with shell-quoted remote command generation.
   - Add regression test for remote ffmpeg command quoting.
3. **Step 3: Unbounded Stream Limits (Issue 5)**
   - Update `feed_cache.go`, `pkg/backend/abs_episodes.go`, `pkg/backend/podfetch_api.go`.
   - Add regression tests in `feed_cache_test.go` and `pkg/backend/podfetch_test.go`.
4. **Step 4: Credential Exposure in Gemini Studio (Issue 6)**
   - Update `gemini_studio.go` to use `x-goog-api-key` header.
   - Add regression tests in `gemini_test.go`.
5. **Step 5: Podcast Title Sanitization (Issue 7)**
   - Update `pkg/backend/abs_scan.go`, `pkg/backend/abs_opml_import.go`, and `backend_cli.go`.
   - Add regression tests in `pkg/backend/abs_scan_test.go`, `pkg/backend/abs_opml_test.go`, and `main_test.go`.
6. **Step 6: Quality Gate & Line Sizing Audit**
   - Run `./tools/check` to format, vet, staticcheck, run all tests with race detector, and build `./abs`.
   - Ensure all functions <= 80 lines and file sizing within limits.
7. **Step 7: Documentation & Commit**
   - Write post-work summary to `reports/002_summary.md`.
   - Commit all work cleanly using `tools/commit`.
