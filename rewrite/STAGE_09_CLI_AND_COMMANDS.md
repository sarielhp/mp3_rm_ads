# Stage 09: Command Line Interface and Presentation

## 1. Goal & Rationale

Stage 09 encapsulates the command-line interface layer into `pkg/cli`:
- Command registration and routing via `github.com/sarielhp/clihelp`.
- Argument and flag parsing with strict help texts and error formatting.
- Implementation of all subcommands (`info`, `ls`, `queue`, `policy`, `config`, `proc`, `remote`, `server`, `player`, `test`).
- Tabular text rendering via `table_draw.go`.
- Adherence to project CLI rules: **No CLI aliases** (each command and subcommand must have exactly one canonical name).

Encapsulating this layer eliminates the remaining CLI files from the root package, leaving only the ultra-lean `main.go` entrypoint.

---

## 2. Package Details

### 2.1 `pkg/cli`

| Target File | Source File(s) | Responsibilities |
|:---|:---|:---|
| `router.go` | `cli_parse.go` | Main command registry (`clihelp`), top-level flags, subcommand routing |
| `cmd_info.go` | `info_cmd.go` | `abs info` command: displays podcast and episode metadata, show notes, and cut intervals |
| `cmd_ls.go` | `ls_cmd.go` | `abs ls` command: lists podcasts and episodes, filterable by AdR status and flags |
| `cmd_queue.go` | `queue_cmd.go` | `abs queue` command: display, push, pop, clear, and prioritize download/ad-removal tasks |
| `cmd_policy.go` | `policy_cmd.go` | `abs policy` command: inspect and configure download policies per podcast |
| `cmd_config.go` | `cli_config_cmds.go`, `config_cli.go` | `abs config` command: view, edit, validate, and migrate config files |
| `cmd_proc.go` | `cli_proc_cmds.go` | `abs proc` command: trigger processing pipeline on files or podcast episodes |
| `cmd_remote.go` | `cli_remote_cmds.go`, `cli_remote_exec.go` | `abs remote` command: manage remote workers, manifests, deployment, and cluster sync |
| `cmd_server.go` | `cli_server_cmds*.go`, `backend_cli.go` | `abs server` command: trigger library scans, rescans, and backend status synchronization |
| `cmd_player.go` | `extra_cli_cmds.go` | `abs player` command: control background audio daemon (`play`, `pause`, `stop`, `status`) |
| `cmd_test.go` | `cli_test_cmds.go` | `abs test` command: verify environment, Whisper Docker connectivity, and audio tools |
| `table.go` | `table_draw.go` | Tabular display formatting: column widths, alignments, truncation, borders, and ANSI styling |

**CLI Rules Enforcement:**
- **No CLI Aliases**: Avoid defining command or subcommand aliases. Each command has a single canonical name.
- **Error Formatting**: CLI handlers catch domain errors returned by packages and print clean stderr messages with standard exit codes.

**Dependencies:**
- Imports all lower packages (`pkg/types`, `pkg/util`, `pkg/config`, `pkg/backend`, `pkg/audio`, `pkg/format`, `pkg/transcribe`, `pkg/detect`, `pkg/pipeline`, `pkg/player`, `pkg/podcast`, `pkg/remote`, `pkg/tui`).

---

## 3. Step-by-Step Implementation Plan

### Step 3.1: Create `pkg/cli` Foundation
1. Move `table_draw.go` to `pkg/cli/table.go`.
2. Move command router from `cli_parse.go` to `pkg/cli/router.go`.
3. Expose `func Execute(args []string) int` returning process exit code.

### Step 3.2: Move Command Handlers
1. Move `info_cmd.go` to `pkg/cli/cmd_info.go`.
2. Move `ls_cmd.go` to `pkg/cli/cmd_ls.go`.
3. Move `queue_cmd.go` to `pkg/cli/cmd_queue.go`.
4. Move `policy_cmd.go` to `pkg/cli/cmd_policy.go`.
5. Move config commands to `pkg/cli/cmd_config.go`.
6. Move processing commands to `pkg/cli/cmd_proc.go`.
7. Move remote commands to `pkg/cli/cmd_remote.go`.
8. Move server commands to `pkg/cli/cmd_server.go`.
9. Move player CLI commands to `pkg/cli/cmd_player.go`.
10. Move test commands to `pkg/cli/cmd_test.go`.

### Step 3.3: Migrate Unit Tests
1. Move all CLI unit tests (`cli_all_test.go`, `info_cmd_test.go`, `ls_cmd_test.go`, `table_draw_test.go`, etc.) into `pkg/cli/`.
2. Verify line limits ($\le 80$ lines per function).

---

## 4. Verification & Quality Gates

```bash
# Verify CLI package tests
go test -v ./pkg/cli/...

# Smoke test CLI execution
go run . --help
go run . info --help
go run . config

# Verify root tests
go test -timeout 30s ./...

# Sizing audit
./tools/audit_lines --quiet
```
