---
title: mail_cli whitelist
has_children: true
---

# mail\_cli whitelist

Manage the personal sender whitelist to bypass spam checks.

## Usage

```
mail_cli whitelist <subcommand> [args...]
```

## Subcommands

| Command | Description |
|---------|-------------|
| [add](whitelist-add.md) | Add one or more email addresses to the whitelist. |
| [del](whitelist-del.md) | Remove one or more email addresses from the whitelist. |
| [list](whitelist-list.md) | List all whitelisted email addresses. |

## Examples

- `mail_cli whitelist list`
- `mail_cli whitelist add friend@example.com`

---

[↑ mail\_cli](index.md) — [nav](nav.md)
