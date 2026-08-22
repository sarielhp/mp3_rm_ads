---
title: mail_cli account new
parent: mail_cli account
---

# mail\_cli account new

Add a new account template to config.json.

## Usage

```
mail_cli account new <jmap|gmail|outlook> [name] [flags]
```

## Parameters

| Parameter | Description |
|-----------|-------------|
| `<jmap|gmail|outlook>` | Account backend provider type. |
| `[name]` | Unique name for the account. |

## Flags

| Flag | Description |
|------|-------------|
| `--type <type>` | Account type: regular or test |
| `--test` | Mark as test account (shortcut for --type test) |

## Examples

- `mail_cli account new personal-gmail`
- `mail_cli account new work-fastmail jmap`

---

[↑ mail\_cli](index.md) — [nav](nav.md)
