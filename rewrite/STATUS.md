# Rewrite Progress and Resumption Dashboard

This document tracks the live status of the modularization and package rewrite of `abs`. Any AI agent or developer resuming this work can immediately determine the current state, what has been completed, and the exact next step.

---

## 1. Stage Progress Matrix

| Stage | Document | Target Packages | Status | Completed In / Notes |
|:---:|:---|:---|:---:|:---|
| **00** | [`00_ARCHITECTURE_AND_DAG.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/00_ARCHITECTURE_AND_DAG.md) | Blueprint & Architecture | **COMPLETED** | Commit `3bd884d` |
| **01** | [`STAGE_01_UTILITIES_AND_TYPES.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/STAGE_01_UTILITIES_AND_TYPES.md) | `pkg/types`, `pkg/util` | **COMPLETED** | Commit `cb26224` |
| **02** | [`STAGE_02_CONFIG_AND_BACKEND.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/STAGE_02_CONFIG_AND_BACKEND.md) | `pkg/config`, `pkg/backend` | **COMPLETED** | Commit `d17bd34` |
| **03** | [`STAGE_03_AUDIO_AND_FORMAT.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/STAGE_03_AUDIO_AND_FORMAT.md) | `pkg/audio`, `pkg/format` | **COMPLETED** | `pkg/format` & `pkg/audio` created and tested |
| **04** | [`STAGE_04_TRANSCRIBE_AND_AI.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/STAGE_04_TRANSCRIBE_AND_AI.md) | `pkg/transcribe`, `pkg/detect`, `pkg/gemini` | **READY** | Next stage to execute |
| **05** | [`STAGE_05_PIPELINE_AND_PLAYER.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/STAGE_05_PIPELINE_AND_PLAYER.md) | `pkg/pipeline`, `pkg/player` | **PLANNED** | Blocked on Stage 04 |
| **06** | [`STAGE_06_PODCAST_MANAGEMENT.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/STAGE_06_PODCAST_MANAGEMENT.md) | `pkg/podcast` | **PLANNED** | Blocked on Stage 05 |
| **07** | [`STAGE_07_REMOTE_WORKER.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/STAGE_07_REMOTE_WORKER.md) | `pkg/remote` | **PLANNED** | Blocked on Stage 02 |
| **08** | [`STAGE_08_TUI_AND_KITTY.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/STAGE_08_TUI_AND_KITTY.md) | `pkg/kitty`, `pkg/tui` | **PLANNED** | Blocked on Stages 05 & 06 |
| **09** | [`STAGE_09_CLI_AND_COMMANDS.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/STAGE_09_CLI_AND_COMMANDS.md) | `pkg/cli` | **PLANNED** | Blocked on Stages 05, 06, 07, 08 |
| **10** | [`STAGE_10_ENTRYPOINT_AND_CLEANUP.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/STAGE_10_ENTRYPOINT_AND_CLEANUP.md) | `cmd/abs`, Cleanup | **PLANNED** | Final verification & gate |

---

## 2. Immediate Next Step for Resuming Agent

To continue execution:
1. Open [`rewrite/STAGE_04_TRANSCRIBE_AND_AI.md`](file:///home/sariel/prog/26/podcasts/abs/rewrite/STAGE_04_TRANSCRIBE_AND_AI.md).
2. Create `pkg/transcribe` (Whisper HTTP client, chunking, Docker log polling).
3. Create `pkg/detect` (LLM ad detection, OpenRouter client, speculative race).
4. Create `pkg/gemini` (Vertex AI client, storage, pipeline).
5. Run verification:
   ```bash
   go test ./pkg/transcribe/...
   go test ./pkg/detect/...
   go test ./pkg/gemini/...
   go test -timeout 30s ./...
   ./tools/audit_lines --quiet
   ```
6. Update this file (`rewrite/STATUS.md`) setting Stage 04 to **COMPLETED** and commit.

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
