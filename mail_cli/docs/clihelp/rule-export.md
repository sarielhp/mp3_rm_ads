---
title: mail_cli rule export
parent: mail_cli rule
---

# mail\_cli rule export

Export local rules to mail server filters (e.g. Gmail filters or Sieve script).

## Usage

```
mail_cli rule export [force] [flags]
```

## Parameters

| Parameter | Description |
|-----------|-------------|
| `[force]` | Optional force flag to overwrite existing remote filters. |

## Flags

| Flag | Description |
|------|-------------|
| `--sieve <file>` | Export rules as a Sieve script to file path |
| `-f, --force` | Force overwrite conflicting remote filters |

## Examples

- `mail_cli rule export`
- `mail_cli rule export --sieve ~/rules.sieve`

---

[↑ mail\_cli](index.md) — [nav](nav.md)
