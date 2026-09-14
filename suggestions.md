# Feature Suggestions for `abs` (Automatic Podcast Ad Remover)

## Current Program Overview

`abs` is a standalone, feature-rich podcast manager and ad removal system:
- Full pipeline: Whisper transcription → LLM ad detection → FFmpeg cutting
- Rich TUI: Podcast browser, episode detail, audio player, dual queues (play + ad-removal)
- Standalone Backend: Native subscription management, upstream feed checking, native downloading, RSS and HTML5 web player generation
- Multiple output formats: MP3, SRT, TXT, JSON transcript, JSON cuts
- Smart caching: Transcript, cuts, podcast metadata, cover art
- Docker support: Whisper container progress monitoring
- Queue persistence: Both play and ad queues survive sessions

## Suggested Features to Consider

### High Impact / Low Effort:
1. **Watch directory mode** — Auto-process new MP3s as they appear in `podcasts_dir`
2. **Resume interrupted processing** — Save progress mid-pipeline to resume after crash
3. **Smart tagging** — Custom metadata tags to organize favorite episodes

### Medium Impact / Medium Effort:
4. **Parallel batch processing** — Process multiple files concurrently across CPU cores
5. **Scheduled processing** — Cron-like auto-processing of new episodes
6. **Volume normalization** — Apply loudness normalization (EBU R128) to output

### High Impact / Higher Effort:
7. **Intro/outro detection** — Detect and optionally remove podcast intro/outro segments (not just ads)
8. **Community ad patterns** — Share/import ad segment definitions between users
9. **Chapter markers** — Write chapter markers for ad segments in output MP3