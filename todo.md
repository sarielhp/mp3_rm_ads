# Todo List — Rich ABS Info, Kitty Image Display & Colorful UI

## 1. ABS Episode & Podcast Metadata (Core Bug Fix & Rich Data)
- [x] **Root cause identified**: ABS `/api/libraries/:id/items` only returns high-level item summaries without `media.episodes`. Full episode list and metadata are at `/api/items/:id`. In addition, authentication requires `accessToken` (JWT) rather than legacy `token`.
- [x] **Extend ABS data models**: Updated `absItem`, `absEpisode`, and `absAudioFile` in `abs.go` to support rich fields (`subtitle`, `episodeType`, `publishedAt`, `bitRate`, `codec`, `genres`, `feedUrl`, `imageUrl`, etc.).
- [x] **Fix ABS data fetching & matching**: Updated `loadTUIPodcastsABS` in `tui.go` to fetch detailed `/api/items/:id` for each podcast and accurately match episodes by filename/title.
- [x] **Expose all rich episode metadata in Episode Detail View**:
  - Season/Episode number, episode type (`full`, `bonus`, `trailer`)
  - Subtitle and formatted publication date/time
  - Formatted duration (from ABS metadata / audio file)
  - Audio technical specs (bitrate, codec, channels/layout, file size)
  - Cleaned & wrapped description text
  - Podcast feed URL, genres, author/publisher

## 2. Kitty Graphics Image Protocol Support (Display Podcast Cover Art)
- [x] **Kitty image rendering helper**:
  - Implement native Kitty graphics protocol encoder (`<ESC>_G...<ESC>\` escape codes) with chunked base64 payload transfer.
  - Detect Kitty terminal support (`$TERM == "xterm-kitty"` or `$KITTY_WINDOW_ID` present).
  - Find cover image associated with podcast (either local `cover.jpg`/`cover.png` in podcast folder or downloaded ABS cover).
  - Add Kitty image display support to the TUI (or an inline overlay/detail action) ensuring clean placement and clearing when switching views.

## 3. Vibrant & Colorful Modern TUI Interface Redesign
- [x] **Color scheme & theme enhancements**:
  - Replace plain monochrome/basic styles with a rich modern palette (cyberpunk/nord/catppuccin inspired gradients and highlights: vibrant cyan, violet, magenta, emerald green, amber yellow).
  - Styled headers, badges, and status pills (e.g., `[✓ Ad-Free]`, `[⏳ Queued]`, `[Full Episode]`, `[Season 1 • Ep 42]`).
  - Distinctive visual separation with styled borders/boxes, progress indicators, and metadata cards.
- [x] **Podcast List View polish**:
  - Add colored badges for episode counts, ad-free stats, author tags, and progress meters.
- [x] **Podcast Detail (Episode List) View polish**:
  - Highlight publication dates, episode numbers, duration pills, and queue indicators with distinct color accents.
- [x] **Episode Detail View polish**:
  - Multi-section card layout: Header, Audio Information, ABS Rich Metadata, and Description.

## 4. Verification & Testing
- [x] Update and run unit tests (`tui_test.go`, etc.) to match new models and UI outputs.
- [x] Run full project quality checks (`make check`).
