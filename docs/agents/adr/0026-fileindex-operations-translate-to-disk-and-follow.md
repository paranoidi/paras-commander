# fileindex:// operations translate to the disk, and a gated sink makes the index follow

Status: accepted, partly superseded by [0027](0027-fileindex-browse-is-index-only.md): a `fileindex://` panel is index-only again, so the translation of `fileindex://` paths to disk paths applies to the Find duplicates delete only. The sink, `Index.Apply`, the drift walk and Browse-with behaviour stand.

An operation on a `fileindex://` path acts on the real file; the index follows through an `fschange` sink, for operations started from a panel that shows the index. The fileindex backend stays an index-only store for records whose file is gone.

## Context

- Delete, rename, move and flatten on a `fileindex://` panel changed only the database, and on a Browse-with panel only the disk, so a deleted file came back as a ghost row. Both should change both.
- `Backend.List` shows indexed files only. A removal or flatten cleanup driven by that listing (a write-through `RemoveTree`, `removeDirIfEmpty` calling the backend's `Remove`, which drops the whole index tree) deletes, or fails to carry along, files the index never listed (`node_modules`, `.git`), and `DeleteRemoveEmptyDirs` or the dangling-directory prompt would call it "empty".
- `rename(2)` replaces a file silently, and the rename planner checks the target through the index.
- ADR 0023's `fschange` already reports every local change the operations make. It supported one sink per context.
- The index stores symlink-resolved paths; reports carry the paths the user acted on. Changes can also arrive for paths the index never recorded.

## Decision

- **Translate, do not write through.** `pathloc.Path.Local()` maps a `fileindex://` path to the file path it names (a string operation). Transfer and flatten jobs translate sources and destination when they are enqueued (`AddTransferJob` and `AddFlattenJob`, through `Handler.diskJobPaths`), and `ExecuteExtract` before planning, all before the destination probe; `ops.SelfTargetCount` and `ops.ValidateFlattenTarget` translate what they are given, so a dialog does not. So the existing local code does the work: real listing, conflicts, cross-device moves, elevation, change reports. Delete and rename keep the path and decide per path on the worker with `ops.OnDisk` (`Lstat`): an entry on disk is handled locally; one gone from disk is a record and goes to the index backend. `ExecuteRename` also refuses a target that exists on disk. The backend's `Rename`, `Remove` and `RemoveTree` stay index-only.
- **No cleanup through the index.** Empty-directory cleanup, the dangling-directory check and the dedup preview translate their roots to disk paths and skip a root that is not on disk; `removeDirIfEmpty` never removes a `fileindex://` directory and `ExecuteFlatten` refuses `fileindex://` paths. Real cleanup uses non-recursive removal only.
- **`fschange.Multi`** fans a batch out to several sinks, so the disk-usage engine and the index both listen.
- **`Index.Apply`** (an `fschange.Sink`) queues batches and applies them on one goroutine, in order, off the caller: `Removed` removes the file or directory tree the index holds, `Renamed` drops a record at the target and moves the record (a chain of renames in one batch goes through temporary names), everything else and every path the index does not know is ignored. It does nothing while the index is unavailable and never prompts or creates a database. Paths are resolved like `db.Resolve` for their directory and keep their name; the index decides file or directory. A pass that changed records is followed by a short pause, so a job's small changes share one pass and one panel reload. A busy database is retried; pc's own index writes share a mutex.
- **The sink is gated.** `Job.FollowIndex` is set once at enqueue by `panel.FollowsIndex`: the active panel has `WithIndex` or shows `fileindex://`, or any source or destination is `fileindex://`. Undo and retry copy it. `jobbridge.TransferFunc` adds the index sink for a following job only, and the dialog handler's `fsCtx` does the same for the active panel. An operation from an ordinary local panel leaves the index alone.
- **Panels showing the index refresh** when a record changes: `reloadIndexPanels` includes `fileindex://` panels, whose listing is the index and whose paths no longer appear in a translated job's sources.
- **One walk for the icon and the report.** `driftWalk` is the comparison behind the directory `differs` state and `fileindex.report-differences`, so they cannot disagree; a recorded file the walk did not reach differs only when it is gone from disk.

## Consequences

- Copies, extracts and moves from outside the index put files on disk that the index learns of on the next scan. A file moved by copy and delete across file systems loses its tags, because its record is removed rather than moved.
- Mkdir and symlink on `fileindex://` stay refused.
- Any new operation that reaches `ops` with a `fileindex://` path must translate it first or go through `OnDisk`; the backend must never decide what is removed from disk.
- Deleting from the Find duplicates view, whose paths are `fileindex://`, deletes the files from disk.
