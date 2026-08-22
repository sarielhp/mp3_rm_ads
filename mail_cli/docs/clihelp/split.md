---
title: mail_cli split
---

# mail\_cli split

Scan messages in the source label. If their subject matches the pattern (which may contain wildcards * and ?), move them to the target label. Runs in dry-run mode by default; use --do to perform actual operations.

## Usage

```
mail_cli split <source_label> <pattern> <target_label> [--do]
```

## Parameters

| Parameter | Description |
|-----------|-------------|
| `<source_label>` | Source folder/label containing messages to split. |
| `<pattern>` | Subject pattern with wildcards (*, ?) to match. |
| `<target_label>` | Destination folder/label to move matched messages to. |

## Flags

| Flag | Description |
|------|-------------|
| `--do` | Perform the actual move operations |

## Examples

- `mail_cli split Inbox "*invoice*" Receipts`
- `mail_cli split Inbox "*invoice*" Receipts --do`

---

[↑ mail\_cli](index.md) — [nav](nav.md)
