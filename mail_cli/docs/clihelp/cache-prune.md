---
title: mail_cli cache prune
parent: mail_cli cache
---

# mail\_cli cache prune

Prune cached emails and spam scores older than the specified number of days (default: 30). With --wipe, purges all cached messages regardless of age.

## Usage

```
mail_cli cache prune [days] [-w, --wipe]
```

## Parameters

| Parameter | Description |
|-----------|-------------|
| `[days]` | Age threshold in days for pruning cached emails (default: 30). |

## Flags

| Flag | Description |
|------|-------------|
| `-w, --wipe` | Purge all cached email files regardless of age. |

## Examples

- `mail_cli cache prune`
- `mail_cli cache prune 7`
- `mail_cli cache prune --wipe`

---

[↑ mail\_cli](index.md) — [nav](nav.md)
