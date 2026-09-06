# Architectural Plan: Performance Audit Remediation (#005)

- **Date**: 2026-09-06
- **Focus**: Performance Optimization & Algorithmic Complexity Remediation
- **Target Report**: `reports/005_performance.md`
- **Output Artifact**: `reports/005_performance_plan.md`

---

## 1. Overview and Architecture Principles

The performance audit (#005) identified 7 critical performance bottlenecks spanning excessive heap allocations, unbounded network reads leading to potential OOM vulnerabilities, quadratic \(O(N^2)\) string concatenations, quadratic sorting algorithms, and unindexed character scanning.

Every remediation follows the guidelines set forth in `AGENTS.md`:
- Pure Go standard library without external dependencies.
- Function length hard limit: \(\le 80\) lines, decomposing in place when needed.
- File length comfort metric (300–700 lines, warn >800, hard limit 1100).
- No developer comments in Go source code (self-documenting code).
- Invariant: Each remediated defect includes at least one dedicated regression unit test.
- Verification through `./tools/check` (format, vet, staticcheck baseline, sizing audit, race detector tests, local build).

---

## 2. Issue Analysis and Remediation Plan

### Issue 1: High Memory Allocation in Multipart Body Construction
- **Location**: `transcribe.go:55-91` (`buildWhisperMultipartBody`), `executeWhisperAttempt`, `transcribeWhisper`
- **Severity**: Major
- **Root Cause**: Audio files and PCM data are fully read into memory (`audioContent`) and duplicated into an in-memory `bytes.Buffer` during multipart boundary formulation. For large audio files (hundreds of MiBs), this causes peak memory consumption 2–3× the audio file size, multiplying under concurrent operations and risking process termination.
- **Architectural Solution**:
  - Refactor `buildWhisperMultipartBody` to return `(io.ReadCloser, string, error)` using `io.Pipe()` and a background streaming writer goroutine.
  - For file inputs (`pcmData == nil`), open the file directly via `os.Open` and stream with `io.Copy(part, f)`.
  - For PCM buffer inputs (`pcmData != nil`), stream the 44-byte WAV header and raw PCM without concatenation using `io.MultiReader(bytes.NewReader(header), bytes.NewReader(pcmData))`.
  - In `transcribeWhisper`, instantiate a fresh streamed body on each retry attempt.
  - In `executeWhisperAttempt`, ensure the `io.ReadCloser` body is closed upon exit via `defer bodyReader.Close()`, ensuring any early termination or network failure signals `io.ErrClosedPipe` to terminate the producer goroutine cleanly without leaks.
- **Regression Testing**:
  - Test streaming multipart body creation with both file-based input and in-memory PCM data in `transcribe_test.go`.
  - Verify multipart boundary headers, fields (`response_format`, `temperature`, `language`, `prompt`), and streamed content integrity.

---

### Issue 2: Unbounded Network Response Reading in Whisper and Audiobookshelf Clients
- **Location**: `transcribe.go:120` (`executeWhisperAttempt`), `pkg/backend/audiobookshelf.go:231` (`executeRequestAttempt`)
- **Severity**: Major
- **Root Cause**: Both network clients invoke `io.ReadAll(resp.Body)` without an upper bound on received bytes. Malfunctioning endpoints, proxy error pages, or compromised upstream servers can stream infinite or gigantic responses, inducing unbounded heap growth and OOM crashes.
- **Architectural Solution**:
  - Introduce a bounded reader helper `readLimitedBody(r io.Reader, maxBytes int64) ([]byte, error)` utilizing `io.LimitReader(r, maxBytes+1)`. If the read length exceeds `maxBytes`, return a clear error indicating payload limit exceeded.
  - In `transcribe.go`: Apply `readLimitedBody` with `maxWhisperResponseBytes = 128 << 20` (128 MiB) for Whisper responses.
  - In `pkg/backend/audiobookshelf.go`: Apply `readLimitedBody` with `maxBackendResponseBytes = 128 << 20` (128 MiB) for Audiobookshelf JSON API payloads.
- **Regression Testing**:
  - Add regression tests in `transcribe_test.go` and `pkg/backend/audiobookshelf_test.go` validating:
    - Responses within limits read successfully without truncation.
    - Responses exceeding limits immediately return an error and do not consume excess memory.

---

### Issue 3: Quadratic Selection Sort in Transcription Segments
- **Location**: `transcribe.go:150-158` (`sortSegments`)
- **Severity**: Major
- **Root Cause**: `sortSegments` implements a naive nested-loop selection sort performing \(O(N^2)\) comparisons and element swaps. Long transcripts with thousands of chunks or segments experience quadratic CPU stalls during assembly.
- **Architectural Solution**:
  - Replace the nested loop selection sort with standard library `sort.Slice` (or `sort.SliceStable`) with comparator `segs[i].Start < segs[j].Start`, reducing algorithmic complexity to \(O(N \log N)\).
- **Regression Testing**:
  - Add regression unit test `TestSortSegments` in `transcribe_test.go` covering empty slices, single element, unsorted slices, reversed slices, duplicate timestamps, and a large slice (1,000+ elements) verifying sorted invariant and order preservation.

---

### Issue 4: Quadratic String Concatenation in Transcript Assembly and Segment Merging
- **Location**: `transcribe.go:160-178` (`mergeSegments`), `transcribe_chunks.go:196-204` (`assembleTranscriptionResult`), `format.go:119-123` (`validateTranscriptSanity`)
- **Severity**: Major
- **Root Cause**: Sequential string concatenation with `+` and `+=` inside loops re-allocates and copies all accumulated characters on every segment iteration, creating quadratic \(O(N^2)\) memory allocation and heavy GC thrashing.
- **Architectural Solution**:
  - Introduce `joinSegmentText(segs []TranscriptionSegment) string` utilizing `strings.Builder` with `b.Grow(total)` pre-calculated across segment text lengths.
  - Update `assembleTranscriptionResult` in `transcribe_chunks.go` and `validateTranscriptSanity` in `format.go` to use `joinSegmentText`.
  - In `mergeSegments` (`transcribe.go`), accumulate merged segment text parts in a slice (`[]string`) and join once using `strings.Join(parts, " ")` when the continuous overlapping run closes.
- **Regression Testing**:
  - Add unit tests in `transcribe_test.go` and `format_test.go` verifying:
    - `joinSegmentText` produces exact space-delimited text without leading/trailing whitespace.
    - `mergeSegments` correctly aggregates overlapping segments and sets concatenated text identically to previous behavior.

---

### Issue 5: Quadratic Ampersand Scanning in TUI HTML Rendering
- **Location**: `tui_format.go:193-219` (`renderHTML`)
- **Severity**: Moderate
- **Root Cause**: Every encounter of the `&` character triggers `strings.IndexByte(html[i:], ';')`, scanning the entire remaining string. When rendering texts with multiple ampersands that do not form valid entities, this causes \(O(N^2)\) byte searches and freezes the TUI.
- **Architectural Solution**:
  - Restrict the entity search window to `maxEntityBytes = 10` bytes (the longest supported entity `&quot;` is 6 bytes).
  - Search for `;` only within `html[i : min(i+maxEntityBytes, len(html))]`. If no semicolon is found within this small window, treat `&` as literal and proceed immediately.
- **Regression Testing**:
  - Add unit test `TestRenderHTML_AmpersandPerformance` in `tui_format_test.go` verifying that a string with 10,000 bare ampersands renders instantaneously without quadratic latency, while standard entities (`&amp;`, `&lt;`, `&gt;`, `&quot;`, `&apos;`, `&nbsp;`) decode accurately.

---

### Issue 6: Redundant Allocations and Case Conversion in Feed Date Parsing
- **Location**: `feed_cache.go:166-207` (`parseFeedDate`)
- **Severity**: Moderate
- **Root Cause**: For every episode date parsed, `tzMap` (map of 8 timezones) and `dateFormats` (slice of 10 layouts) are allocated on the heap. Furthermore, `strings.ToUpper(normalizedPubDate)` is re-evaluated in each loop iteration, generating up to 8 uppercase string allocations per episode.
- **Architectural Solution**:
  - Declare static package-level immutable arrays `feedTZOffsets` and `feedDateFormats`.
  - Implement `normalizeFeedTimezone(s string) string` using `strings.EqualFold` on the suffix, avoiding all intermediate uppercase string allocations.
  - Iterate over the static `feedDateFormats` array to eliminate per-call slice allocations.
- **Regression Testing**:
  - Add comprehensive unit test `TestParseFeedDate_TimezonesAndFormats` in `feed_cache_test.go` testing all timezone abbreviations (`PDT`, `PST`, `EDT`, `EST`, `CDT`, `CST`, `MDT`, `MST`) across mixed cases, standard RFC formats, custom layouts, empty strings, and malformed dates.

---

### Issue 7: Quadratic String Concatenation in FFmpeg Filter Construction
- **Location**: `audio.go:119-129` (`cutAudioFFmpegWithHost`)
- **Severity**: Moderate
- **Root Cause**: Building the FFmpeg filter complex using `filterParts += ...` and `concatInputs += ...` in a loop performs \(O(N^2)\) string copies for podcasts with numerous cut segments.
- **Architectural Solution**:
  - Extract a dedicated helper function `buildCutFilterComplex(keepSegments [][2]float64) string`.
  - Use `strings.Builder` with capacity preallocated via `filter.Grow(len(keepSegments) * 80)`.
  - Stream all `atrim` filter definitions, input labels, and the final `concat` specification directly into the builder.
- **Regression Testing**:
  - Add unit test `TestBuildCutFilterComplex` in `audio_test.go` verifying proper filter string generation across 0, 1, and 500 segments with zero quadratic string copying.

---

## 3. Execution Plan

1. **Phase 1: Implementation of Core Performance Helpers & Refactorings**
   - Modify `transcribe.go` (streamed multipart upload, `readLimitedBody`, `sortSegments`, `mergeSegments`).
   - Modify `pkg/backend/audiobookshelf.go` (bounded response reading with `readLimitedBody`).
   - Modify `transcribe_chunks.go` and `format.go` (integrate `joinSegmentText`).
   - Modify `tui_format.go` (bounded entity scan).
   - Modify `feed_cache.go` (static date parsing structures & zero-alloc timezone normalization).
   - Modify `audio.go` (`buildCutFilterComplex` with `strings.Builder`).

2. **Phase 2: Unit and Regression Test Suite Implementation**
   - Create `transcribe_test.go` (testing streaming multipart, bounded reads, sort, merge, `joinSegmentText`).
   - Create `audio_test.go` (testing `buildCutFilterComplex`).
   - Create `tui_format_test.go` (testing HTML rendering and ampersand scaling).
   - Expand `pkg/backend/audiobookshelf_test.go` and `feed_cache_test.go`.

3. **Phase 3: Verification and Quality Gate**
   - Run `tools/audit_lines` to ensure all functions remain \(\le 80\) lines and all files comply with sizing guidelines.
   - Run `tools/check` for format, vet, staticcheck baseline, race detector unit tests, and binary build.

4. **Phase 4: Summary & Commit**
   - Author post-work summary in `reports/005_summary.md`.
   - Stage and commit all changes via `tools/commit`.
