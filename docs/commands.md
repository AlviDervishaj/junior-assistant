# Command reference

`assistant --help` lists commands. `--data-dir DIR` and `--json` work before the main command or after its subcommand; ordinary options may follow positional arguments. Use `--` before literal arguments beginning with a dash. Quote paths and titles containing spaces. Paths are literal shell arguments; use `$HOME` or let your shell expand `~`.

## Projects and roots

```sh
assistant project add app /absolute/path/to/app
assistant project list
assistant project edit app --path /new/path/to/app
assistant project remove app
assistant project restore app
assistant root add "$HOME/Documents" --exclude cache --exclude generated
assistant root list
assistant root edit 2 --exclude scratch
assistant root edit 2 --clear-exclusions
assistant root remove "$HOME/Documents"
```

Any folder can be a project; Git is not required. Names are unique, case-insensitively, including archived projects. IDs are stable. Registration resolves symlinks and stores absolute paths; missing folders warn but remain registered. An existing regular file cannot be registered as a folder. Active projects cannot share the same canonical path.

Each project owns a search registration. Removing a project archives it and disables that registration, preserving tasks and exclusions. Restoring reactivates it. An independent root at the same path is unaffected; remove that root separately if desired. Root IDs in `root list` identify both independent and project-owned registrations; `root edit` replaces the custom exclusion list for either. Root removal accepts independent paths only.

The nearest active registered ancestor of your current directory supplies the default project. Archived projects are never inferred, but their records remain available through explicit selection and global views. Editing a project's folder path preserves ownership and updates its search registration.

## Find

```sh
assistant find invoice
assistant find invoice --limit 10
assistant find invoice --all --hidden --include-excluded
```

Searches scan all active registered roots live. Matching is literal and case-insensitive, with exact full filename matches first, then filename prefixes, filename substrings, and other path matches. Ties sort by absolute path. Results include files, folders, and symlink entries; directory symlinks are not traversed. Overlapping roots produce one result per absolute path. Each root applies its own exclusions, so an explicitly registered descendant can expose files its ancestor excludes.

Default exclusions: hidden descendant entries and directories named `.git`, `node_modules`, `vendor`, `dist`, or `build`, plus the root's custom directory basenames. `--hidden` enables hidden entries; `--include-excluded` enables excluded directories. An explicitly registered root is traversed regardless of its own name. Neither option follows directory symlinks or searches file contents.

The default limit is 50. Ranking happens across all matches before truncation. `--all` and `--limit` cannot be combined; limits must be positive. Unreadable/missing roots or folders produce warnings and exit code 3; accessible results are retained. An empty result is success. Large trees still require a full walk to establish global ranking; there is no index or background cache.

## Tasks and bugs

```sh
assistant task add "Fix login" --type bug --notes "Fails after sleep"
assistant task add "Renew passport" --personal --due 2026-11-01
assistant task add "Review CLI" --project app --planned 2026-10-05
assistant task list
assistant task list --all --include-finished
assistant task list --personal
assistant task show 42
assistant task edit 42 --title "Fix session expiry" --planned 2026-10-06
assistant task edit 42 --clear-planned --clear-due --clear-notes
assistant task edit 42 --personal
assistant task log 42 "Reproduced after sleep"
assistant task logs 42
assistant task start 42
assistant task done 42
assistant task cancel 42
assistant task reopen 42
assistant task delete 42 --yes
```

Records have numeric IDs that are never reused, type task/bug, title, notes, optional project, optional planned date, optional due date, and status open/in-progress/done/cancelled. New records default to task/open. Dates use YYYY-MM-DD; planned dates express intention and due dates express deadlines. Empty date/notes values or explicit clear options remove optional values. A value and its clear option cannot be combined.

Creation infers your project, otherwise personal work. `--project NAME` or `--personal` overrides it. List defaults to the current project, otherwise all records. `--all`, `--project`, and `--personal` are mutually exclusive. Show/edit/lifecycle commands identify records directly by ID, regardless of your current directory. Edit keeps ownership unless explicitly changed. All lifecycle corrections are allowed; reopening sets open. Done/cancelled records are hidden from list unless `--include-finished` is supplied.

Cancellation retains a record. Permanent deletion prompts on a real interactive terminal, requiring the literal answer `yes`. Noninteractive or JSON deletion requires `--yes`; piped approval is not accepted.

Progress logs supplement notes without changing status. See [progress logs](progress-logs.md) for commands, backup compatibility, and migration/rollback details.

## Today

```sh
assistant today
assistant today --project app
assistant today --personal
```

Today defaults to all projects and personal work, using the local timezone. Active records appear once, under the first matching group: overdue deadline, due today, missed plan, planned today, in-progress. Dated groups sort by their qualifying date oldest first, then ID; in-progress sorts by ID. Unscheduled open records stay in task list. Archived project records remain visible. JSON returns all five groups, including empty ones.

## State and backup

Default state directory: `~/Library/Application Support/Junior Assistant`, containing `assistant.db`. `--data-dir DIR` selects independent state for experiments or transfers. The app creates private state and backup files. Filesystem search does not modify the folders it reads; writes are limited to application state and explicit backup destinations.

```sh
assistant export "$HOME/assistant-backup.json"
assistant --data-dir /new/empty/state restore "$HOME/assistant-backup.json"
assistant --data-dir /new/empty/state restore "$HOME/assistant-backup.json" \
  --remap /old/home/Projects=/new/home/Projects
```

Exports use version 2 JSON with records and progress logs, stable IDs/counters, active and archived projects, independent/project roots, settings and exclusions. They contain paths and notes, so keep backups private. They contain no searched file contents. Existing export destinations are never overwritten.

Restore requires a fresh, empty application database, including untouched ID counters. Version 1 backups remain readable by the new build. It validates the document and relationships before committing a single transaction; failed restore preserves state. Unsupported versions and unknown fields are rejected. Path mappings use absolute source prefixes; the longest match wins, once per original path. Mappings cannot create duplicate active project paths or independent roots. Missing paths warn and stay registered for later reconnection. Keep the original database until you have inspected a transfer.

## Output and exit codes

Every successful `--json` command emits one JSON object: `{"data": ..., "warnings": []}`. Warnings also go to stderr. No prompts or formatted text are mixed into JSON stdout. Failed commands emit diagnostics to stderr without a success object.

Data is a project/root/record object for a single-object command; an entry object for task log and an entry array for task logs; an array for lists; category/records groups for today; a string for successful delete/root removal/export/restore; and a search object containing `matches`, `total`, `truncated`, and `warnings` for find. Search matches have `path`, `kind` (file/directory/symlink), and `rank` (0 exact filename, 1 prefix, 2 substring, 3 path). Project/root/record ownership uses stable numeric IDs; project ID 0 or an omitted JSON project_id means personal/independent. Result rows escape control characters in names, titles, and paths.

| Code | Meaning |
| --- | --- |
| 0 | Success, including empty results, ordinary missing-registration warnings, or a declined deletion |
| 1 | Operational failure, such as an unknown record, database error, or invalid backup |
| 2 | Invalid usage or missing noninteractive deletion approval |
| 3 | Partial search: results returned, but some roots/folders could not be read |
| 130 | Interrupted operation |

Help is plain text even with `--json`, and does not initialize state. Ctrl-C cancels filesystem traversal and database work. Runtime requires no network. macOS is the supported platform; a Linux/Windows release and background services are outside version one.
