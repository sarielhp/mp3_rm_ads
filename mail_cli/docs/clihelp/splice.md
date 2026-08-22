---
title: mail_cli splice
---

# mail\_cli splice

Move messages from a folder into the keep/YYYY/MM/<folder> structure. The root "keep" is fixed. Use -f to change the target folder name, or -F to change the target folder name and automatically suffix it with the year and month.

## Usage

```
mail_cli splice <folder> [flags]
```

## Parameters

| Parameter | Description |
|-----------|-------------|
| `<folder>` | The source folder name (e.g. "receipts" or "acc1:folder"). |

## Flags

| Flag | Description |
|------|-------------|
| `--move` | Actually move the messages instead of dry run |
| `--copy` | Copy messages instead of moving them |
| `-n <num>` | Number of messages to process |
| `-f, --folder <name>` | Destination folder name (without year/month suffix) |
| `-F, --folder-suffix <name>` | Destination folder name with year/month suffix attached |
| `-Y, --folder-year <name>` | Destination folder name under year directory with year/month suffix attached |
| `--allow` | Allow sourcing from folders that start with keep/ |

## Examples

- `mail_cli splice receipts`
- `mail_cli splice receipts --move`
- `mail_cli splice receipts -f Archive`
- `mail_cli splice receipts -F Receipts`
- `mail_cli splice receipts -n 50 --move`
- `mail_cli splice receipts -p "*order*" --move`

---

[↑ mail\_cli](index.md) — [nav](nav.md)
