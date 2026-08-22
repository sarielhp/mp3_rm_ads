---
title: mail_cli labels list full
has_children: true
parent: mail_cli labels list
---

# mail\_cli labels list full

List labels with their full name (without hierarchical view).

## Usage

```
mail_cli labels list full [flags]
```

## Subcommands

| Command | Description |
|---------|-------------|
| [ext](labels-list-full-ext.md) | List labels with their full name and (unread/total) message counts. |
| [nozero](labels-list-full-nozero.md) | List non-empty labels with their full name. |

## Flags

| Flag | Description |
|------|-------------|
| `-a, --all` | List all labels, including those with zero messages |
| `-e, --ext` | Show (unread/total) message counts |
| `-z, --nozero` | Only list folders that have at least one message in them |
| `--real` | Fetch real-time counts from server (bypasses cache) |

## Examples

- `mail_cli labels list full`
- `mail_cli labels list full --ext`

---

[↑ mail\_cli](index.md) — [nav](nav.md)
