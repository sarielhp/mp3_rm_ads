# Code Review 008 - Fix Report

## Overview
This document details the fixes applied as a result of the deep code review in `reports/code_review_008.md`.

## 1. Token Caching & Auto-Refresh (Audiobookshelf Backend)
- **File Modified**: `pkg/backend/audiobookshelf.go`
- **Issue**: The `Request()` function did not handle token expiry and immediately failed on HTTP 401 Unauthorized errors.
- **Fix Applied**: Patched `Request()` to detect HTTP 401 (Unauthorized) status codes. If credentials are set, it automatically invokes `c.Login()` to refresh the cached token, and if successful, retries the request using the existing `continue` loop. This resolves issues with long-running sessions facing sudden token expiration.

## 2. CLI Parameter Validation Fixes
- **File Modified**: `cli_server_cmds_extra.go`
- **Issue**: The `abs server opml import` and `abs server opml export` subcommands lacked argument validators, which could lead to missing file arguments unhandled by `clihelp`. 
- **Fix Applied**: Added `Args: clihelp.ExactArgs(1)` to both `import` and `export` subcommand definitions. This securely binds the required `<file>` parameter during argument parsing, natively rendering usage errors if omitted and ensuring `len(ctx.Args) > 0` assumptions are always valid.

## 3. File Sizing Improvements (TUI Audit)
- **Files Modified/Created**: `tui_transcript_view.go`, `tui_transcript_data.go`
- **Issue**: `tui_transcript_view.go` had grown to 455 lines, exceeding the 150-300 lines recommended length.
- **Fix Applied**: Split the transcript data processing functions (`loadEpisodeAdIntervals`, `loadEpisodeTranscriptData`, etc.) into a new `tui_transcript_data.go` file. `tui_transcript_view.go` was reduced to 326 lines (focused exclusively on drawing and UI events), keeping both components much closer to the target bounds.

## Verification
- Executed `make check` full quality gate (formatting, linting, tests, build).
- Result: **Success: Quality Gate Passed**. All 189 files successfully pass the sizing audit, and all 164 tests pass.
