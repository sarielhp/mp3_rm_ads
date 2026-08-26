# Feature Suggestions for `abs` (Automatic Podcast Ad Remover)

## Current Program Overview

Your program (`abs`) is already quite feature-rich with:
- Full pipeline: Whisper transcription → LLM ad detection → FFmpeg cutting
- Rich TUI: Podcast browser, episode detail, audio player, dual queues (play + ad-removal)
- Audiobookshelf integration: Basic metadata mapping and cover art download
- Multiple output formats: MP3, SRT, TXT, JSON transcript, JSON cuts
- Smart caching: Transcript, cuts, podcast metadata, cover art
- Docker support: Whisper container progress monitoring
- Queue persistence: Both play and ad queues survive sessions

## Audiobookshelf Features Your Program Doesn't Yet Leverage

Audiobookshelf is a full self-hosted podcast server. Key features you're missing:

| ABS Feature | Your Current State |
|---|---|
| Episode download from ABS | Not implemented |
| Upload cleaned MP3s back to ABS | Not implemented |
| Mark episodes as processed in ABS | Not implemented |
| API key auth (v2.26.0+) | Still using user/pass |
| Trigger ABS library scan | Not implemented |
| RSS feed subscription management | Not implemented |
| Auto-download new episodes | Not implemented |
| OPML import/export | Not implemented |

## Suggested Features to Consider

### High Impact / Low Effort:
1. **Upload cleaned MP3s back to ABS** — After ad removal, POST the clean file to ABS so it's available in the server
2. **Mark processed status in ABS** — Set media progress or a custom tag to avoid reprocessing
3. **API key auth** — Migrate from user/pass to JWT API keys (ABS v2.26.0+)

### Medium Impact / Medium Effort:
4. **Download episodes from ABS** — Select episodes in TUI and download them from ABS for processing
5. **Watch directory mode** — Auto-process new MP3s as they appear in `podcasts_dir`
6. **Parallel processing** — Process multiple files concurrently (currently sequential)
7. **Resume interrupted processing** — Save progress mid-pipeline to resume after crash

### High Impact / Higher Effort:
8. **RSS feed generation** — Expose cleaned episodes as a private RSS feed (listen ad-free in any podcast app)
9. **Scheduled processing** — Cron-like auto-processing of new episodes
10. **Community ad patterns** — Share/import ad segment definitions between users
11. **Intro/outro detection** — Detect and optionally remove podcast intro/outro segments (not just ads)

### Nice-to-Have:
12. **Volume normalization** — Apply loudness normalization (EBU R128) to output
13. **Chapter markers** — Write chapter markers for ad segments in output MP3
14. **OPML import** — Import podcast subscriptions from other apps

## Integration Points with Existing ABS Features

Your current code already has:
- `testAudiobookshelfServer` -- Validates ABS connection
- `absMapPodcasts` -- Lists podcasts from ABS, maps MP3 files to episode metadata
- `absDownloadAllData` -- Downloads all podcast metadata from ABS
- Config: Stores `audiobookshelf_url`, `audiobookshelf_user`, `audiobookshelf_pass`

The most valuable ABS integrations would be:
1. **Reading episode lists** (already done) -- to know what to process
2. **Uploading cleaned files** -- to put results back into the ABS ecosystem
3. **Progress/status tracking** -- to avoid reprocessing