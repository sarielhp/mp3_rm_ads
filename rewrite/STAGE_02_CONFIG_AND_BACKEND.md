# Stage 02: Configuration Management & Backend Integration

## 1. Goal & Rationale

Stage 02 encapsulates all configuration logic into `pkg/config` and solidifies the interface between configuration and `pkg/backend` (which already exists under `pkg/backend/`).

In the current monolith, configuration state is intertwined with global flags and path lookups, and files like `config.go` (747 lines) and `podcast_config.go` (400 lines) contain a mix of models, file I/O, and migration logic. Moving them into `pkg/config` creates a clean, testable configuration engine.

---

## 2. Package Details

### 2.1 `pkg/config`

| Target File | Source File(s) | Responsibilities |
|:---|:---|:---|
| `config.go` | `config.go`, `types.go` | Core `Config` struct, default values, JSON serialization, validation |
| `paths.go` | `config_path.go` | Canonical paths (`~/.config/abs/`, `~/.cache/abs/`), environment variable overrides (`ABS_URL`, `WHISPER_URL`, `PODCASTS_DIR`) |
| `podcast.go` | `podcast_config.go` | Per-podcast overrides, download policy rules, episode frequency criteria |
| `profiles.go` | `profiles.go`, `profiles_whisper.go`, `profile_cost.go` | LLM and Whisper engine profile definitions, pricing parameters, cost estimation |
| `keys.go` | `api_keys.go` | API key resolution (environment variables, config file, keyring) |
| `opencode.go` | `opencode.go` | OpenCode integration config and auto-discovery |
| `migrate.go` | `config.go` | Migration of legacy configuration files from `podcasts_manager` and `mp3_rm_ads` |

**Dependencies:**
- Imports `pkg/types` and `pkg/util`.
- Zero dependencies on higher layers (`pkg/audio`, `pkg/podcast`, `pkg/cli`).

### 2.2 `pkg/backend` Integration

`pkg/backend` is already located at `pkg/backend/` and defines the `Backend` interface along with `AudiobookshelfClient` and `PodfetchClient`. In this stage:
- Define a unified factory function in `pkg/backend/factory.go`:
  ```go
  func NewBackend(cfg *config.Config) (Backend, error)
  ```
- Eliminate the root bridge `backend_client.go` by adopting the factory directly.

---

## 3. Step-by-Step Implementation Plan

### Step 2.1: Populate `pkg/config`
1. Move `Config` struct definition from `types.go` to `pkg/config/config.go`.
2. Extract path resolution functions to `pkg/config/paths.go`.
3. Extract per-podcast configuration and rules to `pkg/config/podcast.go`.
4. Move profiles and cost calculations to `pkg/config/profiles.go`.
5. Move API key resolution into `pkg/config/keys.go`.
6. Extract migration logic into `pkg/config/migrate.go`.
7. Ensure all functions strictly adhere to the $\le 80$ lines limit.

### Step 2.2: Backend Factory & Decoupling
1. Implement `NewBackend(cfg *config.Config) (Backend, error)` inside `pkg/backend`.
2. Ensure `pkg/backend` accepts typed config models without depending on the root package.
3. Migrate `backend_client.go` logic into `pkg/backend/factory.go`.

### Step 2.3: Tests & Verification
1. Move `config_test.go` and `config_test_extra.go` to `pkg/config/config_test.go`.
2. Verify all backend tests in `pkg/backend/*_test.go`.
3. Update root imports and verify that `make test` passes.

---

## 4. Verification & Quality Gates

```bash
# Verify config and backend packages
go test -v ./pkg/config/...
go test -v ./pkg/backend/...

# Verify root tests
go test -timeout 30s ./...

# Sizing audit
./tools/audit_lines --quiet
```
