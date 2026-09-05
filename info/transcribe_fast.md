# Fast Speech-to-Text Transcription Guide

This document outlines the optimal speed configuration for transcribing podcasts and audio files on an **AMD Radeon RX 6600 XT (8GB VRAM)** and **AMD Ryzen 7 5700G (16-thread CPU)** running Linux.

---

## 1. System Architecture

The setup utilizes a **dual-tier architecture**:

1. **Docker Background Daemon (High-Accuracy Hebrew & English):**
   * **Endpoint:** `http://localhost:8088` (managed via Traefik + Sablier auto-suspend).
   * **Model:** `ivrit-large-v3-turbo` (specialized on thousands of hours of Hebrew speech, with native English accuracy).
   * **Role:** Background service for automated podcast downloaders and ad removal pipelines (`mp3_rm_ads`).
2. **Host CLI (`transcribe`):**
   * **Location:** [`~/.local/bin/transcribe`](file:///home/sariel/.local/bin/transcribe) (native Ruby executable).
   * **Backend:** Native `whisper.cpp` compiled with **Vulkan (`-DGGML_VULKAN=ON`)** and **Flash Attention (`-fa`)**.
   * **Role:** Ad-hoc manual jobs, batch processing, and ultra-fast draft generation for LLM ad detection.

---

## 2. Speed Tiers & Model Selection

All models reside in `/media/dockers/whisper/models/` (symlinked to `~/.local/share/whisper-models/`).

| Tier / Alias | Model File | Language | Speedup (RX 6600 XT) | Time for 30m Audio | Best For |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **`base`** | `ggml-base.en.bin` | English | **~18x–20x real-time** | **~90 seconds** | Ultra-fast English ad detection and quick drafts. |
| **`distil`** | `ggml-distil-large-v3.bin` | English | **~12x–15x real-time** | **~2 minutes** | Near-Large accuracy with pruned 2-layer decoder. |
| **`ivrit`** | `ggml-ivrit-large-v3-turbo.bin` | Hebrew / English | **~6x–8x real-time** | **~4 minutes** | Hebrew speech, slang, names, and mixed Hebrew/English. |
| **`turbo`** | `ggml-large-v3-turbo.bin` | Multilingual (99 langs) | **~6x–8x real-time** | **~4 minutes** | Non-English/non-Hebrew languages (German, French, etc.). |

---

## 3. The `transcribe` CLI Command

The [`transcribe`](file:///home/sariel/.local/bin/transcribe) script automatically applies optimal GPU device flags, thread budgets, greedy decoding, and parallel streams.

### Quick Start Examples

```bash
# 1. Fastest English draft (Base model, auto-parallel, auto-greedy)
transcribe -m base episode.mp3

# 2. High-accuracy Hebrew transcription
transcribe -m ivrit hebrew_podcast.mp3

# 3. Balanced high-quality English (Distil-Whisper)
transcribe -m distil english_podcast.mp3

# 4. Generate Subtitles (SRT or VTT)
transcribe -m base -f srt -o /tmp/subtitles episode.mp3

# 5. Multilingual audio (auto-detects language)
transcribe episode.mp3
```

### CLI Options

```text
Usage: transcribe [options] <audio_file>
    -l, --language LANG              Language (he, en, auto) [default: auto]
    -m, --model MODEL                Model alias (ivrit, distil, turbo, base) or file path
    -f, --format FORMAT              Output format: txt, srt, vtt, json [default: txt]
    -o, --output FILE                Output file path (without extension)
    -p, --processors N               Number of parallel GPU streams [default: 4 for base, 1 for large]
    -t, --threads N                  Threads per processor [default: auto-budgeted to 16 total]
        --greedy                     Force fast greedy decoding (beam_size=1, best_of=1)
        --beam                       Force beam search (beam_size=5)
        --vad                        Enable Silero VAD (best for single-stream jobs)
    -h, --help                       Show help message
```

---

## 4. Technical Optimizations for Maximum Speed

### A. Greedy Decoding (`-bs 1 -bo 1 -nf`)
* **Default `whisper-cli`:** Uses beam search with 5 beams (`-bs 5`). It evaluates 5 candidate hypotheses for every word (5x decoder compute).
* **Optimal Speed:** Using `--greedy` evaluates only the highest-probability token directly and disables temperature fallbacks (`-nf`). 
* **Impact:** Cuts decode time by **~65%** with zero noticeable loss in ad-detection accuracy.

### B. Parallel Stream Chunking (`-p 4`)
* **Default `whisper-cli`:** Processes audio sequentially in a single thread (`-p 1`), which only uses ~25% of the RX 6600 XT's 2,048 stream cores.
* **Optimal Speed:** Specifying `-p 4` slices long audio into 4 continuous chunks and launches 4 concurrent Vulkan compute streams.
* **Impact:** Pushes GPU utilization to **99%** and handles massive marathon episodes efficiently.

### C. The Voice Activity Detection (Silero VAD) Rule
* **VAD Models:** Pre-downloaded at `/media/dockers/whisper/models/ggml-silero-v5.1.2.bin`.
* **Single-Stream Jobs (`-p 1`):** VAD works well to strip long stretches of silence and music (~20% time reduction).
* **Multi-Stream Jobs (`-p 4`):** **Do NOT combine `--vad` with `-p`**. In `whisper.cpp`, `--vad` overrides parallel chunking and serializes the file into 10,000+ tiny 2-second clips, causing severe GPU kernel launch overhead. `transcribe` automatically handles this rule.

### D. CPU Thread Budgeting
* The AMD Ryzen 7 5700G has **16 CPU threads**.
* When running $N$ parallel processors (`-p N`), each processor spawns $T$ worker threads.
* `transcribe` automatically caps $N \times T \le 16$ (e.g., `-p 4 -t 4`), preventing context-switch thrashing.

---

## 5. Direct `whisper-cli` Low-Level Commands

If you ever need to invoke `whisper-cli` directly without the Ruby wrapper:

```bash
# Fastest English run (Base, 4 streams, greedy, Vulkan, Flash Attention)
whisper-cli \
  -m /media/dockers/whisper/models/ggml-base.en.bin \
  -f episode.mp3 \
  -dev 0 \
  -fa \
  -p 4 \
  -t 4 \
  -bs 1 \
  -bo 1 \
  -nf \
  -otxt \
  -of ./output_transcript
```

```bash
# Maximum Hebrew accuracy (Ivrit Large Turbo, single stream)
whisper-cli \
  -m /media/dockers/whisper/models/ggml-ivrit-large-v3-turbo.bin \
  -f hebrew_episode.mp3 \
  -l he \
  -dev 0 \
  -fa \
  -t 8 \
  -otxt \
  -of ./hebrew_transcript
```

---

## 6. Real-World Benchmark Verification

Tested on *Fall of Civilizations Episode 21* (**4 hours, 22 minutes, 11 seconds** of audio):

* **Total Time:** **14 minutes, 30 seconds** (870 seconds).
* **Effective Speed:** **18.1x real-time**.
* **GPU Utilization:** **99%** sustained on AMD Radeon RX 6600 XT.
* **Accuracy:** Transcribed 29,000+ words with complete historical fidelity, proper noun recognition (*Kublai Khan*, *Ptolemy*, *Prambanan*, *Lombok*), and zero repetition loops.
