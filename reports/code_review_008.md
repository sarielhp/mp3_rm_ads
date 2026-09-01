# Code Review 008 - Go Codebase Audit

## 1. API Pagination & Token Caching (pkg/backend)

### Token Caching & Refresh
- **Audiobookshelf (`audiobookshelf.go`)**: The `Request()` method handles HTTP retries but does not handle token expiration. If an API request returns HTTP 401 (Unauthorized), the function fails immediately. It should automatically attempt to re-authenticate via `c.Login()` to refresh the token and then retry the request.
- **PodFetch (`podfetch.go`)**: Uses a static API key or basic auth, so token expiration is not a dynamic issue. However, authentication logic should be consistently applied.

### Pagination
- **Audiobookshelf (`abs_libraries.go`)**: The `Podcasts()` method fetches items via `/api/libraries/%s/items` without explicit pagination. While Audiobookshelf currently returns a list, large libraries could cause performance issues or be implicitly truncated if server-side limits apply. The API supports `?limit=0` or explicit pagination which should be considered.
- **PodFetch (`podfetch_api.go`)**: The `/api/v1/podcasts` endpoint is requested without any pagination parameters. 

## 2. CLI Parameter Validation & Bounds Checking
- **Command Definitions (`cli_server_cmds_extra.go`)**: The subcommands `import` and `export` under `abs server opml` expect a file parameter but do not declare `Args: clihelp.ExactArgs(1)`. While index bounds are protected manually inside the `Run` function via `len(ctx.Args) > 0`, relying on `clihelp` argument validators is safer and automatically generates correct usage errors.
- **Other Commands**: Most other commands properly use `clihelp.ExactArgs(1)` or `clihelp.MaximumNArgs(1)`, combined with inline length checks, preventing panics.

## 3. File Sizing Audit
The codebase has a few files exceeding the 300 lines soft limit (with none exceeding the 600 lines hard limit).
The most notable offender is:
- `tui_transcript_view.go` (455 lines)

**Action**: We will split `tui_transcript_view.go` by extracting the data loading logic (`loadEpisodeAdIntervals`, `loadEpisodeTranscriptData`, etc.) into a new file `tui_transcript_data.go`. This keeps each file well within the 150-300 lines target.

## 4. AGENTS.md Compliance
- **No Comments in Code**: The codebase strictly adheres to the rule. A search for `//` in Go source files found only the `//go:embed VERSION` directive, which is syntactically required by the compiler.
- **Temporary Files**: All intermediate outputs properly use the `.work/` prefix convention.

