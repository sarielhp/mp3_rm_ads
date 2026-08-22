---
title: mail_cli cache
has_children: true
---

# mail\_cli cache

Manage the local email download cache.

## Usage

```
mail_cli cache <subcommand> [args...]
```

## Subcommands

| Command | Description |
|---------|-------------|
| [prune](cache-prune.md) | Prune cached emails and spam scores older than the specified number of days (default: 30). With --wipe, purges all cached messages regardless of age. |
| [reset](cache-reset.md) | Recreate the cache directory for the current account, wiping all cached data. |

## Examples

- `mail_cli cache prune`
- `mail_cli cache prune 7`
- `mail_cli cache prune --wipe`
- `mail_cli cache reset`

---

[↑ mail\_cli](index.md) — [nav](nav.md)
