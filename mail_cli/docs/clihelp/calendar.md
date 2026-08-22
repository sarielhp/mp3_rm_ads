---
title: mail_cli calendar
has_children: true
---

# mail\_cli calendar

Manage calendar events extracted from email attachments.

## Usage

```
mail_cli calendar <subcommand> [args...]
```

## Subcommands

| Command | Description |
|---------|-------------|
| [add](calendar-add.md) | Add a calendar event from an .ics attachment in a specific email. |
| [week](calendar-week.md) | Show all calendar events in the upcoming week. |
| [add-all](calendar-add-all.md) | Scan inbox for .ics attachments and add them to calendar. |

## Examples

- `mail_cli calendar week`
- `mail_cli calendar add 1234abcd`
- `mail_cli calendar add-all`

---

[↑ mail\_cli](index.md) — [nav](nav.md)
