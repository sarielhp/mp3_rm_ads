# AI Agent Code Review Guide

## Overview

This document describes how to perform a systematic code review of a Go codebase using the `issues/` subdirectory workflow. The goal is to identify, document, prioritize, and (optionally) fix code quality issues.

## Workflow Summary

```
1. Explore → 2. Analyze → 3. Write issues → 4. Prioritize → 5. Report → 6. (Optional) Fix
```

---

## Phase 1: Exploration

First, build a complete picture of the codebase:

### 1a. Directory Structure

Get the full directory tree. Identify:
- All packages and their files
- The entry point (usually `main.go` or `cmd/`)
- Test files vs production files
- Config files, scripts, docs

### 1b. File Sizes

Count lines per file. Note files that exceed `SOFT_LIMIT` (usually 500 lines). These are candidates for splitting.

### 1c. Build System

Read `go.mod` — understand:
- Module name and Go version
- External dependencies and their purpose
- Test framework (if any custom test harness)

Read `Makefile` / build scripts.

### 1d. Project Conventions

Read `AGENTS.md`, `CONTRIBUTING.md`, or similar:
- Coding standards
- Linting requirements
- Testing requirements
- Any hard constraints (e.g., "no log.Fatalf in handlers", "500-line soft limit")

---

## Phase 2: Deep Analysis

Read every `.go` file in the codebase. Focus on these categories:

### 2a. Bugs and Correctness

Look for:

| Pattern | What to check |
|---------|---------------|
| **Ignored errors** | `_ = funcCall()` — especially in cache writes, file I/O, subprocess calls |
| **Nil pointer dereferences** | Variables declared nil, then used conditionally |
| **Wrong error variable** | `err` from a previous call used instead of from the current call |
| **Data races** | Package-level mutable state accessed without synchronization |
| **Off-by-one** | Slice bounds, loop conditions |
| **Resource leaks** | Goroutines that never terminate, files not closed, HTTP bodies not drained |
| **Shadowed variables** | `:=` creating a new `err` in nested scope while outer `err` is stale |
| **Incorrect string comparison** | Case-sensitive comparison of user input, email addresses, paths |
| **Slice modification during iteration** | `for i, e := range slice { slice = append(slice[:i], slice[i+1:]...) }` |

### 2b. Code Duplication

Look for repeated patterns:

| Pattern | What to check |
|---------|---------------|
| **Copy-paste functions** | Same logic with different variable names |
| **Duplicate implementations** | Same algorithm in multiple files (label resolution, email parsing, base64 decoding, pattern matching) |
| **Repeated boilerplate** | Same 5-line block appearing 15+ times (e.g., get client + validate) |
| **Duplicate constants** | Same constants defined in multiple packages with different values |
| **Duplicate interface definitions** | Same method signatures redefined as inline anonymous interfaces in multiple files |

### 2c. Interface Design

| Pattern | What to check |
|---------|---------------|
| **God interfaces** | Interfaces with 20+ methods — every implementor must implement all |
| **Unused parameters** | Parameters in interface methods that all implementations ignore |
| **Misplaced methods** | Methods on the wrong interface (e.g., `SendEmail` on `RuleManager`) |
| **Inline anonymous interfaces** | Interfaces redefined per-function instead of using shared ones |
| **Function-pointer structs** | Structs wrapping a single function field + constructor + delegation method — use closures directly |
| **Hidden coupling via type aliases** | `type Config = otherpkg.Config` creating invisible dependencies |

### 2d. Structural Issues

| Pattern | What to check |
|---------|---------------|
| **God packages** | Packages with 5+ unrelated responsibilities |
| **Circular dependencies** | Package A imports B, B imports A |
| **Global mutable state** | Package-level vars, global flag variables |
| **Dual architectures** | Two parallel systems for the same purpose (old + new), both in use |
| **Unclear package boundaries** | Two packages with overlapping responsibilities |
| **Dead code** | Exported functions never called, empty files, empty directories |

### 2e. Consistency Issues

| Pattern | What to check |
|---------|---------------|
| **Mixed error handling** | Some functions wrap with `%w`, some with `%v`, some use `log.Fatalf`, some discard |
| **Mixed logging** | Structured (`slog`) vs unstructured (`fmt.Printf`) used inconsistently |
| **Inconsistent naming** | camelCase vs PascalCase for the same category of things |
| **Inconsistent parameter order** | Similar functions taking parameters in different orders |
| **Mixed test package styles** | Some tests use `package foo`, others `package foo_test` |

### 2f. Efficiency

| Pattern | What to check |
|---------|---------------|
| **Multiple disk reads** | Same file read 2-3 times in the same operation |
| **Unnecessary allocations** | Creating new objects in loops when they could be reused |
| **O(n²) where O(n) works** | Linear scans that could use a map |
| **Loading everything into memory** | Collecting all items before processing instead of streaming |
| **Repeated computations** | Same value computed in multiple places instead of cached |

### 2g. Project Constraint Violations

Check AGENTS.md or similar for explicit rules:
- "No log.Fatalf in handlers" — verify compliance
- "500-line soft limit" — flag violations
- "Never test X" — ensure tests don't exist
- "Bump version on every change" — check version constant

### 2h. Long Functions

Flag any function longer than 80 lines. For each:
- Report line count
- List all distinct responsibilities within the function
- Suggest how to split it

Focus especially on functions >150 lines — these are almost always too long.

---

## Phase 3: Write Issue Files

Create `issues/01.md`, `02.md`, etc. Each file should contain:

```markdown
# NN — Short descriptive title

**File:** `path/to/file.go:start_line-end_line`

**Severity:** CRITICAL | HIGH | MEDIUM | LOW

Description of the problem, including relevant code snippets.

**Fix:** Specific, actionable instructions for how to fix it.
```

### Severity Guidelines

| Severity | Criteria |
|----------|----------|
| **CRITICAL** | Causes crashes, data loss, security issues, or violates explicit project constraints |
| **HIGH** | Incorrect behavior, data corruption risk, major maintainability blockers |
| **MEDIUM** | Duplicated code, long functions, efficiency issues, style inconsistencies |
| **LOW** | Naming issues, minor style problems, documentation gaps |

### Numbering Order

Number issues so that the most important ones come first:
1. CRITICAL bugs first
2. HIGH severity issues
3. MEDIUM severity (actual bugs before style)
4. LOW severity

---

## Phase 4: Report

Once all issues are written, display a detailed report:

```
## Code Review Report — N Issues Found

### CRITICAL (fix immediately)
| # | Issue | File |
|---|-------|------|

### HIGH (fix soon)
...

### MEDIUM
...

### LOW
...
```

---

## Phase 5: (Optional) Fix Issues

For each issue in order:

### 5a. Read the issue file
### 5b. Decide: fix or delete?
- **If unimportant** (cosmetic, won't improve correctness or maintainability): delete the file
- **If fixable**: proceed to 5c
- **If architectural debt** (needs planned refactoring, too risky for quick fix): delete the file but document the reasoning

### 5c. Plan the fix
- Read the relevant source code
- Understand the full context (not just the issue description)
- Verify the issue is still accurate
- Plan the exact changes needed

### 5d. Execute the fix
- Make the code changes
- Run `go vet ./...` and `go build ./...` after each change
- Verify with tests if applicable

### 5e. Document
- Rename `issues/NN.md` → `issues/NN_handled.md`
- Append a "## How it was handled" section describing what was done and which files changed

### 5f. Update done.md
Append the fix to `issues/done.md` with status and files changed.

---

## Common Pitfalls to Avoid

1. **Don't fix everything** — architectural debt is real, document it and move on
2. **Don't change interfaces** without checking all implementors
3. **Don't remove exported functions** without checking all callers (including tests)
4. **Don't introduce circular dependencies** when moving code between packages
5. **Always verify with `go vet`** after changes
6. **One issue at a time** — don't batch fixes unless they're trivially related
7. **Read the full function** before deciding it's too long — some 80-line functions are fine if they do one thing clearly
8. **Check for existing tests** before changing function signatures — test may depend on specific error messages
