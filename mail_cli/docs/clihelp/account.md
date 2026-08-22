---
title: mail_cli account
has_children: true
---

# mail\_cli account

Manage and list configured mail accounts.

## Usage

```
mail_cli account <subcommand> [args...]
```

## Subcommands

| Command | Description |
|---------|-------------|
| [list](account-list.md) | List all configured accounts and their properties. |
| [new](account-new.md) | Add a new account template to config.json. |
| [associate](account-associate.md) | Associate a program/symlink name with an account for automatic account selection. |
| [test](account-test.md) | Test validation and server connection for an account. |
| [calendar](account-calendar.md) | Designate or show the calendar manager account. |
| [login](account-login.md) | Perform interactive OAuth login for a Gmail or Outlook account. |
| [rename](account-rename.md) | Rename an existing account's display name. |
| [delete](account-delete.md) | Delete an existing account and its credentials. |

## Examples

- `mail_cli account list`
- `mail_cli account new personal-gmail`
- `mail_cli account test`
- `mail_cli account rename old-gmail new-gmail`
- `mail_cli account delete temp-account`

---

[↑ mail\_cli](index.md) — [nav](nav.md)
