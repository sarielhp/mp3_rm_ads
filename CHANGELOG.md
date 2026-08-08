# Changelog

All notable changes to mp3_rm_ads will be documented in this file.

## [Unreleased]

### Added
- Initial release with core functionality

### Features
- Automatic ad detection using Whisper + LLM
- Batch processing support
- SRT/TXT transcript export
- Recut mode for re-processing with existing metadata
- Configurable LLM profiles (Ollama, OpenRouter, etc.)
- Interval merging for adjacent ad segments
- Skip processing if output files exist
- Force options for re-transcribe and re-detection

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