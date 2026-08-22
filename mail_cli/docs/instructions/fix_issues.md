# AI Agent Issue Fix Guide

## Overview

This document describes how to process code review issues one by one: deciding whether to fix or skip, planning the fix, implementing it, documenting it, and tracking progress. This is the second phase of the code review workflow (the first phase is documented in `docs/instructions/code_review.md`).

## Prerequisite: Create the issues/ directory

If the `issues/` directory does not exist, create it:

```bash
mkdir -p issues
```

## Workflow Summary

```
For each issue (in order):
  1. Read the issue file
  2. Decide: fix, delete, or defer
  3. If fix: plan → implement → verify → document → rename
  4. Update done.md
  5. Next issue
```

---

## Step 1: Read the Issue

Read the full issue file from `issues/`. Understand:
- What the problem is
- Which file(s) and line(s) are affected
- The suggested fix
- The severity

## Step 2: Decide

### Decision Matrix

| If the issue is... | Then... |
|---|---|
| **A real bug** (crash, data loss, incorrect behavior) | **Fix it** |
| **A project constraint violation** (e.g., `log.Fatalf` in non-main) | **Fix it** |
| **Duplicate code** that can be consolidated | **Fix it** |
| **Repeated boilerplate** that can be extracted into a helper | **Fix it** |
| **Minor style issue** (naming, formatting, color choice) | **Fix it** (quick, low risk) |
| **Architectural debt** (god interface, god package, fundamental redesign needed) | **Delete the issue file** and document why |
| **Long function that needs splitting** | **Delete** — needs planned refactoring, risky to split without tests |
| **Performance optimization** with unclear impact | **Delete** unless benchmarks show it matters |
| **Cosmetic issue** with no correctness impact | **Delete** |
| **Already fixed by a previous issue** | **Delete** (duplicate) |

### Decision Rules

- **If fixing requires changing an interface** that has 3+ implementations (Gmail, JMAP, Outlook, Mock) — think twice. The blast radius is large.
- **If fixing requires changing a function signature** that is called from 15+ places — consider adding a wrapper instead.
- **If the fix is a one-line change** — always do it.
- **If the fix touches 10+ files** — it's probably architectural debt, not a bug fix.
- **If the fix could break existing behavior** and there are no tests — be very careful. Consider not fixing.

### Deleting an Issue

When you delete an issue, **document why** in `done.md`:

```
## 04.md — Global flag variables

**Status:** DELETED — architectural debt. Fixing would require changing 14 globals
used across 20+ files. Works correctly in single-threaded CLI usage.
```

---

## Step 3: Plan the Fix

Before writing any code:

### 3a. Read the Source Code

Read the relevant source files to understand the full context. The issue file may describe a symptom but miss the root cause. Always verify:
- The issue is still present (code may have changed since the issue was written)
- The suggested fix is correct
- There are no side effects

### 3b. Check for Callers

Run `grep` to find all callers of any function you plan to change:

```bash
grep -rn "FunctionName" --include="*.go"
```

### 3c. Check for Tests

Find related tests:

```bash
grep -rn "FunctionName" --include="*_test.go"
```

Read the tests. They may depend on specific error messages, return types, or behavior.

### 3d. Check Imports

If you need to add a new dependency (e.g., `slog`, `context`), check if it's already imported in the file. Be careful about circular imports.

### 3e. Define the Plan

Write down the exact changes needed:
- Which files to modify
- What to change in each file
- Whether to add/remove imports
- Whether to update tests

---

## Step 4: Implement the Fix

### 4a. Make the Changes

Edit files one at a time. For each change:

1. **Read the file first** (the edit tool requires it)
2. Make precise edits (use `edit` tool with exact string matching)
3. Verify after each edit with `go vet ./package/...`

### 4b. Verify After Each File

```bash
go vet ./path/to/package/...
```

If there are LSP errors, fix them immediately.

### 4c. Full Verification

After all changes:

```bash
go vet ./...
go build ./...
go test ./...  # if tests exist and are fast
```

### 4d. Commit Style

When committing, follow the project's commit style (e.g., conventional commits):

```
area: brief description of the change

Optional detailed explanation of what was fixed and why.
```

---

## Step 5: Document the Fix

### 5a. Rename the Issue File

```bash
mv issues/01.md issues/01_handled.md
```

### 5b. Append Handling Description

Edit the handled file to add a "## How it was handled" section at the end:

```markdown
---

## How it was handled

**Fix applied:** [brief description of what was done]

**Files changed:** `file1.go`, `file2.go`

**Verified:** `go vet ./...` and `go build ./...` pass with no errors.
```

The description should include:
- What the root cause was
- What the fix does
- Which files were changed
- Any trade-offs or alternatives considered

---

## Step 6: Update done.md

Append the fix to `issues/done.md`:

```markdown
## 01_handled.md — Short title

**Status:** FIXED

**What:** One-line summary of the problem.

**Fix:** One-line summary of the fix.

**Files changed:** `file1.go`, `file2.go`
```

For deleted issues:

```markdown
## 04.md — Short title

**Status:** DELETED — [reason why it was deleted]
```

### done.md Format

Keep `done.md` as a running log at the top of the file. Each entry should be brief. The file should serve as a quick reference for what was done, not a detailed explanation (the `_handled.md` files have the details).

---

## Step 7: Next Issue

Repeat steps 1-6 for the next issue. **Never fix more than one issue at a time.** This ensures:
- Each fix is properly verified
- If a fix breaks something, it's easy to identify which change caused it
- The documentation is accurate

---

## Common Patterns and Recipes

### Pattern: Inline code → shared helper

1. Check if the duplicate code exists in multiple files (grep)
2. Choose the best location for the helper (the package with the most dependencies)
3. Extract the code into a function
4. Replace all copies with calls to the helper
5. Remove now-unused imports from the callers

### Pattern: log.Fatalf → returned error

1. Change the function signature to return `(..., error)`
2. Replace each `log.Fatalf(...)` with `return ..., fmt.Errorf(...)`
3. Remove unused `"log"` import if no longer needed
4. Update all callers to handle the new error return
5. In `main()`, handle the error with `fmt.Fprintf(os.Stderr, ...)` + `os.Exit(1)`

### Pattern: Global state → per-instance state

1. Define a struct to hold the per-instance state (with its own mutex)
2. Use `sync.Map` keyed by instance identifier to store instances
3. Add a `getInstance(key)` function that returns the instance for a given key
4. Replace all references to globals with `getInstance(key).field`
5. Remove the old global variables

### Pattern: Duplicate function → delegation

1. Identify which implementation is the most complete/sophisticated
2. Replace the simpler implementations with calls to the best one
3. If the callers need different error messages, add a parameter or use a wrapper

### Pattern: Repeated boilerplate → helper function

1. Identify the exact boilerplate pattern (grep for it)
2. Write a helper function that encapsulates the pattern
3. Replace a few callers to demonstrate the pattern
4. Note in the issue that remaining callers should be migrated over time

---

## Quick Reference: Commands

```bash
# Verify a single package
go vet ./path/to/package/...

# Verify everything
go vet ./...

# Build everything
go build ./...

# Run all tests
go test ./...

# Find callers of a function
grep -rn "FunctionName" --include="*.go"

# Find imports in a file
head -30 path/to/file.go

# Check for circular imports
go list -e -json ./... | jq 'select(.Deps | contains(["import-cycle"]))'
```