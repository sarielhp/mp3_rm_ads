# mp3_rm_ads

Automatically detects and removes advertisement, sponsor, and promotional segments from podcast MP3 files using local Whisper transcription and configurable LLM detection.

## Features

- **Local Whisper Transcription**: Uses local Whisper GPU server for fast, offline transcription
- **Configurable LLM Detection**: Supports multiple LLM providers (local Ollama, OpenRouter, etc.)
- **Smart Ad Detection**: Detects host-read sponsor plugs, midroll/preroll ads, promotional breaks
- **Batch Processing**: Process multiple files in sequence
- **SRT/TXT Export**: Export transcripts in multiple formats
- **Recut Mode**: Re-cut audio using existing metadata without re-transcribing

## Workflow

```
1. Transcribe audio → Whisper GPU server
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

# Make executable
chmod +x mp3_rm_ads

# Install dependencies (if needed)
gem install rainbow
```

## Configuration

The program creates a config file at `~/.config/mp3_rm_ads/config.json` on first run.

```json
{
  "whisper_url": "http://localhost:8088/inference",
  "active_profile_id": 1,
  "profiles": [
    {
      "id": 1,
      "name": "Ollama Local (llama3.1:8b)",
      "type": "ollama",
      "url": "http://localhost:11434/v1/chat/completions",
      "model": "llama3.1:8b",
      "api_key": ""
    }
  ]
}
```

## Usage

### Basic Usage

```bash
# Process a single episode
./mp3_rm_ads episode.mp3

# Process multiple episodes
./mp3_rm_ads episode1.mp3 episode2.mp3 episode3.mp3

# Quiet mode
./mp3_rm_ads -q episode.mp3
```

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

## Output Files

- `episode.mp3` → `episode_adfree.mp3` (original preserved as `episode.mp3.precut`)
- `episode.transcript.json` (Whisper's raw output)
- `episode.transcript.txt` (human-readable transcript)
- `episode.srt` (subtitle file)
- `episode.cuts.json` (ad cut metadata)

## Architecture

### Whisper Integration
- Local Whisper GPU server for transcription
- Fast, offline processing
- Configurable URL in config

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

- Ruby 3.0+
- FFmpeg
- Whisper GPU server (local or remote)
- LLM API access (local or remote)

## License

MIT