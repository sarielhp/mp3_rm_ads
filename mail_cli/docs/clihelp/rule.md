---
title: mail_cli rule
has_children: true
---

# mail\_cli rule

Manage custom routing rules and auto-labeling filters.

## Usage

```
mail_cli rule <subcommand> [args...]
```

## Subcommands

| Command | Description |
|---------|-------------|
| [add](rule-add.md) | Add a server filter rule to auto-label emails from a sender. |
| [add-by-title](rule-add-by-title.md) | Add a server filter rule to auto-label emails with matching subject prefix. |
| [add-domain](rule-add-domain.md) | Add an auto-labeling rule for all emails from the sender's domain. Extracts the domain of the sender of the specified cached email and creates a rule to auto-label all emails from that domain. |
| [del](rule-del.md) | Remove an auto-labeling rule. |
| [list](rule-list.md) | List custom routing rules. |
| [delete-all](rule-delete-all.md) | Delete all routing rules for the selected account. |
| [update](rule-update.md) | Ensure all blacklisted senders have a corresponding local rule. |
| [export](rule-export.md) | Export local rules to mail server filters (e.g. Gmail filters or Sieve script). |

## Flags

| Flag | Description |
|------|-------------|
| `-export <file>` | Export all existing rules to a file |
| `-import <file>` | Import rules from a file |

## Examples

- `mail_cli rule list`
- `mail_cli rule add notifications@github.com "Dev/GitHub"`
- `mail_cli rule add_by_title "[ALERT]" "Alerts"`
- `mail_cli rule export`

---

[↑ mail\_cli](index.md) — [nav](nav.md)
