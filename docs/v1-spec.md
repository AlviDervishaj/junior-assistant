# Junior Assistant version one

Status: approved by the user and implemented locally; see docs/v1-acceptance.md for evidence. GitHub Issues carry the implementation specifications and acceptance criteria; this document is the local design reference.

## Outcome and scope

Build a Go CLI named `assistant`, initially for macOS, to find local files/folders, record tasks and bugs against projects or personal life, and show work needing attention today. Runtime is offline, without AI, telemetry, external services, activity monitoring, or a background process. Installation may download dependencies. GitHub Issues track development of this repository; the installed application never uses GitHub to store or retrieve its work.

Filesystem access is read-only except application-owned state and explicitly requested backup output. File moves, renames, shell execution, content search, recurrence, notifications, indexing, and interactive sessions are deferred.

## Registration and project ownership

- Register any folder as a project with a unique name, stable internal ID, and absolute path. Git is optional context, not a prerequisite.
- Register personal search roots independently. Adding a project creates a project-owned search registration; removing it archives its identity and disables that registration. Independent search roots remain independent.
- Restore an archived project to reactivate its project search registration. Records retain ownership throughout archive/restore.
- Infer the nearest active registered ancestor of the current directory. `--project` overrides inference; `--personal` bypasses it. Reject conflicting selectors.
- Missing folders remain registered, produce warnings, and can be reconnected with `project edit --path` without changing record ownership.
- Store canonical absolute registration paths; reject ambiguous duplicate active project paths. Resolve the explicitly registered root itself, but do not follow directory symlinks during traversal.

## Search

`assistant find TEXT` scans active registered roots live, matching literal filename/path text case-insensitively. Rank exact full filename matches, filename prefixes, filename substrings, then remaining path matches; break ties by absolute path. Return files and folders with absolute paths, deduplicating overlapping roots.

Skip hidden descendant directories/files and `.git`, `node_modules`, `vendor`, `dist`, and `build` directories by default; support per-root configured exclusions. An explicitly registered root remains searchable even if its own name is hidden or excluded. `--hidden` enables hidden entries; `--include-excluded` bypasses directory exclusions. Do not follow directory symlinks. Report unreadable directories briefly and preserve accessible results.

Show 50 matches by default, disclose truncation, and offer `--limit` and `--all`. Ranking applies globally before truncation. Cancellation must stop traversal promptly.

## Records and commands

Records have stable numeric IDs, type `task` or `bug`, optional project ownership, title, notes, status, optional planned date, and optional due date. Statuses are `open`, `in-progress`, `done`, and `cancelled`. Dates are validated calendar dates, without times or recurrence.

Commands are argument-driven:

```sh
assistant project add NAME PATH
assistant project list
assistant project edit NAME --path PATH
assistant project remove NAME
assistant project restore NAME
assistant root add PATH
assistant root list
assistant root remove PATH
assistant find TEXT
assistant task add "Fix login" --type bug
assistant task add "Renew passport" --personal
assistant task list
assistant task show 42
assistant task edit 42 --title "Fix session expiry"
assistant task start 42
assistant task done 42
assistant task cancel 42
assistant task reopen 42
assistant task delete 42
assistant today
assistant export PATH
assistant restore PATH
```

Task creation infers the current project, otherwise creates personal work; explicit selectors override it. Creation defaults to type task and status open. Editing supports notes, type, ownership, planned/due dates, and explicit clearing of optional fields. All lifecycle changes are available explicitly; reopening sets open. Cancellation is ordinary removal from active work; permanent deletion requires confirmation or `--yes`. Noninteractive deletion without confirmation fails safely.

`task list` defaults to the current project when inside one, otherwise all records. `today` defaults to all projects and personal work. Both support mutually exclusive `--project`, `--personal`, and `--all`. Hide done/cancelled by default and provide an explicit way to include them. Archived projects' active records remain visible in all-project views and explicit project selection.

## Today

Compute today using the Mac's local timezone. Assign each active record once to its first matching category:

1. Overdue deadline: due date before today.
2. Due today.
3. Missed plan: planned date before today.
4. Planned today.
5. In-progress work.

Sort dated categories oldest first using their qualifying date, then ID; sort in-progress by ID. Unscheduled open records remain in task list. A future-due in-progress record still appears under in-progress unless an earlier category matches.

## State, backup, and interfaces

Store SQLite data and settings under macOS Application Support, with `--data-dir` for an alternate location. No searched file contents enter the database or backups. Feature logic owns rules; CLI handlers parse arguments and render results. Use feature packages under `src/`, with filesystem search independent of record storage; avoid plugin infrastructure and generic speculative abstractions.

Versioned JSON export includes records, IDs, active/archived projects, independent and project-owned root registrations, preferences, and exclusions. Restore accepts only an empty application database, validates the entire document and relationships before writing, and commits once. Failed restore leaves state unchanged. Cross-machine paths use explicit remapping; missing paths are reported rather than silently replaced. Preserve ID allocation after restore. Database compatibility migrations must not discard records.

Provide readable terminal output, `--json` for scripts, help, and documented exit codes. JSON results must be machine-readable without prompts or warning text mixed into stdout; diagnostics go to stderr. Define stable output contracts and distinguish complete success, invalid usage, failures, and partial searches. Empty matches are a successful empty result.

## Release acceptance

A clean macOS install can register a project and independent root, find files, create/edit/complete/reopen personal and project records, inspect today, archive/restore a project, and export/restore into an empty data directory without network access. Verify search exclusions/ranking/deduplication, project inference, date/category precedence, archived ownership, JSON output, and restore rollback with focused automated tests and an end-to-end CLI smoke run. The user reviewed and approved the concrete plan before implementation.
