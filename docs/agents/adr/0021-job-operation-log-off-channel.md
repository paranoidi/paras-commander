# Job operation logs bypass the event channel and spill to the cache

Each job records every processed item (done, skipped, failed) in its own `oplog.Log`, written directly by the job goroutine, never through job events. Past `[jobs].operations_memory_rows` the log spills to `$XDG_CACHE_HOME/pc/jobs/<session>/<job>.oplog` and keeps only the newest rows in memory. The jobs view reads it through a small prepared window, never by scanning the whole log per frame.

## Context

- The Operations pane must list every item. Rows built from `EventProgress` were sampled (throttled emits, coalesced drains), so fast copies of small files listed only some files.
- A per-item job event cannot fix that. The event channel holds 100 events: a non-droppable per-item event stalls the copy at the UI's repaint rate (non-progress events wake the UI immediately and each wake drains and repaints), and a droppable one loses rows exactly when there are many.
- A job can process millions of items. At ~150–200 bytes per row, keeping all rows costs hundreds of MB per job, and filtering every row per frame stalls rendering.
- UI-thread IO is forbidden (repo performance rule).

## Decision

- `ops.Options.OnItemDone` (plus the delete and extract item hooks and the existing `OnItemError`) report each item; `jobbridge` calls `Job.RecordOperation`, the single writer (failures from both the executor and the plan walk go through `Job.FailItem`, which records and emits `EventItemFailed`), which appends to `Job.Ops` under the log's own mutex (never `State.mu`). Labels are a string trim (destination relative to the job destination), computed once per item.
- The log keeps records in memory up to `MemoryRows`, then writes them all to the spill file and appends there through a 64 KiB buffered writer on the job goroutine; memory keeps the newest `MemoryRows` (compacted at twice that). Counts, per-result record-number lists and a byte offset per 1024 records always stay in memory, so result filters and seeks are random access. A failure is superseded (hidden) when a later record of the same label succeeds or fails again (retry).
- The jobs handler prepares `Model.JobsOpsWindow` each frame: rows in memory are copied synchronously; rows only in the spill file are read by a goroutine, and fuzzy queries run as a cancellable `Log.Scan` goroutine that continues over rows appended later. Both post `jobsctrl.OpsWakePayload`, which repaints the jobs view unconditionally (`WakePayload` repaints only when job events changed something, and these results produce none).
- Lifecycle: the session directory (0700, created on first spill) is removed when the worker shuts down; a job's file (0600) is removed when the job leaves the state (retention, clear finished). Startup sweeps session directories of dead processes. A spill write failure drops the oldest rows and marks the log incomplete instead of failing the job.

## Consequences

- File names of large jobs are written to the user's cache directory while the job is listed. `operations_memory_rows = 0` keeps everything in memory instead.
- Superseding a failure rebuilds multi-result views once; otherwise views extend incrementally with appends.
- Scan matches are not revisited when a retry later supersedes a matched failure; the next query change rescans.
