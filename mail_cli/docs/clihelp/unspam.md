---
title: mail_cli unspam
has_children: true
---

# mail\_cli unspam

Mark a message as not being spam: train bogofilter as ham and move it from Spam back to Inbox on the server.

## Usage

```
mail_cli unspam <message_id...>
  mail_cli unspam folder <folder_name>
```

## Subcommands

| Command | Description |
|---------|-------------|
| [folder](unspam-folder.md) | Mark all messages in the specified folder as ham and move them back to Inbox. |

## Parameters

| Parameter | Description |
|-----------|-------------|
| `<message_id...>` | One or more message IDs to unspam (short 8-char or full). |

## Examples

- `mail_cli unspam abc123de`
- `mail_cli unspam folder Spam`

---

[↑ mail\_cli](index.md) — [nav](nav.md)
