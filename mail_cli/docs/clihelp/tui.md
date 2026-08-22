---
title: mail_cli tui
---

# mail\_cli tui

Open the interactive terminal email browser. With an optional label_prefix argument, open the TUI with the matching label as the initial folder. The prefix is matched case-insensitively as a substring against the full label path. If exactly one label matches, the TUI opens on that label. If multiple match, all matching labels are printed and the program exits.

## Usage

```
mail_cli tui [label_prefix]
```

## Parameters

| Parameter | Description |
|-----------|-------------|
| `[label_prefix]` | Optional folder/label prefix to open directly into. |

## Examples

- `mail_cli tui`
- `mail_cli tui receipts`

---

[↑ mail\_cli](index.md) — [nav](nav.md)
