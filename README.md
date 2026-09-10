# abs

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
git clone https://github.com/yourusername/abs.git
cd abs

# Build the Go binary
go build -o abs .

# Or use the Makefile
make build
```

## Configuration

The program creates a config file at `~/.config/abs/config.json` on first run, using the local machine's IP address automatically.

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
  "audiobookshelf_pass": "",
  "backend_type": "audiobookshelf",
  "podfetch_url": "",
  "podfetch_user": "",
  "podfetch_pass": "",
  "podfetch_db_path": ""
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
| `backend_type` | "audiobookshelf" | Backend provider: `"audiobookshelf"` or `"podfetch"` |
| `audiobookshelf_url` | "" | Audiobookshelf server URL |
| `audiobookshelf_user` | "" | Audiobookshelf username |
| `audiobookshelf_pass` | "" | Audiobookshelf password |
| `podfetch_url` | "" | PodFetch server URL |
| `podfetch_user` | "" | PodFetch username |
| `podfetch_pass` | "" | PodFetch password |
| `podfetch_db_path` | "" | Path to PodFetch SQLite database file |

## Podcast Server Backends

`abs` supports two podcast server backends for metadata sync, episode downloads, and feed management:

### 1. Audiobookshelf (Default)
Integrates with Audiobookshelf podcast libraries via REST API and direct SQLite database access.
- Config keys: `audiobookshelf_url`, `audiobookshelf_user`, `audiobookshelf_pass`, `audiobookshelf_sqlite_db_path`
- Env overrides: `ABS_URL`, `ABS_USER`, `ABS_PASS`

### 2. PodFetch
Full-featured alternative backend (`pkg/backend/podfetch*.go`) communicating via PodFetch REST API and GORM/SQLite database.
- Config keys: `backend_type: "podfetch"`, `podfetch_url`, `podfetch_user`, `podfetch_pass`, `podfetch_db_path`
- Env overrides: `PODFETCH_URL`, `PODFETCH_USER`, `PODFETCH_PASS`, `PODFETCH_DB_PATH`

## Ad Removal Terminology (AdR)

The codebase and CLI standardize on concise **AdR** (Ad Removal) terminology:
- **`✂ NeedAdR`**: Episode has not yet undergone ad removal.
- **`✓ Ad-Free`**: Commercial segments have been detected and cut.
- **`AdR Policy`**: Per-podcast ad removal setting (`none`, `latest`, `all`).
- **`AdR Queue`**: Priority queue of episodes pending commercial excision.

## Background Audio Player (`abs player`)

Headless audio playback daemon with MPRIS D-Bus integration:
```bash
abs player play [id]     # Play episode by ID (eXXXXX) or resume playback
abs player pause         # Toggle playback pause state
abs player stop          # Stop playback and shutdown daemon
abs player status        # Display current track, position, duration, and status
```
Spawns a detached background player (prefers headless `mpv` with `--input-ipc-server=/tmp/abs_player.sock`, with graceful fallback to `cvlc --control dbus` or built-in players). Player state reflects in real time inside the interactive TUI.

## Usage

### Basic Usage

```bash
# Process audio files or directories for ad removal
abs rm_ads episode.mp3
abs rm_ads /path/to/podcasts/
abs rm_ads recut episode.mp3
abs rm_ads export srt episode.transcript.json

# Library query and inspection (absorbs ls, transcript, and status diagnostics)
abs info                    # List all podcasts in library
abs info latest 10          # List latest 10 episodes across library
abs info p0001              # Display podcast metadata and episode list
abs info e12345             # Display episode cuts and metadata
abs info e12345 --cuts      # Show detailed cuts breakdown
abs info e12345 --transcript # Display transcript text
abs info transcript e12345  # Read transcript in $PAGER or the system pager
abs info e12345 --export srt # Export transcript to SRT
abs info status             # Show library summary and worker status
abs info check              # Test external services (Whisper, ABS, Kitty)

# Podcast sync & server operations (absorbs fetch, server, policy)
abs sync                    # Scan library for podcasts and new episodes
abs sync feeds              # Fetch latest RSS feeds
abs server download         # Download pending episodes according to podcast policy
abs server download p0001 -k 3 # Download up to 3 missing episodes for a podcast
abs server flush p0001 --dry-run # Preview removing this podcast's audio, keeping transcripts
abs server flush p0001       # Remove MP3/precut audio and disable automatic downloads
abs server publication-sync --dry-run # Preview correcting local publication metadata from the source catalog
abs server publication-sync # Correct cached/status dates; unknown source dates stay unknown
abs sync prune              # Prune old episodes per retention policy
abs sync policy p0001       # View or update download/AdR policy
abs sync timeline           # Display online availability timestamps table

# Manage AdR queue
abs queue list
abs queue add e12345
abs queue today             # Queue downloaded, uncleaned episodes published today (local date)
abs queue priority tdbwg 8   # Persist podcast priority (0–10; default 0)
abs queue priority tdbwg     # Show the podcast's priority
abs rm_ads e79636            # Queue this episode at priority 10 and process it immediately
abs queue remove e12345
abs queue clear

# Background playback
abs player play e12345
abs player pause
abs player status
abs player stop

# Remote processing cluster offload
abs offload status
abs offload push
abs offload pull
abs offload worker

# Configuration
abs config show
abs config get <key>
abs config set <key> <value>

# Interactive TUI browser
abs tui
```

### Commands Overview

Every canonical command begins with a distinct letter (`c`, `i`, `o`, `p`, `q`, `r`, `s`, `t`), enabling unambiguous single-letter prefixes:

| Command | Prefix | Usage | Description |
|---------|--------|-------|-------------|
| `config` | `c` | `abs config [command]` | View and manage application configuration, profiles, and cache |
| `info` | `i` | `abs info [options] [id\|latest [N]\|status\|check]` | Library query, inspection, cuts breakdown, transcripts, and status diagnostics |
| `offload` | `o` | `abs offload [command]` | Manage remote cluster batch processing and worker orchestration |
| `player` | `p` | `abs player [command]` | Control background audio playback (`play`, `stop`, `pause`, `status`) |
| `queue` | `q` | `abs queue [command]` | Manage the ad removal (AdR) processing queue (`list`, `add`, `remove`, `clear`) |
| `rm_ads` | `r` | `abs rm_ads [command] [paths...]` | Process audio files for ad removal (`recut`, `export`, `collect`, `clear`) |
| `sync` | `s` | `abs sync [command] [options]` | Podcast RSS feed sync, episode downloads, and retention policies |
| `tui` | `t` | `abs tui [directory]` | Interactive TUI browser for podcasts and episodes |

### Chunked Transcription

For long files that whisper fails to decode, use chunked transcription:

```bash
# Enable chunking (10-minute chunks with 30s overlap)
abs rm_ads --use-chunks episode.mp3

# Or enable permanently in config:
# "chunk_duration_sec": 600
```

The script also auto-detects whisper decode failures and falls back to chunking automatically.

### Export Options

```bash
# Export transcript to SRT or plain text format
abs rm_ads export srt episode.transcript.json
abs rm_ads export txt episode.transcript.json

# Or export via info command
abs info e12345 --export srt
abs info e12345 --export txt
```

### Recut Mode

```bash
# Re-cut using existing .cuts.json metadata
abs rm_ads recut episode.mp3
```

### Force Options

```bash
# Force re-transcribe
abs rm_ads -f whisper episode.mp3

# Force re-run LLM detection
abs rm_ads -f llm episode.mp3

# Force all pipeline stages
abs rm_ads -f all episode.mp3
```

### LLM Profiles

```bash
# List available profiles
abs config llm list

# Use specific profile
abs rm_ads --profile 2 episode.mp3

# Set default profile
abs config llm default 2
```

### External Service Diagnostics

```bash
# Test connection to Whisper server
abs info check whisper

# Test connection to Audiobookshelf
abs info check abs

# Map podcast directories with Audiobookshelf metadata
abs info check abs map
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
