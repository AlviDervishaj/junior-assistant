# Task progress logs

A progress log is an explicitly entered, append-only timestamped update on a task or bug. It supplements editable notes; there is no automatic activity monitoring.

```sh
assistant task log 42 "Reproduced after sleep"
assistant task log 42 "Waiting on review"
assistant task logs 42
assistant task show 42
```

Entry IDs are local to each record, starting at 1. Entries appear in append order, even when timestamps match or the system clock moves backward. Creation timestamps use UTC RFC3339. Finished/cancelled records and archived projects retain history and can receive new entries. Logging does not change notes, status, dates, or ownership. Blank messages and unknown record IDs fail without mutation. Permanent task deletion removes its logs as part of the same confirmed operation.

`--json` returns an entry object for `task log` and an entry array for `task logs`, inside the normal data/warnings envelope. Entry fields are `id`, `created_at`, and `message`. Record JSON has an optional `progress_logs` array; `task show` prints history after notes. Terminal controls are escaped in displayed messages; JSON preserves original text.

## Compatibility and rollback

Schema 2 adds a progress_logs column to records in the same initialization transaction. Existing V1 records, IDs/counters, roots, project ownership/archive state, settings, and notes remain intact. Reopening does not rerun the migration. New backups use version 2 and preserve history; the new application also restores version 1 backups. Unsupported backup versions and malformed history are rejected. V1-format backups cannot contain history.

Before switching from a V1 binary, export a backup with that binary if you need the option of returning to V1. V1 binaries reject schema 2; they cannot silently write away log history. To roll back, use the V1 binary with an empty separate data directory and restore the V1 backup. Keep the upgraded database and a version 2 backup to preserve entries created since upgrading. There is no destructive down-migration or log-stripping export.

Implementation acceptance is tracked in [issue #10](https://github.com/AlviDervishaj/junior-assistant/issues/10).

## Acceptance evidence

On 2026-10-04, `go test -race ./...`, `go vet ./...`, and a CGo-disabled macOS arm64 build passed. The executable smoke harness passed 34 invocations with runtime network denied, including finished-task logs, append ordering, backup/restore history, and unchanged search fixture bytes. Dedicated tests verify V1 schema preservation, reopen idempotency, old backup restore, malformed history rejection, and unchanged notes/status.

An additional compatibility check built the actual V1 source at commit `9f1c901` in a temporary directory. With network denied, V1 created/exported records; the new build migrated them and preserved notes; V1 then rejected schema 2; both new-build legacy restore and V1 rollback restore into separate state succeeded.
