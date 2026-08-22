---
title: mail_cli spam
has_children: true
---

# mail\_cli spam

Manage Spam folder, train filters, and unsubscribe from political mail.

## Usage

```
mail_cli spam <subcommand>
mail_cli spam <message_id...>
```

## Subcommands

| Command | Description |
|---------|-------------|
| [del](spam-del.md) | Permanently purge all emails in the Spam folder. |
| [bye](spam-bye.md) | Execute a complete sweep: unsubscribe from political lists, train spam filters on all messages in Spam, and purge the Spam folder. |
| [learn](spam-learn.md) | Train Bogofilter on all messages currently in the Spam folder, then move them to the LearnSpam folder. |
| [pol](spam-pol.md) | Manage political spam processing and unsubscribing. |

## Parameters

| Parameter | Description |
|-----------|-------------|
| `<message_id...>` | One or more message IDs to mark as spam. |

## Examples

- `mail_cli spam del`
- `mail_cli spam bye`
- `mail_cli spam learn`
- `mail_cli spam pol audit`
- `mail_cli spam pol unsub`
- `mail_cli spam abc123de`

---

[↑ mail\_cli](index.md) — [nav](nav.md)
