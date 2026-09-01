# Code Review 001

## 1. Architecture
The Go application (`abs`) is structured around a central processing pipeline that fetches podcasts, manages caching, transcribes audio (using a local or remote Whisper API), and detects ads using an LLM. It includes a robust TUI for user interactions and a CLI for batch processing. The design leverages `bubbletea` for the TUI and a SQLite-backed DB for caching. The architecture is sound but could benefit from tighter modularity (many global-like configs).

## 2. AGENTS.md Compliance
- **No Comments in Code**: **FAIL**. The codebase contains multiple comments across various files (e.g., `ads.go`, `cache_test.go`, `config_test.go`, `debug_test.go`, `docker.go`). This violates the "keep it self-documenting" rule.
- **Line Limits (<300 target / 600 max)**: **PASS (with warnings)**. No files exceed the 600-line hard limit, but 43 files are approaching it (e.g., `tui_screens_test.go` at 595, `batch_proc.go` at 544).
- **Temp Files in `.work/`**: **PASS**. Most audio processing respects the `.work/` directory via `verifyTempFile`, though some cache and lock files use other paths (which is expected for configuration/DB).
- **Stdlib Only**: **FAIL**. The codebase uses external dependencies like `bubbletea`, `lipgloss`, `go-sqlite3`, `clihelp`, `mp3`, and `x/term`. While `AGENTS.md` says "stdlib only", `go.mod` explicitly requires these. We will skip modifying the core dependencies for now, as it would require rewriting the entire TUI and DB layers.
- **Error Handling (No `os.Exit(1)` outside `main()`)**: **FAIL**. Dozens of instances of `os.Exit(1)` exist outside of `main.go` and fatal helpers (e.g., `profiles.go`, `backend_client.go`, `pm_server_exec.go`).

## 3. Bugs, Edge Cases, and Performance
- **Error Handling**: The widespread use of `os.Exit(1)` outside `main()` makes unit testing brittle and prevents the CLI/TUI from gracefully recovering from errors. This is the most critical bug/anti-pattern.
- **Test Coverage**: There is a solid foundation of unit tests (passing), but files with `os.Exit(1)` are hard to cover comprehensively on failure paths.

## 4. Highest Priority Issues
1. **Remove Code Comments**: Strip all comments from `.go` files to strictly comply with `AGENTS.md`, excluding pragmas like `//go:embed`.
2. **Refactor `os.Exit(1)`**: Fix `os.Exit(1)` in critical utility files like `profiles.go` and `backend_client.go` to return standard Go `error` types instead, allowing graceful error handling.

