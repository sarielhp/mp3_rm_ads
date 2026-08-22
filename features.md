# Missing Features Analysis (from Absotui)

Based on analysis of [Absotui](https://github.com/pdwaldrop/Absotui) — a full Audiobookshelf TUI client in Rust.

## Features Relevant to mp3_rm_ads TUI

### 1. Cover Art / Graphics on Left Side of Episode Info
- **Absotui**: 30-column left area for cover art (`Constraint::Length(30)`), 3-column gap, then description text. Uses Kitty/Sixel/iTerm2 image protocols via `ratatui-image`.
- **mp3_rm_ads**: Cover art is displayed at the **bottom** of episode detail view only. No cover art in podcast detail list view.
- **Fix needed**: Display cover art on the left side of episode list in `drawPodcastDetail()`.

### 2. Inline Search / Filter
- **Absotui**: Press `/` to open search input overlay at bottom. Live filtering of all parallel arrays (titles, authors, IDs, descriptions, durations, progress). Supports both books and podcasts.
- **mp3_rm_ads**: No search or filter capability at all.

### 3. Sort Toggle (Newest/Oldest)
- **Absotui**: `D` key toggles newest-first vs oldest-first sort for podcast episodes.
- **mp3_rm_ads**: Episodes are sorted by modTime descending only. No toggle.

### 4. Key Binding Hints Toggle
- **Absotui**: `B` key toggles display of key binding hints in player bar.
- **mp3_rm_ads**: Help text is always shown. No toggle.

### 5. Marquee / Auto-Scroll Truncated Titles
- **Absotui**: Selected row with truncated title auto-scrolls horizontally (300ms per tick, pauses at start/end).
- **mp3_rm_ads**: Titles are truncated with `...` but no auto-scroll.

### 6. HTML Description Rendering
- **Absotui**: Parses `<b>`, `<strong>`, `<i>`, `<em>`, `<br>`, `<p>`, `<li>`, `<div>` for styled spans and line breaks. Decodes HTML entities.
- **mp3_rm_ads**: Uses `stripHTML()` which just removes all tags. No formatting preserved.

### 7. Relative Age Display
- **Absotui**: "Today", "1Day", "2Weeks", "3Months", "1Year" for podcast episode age.
- **mp3_rm_ads**: No age display at all.

### 8. Progress / Status Indicators
- **Absotui**: Progress bars with text underline fill, now-playing markers, download status indicators.
- **mp3_rm_ads**: Basic checkmark for ad-free, [Q] badge for queued. No progress bars.

### 9. Pop-up / Status Messages
- **Absotui**: "Loading...", "Downloading...", "Exiting..." overlay messages.
- **mp3_rm_ads**: No overlay/pop-up messages.

### 10. Context-Sensitive Footer Help
- **Absotui**: Dynamic footer per screen (Home, Library, Settings, etc.) with reusable fragments.
- **mp3_rm_ads**: Static help text per screen.

### 11. Error Recovery Screen
- **Absotui**: Connection-error screen with Retry/Change Address/Quit options.
- **mp3_rm_ads**: Error displayed as text, no recovery options.

### 12. Configurable Color Scheme
- **Absotui**: 14 configurable colors in `config.toml` (background, header, list, selection, progress bar, etc.).
- **mp3_rm_ads**: Hardcoded colors in `tui_styles.go`.

### 13. Terminal Window Title Sync
- **Absotui**: Sets terminal title to currently-playing item name.
- **mp3_rm_ads**: No terminal title management.

### 14. Scroll Wheel Support
- **Absotui**: Disables/restores terminal scroll wheel on startup/exit.
- **mp3_rm_ads**: No scroll wheel handling.

### 15. File Size Formatting (B/KB/MB/GB)
- **Absotui**: Human-readable file sizes with B/KB/MB/GB.
- **mp3_rm_ads**: Has `formatFileSize` but only goes up to MB.

## Features NOT Relevant to mp3_rm_ads

- VLC-based playback engine (mp3_rm_ads is a batch processor, not a player)
- Server progress syncing (no Audiobookshelf playback)
- Offline download/playback (not a media player)
- Podcast autoplay (not a player)
- Chapter support (not a player)
- Multi-track books (not a player)
- Per-item speed rate (not a player)
- In-app update/uninstall (different distribution model)
- Full settings UI with 7 screens (overkill for batch tool)
- Multiple libraries (single-purpose tool)
- SQLite persistent state (not needed)
- Multiple user support (single-user tool)
- Token encryption (not needed)
- Background data fetching (not needed)
- Concurrent API fan-out (not needed)