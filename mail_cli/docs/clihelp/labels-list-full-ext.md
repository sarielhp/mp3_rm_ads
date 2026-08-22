---
title: mail_cli labels list full ext
has_children: true
parent: mail_cli labels list full
---

# mail\_cli labels list full ext

List labels with their full name and (unread/total) message counts.

## Usage

```
mail_cli labels list full ext [flags]
```

## Subcommands

| Command | Description |
|---------|-------------|
| [real](labels-list-full-ext-real.md) | List labels with real (uncached) counts from the server. |

## Flags

| Flag | Description |
|------|-------------|
| `-a, --all` | List all labels, including those with zero messages |
| `-z, --nozero` | Only list folders that have at least one message in them |
| `--real` | Fetch real-time counts from server (bypasses cache) |

## Examples

- `mail_cli labels list full ext`

---

[↑ mail\_cli](index.md) — [nav](nav.md)
