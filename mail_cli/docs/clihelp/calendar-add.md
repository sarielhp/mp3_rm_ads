---
title: mail_cli calendar add
parent: mail_cli calendar
---

# mail\_cli calendar add

Add a calendar event from an .ics attachment in a specific email.

## Usage

```
mail_cli calendar add [label_prefix] <message_id>
```

## Parameters

| Parameter | Description |
|-----------|-------------|
| `[label_prefix]` | Label prefix to locate the message (defaults to inbox). |
| `<message_id>` | Message ID or prefix containing the .ics attachment. |

## Examples

- `mail_cli calendar add 1234abcd`
- `mail_cli calendar add Receipts 5678efgh`

---

[↑ mail\_cli](index.md) — [nav](nav.md)
