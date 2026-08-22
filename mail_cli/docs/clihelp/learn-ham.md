---
title: mail_cli learn-ham
---

# mail\_cli learn-ham

Train Bogofilter on ham (non-spam) emails in a folder. The folder must be an exact match and cannot have subfolders.

## Usage

```
mail_cli learn-ham <label> [flags]
```

## Parameters

| Parameter | Description |
|-----------|-------------|
| `<label>` | The exact folder/label name to learn as ham. |

## Flags

| Flag | Description |
|------|-------------|
| `--force` | Bypass trained message database and re-train all emails |
| `--batch` | Process messages in larger batches for speed. |
| `--rescan` | Rescan all messages in folder regardless of cache status. |

## Examples

- `mail_cli learn-ham INBOX`
- `mail_cli learn-ham INBOX --force`

---

[↑ mail\_cli](index.md) — [nav](nav.md)
