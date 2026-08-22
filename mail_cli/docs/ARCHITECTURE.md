# Architecture & Developer Guide

Quick-reference for anyone maintaining or modifying the `mail_cli` codebase.

---

## 📁 File Layout (all `package main`, flat directory)

**Why flat?** Splitting into subdirectories (e.g. `gmail/`, `jmap/`) would either break the `package main` constraint or create circular dependencies, since types, UI, and config helpers are shared across all backends.

### Entry Point
| File | Purpose |
|---|---|
| `main.go` | CLI entrypoint, config loading, pre-flight checks, version banner |

### Core Types & Config
| File | Purpose |
|---|---|
| `types.go` | All struct definitions: `Config`, `AccountConfig`, `Rule`, `FileConfig`, `MailClient` interface |
| `config.go` | Config file bootstrapping, default value auto-population, JSON schema merging |

### CLI Infrastructure
| File | Purpose |
|---|---|
| `cli/cli.go` | Root command setup, persistent flags, subcommand registration via `clihelp` |
| `cli/accounts.go` | `account list/new/associate/test` commands |
| `cli/labels.go` | `labels list/rename/fix/del` commands |
| `cli/rules.go` | `rule list/add/del/export/import` commands, Sieve script generation |
| `cli/spam.go` | `spam` commands (learn, political, del, etc.) |

### Config Handlers (shared by multiple commands)
| File | Purpose |
|---|---|
| `config_handlers.go` | `handleBlacklistCommand`, `handleWhitelistCommand` (shared helper functions) |
| `config_rule_handlers.go` | `handleRuleCommand` (Gmail filter check, rule persistence) |

### Client Abstraction Layer
| File | Purpose |
|---|---|
| `client.go` | `NewMailClient()`, `MailClient` interface definition, account selection logic |
| `client_resolve.go` | `resolveLabel()` for fuzzy suffix label matching |

### Gmail REST API Backend
| File | Purpose |
|---|---|
| `gmail_client.go` | `GmailClient` struct, `NewGmailClient()`, auth setup, IMAP fallback |
| `gmail_api.go` | Core API: `GetEmails()`, `GetEmailByID()`, `Validate()`, `CheckCapabilities()` |
| `gmail_api_download.go` | Email downloading from Gmail REST API |
| `gmail_api_labels.go` | `ListLabels()`, `RenameLabel()`, `DeleteLabel()`, `FixLabels()` |
| `gmail_api_rules.go` | Gmail filter listing, label ID resolution for rule export detection |
| `gmail_api_rules_export.go` | Creating/deleting Gmail server-side filters (export/import rules) |
| `gmail_api_spam.go` | Spam folder scanning, Rspamd integration |
| `gmail_api_spam_learn.go` | Training Rspamd on learned spam |
| `gmail_api_spam_political.go` | Political donation detection scoring |
| `gmail_api_helpers.go` | Scope error handling, label name→ID resolution |
| `gmail_api_runtests.go` | Integration tests for Gmail REST API flow |

### JMAP Backend (FastMail)
| File | Purpose |
|---|---|
| `jmap_client.go` | `JMAPClient` struct, `NewJMAPClient()`, session auth |
| `jmap_client_ops.go` | Core JMAP methods: `Email/Get`, `Mailbox/Get`, `Mailbox/Set`, `Email/Set`, `Email/Delete` |
| `jmap_client_ops2.go` | Additional JMAP: `EmailSubmission/Set`, mailbox creation |
| `jmap_client_misc.go` | `Validate()`, `ListLabels()`, `FixLabels()`, `RenameLabel()`, `DeleteLabel()`, `LearnSpam()` |

### Heuristics & Scoring
| File | Purpose |
|---|---|
| `heuristics.go` | Unicode script auditing (`isRuneAllowed`), NLP cleaning, language whitelisting, political donation scoring |
| `heuristics_test.go` | 44 unit tests covering all heuristics, sieve sanitization, account validation |

### UI & Output
| File | Purpose |
|---|---|
| `ui.go` | Terminal color definitions, ANSI formatting, print helpers |

### Scanner Runtime
| File | Purpose |
|---|---|
| `main_scan.go` | Core `performScan()` loop, spam detection flow |
| `main_scan_other.go` | Non-Linux platform scan helpers |
| `main_show.go` | `show` command: display cached email bodies |
| `main_spam.go` | `spam` command: spam folder auditing and Rspamd integration |

---

## 🏗️ High-Level Architecture

```
main.go
  └── cli/cli.go (clihelp App setup)
       ├── cli/*.go  (all user-facing commands)
```

Config: ~/.config/mail_cli/config.json
  ├── Legacy top-level fields (single-account, IMAP-only, kept for backward compat)
  └── accounts[] (multi-account, each with type "gmail" or "jmap")

Client Dispatch:
  ┌─────────────────────────────────────────────────────┐
  │ client.go: NewMailClient(account, config)           │
  │   → returns MailClient interface                    │
  │     if account.Type == "gmail" → *GmailClient       │
  │     if account.Type == "jmap"  → *JMAPClient        │
  └─────────────────────────────────────────────────────┘

Scan Flow:
  performScan(client, label, moveSpam, moveInbox)
    → client.GetEmails(label, limit)
    → for each email: heuristics check
      → isRuneAllowed (Unicode script audit)
      → cleanTextForNLP + isDetectedLanguageWhitelisted (whatlanggo)
      → detectPolitical (weighted scoring)
      → Rspamd check (if Rspamd controller available)
    → move to spam if score >= threshold
```

## 🔑 Key Interfaces

### `MailClient` (types.go)
```go
type MailClient interface {
    Config() *Config
    Validate() error
    GetEmails(label string, limit int) ([]Email, error)
    GetEmailByID(id string) (*Email, error)
    MoveEmail(email *Email, destLabel string) error
    DeleteEmail(email *Email) error
    ListLabels() error
    FixLabels() error
    RenameLabel(oldName, newName string) error
    DeleteLabel(labelName string) error
    LearnSpam(email *Email) error
    ExportRules() error
}
```

## 📋 Config Schema Reference

### Top-Level Fields (legacy single-account)
| Field | Type | Default | Notes |
|---|---|---|---|
| `username` | string | — | Email address (Gmail or JMAP) |
| `password` | string | — | App password (Gmail) or JMAP API token |
| `imap_host` | string | — | IMAP server (required for `type: "gmail"`) |
| `download_dir` | string | `~/.cache/gmail-spam-checker` | Email cache directory |
| `limit` | int | 1000 | Max emails to scan |
| `score_threshold` | float64 | 0 | Rspamd spam score threshold |
| `spam_folder` | string | — | Target spam folder name |
| `received_folder` | string | — | Inbox/received folder name |
| `allowed_languages` | []string | — | Whitelisted languages for script audit |
| `block_political` | bool | true | Enable political donation detection |
| `auto_unsubscribe` | bool | false | Auto-send List-Unsubscribe opt-outs |
| `accounts` | []AccountConfig | — | Multi-account entries |

### `AccountConfig` (within `accounts[]`)
| Field | Type | Required | Notes |
|---|---|---|---|
| `name` | string | Yes | Account identifier |
| `type` | string | Yes | `"gmail"` or `"jmap"` |
| `username` | string | Yes | Email address |
| `password` | string | Yes | App password / JMAP token |
| `imap_host` | string | Gmail only | e.g. `imap.gmail.com:993` |
| `session_url` | string | JMAP only | e.g. `https://api.fastmail.com/jmap/session` |
| `spam_folder` | string | Yes | Spam folder name for this account |
| `received_folder` | string | Yes | Inbox name for this account |
| `aliases` | []string | No | CLI command aliases (e.g. `["gmail", "personal"]`) |
| `whitelist` | []string | No | Bypass-all trusted senders |
| `blacklist` | []string | No | Instantly-flagged senders |
| `rules` | []Rule | No | Routing rules (sender → label) |

### `Rule` (within a Rule object)
| Field | Type | Notes |
|---|---|---|
| `sender` | string | Email address to match |
| `label` | string | Target label/folder to route to |
| `exported` | bool | Whether the rule has been pushed to the server (Gmail only) |

## 🔧 Adding a New Backend

1. Define backend struct in new file `backend_api.go` implementing `MailClient` interface
2. Register in `client.go`'s `NewMailClient()` type switch
3. Implement all interface methods; unused ones can return `NotImplementedError`
4. Add any new subcommands to `cli_commands_*.go`
5. Update `types.go` if new config fields are needed

## 🐛 Common Troubleshooting

| Symptom | Cause | Fix |
|---|---|---|
| `log.Fatalf` prints raw stack trace | Missing error handling in handler | Replace `log.Fatalf()` with `handleError(err)` |
| "no accounts configured" after adding account | Config not saved or path wrong | Verify `~/.config/mail_cli/config.json` exists with valid JSON |
| "credentials.json missing" for Gmail | Gmail REST API requires OAuth credentials | Download from Google Cloud Console → save to config dir |
| JMAP commands fail with `unknownMethod` | Server doesn't support JMAP Mail Filters/Sieve | Use `--sieve` flag to export rules for manual upload to FastMail web UI |
| Rules not appearing in Gmail `rule list` | `exported: true` already set in config | Edit config.json to set `exported: false` or run `rule export force` |
| Account selection not working | Account name case mismatch | Account lookup uses `strings.EqualFold` (case-insensitive) |

## 📦 Dependencies

| Package | Version | Purpose |
|---|---|---|
| `github.com/sarielhp/clihelp` | v0.2.11 | CLI command parser, usage & markdown generator |
| `github.com/fatih/color` | v1.19.0 | Terminal color output |
| `github.com/abadojack/whatlanggo` | v1.0.1 | Statistical NLP language detection |
| `google.golang.org/api/gmail/v1` | latest | Gmail REST API client |
| `git.sr.ht/~rockorager/go-jmap` | v0.5.3 | JMAP protocol client |

## ✅ Maintenance Checklist (when modifying code)

1. **File size**: No file should exceed 500 lines. If it does, split it.
2. **No `log.Fatalf` in handlers**: Use `handleError(err)` for clean error output.
3. **Use shared helpers**: `resolveAccountFromConfig()`, `saveConfigFile()`, `findAccountLocally()` for all config read/write.
4. **Run tests**: `go test -v` should pass. Add tests for new public functions.
5. **Version bump**: Increment `Version` in `main.go` by `0.0.1`.
6. **Changelog**: Add entry to `CHANGELOG.md` under current version.
7. **Build verification**: Run `make` and confirm binary compiles and copies to `~/misc/bin_local/`.
8. **Flat structure**: Keep all files flat in `/app/`. Never create subdirectories.
