---
title: mail_cli config
has_children: true
---

# mail\_cli config

Show or manage configuration options.

## Usage

```
mail_cli config [subcommand]
```

## Subcommands

| Command | Description |
|---------|-------------|
| [show](config-show.md) | Display the active configuration values. |
| [set](config-set.md) | Set a configuration parameter. |
| [reset](config-reset.md) | Reset a configuration parameter to its default value. |
| [validate](config-validate.md) | Validate configurations, account parameters, DNS reachability, and Bogofilter service. |

## Examples

- `mail_cli config show`
- `mail_cli config set spam_folder Junk`
- `mail_cli config reset score_threshold`
- `mail_cli config validate`

---

[↑ mail\_cli](index.md) — [nav](nav.md)
