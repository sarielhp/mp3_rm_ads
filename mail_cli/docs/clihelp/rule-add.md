---
title: mail_cli rule add
parent: mail_cli rule
---

# mail\_cli rule add

Add a server filter rule to auto-label emails from a sender.

## Usage

```
mail_cli rule add <email> <lbl>
```

## Parameters

| Parameter | Description |
|-----------|-------------|
| `<email>` | The sender email address to match. |
| `<lbl>` | The target label/folder hierarchy. |

## Examples

- `mail_cli rule add notifications@github.com "Dev/GitHub"`

---

[↑ mail\_cli](index.md) — [nav](nav.md)
