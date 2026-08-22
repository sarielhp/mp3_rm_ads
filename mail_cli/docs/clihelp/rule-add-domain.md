---
title: mail_cli rule add-domain
parent: mail_cli rule
---

# mail\_cli rule add-domain

Add an auto-labeling rule for all emails from the sender's domain. Extracts the domain of the sender of the specified cached email and creates a rule to auto-label all emails from that domain.

## Usage

```
mail_cli rule add_domain <message_id> [lbl]
```

## Parameters

| Parameter | Description |
|-----------|-------------|
| `<message_id>` | The message ID or short ID of the email. |
| `[lbl]` | The target label hierarchy (optional; defaults to message folder or SpamLearn folder). |

## Examples

- `mail_cli rule add_domain 12345 "Sort/Newsletters"`
- `mail_cli rule add_domain 12345`

---

[↑ mail\_cli](index.md) — [nav](nav.md)
