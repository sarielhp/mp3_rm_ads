# Cloud8 Whisper Service Reference

This document describes the Whisper speech-to-text service running on the `cloud8` Google Cloud VM.

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

The service is fully private and is only accessible through your secure **Tailscale** network.

* **Tailscale Hostname:** `cloud8`
* **Tailscale IP Address:** `100.75.239.72`
* **Port:** `8000`
* **Base API URL:** `http://cloud8:8000`
* **Transcription API Endpoint:** `http://cloud8:8000/v1/audio/transcriptions`
* **Translation API Endpoint:** `http://cloud8:8000/v1/audio/translations`

---

## 3. Basic Operations

### Accessing the VM via SSH
Since Tailscale SSH is configured, you can log directly into the VM from any machine logged into your Tailnet:
```bash
ssh cloud8
```
*(As a backup, you can also connect via the Google Cloud SDK: `gcloud compute ssh cloud8 --zone=us-central1-a --project=vm-on-cloud-sariel`)*

### Managing the Docker Container
Once connected to the `cloud8` VM, use standard Docker commands to manage the container named `whisper`:

* **Check container status:**
  ```bash
  sudo docker ps -f name=whisper
  ```
* **View live transcription logs:**
  ```bash
  sudo docker logs -f whisper
  ```
* **Restart the Whisper server:**
  ```bash
  sudo docker restart whisper
  ```
* **View container configuration details:**
  ```bash
  sudo docker inspect whisper
  ```

---

## 4. API Usage Examples

You can interact with the API directly using `curl` or any OpenAI-compatible client library.

### Check Service Health
```bash
curl -i http://cloud8:8000/health
```
*Expected response:* `HTTP/1.1 200 OK` with body `OK`.

### List Supported Models
```bash
curl -s http://cloud8:8000/v1/models | jq
```

### Transcribe an Audio File (Plain Text Output)
```bash
curl -s -X POST "http://cloud8:8000/v1/audio/transcriptions" \
  -H "Content-Type: multipart/form-data" \
  -F "file=@/path/to/your/audio.wav" \
  -F "model=Systran/faster-whisper-large-v3" \
  -F "response_format=text"
```

### Transcribe and Translate to English (JSON Output)
```bash
curl -s -X POST "http://cloud8:8000/v1/audio/translations" \
  -H "Content-Type: multipart/form-data" \
  -F "file=@/path/to/your/audio.wav" \
  -F "model=Systran/faster-whisper-large-v3"
```

---

## 5. Local Orchestration Script

For automated usage (waking the VM, running transcription, saving files, and putting the VM to sleep), utilize the local control script [`cloud8`](file:///home/sariel/info/misc/26/08/28/gc/cloud8) and refer to [`plan.md`](file:///home/sariel/info/misc/26/08/28/gc/plan.md) for architecture specifics.
