---
title: mail_cli
has_children: true
---

# mail\_cli

Gmail / JMAP Spam Checker CLI tool

## Commands

| Command | Description |
|---------|-------------|
| [scan](scan.md) | Scan all folders starting with the given label prefix (case-insensitive) for spam. |
| [show](show.md) | Show the contents of emails in folders matching a label prefix, or show a specific email's details and body without running spam checks. |
| [test](test.md) | Run system and integration self-tests to verify API credentials and mail flow. |
| [whitelist (wlist)](whitelist.md) | Manage the personal sender whitelist to bypass spam checks. |
| [blacklist (blist)](blacklist.md) | Manage the personal sender blacklist to instantly classify messages as spam. |
| [rule](rule.md) | Manage custom routing rules and auto-labeling filters. |
| [labels](labels.md) | Manage and inspect server labels and folder hierarchies. |
| [spam](spam.md) | Manage Spam folder, train filters, and unsubscribe from political mail. |
| [filter](filter.md) | Manage remote filters on Gmail. |
| [account (accounts)](account.md) | Manage and list configured mail accounts. |
| [learn-ham (learn\_ham)](learn-ham.md) | Train Bogofilter on ham (non-spam) emails in a folder. The folder must be an exact match and cannot have subfolders. |
| [unspam](unspam.md) | Mark a message as not being spam: train bogofilter as ham and move it from Spam back to Inbox on the server. |
| [learning](learning.md) | Manage local spam learning and training. |
| [archive (arc)](archive.md) | Move message(s) by ID from their current folder to the Archive or Received folder. Or archive all messages in Inbox (default) or the specified label (by prefix). |
| [config](config.md) | Show or manage configuration options. |
| [cache](cache.md) | Manage the local email download cache. |
| [calendar](calendar.md) | Manage calendar events extracted from email attachments. |
| [caladd (add-all)](caladd.md) | Scan the inbox for messages containing .ics attachments, and add them to the calendar if they are not already present. |
| [tui](tui.md) | Open the interactive terminal email browser. With an optional label_prefix argument, open the TUI with the matching label as the initial folder. The prefix is matched case-insensitively as a substring against the full label path. If exactly one label matches, the TUI opens on that label. If multiple match, all matching labels are printed and the program exits. |
| [color (test-color)](color.md) | Test terminal 24-bit true-color and 256-color support. |
| [splice](splice.md) | Move messages from a folder into the keep/YYYY/MM/<folder> structure. The root "keep" is fixed. Use -f to change the target folder name, or -F to change the target folder name and automatically suffix it with the year and month. |
| [migrate](migrate.md) | Copy configuration and credentials to a remote machine via SSH/SCP. |
| [split](split.md) | Scan messages in the source label. If their subject matches the pattern (which may contain wildcards * and ?), move them to the target label. Runs in dry-run mode by default; use --do to perform actual operations. |
| [download](download.md) | Download all messages in the specified label (which must match a unique label) to a local mbox file. |
| [upload](upload.md) | Upload all email messages from a local mbox file to the specified target label/folder on the server. |

## Global Flags

| Flag | Description |
|------|-------------|
| `-v, --verbose` | Enable verbose output |
| `-A, --account <name>` | Specify target account name (e.g. personal-gmail, work-jmap) |
| `--logappend` | Keep and append to existing log files instead of deleting them at startup |
| `--fix` | Automatically fix received_folder configuration issues |
| `-p, --pattern <pattern>` | Only process messages whose subject contains this pattern. |
| `--read-only` | Run in read-only / dry-run mode (no server modifications; aliases: --dry-run, --ro) |
| `-m, --move [From]` | Move identified spam emails to Spam folder. Optional: specify From address to move a single unique message. |
| `--inbox-move <From>` | Move identified emails from a specific From address back to the Inbox folder. |
| `--version` | Print the version number and exit |

## Version

0.6.7

## About

GitHub: [https://github.com/sarielhp/gmail_cli](https://github.com/sarielhp/gmail_cli)

