# Implementation Plan

## Phase 1: Fix Cover Art Display (Bug Fix)

### Step 1: Fix cover art display on left side of episode info in `drawPodcastDetail()`
- Modify `drawPodcastDetail()` to show cover art on the left side of the episode list
- Use Kitty graphics protocol to render the cover image
- Properly size the image for terminal display (max 30 columns wide)
- Add a `showCover` toggle for the podcast detail view
- **Test**: Verify cover art appears in podcast detail view when Kitty is supported

## Phase 2: New Features (in priority order)

### Step 2: Add Search/Filter (`/` key)
- Add inline search bar at bottom of screen
- Filter podcast list and episode list by name/title
- `/` to open search, Enter to submit, Esc to cancel
- **Test**: Verify search filters podcast list correctly

### Step 3: Add Sort Toggle (`D` key)
- Add `D` key to toggle newest-first / oldest-first sort
- Works in podcast detail view for episodes
- **Test**: Verify sort order changes correctly

### Step 4: Add Key Binding Hints Toggle (`B` key)
- Add `B` key to toggle help text visibility
- Persist preference in model
- **Test**: Verify help text toggles on/off

### Step 5: Add Relative Age Display
- Add `formatRelativeAge()` function
- Show age in episode list (Today, 1Day, 1Week, etc.)
- **Test**: Verify age formatting for various time ranges

### Step 6: Add HTML Description Rendering
- Enhance `stripHTML()` to preserve bold/italic/line breaks
- Parse `<b>`, `<i>`, `<br>`, `<p>`, `<li>` tags
- **Test**: Verify HTML rendering preserves formatting

### Step 7: Add Pop-up Status Messages
- Add overlay message system for loading/saving states
- Show "Saving queue...", "Loading..." messages
- **Test**: Verify pop-up messages display correctly

### Step 8: Add Context-Sensitive Footer Help
- Dynamic footer per screen with reusable fragments
- Show relevant key bindings for current screen
- **Test**: Verify footer changes per screen

### Step 9: Add Marquee Title Scrolling
- Auto-scroll truncated titles when selected
- Timer-based scrolling with pause at start/end
- **Test**: Verify title scrolling works

### Step 10: Add Configurable Color Scheme
- Add color configuration to config.json
- Allow overriding TUI colors via config
- **Test**: Verify colors load from config

### Step 11: Add Error Recovery Screen
- Show retry options on connection errors
- Retry, change directory, quit options
- **Test**: Verify error recovery screen appears on failure

### Step 12: Add Terminal Window Title Sync
- Set terminal title to current podcast/episode name
- **Test**: Verify terminal title changes

### Step 13: Add Scroll Wheel Support
- Disable terminal scroll wheel during TUI
- Restore on exit
- **Test**: Verify scroll wheel is disabled/enabled

### Step 14: Enhance File Size Formatting (GB support)
- Add GB support to `formatFileSize()`
- **Test**: Verify GB formatting

## Implementation Order

Each step will be:
1. Implement the feature
2. Write tests for the feature
3. Run `make check` to verify
4. Commit with conventional commit message
5. Push to GitHub

## Verification

After each step:
- `make check` (format → tidy → vet → staticcheck → test → build)
- Verify all existing tests still pass
- Verify new tests pass