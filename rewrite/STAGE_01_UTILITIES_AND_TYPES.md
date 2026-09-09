# Stage 01: Primitives, Utilities, and Domain Types

## 1. Goal & Rationale

The foundation of the modularization is establishing Level 0 leaf packages:
- `pkg/types`: Core domain entities, data transfer objects, and constants that have **zero** dependencies on any other package in the codebase.
- `pkg/util`: General-purpose primitives, thread-safety abstractions, string manipulation, math, process locks, and terminal ANSI helpers.

By extracting these first, all subsequent layers can depend on them without introducing cycles.

---

## 2. Package Details

### 2.1 `pkg/types` (Domain Entities)

| File | Source File(s) | Exported Types & Constants |
|:---|:---|:---|
| `types.go` | `types.go` | `AdSegment`, `KeepSegment`, `TranscriptionSegment`, `TranscriptionWord`, `TranscriptionData`, `CutEntry`, `MergedCutInterval`, `CutsData`, `CutsResult`, `LLMProfile`, `WhisperEngine`, `WhisperProfile`, `CostInfo` |
| `gemini.go` | `gemini_types.go` | `GeminiCutItem`, `GeminiSegmentItem`, `GeminiResponsePayload`, `GeminiChunkInfo`, `GeminiChunkResult` |
| `remote.go` | `remote_types.go` | `RemoteBatchStatus`, `RemoteBatchJobItem`, `RemoteBatchManifest`, `RemoteServerStatus` |
| `episode.go` | `episode_status.go` | `EpisodeState`, `EpisodeAudioMeta`, `EpisodeAdCut`, `EpisodeStatusFile` |

**Design Notes:**
- All structs must have clear JSON tags for persistence and API interop.
- Any methods on these types that do not perform I/O (e.g. calculation of segment duration, bounds checking) should live directly in `pkg/types`.

### 2.2 `pkg/util` (System Primitives)

| File | Source File(s) | Key Symbols & Functions |
|:---|:---|:---|
| `sync.go` | `sync_types.go` | `SyncMutex`, `SyncMu`, `SyncWG`, `SyncRWMutex`, `SyncOnce` |
| `strings.go` | `string_utils.go`, `format.go`, `tui.go` | `ExtractHost`, `ExtractPort`, `IsLocalHost`, `SplitLines`, `ToLower`, `StripExt`, `FilepathBase`, `RepeatStr`, `Truncate` |
| `shell.go` | `remote_transport.go` | `ShellQuote`, `EscapeArgument` |
| `math.go` | `profile_cost.go` | `RoundFloat`, `ClampFloat`, `MinInt`, `MaxInt` |
| `color.go` | `color.go` | `Colorize`, `Red`, `Green`, `Yellow`, `Cyan`, `Bold` |
| `lock.go` | `lock.go` | `FileLock`, `AcquireLock`, `ReleaseLock` |
| `debug.go` | `debug.go` | `Logger`, `DebugLog`, `SetDebug` (pure interface, no config import) |

---

## 3. Step-by-Step Implementation Plan

### Step 1.1: Create `pkg/types`
1. Initialize directory `pkg/types`.
2. Move data structures from `types.go`, `gemini_types.go`, and `remote_types.go`.
3. Export all public struct fields and types.
4. Ensure `pkg/types` imports **only** Go standard library packages (`time`, `encoding/json`).
5. Write unit tests in `pkg/types/types_test.go`.

### Step 1.2: Create `pkg/util`
1. Initialize directory `pkg/util`.
2. Extract thread-safety primitives to `pkg/util/sync.go`.
3. Extract and unify string helpers into `pkg/util/strings.go` (gathering `repeatStr` from `format.go` and `truncate` from `tui.go`).
4. Extract `shellQuote` from `remote_transport.go` into `pkg/util/shell.go`.
5. Extract `roundFloat` from `profile_cost.go` into `pkg/util/math.go`.
6. Extract ANSI color helpers into `pkg/util/color.go`.
7. Extract file locking into `pkg/util/lock.go`.
8. Refactor `debug.go` so it takes an explicit output writer or directory instead of calling `config_path.go`.
9. Write unit tests for each utility file (`strings_test.go`, `sync_test.go`, `shell_test.go`, etc.).

### Step 1.3: Bridge Monolith to `pkg/types` and `pkg/util`
1. Update existing root files to import `"github.com/sariel/abs/pkg/types"` and `"github.com/sariel/abs/pkg/util"`.
2. Run type aliases or direct reference replacements in the root files.
3. Verify function line lengths ($\le 80$ lines).

---

## 4. Verification & Quality Gates

Run the following checks after completing Stage 01:
```bash
# Verify tests within new packages
go test -v ./pkg/types/...
go test -v ./pkg/util/...

# Verify existing root test suite remains green
go test -timeout 30s ./...

# Sizing audit
./tools/audit_lines --quiet
```
