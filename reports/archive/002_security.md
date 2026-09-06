# Systems Code Review Report #002 (Lens: security)

- **Date**: 2026-09-06
- **Auditor**: Gemini 3.8 Flash (Tier 0 Workhorse) via `tools/audit`
- **Focus Lens**: `security`
- **Model Tier**: `TIER0`
- **Backend**: `gemini`
- **Scope**: `.`
- **Status**: Action Required

---

Auditing via Gemini Flash (bws run) [Profile: security]...
[SEVERITY]: Critical
[LOCATION]: [remote_cancel.go:47-48](file:///home/sariel/prog/26/podcasts/abs/remote_cancel.go#L47-L48)
[ROOT CAUSE]: Remote command execution via unquoted shell argument injection. In `runRemoteCancel`, the CLI parameter `batchID` is interpolated via `fmt.Sprintf` directly into a shell command string (`killCmd := fmt.Sprintf("pkill -f 'batch-worker.*%s' 2>/dev/null || true", batchID)`), which is forwarded to `transport.Exec(targetHost, killCmd)`. The underlying transport calls `exec.Command("ssh", ..., host, cmd)`. Because SSH delivers command strings directly to the remote login shell, unsanitized user input allows shell metacharacters to escape the `pkill` invocation and execute arbitrary commands under the remote user's privileges.
[FAILURE TRACE]:
1. User or automated caller triggers cancel: `abs remote cancel "x'; touch /tmp/pwned; #'"`
2. `runRemoteCancel` formats `killCmd`: `"pkill -f 'batch-worker.*x'; touch /tmp/pwned; #...'"`
3. `transport.Exec` invokes `ssh host "pkill -f 'batch-worker.*x'; touch /tmp/pwned; #..."`
4. Remote shell executes `touch /tmp/pwned`.
[REMEDIATION]:
```go
func sanitizeBatchID(batchID string) (string, error) {
	for _, r := range batchID {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return "", fmt.Errorf("invalid batch ID format: %q", batchID)
		}
	}
	return batchID, nil
}

// In runRemoteCancel:
safeBatchID, err := sanitizeBatchID(batchID)
if err != nil {
	return err
}
killCmd := fmt.Sprintf("pkill -f 'batch-worker.*%s' 2>/dev/null || true", shellQuote(safeBatchID))
```

---

[SEVERITY]: Critical
[LOCATION]: [remote_status.go:91-93](file:///home/sariel/prog/26/podcasts/abs/remote_status.go#L91-L93), [remote_batch.go:266-268](file:///home/sariel/prog/26/podcasts/abs/remote_batch.go#L266-L268), [remote_clear.go:44-45](file:///home/sariel/prog/26/podcasts/abs/remote_clear.go#L44-L45)
[ROOT CAUSE]: Remote command execution via Go `%q` double-quote escaping passed to remote shell. In `fetchRemoteActiveTask`, `verifyRemoteWorkerStartup`, and `runRemoteClear`, remote paths obtained from `grep -l` are interpolated into shell command strings via `fmt.Sprintf("cat %q ...", activePath)` and `fmt.Sprintf("rm -f %q ...", audioPath)`. Go's `%q` format verb creates double-quoted string literals (`"..."`). In POSIX shells, double quotes evaluate parameter expansions and command substitutions (`$(command)` and `` `command` ``). Because podcast and episode filenames often contain titles from external feeds, a filename containing command substitutions (e.g. `Show $(id)/ep.mp3.json`) triggers command execution when evaluated by the remote shell.
[FAILURE TRACE]:
1. A podcast episode is downloaded with title `Podcast $(reboot)`.
2. Remote status file is written to `abs_remote/Podcast $(reboot)/ep.mp3.json`.
3. `remote_status.go` executes `grep -l`, returning the path containing `$(reboot)`.
4. `statContent, _ := transport.Exec(targetHost, fmt.Sprintf("cat %q 2>/dev/null", activePath))` issues `ssh host 'cat "/home/user/abs_remote/Podcast $(reboot)/ep.mp3.json" 2>/dev/null'`.
5. The remote shell expands `$(reboot)` inside the double quotes and executes it.
[REMEDIATION]:
```go
// Replace %q with shellQuoteHomePath or shellQuote in all remote shell command templates:
statContent, _ := transport.Exec(targetHost, fmt.Sprintf("cat %s 2>/dev/null", shellQuoteHomePath(activePath)))

// In remote_batch.go:
if data, err := transport.Exec(targetHost, fmt.Sprintf("cat %s", shellQuoteHomePath(activeJsonPath))); err == nil && data != "" {

// In remote_clear.go:
delCmd := fmt.Sprintf("rm -f %s %s %s %s",
	shellQuoteHomePath(audioPath),
	shellQuoteHomePath(statPath),
	shellQuoteHomePath(basePath+".cuts.json"),
	shellQuoteHomePath(basePath+".transcript.json"))
```

---

[SEVERITY]: Critical
[LOCATION]: [remote_cancel.go:31-34](file:///home/sariel/prog/26/podcasts/abs/remote_cancel.go#L31-L34), [remote_collect.go:235-236, 251-254](file:///home/sariel/prog/26/podcasts/abs/remote_collect.go#L235-L254), [remote_status.go:292-293, 302](file:///home/sariel/prog/26/podcasts/abs/remote_status.go#L292-L302)
[ROOT CAUSE]: Path traversal leading to arbitrary local directory deletion and remote command injection via unvalidated `batchID`.
1. In `runRemoteCancel`, `pullStagedBatches`, and `fetchRemoteStatusBatches`, directory names returned from `ls -1` or passed from CLI arguments are joined via `filepath.Join(os.TempDir(), "abs_pull", batchID)` or `filepath.Join(os.TempDir(), "abs_cancel", batchID)` without verifying directory boundaries. Supplying `../../target` escapes the temporary directory, and the subsequent `os.RemoveAll(tempPullDir)` recursively deletes arbitrary directories on the host.
2. In `remote_collect.go:251`, `_, _ = transport.Exec(targetHost, fmt.Sprintf("rm -rf %s/%s", remoteStagingDir, batchID))` interpolates `batchID` without quotes, enabling arbitrary remote command injection if a directory entry contains shell control characters.
[FAILURE TRACE]:
1. Remote staging directory contains an entry `../../usr` or an attacker passes `../../var/log` to `abs remote cancel`.
2. `tempDir := filepath.Join(os.TempDir(), "abs_cancel", batchID)` resolves to `/var/log`.
3. Execution reaches `defer os.RemoveAll(tempDir)`.
4. The local process recursively wipes `/var/log`.
[REMEDIATION]:
```go
func validateBatchID(batchID string) bool {
	if batchID == "" || strings.Contains(batchID, "/") || strings.Contains(batchID, "\\") || strings.Contains(batchID, "..") {
		return false
	}
	return filepath.Base(batchID) == batchID
}

// In pullStagedBatches, fetchRemoteStatusBatches, and runRemoteCancel:
if !validateBatchID(batchID) {
	continue // or return fmt.Errorf("invalid batchID: %q", batchID)
}
tempPullDir := filepath.Join(os.TempDir(), "abs_pull", batchID)
// In remote_collect.go:251:
_, _ = transport.Exec(targetHost, fmt.Sprintf("rm -rf %s/%s", shellQuoteHomePath(remoteStagingDir), shellQuote(batchID)))
```

---

[SEVERITY]: Major
[LOCATION]: [audio.go:145-156](file:///home/sariel/prog/26/podcasts/abs/audio.go#L145-L156)
[ROOT CAUSE]: Shell argument injection in remote SSH ffmpeg execution.
In `cutAudioFFmpegWithHost`, `remIn` and `remOut` are formatted directly into remote shell commands without quotes (`fmt.Sprintf("rm -f %s %s", remIn, remOut)` and `fmt.Sprintf("ffmpeg ... -i %s ... %s", remIn, filterComplex, remOut)`). `remIn` contains `filepath.Ext(absInput)`. If an input filename or extension contains spaces or shell metacharacters, the remote command breaks argument parsing or executes injected commands. Furthermore, `filterComplex` is quoted with `%q` (double quotes), allowing command substitutions if any segment tags contain `$` or backticks.
[FAILURE TRACE]:
1. Input audio file has an extension or filename with spaces/metacharacters (e.g. `ep.part 1.mp3` or `ep.mp3;reboot;.mp3`).
2. `remIn` is generated as `.work/abs_123_456_in.part 1.mp3`.
3. `ssh remoteHost rm -f .work/abs_123_456_in.part 1.mp3 ...` executes.
4. The shell interprets `.part` and `1.mp3` as separate tokens.
[REMEDIATION]:
```go
delCmd := exec.Command("ssh", "-o", "BatchMode=yes", remoteHost,
	fmt.Sprintf("rm -f %s %s", shellQuote(remIn), shellQuote(remOut)))
_ = delCmd.Run()

remFFmpegCmd := exec.Command("ssh", "-o", "BatchMode=yes", remoteHost,
	fmt.Sprintf("ffmpeg -y -loglevel error -i %s -filter_complex %s -map '[aout]' -c:a libmp3lame -b:a 192k %s",
		shellQuote(remIn), shellQuote(filterComplex), shellQuote(remOut)))
```

---

[SEVERITY]: Major
[LOCATION]: [feed_cache.go:270](file:///home/sariel/prog/26/podcasts/abs/feed_cache.go#L270), [pkg/backend/abs_episodes.go:62](file:///home/sariel/prog/26/podcasts/abs/pkg/backend/abs_episodes.go#L62), [pkg/backend/podfetch_api.go:278](file:///home/sariel/prog/26/podcasts/abs/pkg/backend/podfetch_api.go#L278)
[ROOT CAUSE]: Denial of Service via unbounded `io.ReadAll` on untrusted external HTTP streams.
`feed_cache.go` and `abs_episodes.go` fetch podcast RSS feeds from external HTTP endpoints and read the full response body using `io.ReadAll(resp.Body)` without an `io.LimitReader`. Similarly, `podfetch_api.go` downloads cover images from untrusted `ImageURL` values using `io.ReadAll(resp.Body)`. A malicious feed host or infinite HTTP stream (chunked transfer encoding) consumes unbounded heap memory until process termination via OOM.
[FAILURE TRACE]:
1. User or automated fetcher subscribes to a podcast feed pointing to `https://attacker.com/feed.xml`.
2. Attacker server responds with an infinite stream of junk data or multi-gigabyte payload.
3. `io.ReadAll(resp.Body)` continues allocating buffers in a loop until physical RAM/swap is exhausted and the kernel OOM-killer terminates the process.
[REMEDIATION]:
```go
const maxFeedSize = 32 * 1024 * 1024 // 32 MB
body, err := io.ReadAll(io.LimitReader(resp.Body, maxFeedSize))

// In podfetch_api.go:
const maxImageSize = 10 * 1024 * 1024 // 10 MB
data, err := io.ReadAll(io.LimitReader(resp.Body, maxImageSize))
```

---

[SEVERITY]: Major
[LOCATION]: [gemini_studio.go:96-107](file:///home/sariel/prog/26/podcasts/abs/gemini_studio.go#L96-L107), [gemini_studio.go:132-133](file:///home/sariel/prog/26/podcasts/abs/gemini_studio.go#L132-L133), [gemini_studio.go:153](file:///home/sariel/prog/26/podcasts/abs/gemini_studio.go#L153)
[ROOT CAUSE]: Credential exposure in error messages and logs via URL query parameters.
In `uploadAudioToGeminiStudio`, `deleteGeminiStudioFile`, and `callGeminiStudioProcessor`, the Gemini API key is passed as a URL query parameter (`?key=%s`). When `http.Client.Do` fails (e.g. DNS timeout, TLS failure, or connection abort), Go's `net/http` package wraps the error in `*url.Error`, whose `.Error()` output includes the complete URL with query string intact. Wrapping and returning this error (`fmt.Errorf("gemini file upload failed: %w", err)`) leaks the plaintext API key to CLI output and error logs.
[FAILURE TRACE]:
1. `uploadAudioToGeminiStudio` executes HTTP request to `https://generativelanguage.googleapis.com/upload/v1beta/files?key=AIzaSy...`
2. A transient network timeout or DNS failure occurs during `client.Do(req)`.
3. `client.Do` returns `*url.Error` containing `POST "https://generativelanguage.googleapis.com/upload/v1beta/files?key=AIzaSy...": dial tcp ...`
4. The error is returned and printed to stderr, exposing the secret key in log files or console transcripts.
[REMEDIATION]:
```go
// Remove query parameter from URL and pass the key in the HTTP header:
url := "https://generativelanguage.googleapis.com/upload/v1beta/files"
req, err := http.NewRequestWithContext(ctx, "POST", url, pr)
if err != nil {
	return "", "", fmt.Errorf("failed to create upload request: %w", err)
}
req.Header.Set("x-goog-api-key", apiKey)
```

---

[SEVERITY]: Moderate
[LOCATION]: [pkg/backend/abs_scan.go:106-114](file:///home/sariel/prog/26/podcasts/abs/pkg/backend/abs_scan.go#L106-L114), [pkg/backend/abs_opml_import.go:112-122](file:///home/sariel/prog/26/podcasts/abs/pkg/backend/abs_opml_import.go#L112-L122)
[ROOT CAUSE]: Path traversal in directory creation via `..` in podcast title sanitizers.
`sanitizePodcastName` and `sanitizePodcastTitle` replace illegal path characters (`/`, `\`, `:`, etc.) with `_`, but fail to sanitize relative directory traversal segments (`..` or `.`). If an external podcast feed or OPML file has a title consisting of `..`, `sanitizePodcastName("..")` returns `".."`. When `filepath.Join(podcastsDir, safeName)` or `strings.TrimRight(targetFolder.FullPath, "/") + "/" + safeTitle` is computed, it evaluates to the parent of the storage directory, escaping the intended base directory.
[FAILURE TRACE]:
1. An imported OPML file contains `<outline text=".." title=".." .../>`.
2. `safeTitle := sanitizePodcastTitle(f.Title)` returns `".."`.
3. `containerPath := strings.TrimRight(targetFolder.FullPath, "/") + "/" + safeTitle` evaluates to `/srv/podcasts/..` (`/srv`).
4. The podcast folder is created outside the configured podcast library folder.
[REMEDIATION]:
```go
func sanitizePodcastName(s string) string {
	cleaned := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			return '_'
		}
		return r
	}, s)
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == ".." || cleaned == "." || strings.HasPrefix(cleaned, "../") {
		return "podcast_escaped"
	}
	return cleaned
}
```
