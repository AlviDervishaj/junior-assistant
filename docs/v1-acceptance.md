# Version-one acceptance

Validated locally on 2026-10-04, macOS arm64, Go 1.27.1. The module requires Go 1.26 or newer. User approved the concrete plan before implementation.

## Results

- `go test -race ./...`: passed. Behavioral tests cover CLI workflows and selectors, task lifecycle/deletion safeguards, project inference and archive retention, search ranking/exclusions/overlap/symlinks/cancellation, local-date category precedence, backup validation/remapping, ID preservation, and database transactions.
- `go vet ./...`: passed.
- `CGO_ENABLED=0 go build -o bin/assistant ./cmd/assistant`: passed; produced the local macOS arm64 executable.
- `python3 scripts/smoke.py`: passed 30 executable invocations under macOS sandbox-exec with network denied. Registered project and independent roots, searched normal/hidden/excluded files, created/edited/completed/reopened records, verified today precedence, archived/restored ownership, verified overlap, rejected unapproved deletion and nonempty restore, exported/restored state, and confirmed ID allocation after restore. Search fixture file bytes were unchanged.
- `git diff --check`: passed for tracked changes. The work also includes newly created source/docs/tests.

## Failure and concurrency checks

Tests exercise simultaneous initialization, competing writers using separate SQLite connections, invalid domain updates, a database trigger that forces a mid-write failure, malformed backups, nonempty restore, and future database schema rejection. Persisted data survives rejected operations; ID counters remain unique. Schema version one initializes transactionally, reopening preserves records, and future versions are rejected without modifying record data. There is no older released application schema to migrate yet.

The smoke harness originally compared macOS's `/var` temporary path alias to the canonical `/private/var` path returned by registration. Resolving the harness fixture path fixed that assertion; the assistant's canonical paths were correct.

## Boundaries

This is a locally built version one; no published release or global installation was performed. The binary is in ignored `bin/assistant`. No real personal folder was registered and no real tasks were added during verification.

Search scans live and holds matching results in memory for global ranking. SQLite mutations serialize a small personal dataset and atomically replace its relational rows; this favors simple correctness over high-volume write throughput. Revisit indexing or incremental writes only if measured usage warrants it. Initial builds download pinned dependencies; application runtime is offline. The SQLite driver is the [CGo-free modernc driver](https://pkg.go.dev/modernc.org/sqlite), pinned in go.mod/go.sum.

Content search, notifications, recurrence, activity monitoring, AI, cloud integrations, filesystem changes, and other operating-system releases remain outside version one.
