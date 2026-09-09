# Stage 07: Remote Worker and Cluster Orchestration

## 1. Goal & Rationale

Stage 07 encapsulates distributed job processing into `pkg/remote`.

The remote worker subsystem offloads transcription and processing workloads to remote Linux machines equipped with Whisper and GPU/CPU resources via SSH/SCP. It manages the entire remote lifecycle:
- Binary and dependency deployment.
- Manifest generation, validation, and synchronization.
- Real-time remote log streaming, container status polling, and remote queue compliance.
- Pulling completed artifacts back into the local repository.
- Graceful cancellation, process killing, and cache clearing.

Currently scattered across 14 `remote_*.go` files in root, moving them into `pkg/remote` ensures clean isolation from local processing while maintaining strict remote queue verification.

---

## 2. Package Details

### 2.1 `pkg/remote`

| Target File | Source File(s) | Responsibilities |
|:---|:---|:---|
| `transport.go` | `remote_transport.go` | SSH / SCP execution wrappers, connection timeouts, key configuration |
| `deploy.go` | `remote_deploy.go`, `remote_worker.go` | Packaging and deploying worker binaries, configuring remote systemd/docker |
| `manifest.go` | `remote_manifest.go` | Job manifest (`manifest.json`) generation, episode job descriptors, integrity hashing |
| `batch.go` | `remote_batch.go`, `remote_batch_files.go` | Batch packaging of episode audio files, pushing payloads to remote hosts |
| `scan.go` | `remote_scan.go`, `remote_collect.go` | Polling remote output directories, verifying cuts/transcripts, pulling artifacts locally |
| `status.go` | `remote_status.go`, `remote_status_print.go` | Remote host health, CPU/RAM utilization, active worker count, queue inspection |
| `control.go` | `remote_stop.go`, `remote_cancel.go`, `remote_clear.go` | Gracefully stopping remote tasks, cancelling running containers, purging remote workdirs |

**Dependencies:**
- Imports `pkg/types`, `pkg/util`, `pkg/config`.
- Independent of local audio manipulation or TUI logic.

---

## 3. Step-by-Step Implementation Plan

### Step 7.1: Create `pkg/remote`
1. Move SSH/SCP primitives to `pkg/remote/transport.go`.
2. Move manifest serialization and schema validation to `pkg/remote/manifest.go`.
3. Move batching and file transfer logic to `pkg/remote/batch.go`.
4. Move remote status monitoring to `pkg/remote/status.go`.
5. Move collection and scan logic to `pkg/remote/scan.go`.
6. Move stop/cancel/clear routines to `pkg/remote/control.go`.
7. Move deployment orchestration to `pkg/remote/deploy.go`.

### Step 7.2: Migrate Unit Tests
1. Move all 12 remote unit tests (`remote_batch_test.go`, `remote_transport_test.go`, `remote_queue_verify_test.go`, etc.) to `pkg/remote/`.
2. Verify queue verification suite via `./tools/verify_remote_queue`.

### Step 7.3: Bridge Monolith
1. Update root callers (`cli_remote_cmds.go`, `main.go`) to import `pkg/remote`.
2. Verify sizing limits ($\le 80$ lines per function).

---

## 4. Verification & Quality Gates

```bash
# Verify remote package tests
go test -v ./pkg/remote/...

# Verify remote queue compliance audit
./tools/verify_remote_queue

# Verify root tests
go test -timeout 30s ./...

# Sizing audit
./tools/audit_lines --quiet
```
