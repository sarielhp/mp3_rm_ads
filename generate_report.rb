#!/usr/bin/env ruby

content = <<~MD
# Code Review Report (007)

## 1. Docker Container Logging & Progress Tracking

**Current State**:
- `docker.go` contains functions to fetch and parse Whisper docker container logs for progress reporting (`pollWhisperDockerProgress`, `fetchDockerLogs`, `detectWhisperDockerContainer`).
- `transcribe.go` only outputs elapsed time for progress and currently ignores the `dockerContainer` variable. It does not invoke `pollWhisperDockerProgress` to fetch actual progress (e.g., percentage or time decoded).
- `docker.go` also improperly hosts `fetchOpenRouterModels` and string utilities (e.g., `splitLines`, `toLower`), breaking separation of concerns.

**Recommendations**:
- **Split `docker.go`**: Move Whisper-specific Docker log parsing into `whisper_docker.go`. Move LLM (`OpenRouter`) fetching into an appropriate LLM/config file. Move string manipulation helpers to `utils.go` or equivalent.
- **Integrate Progress Tracking**: Update the background progress goroutine in `transcribe.go` to optionally use `pollWhisperDockerProgress` if `dockerContainer` is provided. If Docker is unavailable or fails, it should seamlessly fall back to just reporting elapsed time.

## 2. OPML Import/Export and XML/RSS Parsing Edge Cases

**Current State**:
- `pm_get_info.go` and `backend_client.go` handle HTTP requests (e.g., fetching RSS feeds).
- Hardcoded timeouts (e.g., `10 * time.Second` in `fetchSimplecastEpisodes`) can fail for slow RSS feeds or large XMLs.

**Recommendations**:
- **HTTP Timeouts**: Replace hardcoded timeouts with a robust timeout strategy, preferably controlled by config or increased to accommodate slow connections (e.g., 30s+ for large OPML or RSS parsing).
- **Error Handling**: Properly handle XML parsing failures and edge cases (empty elements, missing required tags) without crashing.

## 3. Oversized Files (>330 Lines)

The following files exceed the 330-line soft limit and must be split for better maintainability and AI context sizes:
- `cli_server_cmds.go`
- `main_cli_test.go`
- `main_test.go`
- `tui_episode_view.go`
- `tui_data.go`
- `config_test.go`

**Recommendations**:
- Split `cli_server_cmds.go` into functional command groups (e.g., `cli_server_opml.go`).
- Split tests logically based on the systems under test.
- Divide TUI views/data by component to adhere to the 150-300 line target.

## 4. AGENTS.md Compliance (No Comments in Code)

**Current State**:
- Code audit confirmed that the codebase strictly adheres to the rule of "no comments in code". The only remaining comments are system directives like `//go:embed VERSION` which are necessary for the Go compiler.

**Recommendations**:
- Continue enforcing the zero-comments rule for newly generated code and splits.
MD

File.write("reports/code_review_007.md", content)
