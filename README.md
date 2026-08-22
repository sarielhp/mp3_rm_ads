# mp3_rm_ads

Automatically detects and removes advertisement, sponsor, and promotional segments from podcast MP3 files using local Whisper transcription and configurable LLM detection.

## Features

- **Local Whisper Transcription**: Uses local Whisper GPU server for fast, offline transcription
- **Configurable LLM Detection**: Supports multiple LLM providers (local Ollama, OpenRouter, etc.)
- **Smart Ad Detection**: Detects host-read sponsor plugs, midroll/preroll ads, promotional breaks
- **Batch Processing**: Process multiple files in sequence, or pass a directory to process all MP3s
- **SRT/TXT Export**: Export transcripts in multiple formats
- **Recut Mode**: Re-cut audio using existing metadata without re-transcribing
- **Chunked Transcription**: Split long audio into overlapping chunks for reliability (`--use-chunks`)
- **Docker Progress Monitoring**: Auto-detects local whisper Docker container and shows real progress/ETA
- **Auto-fallback**: Detects whisper decode failures and automatically retries with chunked transcription
- **Interactive TUI**: Browse podcasts, queue episodes, and track processing status
- **Audiobookshelf Integration**: Map podcast directories and check ABS server status

## Workflow

```
1. Transcribe audio → Whisper GPU server (HTTP API)
2. Detect ads → LLM (local or remote)
3. Merge intervals → Combine adjacent ad segments
4. Cut audio → ffmpeg
5. Save metadata → .cuts.json
```

## Installation

```bash
# Clone the repository
git clone https://github.com/yourusername/mp3_rm_ads.git
cd mp3_rm_ads

# Build the Go binary
go build -o mp3_rm_ads .

# Or use the Makefile
make build
```

## Configuration

The program creates a config file at `~/.config/mp3_rm_ads/config.json` on first run, using the local machine's IP address automatically.

```json
{
  "whisper_url": "http://<local_ip>:8088/inference",
  "whisper_speed_factor": 7.0,
  "whisper_docker_container": "",
  "chunk_duration_sec": 0,
  "parallel_chunks": 1,
  "active_profile_id": 1,
  "podcasts_dir": "",
  "whisper_language": "",
  "whisper_prompt": "",
  "profiles": [
    {
      "id": 1,
      "name": "Ollama Local (llama3.1:8b)",
      "type": "ollama",
      "url": "http://<local_ip>:11434/v1/chat/completions",
      "model": "llama3.1:8b",
      "api_key": ""
    }
  ],
  "audiobookshelf_url": "",
  "audiobookshelf_user": "",
  "audiobookshelf_pass": ""
}
```

### Config Options

| Option | Default | Description |
|--------|---------|-------------|
| `whisper_url` | Auto-detected | Whisper server HTTP endpoint |
| `whisper_speed_factor` | 7.0 | Estimated transcription speed multiplier (for ETA display) |
| `whisper_docker_container` | Auto-detected | Docker container name for progress polling |
| `chunk_duration_sec` | 0 (disabled) | Split audio into chunks of this duration (seconds) |
| `parallel_chunks` | 1 | Number of chunks to transcribe in parallel |
| `podcasts_dir` | "" | Default directory for podcast processing |
| `whisper_language` | "" | Language override for Whisper transcription |
| `whisper_prompt` | "" | Prompt to prepend to Whisper transcription |
| `audiobookshelf_url` | "" | Audiobookshelf server URL |
| `audiobookshelf_user` | "" | Audiobookshelf username |
| `audiobookshelf_pass` | "" | Audiobookshelf password |

## Usage

### Basic Usage

```bash
# Process a single episode
./mp3_rm_ads episode.mp3

# Process all MP3s in a directory
./mp3_rm_ads /path/to/podcasts/

# Process multiple episodes
./mp3_rm_ads episode1.mp3 episode2.mp3 episode3.mp3

# Quiet mode
./mp3_rm_ads -q episode.mp3
```

### Subcommands

```bash
# Process all MP3s in a directory
./mp3_rm_ads dir /path/to/podcasts/

# Process a single file
./mp3_rm_ads file episode.mp3

# Interactive TUI browser
./mp3_rm_ads tui [path/to/podcasts/dir]

# Configuration management
./mp3_rm_ads config

# Test Whisper server
./mp3_rm_ads test --test-whisper

# Test Audiobookshelf server
./mp3_rm_ads test --test-abs

# Map Audiobookshelf podcasts
./mp3_rm_ads test --abs-map
```

### Chunked Transcription

For long files that whisper fails to decode, use chunked transcription:

```bash
# Enable chunking (10-minute chunks with 30s overlap)
./mp3_rm_ads --use-chunks episode.mp3

# Or enable permanently in config:
# "chunk_duration_sec": 600
```

The script also auto-detects whisper decode failures and falls back to chunking automatically.

### Export Options

```bash
# Export to SRT format
./mp3_rm_ads --srt episode.mp3

# Export to TXT format
./mp3_rm_ads --txt episode.mp3

# Export both
./mp3_rm_ads --srt --txt episode.mp3
```

### Recut Mode

```bash
# Re-cut using existing .cuts.json
./mp3_rm_ads --recut episode.mp3
```

### Force Options

```bash
# Force re-transcribe
./mp3_rm_ads --force-transcribe episode.mp3

# Force re-run LLM detection
./mp3_rm_ads --force-llm episode.mp3

# Force both
./mp3_rm_ads --force-transcribe --force-llm episode.mp3
```

### LLM Profiles

```bash
# List available profiles
./mp3_rm_ads --list-llms

# Use specific profile
./mp3_rm_ads --use-llm 2 episode.mp3

# Set default profile
./mp3_rm_ads --set-default 2
```

### Audiobookshelf Integration

```bash
# Set Audiobookshelf server details
./mp3_rm_ads --set-abs --abs-url http://localhost:8080 --abs-user admin --abs-pass password

# Test connection to Audiobookshelf
./mp3_rm_ads --test-abs

# Map podcast directories
./mp3_rm_ads --abs-map
```

## Output Files

- `episode.mp3` → `episode_adfree.mp3` (original preserved as `episode.mp3.precut`)
- `episode.transcript.json` (Whisper's raw output)
- `episode.transcript.txt` (human-readable transcript)
- `episode.srt` (subtitle file)
- `episode.cuts.json` (ad cut metadata)
- `.work/` (temporary chunk files, auto-cleaned)

## Architecture

### Whisper Integration
- Local Whisper GPU server for transcription via HTTP API
- Docker log polling for real-time progress and error detection
- Auto-detects local whisper container by image name or exposed port
- Falls back to chunked transcription on decode failures

### LLM Integration
- Supports multiple providers:
  - Local Ollama
  - OpenRouter (Claude, DeepSeek, Gemini, etc.)
  - Any OpenAI-compatible API
- Configurable profiles with pricing info

### Ad Detection Logic
- Analyzes transcript for ad patterns
- Merges adjacent intervals with small gaps
- Configurable gap tolerance based on segment duration

## Requirements

- Go 1.26+
- FFmpeg
- Whisper GPU server (local or remote)
- LLM API access (local or remote)
- Docker (optional, for progress monitoring)

## License

MIT