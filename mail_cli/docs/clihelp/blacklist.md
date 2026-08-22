---
title: mail_cli blacklist
has_children: true
---

# mail\_cli blacklist

Manage the personal sender blacklist to instantly classify messages as spam.

## Usage

```
mail_cli blacklist <subcommand> [args...]
```

## Subcommands

| Command | Description |
|---------|-------------|
| [add](blacklist-add.md) | Add one or more email addresses to the blacklist. |
| [del](blacklist-del.md) | Remove one or more email addresses from the blacklist. |
| [list](blacklist-list.md) | List all blacklisted email addresses. |

## Examples

- `mail_cli blacklist list`
- `mail_cli blacklist add spammer@example.com`

---

[↑ mail\_cli](index.md) — [nav](nav.md)
