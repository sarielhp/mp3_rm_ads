# Stage 10: Application Entrypoint, Tooling Updates, and Final Quality Gate

## 1. Goal & Rationale

Stage 10 finalizes the migration:
- Leaves a pristine, minimal `main.go` entrypoint that simply initializes the logger and calls `pkg/cli.Execute(os.Args[1:])`.
- Deletes obsolete root files and bridges.
- Normalizes test filenames (e.g. converting `*_test_extra.go` to proper `*_test.go`).
- Updates all verification and build tooling (`tools/audit_lines`, `tools/build_local`, `tools/check`, `Makefile`, `AGENTS.md`).
- Executes the comprehensive quality gate.

---

## 2. Package & Entrypoint Details

### 2.1 Lean Entrypoint (`main.go`)

The root `main.go` shrinks from over 300 lines down to fewer than 25 lines:
```go
package main

import (
	"os"

	"github.com/sariel/abs/pkg/cli"
)

func main() {
	os.Exit(cli.Execute(os.Args[1:]))
}
```

This preserves backwards compatibility with `go run .`, `go build .`, and `tools/build_local` while offloading 100% of the implementation to organized packages.

---

## 3. Step-by-Step Implementation Plan

### Step 10.1: Normalize Test File Names
1. Rename `main_cli_test_extra.go` $\to$ `pkg/cli/router_test.go`.
2. Rename `main_test_extra.go` $\to$ `pkg/format/intervals_test.go`.
3. Rename `config_test_extra.go` $\to$ `pkg/config/config_extra_test.go`.

### Step 10.2: Remove Obsolete Root Files
1. Delete unused transitional bridge files from the repository root.
2. Ensure only `main.go`, `go.mod`, `go.sum`, `Makefile`, `VERSION`, `README.md`, and metadata exist at the root level.

### Step 10.3: Update Tooling and Scripts
1. Audit `tools/audit_lines` to verify it recurses into `pkg/**/*.go` and enforces $\le 80$ lines per function and $< 800$ lines per file.
2. Verify `tools/build_local` strictly outputs the binary to `./abs` without writing outside the repository.
3. Update `AGENTS.md` File Organization section to document the new package structure.

### Step 10.4: Execute Full Quality Gate
1. Format: `gofmt -s -w .`
2. Staticcheck baseline update if necessary: `tools/lint`
3. Audit line lengths: `./tools/audit_lines --strict`
4. Remote queue verification: `./tools/verify_remote_queue`
5. Visual TUI verification: `./tools/visual_audit`
6. Comprehensive CI check: `./tools/check --full`

---

## 4. Verification & Quality Gates

```bash
# Full quality gate verification
./tools/check --full

# Full visual PTY audit
./tools/visual_audit

# Remote queue policy verification
./tools/verify_remote_queue

# Sizing audit in strict mode
./tools/audit_lines --strict
```
