# Stage 03: Audio Processing, Temp Files, and Format Handling

## 1. Goal & Rationale

Stage 03 establishes the media processing primitives:
- `pkg/audio`: Encapsulates all external FFmpeg/FFprobe invocations, audio stream probing, 16kHz WAV extraction, audio slicing, and strict runtime `.work/` temp file validation.
- `pkg/format`: Houses pure mathematical and parsing logic for interval merging, keep-segment calculations, time clock formatting, and subtitle export (SRT, VTT, TXT, cuts JSON).

Separating `pkg/format` from `pkg/audio` ensures that time formatting and interval math remain fast, dependency-free unit-testable components without requiring audio I/O or FFmpeg binaries.

---

## 2. Package Details

### 2.1 `pkg/audio` (FFmpeg & Media Slicing)

| Target File | Source File(s) | Responsibilities |
|:---|:---|:---|
| `ffmpeg.go` | `audio.go` | FFmpeg wrapper: probe audio duration, convert to 16kHz mono WAV, slice audio with filter_complex concat |
| `tempfile.go` | `audio.go`, `types.go` | Strict `.work/` directory validation (`VerifyTempFile`, `WorkDirFor`, cleanup routines) |
| `output.go` | `output.go` | Final artifact placement, atomic file replacement, MP3 ID3 metadata preservation |

**Rules:**
- All temp file writes must pass `VerifyTempFile(path)`.
- Use `pkg/util.ShellQuote` for argument construction when invoking external CLI tools.
- Return explicit `error` values; do not call `fatalError`.

### 2.2 `pkg/format` (Interval Math & Subtitle Formatting)

| Target File | Source File(s) | Responsibilities |
|:---|:---|:---|
| `intervals.go` | `format_intervals.go` | Interval merging (`MergeIntervals`, `MergeBounds`), keep-segment calculation (`CalculateKeepSegments`) |
| `time.go` | `format.go` | Clock string formatting (`FormatClock`, `FormatSRTTime`, `ParseTimeSeconds`) |
| `subtitles.go` | `format.go` | Subtitle export: convert transcript data to SRT, VTT, and plain TXT |
| `cuts.go` | `format.go`, `output.go` | Cuts JSON serialization and deserialization (`SaveCutsJSON`, `LoadCutsJSON`) |

**Dependencies:**
- `pkg/format` depends only on `pkg/types` and `pkg/util`.
- `pkg/audio` depends on `pkg/types`, `pkg/util`, and `pkg/format`.

---

## 3. Step-by-Step Implementation Plan

### Step 3.1: Create `pkg/format`
1. Move interval calculation algorithms from `format_intervals.go` to `pkg/format/intervals.go`.
2. Move time formatting functions from `format.go` to `pkg/format/time.go`.
3. Move subtitle conversion routines to `pkg/format/subtitles.go`.
4. Move cuts JSON logic to `pkg/format/cuts.go`.
5. Migrate unit tests (`format_test.go`, `save_cuts_test.go`, `ad_bounds_test.go`) into `pkg/format/`.

### Step 3.2: Create `pkg/audio`
1. Move FFmpeg and FFprobe wrappers into `pkg/audio/ffmpeg.go`.
2. Implement temp file validation in `pkg/audio/tempfile.go`.
3. Move atomic output file operations into `pkg/audio/output.go`.
4. Migrate unit tests (`audio_test.go`, `atomic_write_test.go`, `work_dir_test.go`) into `pkg/audio/`.

### Step 3.3: Bridge Monolith
1. Update root files to import `pkg/audio` and `pkg/format`.
2. Run line audit to verify no function exceeds 80 lines.

---

## 4. Verification & Quality Gates

```bash
# Verify new format and audio packages
go test -v ./pkg/format/...
go test -v ./pkg/audio/...

# Verify existing root tests
go test -timeout 30s ./...

# Sizing audit
./tools/audit_lines --quiet
```
