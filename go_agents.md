# AGENTS.md — Generic Golang Project Guidelines

This document defines standard instructions, architectural patterns, workflows, and developer tooling applicable to any Go (Golang) project.

---

## 0. First-Time Setup & Agent Initialization

When an AI agent starts in a new or existing repository containing this file:

1. **Bootstrap `tools/`**: Ensure the `tools/` directory exists with the 5 standard scripts defined in Section 6 (`chmod +x tools/*.sh`).
2. **Audit Existing Codebase**:
   - Run `./tools/audit_lines.sh` to check for any `.go` file exceeding the 500-line soft limit. If found, refactor and decompose them into dedicated files.
   - Check handlers/services for `log.Fatalf` or `os.Exit` and replace with error return propagation (`fmt.Errorf`).
   - Run `go vet ./...` and resolve any static analysis warnings.
3. **Ensure Version Anchor**: Verify `app/constants.go` exists with a `const Version = "..."` definition for `./tools/bump_version.sh`. If not found, create it.
4. **Follow Workflows**: For all subsequent code changes, adhere strictly to the 5-step pre-commit checklist in Section 2.

---

## 1. Quick Commands

```bash
make                          # Build and install/package binaries
./tools/verify_build.sh       # Format + vet + test + build in a single step
go vet ./...                  # Static analysis (standard Go linter)
go test -v -race ./...        # Run all tests with race detection
./tools/audit_lines.sh        # Verify 500-line soft limit per source file
./tools/bump_version.sh       # Increment patch version & auto-commit
./tools/bump.sh               # Bump version, commit, tag, and push release
```

---

## 2. Before Committing (Pre-Commit Verification)

Always complete this 5-step verification checklist before committing code or opening a PR:

1. **Static Analysis**: Run `go vet ./...` (and optionally `staticcheck ./...`) — resolve all warnings.
2. **Build & Test**: Run `./tools/verify_build.sh` — ensure all unit/integration tests pass and the binary compiles cleanly.
3. **Line Count Audit**: Run `./tools/audit_lines.sh` — ensure no `.go` file exceeds the 500-line soft limit.
4. **Version Bump**: Run `./tools/bump_version.sh` — increment the patch version for every functional change.
5. **Conventional Commit**: Format commit messages using conventional styles:
   - `area: description` or `<type>(<scope>): <description>`
   - *Examples*:
     - `feat(storage): add prune subcommand`
     - `fix(tui): correct folder tree alignment`
     - `refactor: decouple cache from session context`
     - `docs: update setup instructions`

---

## 3. Hard Constraints & Code Hygiene

- **500-Line Soft Limit**:
  - Keep `.go` files under 500 lines to ensure modularity, maintainability, and clear single responsibility.
  - When a file grows beyond 500 lines, decompose it by sub-feature, lifecycle stage, or dedicated helper types.
- **Error Handling & Exit Hygiene**:
  - **Never use `log.Fatalf` or `os.Exit` in libraries or handlers**.
  - All domain logic, storage operations, and command handlers must return errors upward using `fmt.Errorf("context: %w", err)`.
  - Use a centralized `handleError(err)` helper at the presentation/CLI boundary.
  - Reserve `log.Fatalf` strictly for fatal, unrecoverable startup configuration failures in `main.go`.
- **Encapsulation & Storage Isolation**:
  - Never leak underlying file paths, database handles, or raw serialization formats outside the storage/cache package.
  - Expose high-level, intention-revealing APIs (e.g., `Store(id, data)`, `Read(id)`, `Delete(id)`, `Move(from, to)`).
- **Safe Destructive Operations**:
  - Always guard destructive actions (e.g., permanent deletion, cache wipe) behind dry-run options or interactive confirmation flags.
  - Never run destructive tests against live external systems.
- **Context Propagation**:
  - Always accept `ctx context.Context` as the first parameter for I/O, database, network calls, and background workers.
  - Never store `Context` in a struct.
- **Goroutine & Resource Management**:
  - Always ensure deterministic cleanup of goroutines, channels, timers, and file descriptors.
  - Use `context.Context` for cancellation, `sync.WaitGroup` for synchronization, and `defer` for resource cleanup.

---

## 4. Architecture & Package Structure

### Standard Directory Layout

```
├── cmd/                      # Application entry points (one per binary)
│   └── myapp/                # Main application
├── internal/                 # Private application code (not importable)
│   ├── config/               # Configuration structs and loading
│   ├── domain/               # Core business logic and domain models
│   ├── service/              # Service layer implementing business logic
│   └── storage/              # Data access layer (database/filesystem)
├── pkg/                      # Publicly exportable libraries (optional)
├── app/                      # Application metadata and constants
├── ui/                       # Presentation layer (TUI, CLI, web)
├── testdata/                 # Test fixtures and golden snapshots
└── tools/                    # Development and maintenance scripts
```

### Architectural Principles

1. **Centralized Domain Models (`types.go`)**:
   - Define core structs, domain models, and primary service interfaces in a central types package first before implementation.
2. **Interface Abstraction & Factory Pattern**:
   - Program against interfaces rather than concrete implementations.
   - Use constructor/factory functions (`NewClient(...)`) combined with the **Decorator / Delegate Pattern** to wrap clients with caching, metrics, logging, or checking capabilities.
3. **Mock Synchronization**:
   - Maintain a mock implementation (e.g., `MockClient`) satisfying all interface methods in `testutil/` or alongside client code.
   - **Rule**: Whenever an interface method is added or modified in the core contract, update the mock implementation in tandem.
4. **Configuration Resolution**:
   - Follow a deterministic resolution hierarchy:
     $$\text{CLI Flags} \longrightarrow \text{Environment Variables} \longrightarrow \text{Config File} \longrightarrow \text{Defaults}$$
   - Centralize config file discovery (`~/.config/<app_name>/config.json`) and migration helpers.

---

## 5. Testing & Quality Assurance

- **Colocated Unit Tests**: Place tests in `*_test.go` files in the same package alongside the code being tested.
- **Mock-Driven Testing**: Use the synchronized mock implementations to test higher-level services and CLI handlers in isolation without network or disk I/O.
- **Golden Snapshot Testing**: Use `testdata/snapshots/` to store golden files for CLI formatting, JSON responses, or TUI renderings to catch visual and structural regressions.
- **Concurrency & Race Detection**: Always run tests with `-race` enabled during verification:
  ```bash
  go test -v -race ./...
  ```
- **Table-Driven Tests**: Use table-driven tests (`[]struct{ name string, ... }` with `t.Run(tt.name, func(t *testing.T) { ... })`) as the default testing idiom for Go.

---

## 6. Tools & Automation Catalog

The scripts below can be copied directly into `./tools/` (or downloaded from [mail_cli/tools/](https://github.com/anomalet/mail_cli/tree/main/tools)):

### A. Verify Build (`tools/verify_build.sh`)
```bash
#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

echo "1. Formatting code..."
go fmt ./...

echo "2. Static analysis..."
go vet ./...

echo "3. Running tests..."
go test -v -race ./...

echo "4. Compiling binary..."
if [ -d "cmd" ]; then
    go build ./cmd/...
else
    go build .
fi

echo "Verification completed successfully!"
```

### B. Audit File Lines (`tools/audit_lines.sh`)
```bash
#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
MAX_LINES=500
EXCEED=0

echo "Auditing Go source files line counts (limit: ${MAX_LINES} lines)..."
echo "-------------------------------------------------------------"

while IFS= read -r file; do
    count=$(wc -l < "$file")
    if [ "$count" -gt "$MAX_LINES" ]; then
        echo -e "\033[31m[!] $file: $count lines (exceeds $MAX_LINES)\033[0m"
        EXCEED=$((EXCEED + 1))
    else
        echo "    $file: $count lines"
    fi
done < <(find . -name "*.go" -not -path "*/vendor/*" -not -path "*/.*")

echo "-------------------------------------------------------------"
if [ "$EXCEED" -gt 0 ]; then
    echo "Audit finished: $EXCEED file(s) exceed the 500-line soft limit."
    exit 1
else
    echo "Audit finished: All files are within the 500-line limit."
fi
```

### C. Version Bump & Auto-Commit (`tools/bump_version.sh`)
```bash
#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

# List of candidate files for version constant
VERSION_FILES=(
    "app/constants.go"
    "version/version.go"
    "internal/version/version.go"
    "version.go"
)

# Find the first existing version file
CONST_FILE=""
for file in "${VERSION_FILES[@]}"; do
    if [ -f "$file" ]; then
        CONST_FILE="$file"
        break
    fi
done

# If no version file found, create app/constants.go
if [ -z "$CONST_FILE" ]; then
    CONST_FILE="app/constants.go"
    mkdir -p "$(dirname "$CONST_FILE")"
    cat > "$CONST_FILE" << EOF
package app

// Version is the semantic version of the application
const Version = "0.0.1"
EOF
    echo "Created new version file: $CONST_FILE"
fi

echo "Using version file: $CONST_FILE"

CURRENT_VERSION=$(grep -oE '\s*Version\s*=\s*"[0-9]+\.[0-9]+\.[0-9]+"' "$CONST_FILE" | grep -oE '[0-9]+\.[0-9]+\.[0-9]+')
if [ -z "$CURRENT_VERSION" ]; then
    echo "Error: Could not parse Version from $CONST_FILE" >&2
    exit 1
fi

IFS='.' read -r major minor patch <<< "$CURRENT_VERSION"
NEW_VERSION="${major}.${minor}.$((patch + 1))"

sed -i "s/Version\s*=\s*\"$CURRENT_VERSION\"/Version = \"$NEW_VERSION\"/g" "$CONST_FILE"
echo "Version bumped from $CURRENT_VERSION to $NEW_VERSION in $CONST_FILE"

git add "$CONST_FILE"
git commit -m "bump: version $NEW_VERSION"
echo "Committed version bump to git: bump: version $NEW_VERSION"
```

### D. Symbol Indexer (`tools/outline_symbols.sh`)
```bash
#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
echo "Index of Go Types and Functions:"
echo "=================================="
grep -nH -E '^(func|type) ' $(find . -name "*.go" -not -path "*/vendor/*") | sort -t: -k1,1 -k2,2n
echo "=================================="
```

### E. Static Analysis Tool Installer (`tools/install_tools.sh`)
```bash
#!/usr/bin/env bash
set -euo pipefail

tools=(
  "honnef.co/go/tools/cmd/staticcheck@latest"
  "golang.org/x/vuln/cmd/govulncheck@latest"
  "github.com/fzipp/gocyclo/cmd/gocyclo@latest"
  "github.com/securego/gosec/v2/cmd/gosec@latest"
  "golang.org/x/tools/cmd/deadcode@latest"
)

echo "Installing Go analysis tools..."
for tool in "${tools[@]}"; do
  echo "  Installing $(basename "$tool")..."
  go install "$tool"
done
echo "Done."
```

---

## 7. AI Agent Keywords

These keywords can be used to trigger specific workflows when working with AI agents:

- `audit`: Run file line counts, staticcheck, and error propagation checks.
- `harden`: Add edge-case table tests (nil pointers, context cancellation, empty slices, malformed inputs).
- `simplify`: Remove dead code, redundant abstractions, and simplify complex loops/conditionals without changing behavior.