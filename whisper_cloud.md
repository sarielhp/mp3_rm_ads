# Remote Whisper Service Reference

This document describes the Whisper speech-to-text service running on a remote VM or server.

---

## 1. What Exactly is the Service?

The transcription service runs inside a Docker container configured to run on startup. It is an OpenAI-compatible API server wrapping the high-performance **`faster-whisper`** engine.

* **Docker Image:** `fedirz/faster-whisper-server:latest-cpu`
* **Underlying Engine:** `faster-whisper` (version `1.0.3`) utilizing **CTranslate2** for optimized CPU inference.
* **API Server Wrapper:** `faster-whisper-server` (version `0.1.0`)
* **Default Model:** `Systran/faster-whisper-large-v3` (1.55 Billion parameters, loaded with `int8` quantization).
* **Optimization Settings:**
  * Runs on CPU with **8 threads** (`WHISPER_CPU_THREADS=8`).
  * Spawns **2 inference workers** (`WHISPER_NUM_WORKERS=2`).
  * Voice Activity Detection (VAD) filter is **enabled** (`WHISPER_VAD_FILTER=true`) to filter out silence and background noise.

---

## 2. Network & Access

The service is fully private and is accessible through your secure network or VPN.

* **Remote Host:** `<remote-host>`
* **Port:** `8000`
* **Base API URL:** `http://<remote-host>:8000`
* **Transcription API Endpoint:** `http://<remote-host>:8000/v1/audio/transcriptions`
* **Translation API Endpoint:** `http://<remote-host>:8000/v1/audio/translations`

---

## 3. Basic Operations

### Connect to Remote Host
```bash
ssh <remote-host>
```

Once connected to the VM, use standard Docker commands to manage the container named `whisper`:

#### Check Container Status
```bash
docker ps -f name=whisper
```

#### View Live Logs
```bash
docker logs -f whisper
```

#### Restart Service
```bash
docker restart whisper
```

#### Stop / Start Container
```bash
docker stop whisper
docker start whisper
```

---

## 4. API Testing & Usage Examples

### Health Check
```bash
curl -i http://<remote-host>:8000/health
```
*(Should return HTTP `200 OK`)*

### Verify Loaded Models
```bash
curl -s http://<remote-host>:8000/v1/models | jq
```

### Run Transcription via API
```bash
curl -s -X POST "http://<remote-host>:8000/v1/audio/transcriptions" \
     -H "Content-Type: multipart/form-data" \
     -F file="@/path/to/audio.mp3" \
     -F model="Systran/faster-whisper-large-v3" \
     -F language="en" \
     -F response_format="verbose_json"
```

### Run Translation via API
```bash
curl -s -X POST "http://<remote-host>:8000/v1/audio/translations" \
     -H "Content-Type: multipart/form-data" \
     -F file="@/path/to/audio.mp3" \
     -F model="Systran/faster-whisper-large-v3" \
     -F response_format="verbose_json"
```

---

## 5. Automation Integration

For automated usage (waking the VM, running transcription, saving files, and putting the VM to sleep), configure your wake command and remote host in `config.json`.
