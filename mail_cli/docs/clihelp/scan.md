---
title: mail_cli scan
---

# mail\_cli scan

Scan all folders starting with the given label prefix (case-insensitive) for spam.

## Usage

```
mail_cli scan <lbl_prefix> [flags]
```

## Parameters

| Parameter | Description |
|-----------|-------------|
| `<lbl_prefix>` | The prefix of the label/folder to scan (e.g. 'inbox' or 'receipts'). |

## Flags

| Flag | Description |
|------|-------------|
| `-m, --move [From]` | Move identified spam emails to Spam folder. Optional: specify From address to move a single unique message. |
| `--inbox-move <From>` | Move identified emails from a specific From address back to the Inbox folder. |

## Examples

- `mail_cli scan inbox`
- `mail_cli scan inbox -m`
- `mail_cli scan receipts -m spammer@example.com`

---

[↑ mail\_cli](index.md) — [nav](nav.md)
