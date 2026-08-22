# Changelog

All notable changes to abs will be documented in this file.

## [0.1.7] - 2026-08-13

### Added
- Display `Scanning: <Directory>` status output when starting folder scan in default mode and `dir` mode
- Dedicated user temporary directory `/tmp/$USER/abs/` created on demand for temporary log files and fallbacks

## [Unreleased]

### Added
- Dynamic `read_timeout` based on audio duration (fixes timeout on long files)
- Chunked transcription: split long audio into overlapping 10-minute chunks (`--use-chunks`)
- Parallel chunk transcription support (`parallel_chunks` config)
- Auto-detection of whisper Docker container for progress monitoring
- Real-time progress/ETA from Docker logs during transcription
- Auto-fallback to chunked transcription when whisper fails to decode/encode
- Directory argument support: recursively finds and processes all `.mp3` files
- `--use-chunks` CLI flag to enable chunked transcription
- `whisper_docker_container` config option for manual container specification
- `chunk_duration_sec` and `parallel_chunks` config options
- Error context display from Docker logs (5 lines before/after failure)
- Local IP auto-detection for default config file generation

### Changed
- Default config now uses local machine IP instead of hardcoded address
- Temp files stored in `.work/` directory alongside audio files
- Chunk merge logic uses overlap midpoint for clean deduplication
- Progress thread polls every 2 seconds instead of 0.5
- `Open3.capture3` used for Docker log fetching (fixes stderr leak)

### Fixed
- Whisper server timeout on files longer than ~70 minutes
- Docker log output leaking to terminal (stderr vs stdout issue)
- Redundant retries on whisper decode/encode failures
- `.work/` directory cleanup on all exit paths (success, failure, early return)

## [1.0.0] - 2026-08-07

### Added
- Initial release
- Whisper GPU transcription integration
- LLM-based ad detection
- Audio cutting with ffmpeg
- Multiple output formats (JSON, SRT, TXT)
- Configurable profiles
- Batch processing
- Quiet mode
- Transcript export options
- Recut mode
- Force options (--force-llm, --force-transcribe)
- Interval merging algorithm
- Skip logic for existing files

### Technical Details
- Ruby implementation
- Rainbow gem for colored output
- Open3 for subprocess management
- Net::HTTP for API calls
- FFmpeg for audio processing