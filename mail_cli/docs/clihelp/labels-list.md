---
title: mail_cli labels list
has_children: true
parent: mail_cli labels
---

# mail\_cli labels list

List all folders and labels in a hierarchical tree view. Pass --full to show full paths, --ext to show unread and total message counts, --nozero to omit empty folders, or --real to query the server in real-time.

## Usage

```
mail_cli labels list [flags]
```

## Subcommands

| Command | Description |
|---------|-------------|
| [full](labels-list-full.md) | List labels with their full name (without hierarchical view). |

## Flags

| Flag | Description |
|------|-------------|
| `-a, --all` | List all labels, including those with zero messages |
| `-z, --nozero` | Only list folders that have at least one message in them |
| `-f, --full` | Show full label names (without hierarchical view) |
| `-e, --ext` | Show (unread/total) message counts |
| `--real` | Fetch real-time counts from server (bypasses cache) |

## Examples

- `mail_cli labels list`
- `mail_cli labels list --ext`
- `mail_cli labels list --full --ext`
- `mail_cli labels list --nozero`

---

[↑ mail\_cli](index.md) — [nav](nav.md)
