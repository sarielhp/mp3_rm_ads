# Architecture, Package Mapping, and DAG Specification

## 1. Complete File-to-Package Mapping

The current monolithic `package main` comprises 134 non-test Go source files and 93 test files. Below is the comprehensive classification into target modular packages under `pkg/` and `cmd/`:

### 1.1 Foundation & Common Packages

#### `pkg/types` (Domain Entities & Data Contracts)
- `types.go` (Domain models: `AdSegment`, `KeepSegment`, `TranscriptionSegment`, `CutEntry`, `MergedCutInterval`, `CutsData`, `CutsResult`, `LLMProfile`, `WhisperProfile`)
- `gemini_types.go` (`geminiCutItem`, `geminiSegmentItem`, `geminiResponsePayload`, `geminiChunkInfo`, `geminiChunkResult`)
- `pm_types.go` (`PodcastMetadata`, `SyncPolicy`, `EpisodeInfo`)
- `remote_types.go` (`RemoteBatchStatus`, `RemoteBatchJobItem`, `RemoteBatchManifest`, `RemoteServerStatus`)

#### `pkg/util` (Primitives & System Utilities)
- `sync_types.go` (`syncMutex`, `syncMu`, `syncWG`, `syncRWMutex`, `syncOnce`)
- `string_utils.go` (`extractHost`, `extractPort`, `isLocalHost`, `splitLines`, `toLower`, `stripExt`, `filepathBase`)
- `color.go` (Terminal ANSI color escape helpers)
- `debug.go` (Structured debug logging and stderr trace output)
- `lock.go` (File locking and cross-process coordination via `flock`)
- Utilities extracted from other files:
  - `repeatStr` (moved from `format.go`)
  - `truncate` (moved from `tui.go`)
  - `shellQuote` (moved from `remote_transport.go`)
  - `roundFloat` (moved from `profile_cost.go`)

---

### 1.2 Configuration & Backend Integration

#### `pkg/config` (App Settings, Profiles, & Secrets)
- `config.go` (`Config` struct, default values, JSON serialization, validation)
- `config_path.go` (`configDir`, `userTmpDir`, environment overrides)
- `podcast_config.go` (`PodcastConfig`, per-podcast overrides, frequency rules)
- `profiles.go` (LLM profile definitions, pricing parameters)
- `profile_cost.go` (Token estimation, cost calculation)
- `profiles_whisper.go` (Whisper model profiles, threads, beam size)
- `opencode.go` (OpenCode configuration discovery and compatibility)
- `api_keys.go` (API key discovery from env, config, and system keyrings)

#### `pkg/backend` (Existing - Audiobookshelf & PodFetch API Drivers)
- `pkg/backend/backend.go` (`Backend` interface definition)
- `pkg/backend/audiobookshelf*.go` (ABS REST client and SQLite query driver)
- `pkg/backend/podfetch*.go` (PodFetch REST client and SQLite query driver)
- `pkg/backend/sync_types.go`, `pkg/backend/types.go`

---

### 1.3 Media & AI Engines

#### `pkg/audio` (FFmpeg Wrapper & Audio Slicing)
- `audio.go` (FFmpeg execution, duration probing, audio cutting, format conversions)
- `output.go` (Audio file export, tagging, atomic moving)
- `.work/` directory validation helpers (`verifyTempFile`, `workDirFor`)

#### `pkg/format` (Time Formatting, Interval Math, & Subtitles)
- `format.go` (`formatClock`, `formatSRTTime`, subtitle parsing)
- `format_intervals.go` (`calculateKeepSegments`, `mergeIntervals`, `mergeBounds`)

#### `pkg/transcribe` (Whisper HTTP API Client)
- `transcribe.go` (Whisper HTTP client, verbose_json parsing)
- `transcribe_chunks.go` (Audio chunk splitting, overlap handling, segment assembly)
- `transcribe_wav.go` (16kHz 16-bit mono WAV header builder)
- `docker.go`, `whisper_docker.go` (Whisper Docker container detection and log polling)

#### `pkg/detect` (LLM Ad Detection)
- `ads.go` (Prompt generation, LLM request/response parsing, ad segment extraction)
- `openrouter.go` (OpenRouter API transport)
- `speculative_race.go` (Speculative execution between local and cloud models)

#### `pkg/gemini` (Google Cloud Vertex AI & Gemini Studio)
- `gemini_client.go` (Vertex AI authentication and client initialization)
- `gemini_pipeline.go` (Cloud multimodal processing pipeline)
- `gemini_storage.go` (Google Cloud Storage bucket upload and URI generation)
- `gemini_studio.go` (Gemini Studio structured prompt format)

---

### 1.4 Orchestration, Daemon, & Management

#### `pkg/pipeline` (End-to-End Processing Engine)
- `pipeline.go` (Core processing state machine: WAV -> Transcribe -> Detect -> Cut -> Output)
- `batch_proc.go` (Batch processing coordinator)
- `batch_proc_file.go` (Single file batch execution)
- `batch_dry_run.go` (Dry run estimation and summary)
- `quarantine.go` (Corrupt or unprocessable audio quarantine handling)

#### `pkg/player` (Audio Playback Daemon & IPC)
- `player.go` (Player controller and state machine)
- `player_ipc.go` (Unix domain socket `/tmp/abs_player.sock`, MPRIS D-Bus integration)
- `player_sink.go` (mpv and cvlc audio sink backends)
- `player_ui.go` (Minimal TUI playback status bar widget)

#### `pkg/podcast` (Podcast & Feed Management)
- `pm_download.go`, `pm_download_episodes.go`, `pm_download_episodes_exec.go`
- `pm_download_policy.go` (Episode sync policies: all, latest N, duration bounds)
- `pm_frequency.go` (Release frequency analysis)
- `pm_get_info.go`, `pm_list.go`, `pm_clean_orphans.go`
- `pm_server_exec.go`, `pm_server_exec2.go`, `pm_simplecast.go`, `pm_utils.go`
- `feed_cache.go` (RSS feed caching, XML parsing)
- `id_registry.go`, `podcast_id.go` (Unified podcast & episode ID resolution)
- `episode_status.go`, `status_report.go` (Episode status tracking: AdR, NeedAdR)
- `cache.go` (Episode metadata cache)
- `download_queue.go`, `queue_persist.go` (Download & ad removal queue manager)
- `rm_ads_podcast.go` (Orchestration of ad removal for podcast episodes)
- `audit_transcripts.go` (Transcript health auditing)

#### `pkg/remote` (Remote Worker & Cluster Orchestration)
- `remote_batch.go`, `remote_batch_files.go` (Remote job batch packing)
- `remote_deploy.go`, `remote_worker.go` (SSH deployment of remote worker binaries)
- `remote_manifest.go` (Job manifest generation and verification)
- `remote_scan.go`, `remote_collect.go` (Remote artifact collection)
- `remote_status.go`, `remote_status_print.go` (Remote worker status polling)
- `remote_stop.go`, `remote_cancel.go`, `remote_clear.go`
- `remote_transport.go` (SSH / SCP command transport)

---

### 1.5 Terminal UI & CLI

#### `pkg/kitty` (Terminal Image Protocol)
- `kitty.go`, `kitty_encode.go`, `kitty_image.go` (Kitty protocol escape generation, terminal detection, image rasterization)

#### `pkg/tui` (Interactive Terminal User Interface)
- `tui.go` (`tuiModel` Bubbletea model, update loop, view switching)
- `tui_views.go`, `tui_list_view.go`, `tui_episode_view.go`, `tui_detail_view.go`
- `tui_latest_view.go`, `tui_queues_view.go`, `tui_timeline_view.go`, `tui_transcript_view.go`
- `tui_modal.go`, `tui_modal_download.go`
- `tui_keys.go`, `tui_keys_actions.go`, `tui_keys_nav.go`, `tui_keys_queue.go`
- `tui_data.go`, `tui_data_abs.go`, `tui_data_queue.go`, `tui_feed_fetch.go`, `tui_transcript_data.go`
- `tui_format.go`, `tui_styles.go`, `tui_text_utils.go`, `tui_transcript_export.go`
- `tui_types.go`, `tui_batch.go`, `tui_nav.go`

#### `pkg/cli` (Command Handlers & Presentation)
- `cli_parse.go` (`clihelp` argument parsing and command routing)
- `cli_commands_parity.go`, `cli_config_cmds.go`, `cli_proc_cmds.go`
- `cli_remote_cmds.go`, `cli_remote_exec.go`, `cli_server_cmds*.go`, `cli_test_cmds.go`
- `config_cli.go`, `backend_cli.go`, `extra_cli_cmds.go`
- `info_cmd.go`, `ls_cmd.go`, `policy_cmd.go`, `queue_cmd.go`
- `table_draw.go` (Formatted ASCII/Unicode table rendering)

#### `cmd/abs` (Application Entrypoint)
- `main.go` (`func main()`, exit code propagation)
- `fatal.go` (`fatalError` with stderr reporting and `os.Exit(1)`)

---

## 2. Directed Acyclic Graph (DAG) Specification

```mermaid
graph TD
    subgraph Level 0: Pure Primitives
        types[pkg/types]
        util[pkg/util]
    end

    subgraph Level 1: Configuration & Low-Level Formats
        config[pkg/config] --> types
        config --> util
        format[pkg/format] --> types
        format --> util
    end

    subgraph Level 2: Media, Clients & Graphics
        audio[pkg/audio] --> types
        audio --> util
        audio --> format

        backend[pkg/backend] --> types
        backend --> util

        kitty[pkg/kitty] --> util

        transcribe[pkg/transcribe] --> types
        transcribe --> util
        transcribe --> config

        detect[pkg/detect] --> types
        detect --> util
        detect --> config

        gemini[pkg/gemini] --> types
        gemini --> util
        gemini --> config
    end

    subgraph Level 3: Processing & Daemon Engines
        pipeline[pkg/pipeline] --> types
        pipeline --> util
        pipeline --> config
        pipeline --> audio
        pipeline --> format
        pipeline --> transcribe
        pipeline --> detect
        pipeline --> gemini

        player[pkg/player] --> types
        player --> util
        player --> audio

        podcast[pkg/podcast] --> types
        podcast --> util
        podcast --> config
        podcast --> backend
        podcast --> audio

        remote[pkg/remote] --> types
        remote --> util
        remote --> config
    end

    subgraph Level 4: UI & CLI Orchestration
        tui[pkg/tui] --> types
        tui --> util
        tui --> config
        tui --> audio
        tui --> format
        tui --> podcast
        tui --> player
        tui --> kitty

        cli[pkg/cli] --> types
        cli --> util
        cli --> config
        cli --> podcast
        cli --> pipeline
        cli --> player
        cli --> remote
        cli --> backend
        cli --> tui
    end

    subgraph Level 5: Entrypoint
        main[cmd/abs] --> cli
        main --> util
    end
```

---

## 3. Resolving Cross-Package Entanglements

Our AST and symbol dependency audit identified several cross-boundary violations in the current monolith. The table below outlines how each is resolved:

| Problematic Coupling | Current Location | Root Cause | Architectural Resolution |
|:---|:---|:---|:---|
| `table_draw.go` calling `ls_cmd.go` (`lsPodcastItem`) | Root package | Presentation hardcoded to specific CLI structs | Move `table_draw.go` into `pkg/cli` or genericize table columns with string/interface rows |
| `audio.go` calling `shellQuote` from `remote_transport.go` | Root package | Utility function defined inside a consumer module | Move `shellQuote` to `pkg/util/shell.go` |
| `color.go` calling `repeatStr` from `format.go` | Root package | String helper defined in subtitle formatter | Move `repeatStr` to `pkg/util/string_utils.go` |
| `audio.go` calling `truncate` from `tui.go` | Root package | String truncation helper placed inside TUI | Move `truncate` to `pkg/util/string_utils.go` |
| `debug.go` calling `isKittyTerminal` & `userTmpDir` | Root package | Debug logger accessing high-level config & kitty | Pass log path and terminal options explicitly into debug logger initialization |
| Ubiquitous `fatalError` calls | All files | Rapid prototyping shortcut calling `os.Exit(1)` | Replace with standard `return fmt.Errorf(...)` in all library packages; only `cmd/abs` exits |

---

## 4. Global State Decoupling Strategy

1. **`tui_styles.go`**:
   - Current: 131 global `lipgloss.Style` variables.
   - Solution: Create `type Styles struct` holding all styled elements, instantiated via `func NewStyles(cfg *config.Config) *Styles`.
2. **`download_queue.go` & `feed_cache.go`**:
   - Current: Global queue slices and cache maps.
   - Solution: Encapsulate into `type DownloadQueueManager struct` and `type FeedCacheManager struct` with explicit mutex synchronization and dependency injection.
3. **`id_registry.go`**:
   - Current: Global ID lookup table.
   - Solution: `type Registry struct` managed within the `pkg/podcast` context.
