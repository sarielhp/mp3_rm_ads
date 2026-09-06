# Systems Code Review Report #005 (Lens: performance)

- **Date**: 2026-09-06
- **Auditor**: Flagship Opus / Deep (Tier 2 Deep) via `tools/audit`
- **Focus Lens**: `performance`
- **Model Tier**: `DEEP`
- **Backend**: `auto`
- **Scope**: `.`
- **Status**: Action Required

---

Auditing via Codex CLI (gpt-5.6-sol, tier: deep) [Profile: performance]...
[SEVERITY]: Major  
[LOCATION]: `transcribe.go:55-91`, `buildWhisperMultipartBody`  
[ROOT CAUSE]: The complete audio is loaded into `audioContent` and then copied again into a growing `bytes.Buffer`. For PCM input, `append(header, pcmData...)` creates another full-size copy first. Peak memory is therefore multiple times the audio size, and `bytes.Buffer` growth can temporarily increase it further.  
[FAILURE TRACE]: Transcribing a long WAV or large podcast episode loads hundreds of MiB into memory; multipart construction duplicates it; concurrent jobs multiply the footprint and can trigger heavy GC or OOM termination before the request starts.  
[REMEDIATION]:
```go
func newWhisperBody(audioPath string) (io.ReadCloser, string, error) {
	f, err := os.Open(audioPath)
	if err != nil {
		return nil, "", err
	}

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	contentType := mw.FormDataContentType()

	go func() {
		defer f.Close()
		part, err := mw.CreateFormFile("file", filepath.Base(audioPath))
		if err == nil {
			_, err = io.Copy(part, f)
		}
		if err == nil {
			err = mw.WriteField("response_format", "verbose_json")
		}
		if err == nil {
			err = mw.WriteField("temperature", "0.0")
		}
		if closeErr := mw.Close(); err == nil {
			err = closeErr
		}
		_ = pw.CloseWithError(err)
	}()

	return pr, contentType, nil
}
```
Create a fresh streamed body for each retry. For in-memory PCM, stream the 44-byte header and PCM with `io.MultiReader` rather than concatenating them.

[SEVERITY]: Major  
[LOCATION]: `transcribe.go:120`, `executeWhisperAttempt`; `pkg/backend/audiobookshelf.go:231`, `executeRequestAttempt`  
[ROOT CAUSE]: Both network paths use unbounded `io.ReadAll`. A peer controls the response size, so neither client has a memory-use invariant. The HTTP timeout limits duration, not bytes, and later `string(body)` conversions may allocate another full copy.  
[FAILURE TRACE]: A malfunctioning endpoint returns an indefinitely large body, a proxy serves a large HTML error page, or a compromised server streams data quickly. The process keeps expanding the byte slice until memory is exhausted.  
[REMEDIATION]:
```go
const maxResponseBytes int64 = 128 << 20

func readLimitedBody(r io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, maxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxResponseBytes {
		return nil, fmt.Errorf("response exceeds %d bytes", maxResponseBytes)
	}
	return body, nil
}
```
Replace both raw `io.ReadAll` calls with `readLimitedBody`, selecting endpoint-appropriate limits.

[SEVERITY]: Major  
[LOCATION]: `transcribe.go:150-158`, `sortSegments`  
[ROOT CAUSE]: The nested-loop selection sort performs \(N(N-1)/2\) comparisons and swaps. Transcription segment count is not bounded and grows with recording duration and chunk count.  
[FAILURE TRACE]: Chunked transcription produces thousands or tens of thousands of segments. Assembly performs tens or hundreds of millions of comparisons before any merging occurs, making CPU time grow quadratically.  
[REMEDIATION]:
```go
import "sort"

func sortSegments(segs []TranscriptionSegment) {
	sort.Slice(segs, func(i, j int) bool {
		return segs[i].Start < segs[j].Start
	})
}
```

[SEVERITY]: Major  
[LOCATION]: `transcribe.go:160-178`, `mergeSegments`; `transcribe_chunks.go:196-204`, `assembleTranscriptionResult`; `format.go:119-123`, `validateTranscriptSanity`  
[ROOT CAUSE]: Repeated immutable-string concatenation copies all previously accumulated text on every iteration. Total copying becomes quadratic in transcript size. The merge path is especially expensive when many overlapping segments collapse into one segment.  
[FAILURE TRACE]: A long transcript contains many segments, or overlapping chunk output continually extends one merged segment. Each append recopies the accumulated transcript, causing excessive allocation, GC pressure, and \(O(C^2)\) copied bytes for \(C\) text bytes.  
[REMEDIATION]:
```go
func joinSegmentText(segs []TranscriptionSegment) string {
	var b strings.Builder
	total := 0
	for _, seg := range segs {
		total += len(seg.Text) + 1
	}
	b.Grow(total)
	for i, seg := range segs {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(seg.Text)
	}
	return strings.TrimSpace(b.String())
}
```
Use this helper in assembly and sanity validation. In `mergeSegments`, accumulate each merged run as `[]string` and assign `strings.Join(parts, " ")` once when the run closes.

[SEVERITY]: Moderate  
[LOCATION]: `tui_format.go:193-219`, especially line 211 in `renderHTML`  
[ROOT CAUSE]: Every `&` searches the entire remaining input for `;`. Inputs containing many ampersands without semicolons therefore rescan overlapping suffixes and produce quadratic work.  
[FAILURE TRACE]: Rendering a description such as thousands of `&` characters followed by no semicolon performs approximately \(N^2/2\) byte inspections, stalling the interactive TUI.  
[REMEDIATION]:
```go
if c == '&' {
	const maxEntityBytes = 8
	end := i + maxEntityBytes
	if end > len(html) {
		end = len(html)
	}
	if off := strings.IndexByte(html[i:end], ';'); off >= 0 {
		textBuf.WriteString(decodeHTMLEntity(html[i : i+off+1]))
		i += off
		continue
	}
}
```

[SEVERITY]: Moderate  
[LOCATION]: `feed_cache.go:166-207`, `parseFeedDate`  
[ROOT CAUSE]: The time-zone map and date-layout slice are reconstructed for every episode. More importantly, `strings.ToUpper(normalizedPubDate)` is recomputed inside the eight-entry timezone loop, potentially allocating eight full copies per date.  
[FAILURE TRACE]: Refreshing feeds with thousands of historical episodes repeatedly allocates maps, slices, and uppercase date strings during XML conversion, increasing GC work linearly with a large constant factor.  
[REMEDIATION]:
```go
var feedTZOffsets = [...]struct {
	suffix string
	offset string
}{
	{" PDT", " -0700"}, {" PST", " -0800"},
	{" EDT", " -0400"}, {" EST", " -0500"},
	{" CDT", " -0500"}, {" CST", " -0600"},
	{" MDT", " -0600"}, {" MST", " -0700"},
}

var feedDateFormats = [...]string{
	time.RFC1123Z, time.RFC1123, time.RFC822Z,
	time.RFC822, time.RFC3339,
	"Mon, 2 Jan 2006 15:04:05 -0700",
	"Mon, 2 Jan 2006 15:04:05 MST",
	"Mon, 02 Jan 2006 15:04:05 -0700",
	"2 Jan 2006 15:04:05 -0700",
	"2006-01-02 15:04:05",
}

func normalizeFeedTimezone(s string) string {
	for _, tz := range feedTZOffsets {
		if len(s) >= len(tz.suffix) &&
			strings.EqualFold(s[len(s)-len(tz.suffix):], tz.suffix) {
			return s[:len(s)-len(tz.suffix)] + tz.offset
		}
	}
	return s
}
```

[SEVERITY]: Moderate  
[LOCATION]: `audio.go:119-129`, `cutAudioFFmpegWithHost`  
[ROOT CAUSE]: `filterParts +=` and `concatInputs +=` repeatedly copy their accumulated strings. `keepSegments` is accepted without a local bound, so filter construction is quadratic in segment count.  
[FAILURE TRACE]: A caller supplies a large cut set or externally sourced metadata generates thousands of keep intervals. Building the FFmpeg argument repeatedly copies an increasingly large filter before FFmpeg is even launched.  
[REMEDIATION]:
```go
var filter strings.Builder
filter.Grow(len(keepSegments) * 80)

for idx, seg := range keepSegments {
	fmt.Fprintf(&filter,
		"[0:a]atrim=start=%.3f:end=%.3f,asetpts=PTS-STARTPTS[a%d];",
		seg[0], seg[1], idx)
}
for idx := range keepSegments {
	fmt.Fprintf(&filter, "[a%d]", idx)
}
fmt.Fprintf(&filter, "concat=n=%d:v=0:a=1[aout]", len(keepSegments))

filterComplex := filter.String()
```
