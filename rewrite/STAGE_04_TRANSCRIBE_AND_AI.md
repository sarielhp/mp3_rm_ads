# Stage 04: Transcription and AI Ad Detection Engines

## 1. Goal & Rationale

Stage 04 extracts the speech-to-text transcription engine and the artificial intelligence ad detection providers:
- `pkg/transcribe`: Communicates with `whisper.cpp` servers via HTTP, handles multi-chunk splitting and reassembly, builds PCM WAV headers, and monitors Whisper Docker container progress.
- `pkg/detect`: Constructs ad detection prompts, calls LLM providers (Ollama, OpenRouter), parses JSON array outputs, and runs speculative races between local and cloud models.
- `pkg/gemini`: Google Cloud Vertex AI and Gemini Studio integration, handling audio upload to Cloud Storage and cloud multimodal analysis.

---

## 2. Package Details

### 2.1 `pkg/transcribe` (Whisper Transcription)

| Target File | Source File(s) | Responsibilities |
|:---|:---|:---|
| `client.go` | `transcribe.go` | HTTP POST client for `whisper.cpp` server `/inference` endpoint, parsing `verbose_json` response |
| `chunks.go` | `transcribe_chunks.go` | Splitting oversized audio into overlapping chunks, parallel transcription, seamless word offset stitching |
| `wav.go` | `transcribe_wav.go` | In-memory and streaming 16kHz 16-bit mono WAV header builder |
| `docker.go` | `docker.go`, `whisper_docker.go` | Whisper Docker container detection, log tail polling, and progress percentage computation |

### 2.2 `pkg/detect` (LLM Ad Detection)

| Target File | Source File(s) | Responsibilities |
|:---|:---|:---|
| `detector.go` | `ads.go` | Ad detection coordinator, system prompt generation, JSON response extraction |
| `openrouter.go` | `openrouter.go` | OpenRouter REST API transport, token header management, error retries |
| `race.go` | `speculative_race.go` | Speculative racing: launching local and cloud detection concurrently, picking fastest reliable result |

### 2.3 `pkg/gemini` (Google Cloud Vertex AI)

| Target File | Source File(s) | Responsibilities |
|:---|:---|:---|
| `client.go` | `gemini_client.go` | Google Cloud Vertex AI client initialization and credentials handling |
| `storage.go` | `gemini_storage.go` | GCS audio bucket upload and signed/gs URI generation |
| `pipeline.go` | `gemini_pipeline.go` | Vertex AI multimodal audio ad identification pipeline |
| `studio.go` | `gemini_studio.go` | Gemini Studio prompt export and payload formatting |

**Dependencies:**
- `pkg/transcribe` depends on `pkg/types`, `pkg/util`, `pkg/config`.
- `pkg/detect` depends on `pkg/types`, `pkg/util`, `pkg/config`, `pkg/transcribe`.
- `pkg/gemini` depends on `pkg/types`, `pkg/util`, `pkg/config`.

---

## 3. Step-by-Step Implementation Plan

### Step 4.1: Create `pkg/transcribe`
1. Move Whisper HTTP client into `pkg/transcribe/client.go`.
2. Move chunking logic into `pkg/transcribe/chunks.go`.
3. Move WAV header creation into `pkg/transcribe/wav.go`.
4. Move Docker log polling into `pkg/transcribe/docker.go`.
5. Move tests (`transcribe_test.go`, `transcribe_chunks_test.go`) into `pkg/transcribe/`.

### Step 4.2: Create `pkg/detect`
1. Move LLM prompt and parsing logic into `pkg/detect/detector.go`.
2. Move OpenRouter client into `pkg/detect/openrouter.go`.
3. Move speculative race logic into `pkg/detect/race.go`.
4. Move tests (`speculative_race_test.go`, LLM tests from `misc_test.go`) into `pkg/detect/`.

### Step 4.3: Create `pkg/gemini`
1. Move Vertex AI client and storage handling into `pkg/gemini/`.
2. Move `gemini_test.go` into `pkg/gemini/`.

### Step 4.4: Bridge Monolith
1. Replace root implementations with package imports.
2. Verify all functions stay $\le 80$ lines.

---

## 4. Verification & Quality Gates

```bash
# Verify AI and transcription packages
go test -v ./pkg/transcribe/...
go test -v ./pkg/detect/...
go test -v ./pkg/gemini/...

# Verify root tests
go test -timeout 30s ./...

# Sizing audit
./tools/audit_lines --quiet
```
