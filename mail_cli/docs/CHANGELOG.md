# 📝 Changelog

All notable changes to the `gmail_spam_checker` project will be documented in this file.

## [0.2.20] - 2026-06-22

### Changed
* **TUI Message Detail View:** Redesigned the detail view when pressing Enter on an email: the top bar is now always shown, the second line displays the selected index row with highlighting, headers and message body use natural text coloring (yellow labels with default foreground body on dark background), and a special `< End of message >` line with green background appears at the bottom.

## [0.2.19] - 2026-06-06

### Fixed
* **JMAP Filter Compilation Error:** Implemented the missing `Requires() []jmap.URI` method on the custom `JMAPFilterSetMethod` struct to satisfy the `jmap.Method` interface.

## [0.2.18] - 2026-06-06

### Added
* **JMAP Server-Side Rule Migration:** Implemented server-side rule migration for JMAP accounts using the JMAP Filters extension (`Filter/set` method). This uploads local routing rules to the JMAP server so they apply automatically to incoming emails.

## [0.2.17] - 2026-06-06

### Added
* **JMAP Rule Export Persistence:** Enhanced `ExportRules` for JMAP accounts to persist the `Exported` status of rules back to `config.json` and update the in-memory configuration.

## [0.2.16] - 2026-06-06

### Added
* **JMAP Rule Export Support:** Implemented `ExportRules` for JMAP accounts. Since JMAP rules are processed locally by `mail_cli` during scans, they do not need to be uploaded to the server. The command now marks local rules as exported and prints a success message instead of failing.

## [0.2.15] - 2026-06-06

### Fixed
* **Cobra Optional Flag Argument Parsing:** Added logic to `preprocessArgs` to automatically rewrite `-m <argument>` / `--move <argument>` to use the assignment operator (`-m=<argument>` / `--move=<argument>`), preventing Cobra from interpreting the flag value as a positional argument when `NoOptDefVal` is active.

## [0.2.10] - 2026-06-06

### Added
* **Unique Message Move-to-Spam:** Enabled specifying a sender email address directly to `-m` / `--move` flag (e.g. `-m spammer@example.com`). If exactly one message from this sender is found in the scanned folder, it is moved to the spam folder and removed from the remainder of the scan.

## [0.2.9] - 2026-06-06

### Added
* **Version Command:** Added an explicit `version` command to the CLI that prints the active program version.

## [0.2.8] - 2026-06-06

### Fixed
* **ProgName Auto-Selection:** Removed the legacy exclusion of `"gmail"` from the binary name/symlink auto-selection check in `config.go`, enabling correct auto-selection of the associated account when the tool is executed via a `gmail` symbolic link.

## [0.2.7] - 2026-06-06

### Added
* **Command display in account list:** Modified the `account list` command output to display the associated CLI command/alias for each account on a new line.

## [0.2.6] - 2026-06-06

### Fixed
* **JMAP Spam Learning Responses:** Updated `LearnSpam` in `jmap_client.go` to properly handle HTTP status codes 204 (ignored) and 208 (already trained) returned by the Rspamd controller. This ensures that the local metadata cache is successfully cleared and trained message IDs are marked in the database.

## [0.2.5] - 2026-06-06

### Changed
* **Documentation update:** Updated HISTORY.md for the Rspamd Bayes statistics turn.

## [0.2.4] - 2026-06-06

### Changed
* **Default Scan Limit raised to 1000:** Increased the global default scan and download limit on everything to 1000 (up from 50) for users with smaller inboxes who want to process all messages.

## [0.2.3] - 2026-06-06

### Changed
* **Increased Ham Learning Limit:** Raised the default download/scan limit specifically for the `learn_ham` subcommand to 1000 messages (up from 50) to allow more comprehensive training of the Bayes classifier.

## [0.2.2] - 2026-06-06

### Added
* **Stat Warning for Ham Learning:** Added stat warnings to main.go in verbose mode when local email cache files cannot be found.
* **Automatic versioning policy:** Added rule 6 to GEMINI.md enforcing 0.0.1 version increments on all code changes.

### Fixed
* **Ham Learning Path Bug:** Passed account-specific client configuration down to performHamLearning so Rspamd learns emails using the correct account cache directory.

## [0.2.1] - 2026-06-06

### Added
* **Fuzzy Suffix Label Matching:** Enabled specifying target folders/labels without parent and grandparent hierarchies. If a unique leaf folder matches the suffix, the CLI prints the resolved full path and executes the command as usual.

### Fixed
* **JMAP Error Handling Type Bug:** Fixed a compilation failure in `jmap_client.go` where pointer strings (`*string`) were printed directly using `%s`.

## [0.1.1] - 2026-05-30

### Added
* **Personal Whitelist System**: Implemented a local, configuration-driven whitelist (`-wlist add <email>`, `-wlist del <email>`, and `-wlist list` subcommands) stored in `config.json`.
* **Automated Ham Training**: Integrated automatic local Rspamd HAM (non-spam) learning for whitelisted emails to improve Rspamd's statistical classifier accuracy.

### Changed
* **Visual Cleanup**: Cleaned up the inbox scan (`-i`) console output by hiding the red "SPAM EMAILS DETECTED" banner block when zero spam emails are found, replacing it with a clean green success status.

---

## [1.2.0] - 2026-05-29

This major release refactors the command-line interface, implements structured JSON configuration auto-population and bootstrapping, integrates standard Linux system directories, and enhances the language validation architecture.

### Added
* **Flag `-i` (Inbox Scan)**: The default inbox scan mode has been assigned to the new `-i` flag.
* **Flag `-spam` (Spam Auditor)**: Connects securely to the Spam folder, downloads/caches spam emails, and color-prints sender and subject headers.
* **Flag `-spamdel` (Permanent Purge)**: Connects in read-write mode, tags spam messages with `\Deleted`, and expunges them, permanently purging them from Google's servers forever (bypassing Trash).
* **Length-Constrained NLP Validation**: Restored `whatlanggo` statistical NLP language detection to audit Latin-script emails. To eliminate false positives on sparse texts, it is **only run if the email body has at least 20 words**.
* **Auto-Bootstrapping Config**: If `config.json` does not exist, a credentials template is written automatically on startup.
* **Dynamic Config Auto-Population**: Automatically updates existing configuration files with any missing default properties (such as whitelisted languages, thresholds, and limits) without modifying user credentials.
* **Standard System Caching**: Caching directories default to standard Linux user caches (`~/.cache/gmail-spam-checker/`).
* **Cache Auto-Migration**: Automatically detects legacy `./downloaded_emails` configurations and migrates them to the standard `~/.cache` directory.
* **Descriptive Operational Menu**: Displays a detailed operational guide of all commands and options when the program is run with no arguments.
* **Command Validation**: Alerts the user and presents the usage guide if arguments are parsed but no action/operation mode is specified.
* **Graceful Pre-flight Stack Check**: Automatically audits local Redis and Rspamd service ports before making remote IMAP connections, failing early with detailed instructions and exact, colorized Debian/Kali startup commands (`sudo systemctl start redis-server rspamd`).
* **MIME Normalization**: Decodes malformed and non-standard UTF-8 subject headers (e.g. `=?UTF8?Q?` which are missing the hyphen).

### Changed
* **Simplified CLI**: Moved all configuration-centric command-line options (e.g., `-host`, `-user`, `-pass`, `-dir`, `-limit`, `-languages`) entirely into `config.json` to declutter the user interface.
* **Binary Renaming**: Renamed the compiled binary to `gmail_spam_checker`.
* **Usage Outputs**: Extended `flag.Usage` to explicitly output the absolute file path of the configuration file at the bottom of the CLI usage block.

### Dependencies
* Added `github.com/abadojack/whatlanggo` (v1.0.1) for statistical trigram language analysis.
* Retained `github.com/emersion/go-imap` (v1.2.1) and `github.com/fatih/color` (v1.19.0).
