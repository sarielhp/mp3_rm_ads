# Rewrite Progress and Resumption Dashboard

This document tracks the live status of the modularization and package rewrite of `abs`. Any AI agent or developer resuming this work can immediately determine the current state, what has been completed, and the exact next step.

---

## 1. Stage Progress Matrix

| Stage | Document | Target Packages | Status | Completed In / Notes |
|:---:|:---|:---|:---:|:---|
| **00** | [`00_ARCHITECTURE_AND_DAG.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/00_ARCHITECTURE_AND_DAG.md) | Blueprint & Architecture | **PLANNED** | Full file mapping & DAG defined |
| **01** | [`STAGE_01_UTILITIES_AND_TYPES.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/STAGE_01_UTILITIES_AND_TYPES.md) | `pkg/types`, `pkg/util` | **READY** | Next stage to execute |
| **02** | [`STAGE_02_CONFIG_AND_BACKEND.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/STAGE_02_CONFIG_AND_BACKEND.md) | `pkg/config`, `pkg/backend` | **PLANNED** | Blocked on Stage 01 |
| **03** | [`STAGE_03_AUDIO_AND_FORMAT.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/STAGE_03_AUDIO_AND_FORMAT.md) | `pkg/audio`, `pkg/format` | **PLANNED** | Blocked on Stage 01 |
| **04** | [`STAGE_04_TRANSCRIBE_AND_AI.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/STAGE_04_TRANSCRIBE_AND_AI.md) | `pkg/transcribe`, `pkg/detect`, `pkg/gemini` | **PLANNED** | Blocked on Stages 02 & 03 |
| **05** | [`STAGE_05_PIPELINE_AND_PLAYER.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/STAGE_05_PIPELINE_AND_PLAYER.md) | `pkg/pipeline`, `pkg/player` | **PLANNED** | Blocked on Stages 03 & 04 |
| **06** | [`STAGE_06_PODCAST_MANAGEMENT.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/STAGE_06_PODCAST_MANAGEMENT.md) | `pkg/podcast` | **PLANNED** | Blocked on Stages 02 & 05 |
| **07** | [`STAGE_07_REMOTE_WORKER.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/STAGE_07_REMOTE_WORKER.md) | `pkg/remote` | **PLANNED** | Blocked on Stage 02 |
| **08** | [`STAGE_08_TUI_AND_KITTY.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/STAGE_08_TUI_AND_KITTY.md) | `pkg/kitty`, `pkg/tui` | **PLANNED** | Blocked on Stages 05 & 06 |
| **09** | [`STAGE_09_CLI_AND_COMMANDS.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/STAGE_09_CLI_AND_COMMANDS.md) | `pkg/cli` | **PLANNED** | Blocked on Stages 05, 06, 07, 08 |
| **10** | [`STAGE_10_ENTRYPOINT_AND_CLEANUP.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/STAGE_10_ENTRYPOINT_AND_CLEANUP.md) | `cmd/abs`, Cleanup | **PLANNED** | Final verification & gate |

---

## 2. Immediate Next Step for Resuming Agent

To begin execution:
1. Open [`rewrite/STAGE_01_UTILITIES_AND_TYPES.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/STAGE_01_UTILITIES_AND_TYPES.md).
2. Execute **Step 1.1**: Create `pkg/types` and extract domain structs from `types.go`, `gemini_types.go`, and `remote_types.go`.
3. Execute **Step 1.2**: Create `pkg/util` and extract `sync`, `strings`, `shell`, `math`, `color`, and `lock`.
4. Bridge root callers to import `pkg/types` and `pkg/util`.
5. Run verification:
   ```bash
   go test ./pkg/types/...
   go test ./pkg/util/...
   go test -timeout 30s ./...
   ./tools/audit_lines --quiet
   ```
6. Update this file (`rewrite/STATUS.md`) setting Stage 01 to **COMPLETED** and commit.

---

## 3. Resumption Checklist

When picking up this codebase from another account or new session:
- [ ] Run `git status` to verify the working tree is clean.
- [ ] Read `rewrite/STATUS.md` to see the current stage.
- [ ] Read the corresponding `rewrite/STAGE_XX_*.md` file.
- [ ] Ensure `go test -timeout 30s ./...` and `./tools/audit_lines` pass before making any changes.
- [ ] Implement the stage step by step.
- [ ] Commit frequently using `git_commit` via git MCP or git commands with conventional commit messages (`refactor(...)`, `feat(...)`).
- [ ] Mark completed tasks in `rewrite/STATUS.md`.
