# ABS Modularization & Package Architecture Rewrite Plan

## 1. Executive Summary & Objectives

The `abs` codebase is currently structured primarily as a single, large Go package (`package main`) spanning 134 source files (~30,000 lines of Go code) and 93 test files (~18,000 lines) in the repository root, with only `pkg/backend` extracted as an external package. 

While the codebase is well-tested and strictly conforms to function-length constraints (all functions $\le 80$ lines), keeping the entire application inside `package main` presents several structural drawbacks:
- **Namespace Congestion**: All 1,270+ package-level identifiers (types, functions, global variables, and constants) exist in a single flat scope.
- **Hidden Coupling**: Functions can call any other function across arbitrary domain boundaries (e.g. table drawing calling low-level audio formatting, or remote transport calling audio processing), making dependency boundaries blurry.
- **Global Variable Dependencies**: Package-level variables in files like `tui_styles.go` or `download_queue.go` hinder test isolation and prevent instantiating multiple subsystems in parallel.
- **Compile Time & Boundary Enforcement**: Go's package compiler cannot enforce architectural boundaries or prevent circular dependencies when everything is in one package.

### Primary Objectives
1. **Modular Architecture**: Restructure `abs` into a hierarchy of clean, single-responsibility Go packages under `pkg/` with a lightweight entrypoint in `cmd/abs/` (or root `main.go`).
2. **Strict Directed Acyclic Graph (DAG)**: Ensure a pure top-down dependency hierarchy with zero import cycles.
3. **Decoupled State & Dependency Injection**: Replace package-global state with explicit struct receivers and options patterns.
4. **Library Error Handling**: Replace `fatalError` (`os.Exit(1)`) in library packages with standard Go error returns (`error`), confining exit handling to the CLI entrypoint layer.
5. **Preserve Sizing & Style Invariants**: Strictly maintain the repository rule of **$\le 80$ lines per function**, **$<800$ lines comfort per file (hard limit 1,100)**, and zero comments in code (self-documenting).
6. **Zero-Downtime Incremental Migration**: Execute the refactoring in 10 safe, verifiable stages, ensuring that `make test`, `make lint`, and `./tools/check` pass at every stage.

---

## 2. Target Package Layout

```
abs/
├── cmd/
│   └── abs/                     # Lean application entrypoint (main, fatal, CLI bootstrap)
├── pkg/
│   ├── types/                   # Fundamental domain entities & data contracts (Leaf package)
│   ├── util/                    # Low-level primitives: strings, syncMutex, color, lock, math (Leaf package)
│   ├── config/                  # App configuration, profiles, secrets, podcast settings
│   ├── backend/                 # Audiobookshelf & PodFetch client drivers (Existing)
│   ├── audio/                   # FFmpeg wrappers, probing, WAV cache, .work/ temp enforcement
│   ├── format/                  # Interval merging, timestamp formatting (SRT, VTT, clock), cuts JSON
│   ├── transcribe/              # Whisper HTTP client, audio chunking, Docker log polling
│   ├── detect/                  # Ad detection via LLMs (Ollama, OpenRouter, speculative race)
│   ├── gemini/                  # Google Cloud Vertex AI & Gemini Studio storage/pipeline
│   ├── pipeline/                # End-to-end processing pipeline (transcribe -> detect -> cut -> export)
│   ├── player/                  # Audio playback daemon, IPC socket, MPRIS DBus, mpv/cvlc sinks
│   ├── podcast/                 # Feed cache, ID registry, sync policies, episode metadata, queue persist
│   ├── remote/                  # SSH transport, remote worker deployment, cluster sync & manifests
│   ├── kitty/                   # Kitty graphics protocol & terminal image rendering
│   ├── tui/                     # Interactive Bubbletea TUI (19 screens/modes, styling, keys)
│   └── cli/                     # CLI argument parsing (clihelp), subcommands, table rendering
├── rewrite/                     # Multi-stage rewrite roadmap & state documentation
├── tools/                       # Build, lint, audit, and verification tooling
├── Makefile                     # Build & test task runner
└── VERSION                      # Semantic version
```

---

## 3. Dependency Hierarchy (DAG Layers)

To prevent circular dependencies in Go, packages are organized into strict hierarchical layers. A package may only import packages from lower layers, never from the same or higher layers (except through interfaces).

```
Layer 5: Entrypoint & CLI
   cmd/abs  ──>  pkg/cli
       │              │
Layer 4: User Interface & Interactive Orchestration
       ├──>  pkg/tui  ──>  pkg/kitty
       │        │
Layer 3: Core Domain Subsystems & Workflows
       ├──>  pkg/pipeline  ──>  pkg/transcribe, pkg/detect, pkg/gemini
       ├──>  pkg/player
       ├──>  pkg/podcast   ──>  pkg/backend
       └──>  pkg/remote
                │
Layer 2: Infrastructure & Media Processing
       ├──>  pkg/audio     ──>  pkg/format
       ├──>  pkg/config
       └──>  pkg/backend
                │
Layer 1: Shared Models & Primitives
       ├──>  pkg/types     (Pure data structs & constants)
       └──>  pkg/util      (String utils, sync wrappers, math, colors, lock)
```

---

## 4. Multi-Stage Execution Roadmap

The rewrite is divided into 10 structured, bite-sized stages. Each stage is fully documented in its own dedicated markdown file in `rewrite/`:

| Stage | Document | Target Packages | Source Files Migrated | Focus Area |
|:-----:|:---------|:----------------|:---------------------:|:-----------|
| **00** | [`00_ARCHITECTURE_AND_DAG.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/00_ARCHITECTURE_AND_DAG.md) | Blueprint | All | Package mapping, DAG proof, error handling standards |
| **01** | [`STAGE_01_UTILITIES_AND_TYPES.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/STAGE_01_UTILITIES_AND_TYPES.md) | `pkg/types`, `pkg/util` | 7 | Domain structs, strings, sync primitives, locks, colors |
| **02** | [`STAGE_02_CONFIG_AND_BACKEND.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/STAGE_02_CONFIG_AND_BACKEND.md) | `pkg/config`, `pkg/backend` | 10 | Configuration loading, profiles, podcast configs, backend bridge |
| **03** | [`STAGE_03_AUDIO_AND_FORMAT.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/STAGE_03_AUDIO_AND_FORMAT.md) | `pkg/audio`, `pkg/format` | 4 | FFmpeg cut/probe, WAV cache, time formatting, interval math |
| **04** | [`STAGE_04_TRANSCRIBE_AND_AI.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/STAGE_04_TRANSCRIBE_AND_AI.md) | `pkg/transcribe`, `pkg/detect`, `pkg/gemini` | 14 | Whisper HTTP client, Docker logs, LLM ad detection, Gemini |
| **05** | [`STAGE_05_PIPELINE_AND_PLAYER.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/STAGE_05_PIPELINE_AND_PLAYER.md) | `pkg/pipeline`, `pkg/player` | 9 | End-to-end pipeline, audio player daemon, IPC, MPRIS |
| **06** | [`STAGE_06_PODCAST_MANAGEMENT.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/STAGE_06_PODCAST_MANAGEMENT.md) | `pkg/podcast` | 24 | Feed caching, episode status, ID registry, sync policies, queues |
| **07** | [`STAGE_07_REMOTE_WORKER.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/STAGE_07_REMOTE_WORKER.md) | `pkg/remote` | 14 | SSH transport, manifests, worker deployment, batch sync |
| **08** | [`STAGE_08_TUI_AND_KITTY.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/STAGE_08_TUI_AND_KITTY.md) | `pkg/kitty`, `pkg/tui` | 31 | Kitty image protocol, Bubbletea TUI, 19 views, key bindings |
| **09** | [`STAGE_09_CLI_AND_COMMANDS.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/STAGE_09_CLI_AND_COMMANDS.md) | `pkg/cli` | 17 | CLI argument parsing, subcommands, table drawing |
| **10** | [`STAGE_10_ENTRYPOINT_AND_CLEANUP.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/STAGE_10_ENTRYPOINT_AND_CLEANUP.md) | `cmd/abs` | 2 | Lean entrypoint, remove legacy root bridges, verify tooling |

---

## 5. Token Limit & Continuation Protocol (Resume Guide)

Because refactoring 134 files and ~30,000 lines across packages involves extensive analysis and multiple steps, sessions may reach token limits. The project enforces an explicit checkpointing and resumption protocol:

1. **Live State Tracking**: The file [`rewrite/STATUS.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/STATUS.md) maintains the authoritative status of all stages (e.g. `PLANNED`, `IN_PROGRESS`, `COMPLETED`), noting exact commits and pending actions.
2. **Atomic Micro-Commits**: After completing any documentation or migration step, changes are immediately committed to Git.
3. **Resumption Checklist for Next Agent**:
   - Inspect `git status` and read `rewrite/STATUS.md`.
   - Identify the current in-progress stage and check its corresponding `rewrite/STAGE_XX_*.md` file.
   - Run `make test` and `./tools/audit_lines` to ensure the existing codebase is green.
   - Proceed with the next uncompleted task in the stage plan.
   - Update `rewrite/STATUS.md` and commit.

---

## 6. Sizing and Style Constraints

Every newly created package and migrated file must conform to:
1. **Function Line Count**: $\le 80$ lines per function (verified by `./tools/audit_lines`). Decompose in place into private package helpers.
2. **File Line Count**: Comfort range 300–700 lines, soft warning at 800 lines, hard error at 1,100 lines.
3. **No Unnecessary Comments**: Self-documenting code with clear variable and function names.
4. **Standard Library Over External Dependencies**: Retain stdlib focus (`os/exec`, `net/http`, custom `syncMutex`).
