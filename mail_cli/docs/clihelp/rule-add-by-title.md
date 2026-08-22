---
title: mail_cli rule add-by-title
parent: mail_cli rule
---

# mail\_cli rule add-by-title

Add a server filter rule to auto-label emails with matching subject prefix.

## Usage

```
mail_cli rule add_by_title <title> <lbl>
```

## Parameters

| Parameter | Description |
|-----------|-------------|
| `<title>` | Subject prefix/substring to match. |
| `<lbl>` | The target label/folder hierarchy. |

## Examples

- `mail_cli rule add_by_title "[JIRA]" "Work/Jira"`

---

[↑ mail\_cli](index.md) — [nav](nav.md)
