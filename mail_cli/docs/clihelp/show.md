---
title: mail_cli show
---

# mail\_cli show

Show the contents of emails in folders matching a label prefix, or show a specific email's details and body without running spam checks.

## Usage

```
mail_cli show <lbl_prefix> [message-id] [flags]
```

## Parameters

| Parameter | Description |
|-----------|-------------|
| `<lbl_prefix>` | Prefix of folder/label to inspect. |
| `[message-id]` | Optional message ID or short prefix to show a single message. |

## Flags

| Flag | Description |
|------|-------------|
| `-w, --web` | Open the HTML body of the email in your configured browser |

## Examples

- `mail_cli show receipts`
- `mail_cli show receipts 1234abcd`
- `mail_cli show receipts 1234abcd --web`

---

[↑ mail\_cli](index.md) — [nav](nav.md)
