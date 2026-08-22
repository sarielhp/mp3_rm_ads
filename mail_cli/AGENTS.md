# AGENTS.md — mail_cli

## Quick commands

```bash
make                          # build + install to ~/misc/bin_local/
./scripts/verify_build.sh     # format + test + build in one
go vet ./...                  # static analysis (standard Go linter)
go test -v ./...              # run all tests
./scripts/audit_lines.sh      # check 500-line soft limit per file
./scripts/bump.sh              # bump version, commit, tag, push (triggered by "bump")
./scripts/bump_version.sh     # increment patch version in app/constants.go & git commit
```

## Before committing

1. Run `go vet ./...` — no warnings.
2. Run `./scripts/verify_build.sh` — all tests pass, binary compiles.
3. Run `./scripts/audit_lines.sh` — no file exceeds the 500-line soft limit.
4. Run `./scripts/bump_version.sh` — every code change bumps the version (and auto-commits to git).
5. Commit messages follow conventional style: `area: description` or `Type(scope): description`.
   Examples: `tui: fix folder tree alignment`, `feat(cache): add prune subcommand`, `Refactor: decouple cache from activeAccount`.

## Hard constraints

- **500-line soft limit** per `.go` file. Check with `./scripts/audit_lines.sh`.
- **500-line soft limit** per `.go` file. Check with `./scripts/audit_lines.sh`.
- **No `log.Fatalf` in handlers** — use `handleError(err)` (see `config_handlers.go`). Only `main.go` uses `log.Fatalf` for startup-only errors.
- **Config path is fixed** — `~/.config/mail_cli/config.json`. The app auto-migrates from the legacy name `gmail-spam-checker`.
- **Never test `spam del`** — permanently deletes live server messages.
- **Bump version** on every code change via `./scripts/bump_version.sh` (auto-commits version bump).
- **`golangci-lint` is not installed** — use `go vet ./...` for static analysis instead.

## Architecture (what matters for changes)

- **Packages** — `main` (root, CLI handlers), `tui/` (bubbletea model), `uicommon/` (shared UI types/rendering), `app/`, `cache/`, `cli/`, `mailclient/`, etc.
- **`types.go`** — all structs: `Config`, `AccountConfig`, `Rule`, `MailClient` interface. Modify here first when adding fields.
- **`types.go`** — all structs: `Config`, `AccountConfig`, `Rule`, `MailClient` interface. Modify here first when adding fields.
- **`client.go`** — `NewMailClient()` factory + `checkingMailClient` delegate that handles caching per active/inactive account. Add new backends here via the type switch.
- **`gmail_tui_labels.go`** — TUI-specific folder listing (extracted from `gmail_api_labels.go`). TUI code lives in `tui/` package.
- **`uicommon/`** — shared UI types (`FolderEmail`, `FolderTreeNode`, `ColoredString`, `Theme`, etc.) and pure rendering functions used by both `main` and `tui/`.
- **`tui/`** — bubbletea model (`tuiModel`), keyboard handlers, view functions. Imports `uicommon/` for types; uses `Backend` struct for main-package function callbacks.
- **`cache/`** — three subpackages: `cache/msg/` (message body + metadata), `cache/label/` (folder index operations), and top-level `cache/` (read state, `FindCachedEmailByID`). Never use file paths — use `cache.msg.Read`, `cache.msg.Store`, `cache.label.Move`, etc.
- **`cache/msg/`** — `Read`, `Exists`, `Store`, `Delete`, `GetInfo`, `SetClassification`, `ClearClassification`, `ForEachID`. No file paths exposed.
- **`cache/label/`** — `IDs`, `Add`, `Remove`, `ReplaceAll`, `Move`, `HasStructure`. No file paths exposed.
- **`cli_helpers.go`** — shared config read/write helpers (`resolveAccountFromConfig`, `saveConfigFile`, `findAccountLocally`). Handlers should use these, never call `os.UserHomeDir()` directly.
- **`cli/`** — command line interface built with `clihelp`. Commands return `clihelp.Command` instances.
- **Mock (`mailclient/mock.go`)** — `MockMailClient` implements all `MailClient` interface methods. When adding new interface methods to `MailClient` in `client.go`, update `MockMailClient` in `mailclient/mock.go` too.

## Testing

- Tests are in `*_test.go` files alongside the code they test (e.g. `archive/`, `gmail/`, `tui/`).
- `mailclient/mock.go` provides `MockMailClient` implementing the `MailClient` interface.
- `testdata/snapshots/` contains golden files (e.g. `tui` output snapshots).
- `go test -v ./...` runs all tests.

## Scripts catalog

| Script | Purpose |
|---|---|
| `verify_build.sh` | `go fmt` → `go test` → `make` |
| `audit_lines.sh` | flag files > 500 lines |
| `bump_version.sh` | increment patch in `app/constants.go` & git commit |
| `clean_cache.sh` | `mail_cli cache prune --wipe` |
| `outline_symbols.sh` | sorted index of all Go types/functions |
| `show_symbol.sh <sym>` | display a single symbol's code block |
| `generate_config_template.sh` | regenerate `examples/config.json.template` |
| `mcp_github_setup` | configure local GitHub PAT in `.env` via `gh` |

## Config details

- Accounts configured in `~/.config/mail_cli/config.json` under `accounts[]`.
- Each account has `type: "gmail"` or `type: "jmap"`.
- `aliases[]` — symlink-based auto-selection: if binary name matches an alias, that account is auto-selected.
- Env vars override config: `GMAIL_USER`, `GMAIL_PASS`.