# Performance Remediation Summary #005

- **Round**: Performance Code Review (#005)
- **Issues Found**: 7 issues found across the codebase (4 Major, 3 Moderate), all 7 fully mitigated and covered by regression unit tests.

## Remediated Issues

### Issue 1: High Memory Allocation in Multipart Body Construction (`transcribe.go:55-91`)
- **Description**: Full audio files and PCM buffers were loaded entirely into memory and duplicated into a growing `bytes.Buffer` during multipart formation, causing memory spikes of 2–3× audio size and risking OOM.
- **Mitigation**: Implemented streaming multipart body creation via `io.Pipe()` with background streaming using `io.Copy()` and `io.MultiReader()`, eliminating full audio in-memory buffering.

### Issue 2: Unbounded Network Response Reading (`transcribe.go:120`, `pkg/backend/audiobookshelf.go:231`)
- **Description**: Network calls invoked unbounded `io.ReadAll(resp.Body)`, permitting misconfigured or malicious endpoints to stream unlimited data and exhaust memory.
- **Mitigation**: Added `readLimitedBody` using `io.LimitReader` with an enforced 128 MiB boundary to reject oversized HTTP responses before memory exhaustion occurs.

### Issue 3: Quadratic Selection Sort in Transcription Segments (`transcribe.go:150-158`)
- **Description**: `sortSegments` used a nested-loop selection sort with \(O(N^2)\) comparisons and swaps, degrading performance on transcripts with many chunks or segments.
- **Mitigation**: Replaced the nested-loop selection sort with `sort.Slice` to achieve \(O(N \log N)\) sorting complexity.

### Issue 4: Quadratic String Concatenation in Transcript Merging and Assembly (`transcribe.go:160-178`, `transcribe_chunks.go:196-204`, `format.go:119-123`)
- **Description**: Loops repeatedly concatenated immutable strings using `+` and `+=`, copying all previously accumulated text on each iteration and generating quadratic memory allocations.
- **Mitigation**: Added `joinSegmentText` with pre-allocated `strings.Builder` and updated `mergeSegments` to accumulate slices of text parts joined once per merged run.

### Issue 5: Quadratic Ampersand Scanning in TUI HTML Rendering (`tui_format.go:193-219`)
- **Description**: Every `&` scanned the entire remaining string for `;`, causing quadratic byte inspections and TUI freezes on text with many bare ampersands.
- **Mitigation**: Bounded the entity search window to a maximum of 8 bytes after `&`, avoiding scans of the remaining string when no semicolon is nearby.

### Issue 6: Redundant Allocations and Case Conversion in Feed Date Parsing (`feed_cache.go:166-207`)
- **Description**: `parseFeedDate` repeatedly allocated timezone maps and date layout slices per episode while repeatedly allocating uppercase copies of the date string.
- **Mitigation**: Replaced per-call structures with static package-level arrays and implemented zero-allocation suffix checking via `strings.EqualFold`.

### Issue 7: Quadratic String Concatenation in FFmpeg Filter Construction (`audio.go:119-129`)
- **Description**: FFmpeg filter complex generation iteratively concatenated filter parts with `+=`, scaling quadratically with the number of keep intervals.
- **Mitigation**: Extracted `buildCutFilterComplex` utilizing `strings.Builder` pre-allocated to the expected total filter graph length.

## Verification & Quality Gate
- **Line Sizing Gate**: All 244 files pass; 0 files exceed 800 lines; 0 functions exceed the 80-line hard limit.
- **Quality Gate**: Passed `./tools/check` cleanly (formatting, vetting, staticcheck baseline, race-detector unit test suite, and local build).
- **Footprint**: 15 files changed, 699 insertions(+), 97 deletions(-)
- **Differential Audit**: Clean (0 defects in diff)
