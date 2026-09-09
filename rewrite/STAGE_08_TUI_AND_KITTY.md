# Stage 08: Terminal Graphics and Interactive TUI

## 1. Goal & Rationale

Stage 08 extracts the presentation and terminal user interface layer:
- `pkg/kitty`: Standalone Kitty graphics protocol encoder, image resizer, and terminal cell mapping (with pixterm ANSI fallback).
- `pkg/tui`: Full-featured interactive terminal user interface built on Charm's Bubbletea, Bubbles, and Lipgloss, supporting 19 screens/modes, asynchronous background commands, keyboard navigation, and modal dialogs.

Currently spanning 31 root files (~6,500 lines of Go code), this layer is heavily coupled via package-global styles and global models. Encapsulating it into `pkg/tui` with structured `Styles` and model state machines dramatically improves testability and cleans the root package.

---

## 2. Package Details

### 2.1 `pkg/kitty` (Terminal Image Protocol)

| Target File | Source File(s) | Responsibilities |
|:---|:---|:---|
| `detect.go` | `kitty.go` | Terminal emulator detection (`TERM=xterm-kitty`, `KITTY_WINDOW_ID`) |
| `encode.go` | `kitty_encode.go` | Base64 chunking of graphics payloads, generating `\x1b_G...` escape codes |
| `image.go` | `kitty_image.go` | Image decoding, aspect-preserving downsampling, pixterm fallback rendering |

### 2.2 `pkg/tui` (Bubbletea Terminal Interface)

| Target File | Source File(s) | Responsibilities |
|:---|:---|:---|
| `model.go` | `tui.go` | Central `tuiModel` implementing `tea.Model` (`Init`, `Update`, `View`) |
| `styles.go` | `tui_styles.go` | `type Styles struct` encapsulating all Lipgloss styles, themes, and badges |
| `keys.go` | `tui_keys*.go` | Keymap definitions, vim navigation (`j`/`k`/`h`/`l`), search `/`, tab switching |
| `screens.go` | `tui_views.go`, `tui_nav.go` | Screen router, history stack, breadcrumb header, footer status bar |
| `view_list.go` | `tui_list_view.go` | Podcast list screen with cover art previews |
| `view_episode.go` | `tui_episode_view.go` | Episode list screen with AdR badges, duration, and status indicators |
| `view_detail.go` | `tui_detail_view.go` | Detailed episode inspection, metadata, and show notes |
| `view_latest.go` | `tui_latest_view.go` | Unified latest episodes across all subscribed feeds |
| `view_queues.go` | `tui_queues_view.go` | Download and ad removal queue view, reordering, and worker tracking |
| `view_timeline.go` | `tui_timeline_view.go` | Visual audio timeline depicting keep segments vs detected ad intervals |
| `view_transcript.go` | `tui_transcript_view.go` | Interactive transcript viewer with search and subtitle export modal |
| `modals.go` | `tui_modal*.go` | Generic confirmation dialogs, download policy picker, progress bars |
| `data.go` | `tui_data*.go` | Async `tea.Cmd` data loaders bridging to `pkg/podcast` and `pkg/backend` |
| `format.go` | `tui_format.go`, `tui_text_utils.go` | Text wrapping, status badge rendering, and column alignment |

**Global Variable Decoupling:**
- Transform 131 global `lipgloss.Style` variables in `tui_styles.go` into fields on `Styles struct`.
- Pass `*Styles` into all view renderers.

**Dependencies:**
- `pkg/kitty` depends only on `pkg/util`.
- `pkg/tui` depends on `pkg/types`, `pkg/util`, `pkg/config`, `pkg/backend`, `pkg/audio`, `pkg/format`, `pkg/podcast`, `pkg/player`, `pkg/kitty`.

---

## 3. Step-by-Step Implementation Plan

### Step 8.1: Create `pkg/kitty`
1. Move kitty detection, base64 encoding, and image rendering into `pkg/kitty/`.
2. Write unit tests for encoding and dimension scaling.

### Step 8.2: Create `pkg/tui` Foundation
1. Move `tui_types.go` into `pkg/tui/types.go`.
2. Refactor `tui_styles.go` into `type Styles struct` in `pkg/tui/styles.go`.
3. Move keyboard dispatchers (`tui_keys*.go`) into `pkg/tui/keys.go`.

### Step 8.3: Move Screen Views & Data Loaders
1. Move view renderers (`tui_*_view.go`, `tui_modal*.go`) into `pkg/tui/`.
2. Move data commands (`tui_data*.go`) into `pkg/tui/data.go`.
3. Move core model lifecycle (`tui.go`) into `pkg/tui/model.go`.
4. Expose `func RunTUI(cfg *config.Config, pm *podcast.PodcastManager) error`.

### Step 8.4: Migrate Tests & Live Visual Audit
1. Move all 12 TUI test files into `pkg/tui/`.
2. Run live PTY visual audit (`./tools/visual_audit`) to verify all 19 screens snapshot identically.

---

## 4. Verification & Quality Gates

```bash
# Verify kitty and TUI packages
go test -v ./pkg/kitty/...
go test -v ./pkg/tui/...

# Full live PTY visual audit across all 19 screens
./tools/visual_audit

# Verify root tests
go test -timeout 30s ./...

# Sizing audit
./tools/audit_lines --quiet
```
