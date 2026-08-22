---
title: mail_cli labels
has_children: true
---

# mail\_cli labels

Manage and inspect server labels and folder hierarchies.

## Usage

```
mail_cli labels <subcommand> [args...]
```

## Subcommands

| Command | Description |
|---------|-------------|
| [list](labels-list.md) | List all folders and labels in a hierarchical tree view. Pass --full to show full paths, --ext to show unread and total message counts, --nozero to omit empty folders, or --real to query the server in real-time. |
| [print](labels-print.md) | Print all labels/folders, one per line (full path only). |
| [rename](labels-rename.md) | Rename a label/folder and update all existing routing rules that reference it. |
| [fix](labels-fix.md) | Ensure that all parent folders exist for every nested folder path in the account. |
| [del](labels-del.md) | Delete a label/folder from the server. |
| [search](labels-search.md) | Search cached labels by one or more substring patterns (all must match). |
| [cache](labels-cache.md) | Manage the local labels cache. |
| [create](labels-create.md) | Create a new label or folder path on the server. |

## Examples

- `mail_cli labels list`
- `mail_cli labels list --ext`
- `mail_cli labels search receipts`
- `mail_cli labels create "Receipts/2026"`
- `mail_cli labels rename "OldName" "NewName"`

---

[↑ mail\_cli](index.md) — [nav](nav.md)
