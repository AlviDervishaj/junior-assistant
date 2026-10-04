# Local command assistant with durable project identity

The accepted version-one design is a Go executable for macOS with explicit terminal commands, offline runtime, and local SQLite storage. Filesystem search remains read-only and scans registered roots live; feature packages own project registration, search, records, and today's view, sharing only storage, configuration, and output facilities. This trades conversational interpretation, cloud synchronization, and background indexing for predictable behavior and current filesystem results.

Project identity is independent of folder location and registration activity: paths may change and projects may be archived without losing record ownership. Planned dates and due dates remain distinct so an intention is never silently converted into a deadline. Versioned backups preserve identities and settings, while excluding searched file contents.

GitHub Issues are a development tracker for this repository only. The installed assistant has no GitHub integration, AI model, telemetry, activity monitoring, or background service in version one; initial installation may download dependencies.
