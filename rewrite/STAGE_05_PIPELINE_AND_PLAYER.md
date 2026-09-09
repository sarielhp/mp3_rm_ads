# Stage 05: Processing Pipeline and Audio Playback Daemon

## 1. Goal & Rationale

Stage 05 moves the operational engines into structured packages:
- `pkg/pipeline`: The central processing coordinator that orchestrates the media pipeline (WAV extraction $\to$ transcription $\to$ ad detection $\to$ cutting $\to$ exporting), along with batch execution, dry-run estimation, and error quarantine.
- `pkg/player`: The headless background audio playback daemon, featuring an IPC Unix domain socket (`/tmp/abs_player.sock`), MPRIS D-Bus media controls, and sink backends (`mpv` and `cvlc`).

---

## 2. Package Details

### 2.1 `pkg/pipeline` (End-to-End Media Pipeline)

| Target File | Source File(s) | Responsibilities |
|:---|:---|:---|
| `pipeline.go` | `pipeline.go` | Single-file processing state machine: orchestrates audio conversion, transcription, LLM detection, cutting, and export |
| `batch.go` | `batch_proc.go` | Batch orchestration across directories or episode lists, concurrency controls |
| `batch_file.go` | `batch_proc_file.go` | Per-file batch execution, status tracking, temp directory cleanup |
| `dry_run.go` | `batch_dry_run.go` | Dry-run simulation, estimate compute time, tokens, and cut durations |
| `quarantine.go` | `quarantine.go` | Isolating failed or corrupt audio files into quarantine subdirectories |

**Dependencies:**
- Imports `pkg/types`, `pkg/util`, `pkg/config`, `pkg/audio`, `pkg/format`, `pkg/transcribe`, `pkg/detect`, `pkg/gemini`.

### 2.2 `pkg/player` (Audio Playback Daemon & IPC)

| Target File | Source File(s) | Responsibilities |
|:---|:---|:---|
| `controller.go` | `player.go` | Player state machine: play, pause, toggle, seek, volume, speed |
| `ipc.go` | `player_ipc.go` | Unix domain socket server (`/tmp/abs_player.sock`), JSON IPC protocol, daemon lifecycle |
| `mpris.go` | `player_ipc.go` | Linux MPRIS D-Bus interface (`org.mpris.MediaPlayer2.abs`) for system media key integration |
| `sink.go` | `player_sink.go` | Hardware audio output sinks: spawned `mpv` IPC process and `cvlc` fallback |
| `widget.go` | `player_ui.go` | Player status line formatting for CLI display and TUI bottom bar |

**Dependencies:**
- Imports `pkg/types`, `pkg/util`, `pkg/audio`.

---

## 3. Step-by-Step Implementation Plan

### Step 5.1: Create `pkg/player`
1. Move audio sink implementations to `pkg/player/sink.go`.
2. Move player state and controls to `pkg/player/controller.go`.
3. Move IPC socket and MPRIS logic to `pkg/player/ipc.go` and `mpris.go`.
4. Move status rendering widget to `pkg/player/widget.go`.
5. Move tests (`player_test.go`, `player_queue_test.go`, `player_view_race_test.go`) into `pkg/player/`.

### Step 5.2: Create `pkg/pipeline`
1. Move core single-file pipeline orchestration to `pkg/pipeline/pipeline.go`.
2. Move batch processing to `pkg/pipeline/batch.go` and `batch_file.go`.
3. Move dry-run logic to `pkg/pipeline/dry_run.go`.
4. Move quarantine handling to `pkg/pipeline/quarantine.go`.
5. Move tests (`pipeline_test.go`, `batch_dry_run_test.go`, `batch_proc_file_test.go`, `quarantine_test.go`) into `pkg/pipeline/`.

### Step 5.3: Bridge Monolith
1. Update root callers to import `pkg/pipeline` and `pkg/player`.
2. Audit line counts ($\le 80$ lines per function).

---

## 4. Verification & Quality Gates

```bash
# Verify player and pipeline packages
go test -v ./pkg/player/...
go test -v ./pkg/pipeline/...

# Verify root tests
go test -timeout 30s ./...

# Sizing audit
./tools/audit_lines --quiet
```
