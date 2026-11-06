# A fileindex:// panel changes the index only

Status: accepted. Supersedes ADR 0026's translation of `fileindex://` paths to disk paths for operations on a `fileindex://` panel.

Browse fileindex shows the database, and operations on it change the database. Browse with fileindex, whose panel is a local directory, keeps acting on the disk with the index following. The Find duplicates delete is the one operation that takes a `fileindex://` path to the disk.

## Context

- ADR 0026 translated `fileindex://` paths to the real files they name, so a delete on a Browse fileindex panel removed files from disk. Browse fileindex is meant to work on the database directly.
- `Backend.List` shows indexed files only, which is why a flatten or an emptiness decision cannot be made from it.
- The Find duplicates and Copies outside views list `fileindex://` paths, but their delete means removing the duplicate from disk.

## Decision

- **Browse is index-only.** `ExecuteDeletePaths` and `ExecuteRename` give a `fileindex://` path to the backend, whose `Remove`, `RemoveTree` and `Rename` change the index and never the disk. These paths have no host path, so there is no elevation and no `fschange` report; the backend's `changed()` refreshes the panels showing the index. A move inside the index is a record rename. Copying out to a real directory reads through the backend and works.
- **Refusals.** `AddTransferJob` refuses (`indexTransferRefusal`, wording in `ops.IndexReadOnlyRefusal` and `ops.IndexMoveOutRefusal`) a copy or move into the index from outside, a copy inside the index, and a move out of it. It reports whether a job was queued, so the transfer dialog stays open on a refusal and no caller replaces the refusal with a "queued" message. Sources and destination are never translated. `ops.ValidateFlattenTarget` refuses any `fileindex://` root or destination (`ops.FlattenIndexRefusal`), for the dialog and `ExecuteFlatten` alike. The extract dialog refuses a `fileindex://` destination; an indexed archive is extracted to a real directory with its source translated by `pathloc.Path.Local`, as it is only read.
- **No cleanup through the index.** `RemoveEmptyDirsUnder` and `DanglingDirsAfter` skip `fileindex://` roots, and `removeDirIfEmpty` never removes a `fileindex://` directory, so a browse delete or move never removes a real directory.
- **Dedup deletes on disk.** `Job.DeleteOnDisk` (TypeDelete only) is set by `Handler.EnqueueDedupDeleteJob`. The worker maps the sources with `ops.OnDiskPaths` (`ops.OnDisk`, an `Lstat`): a file still on disk is deleted there and the index follows through the sink, a file already gone keeps its `fileindex://` path and only loses its record, and cleanup skips its parent. The dedup view's empty-directory preview (`PreviewEmptyDirsUnder`) translates the same way. `ops.OnDisk` is for this delete only.
- **Kept from ADR 0026.** `fschange.Multi`, `Index.Apply`, `Job.FollowIndex` and `panel.FollowsIndex` (a Browse-with panel, or a job that names `fileindex://` paths), the drift walk shared by the `differs` icon and `fileindex.report-differences`, and Browse-with behaviour: real entries act on disk and the index follows, ghost rows are removed from the index only. `ops.SelfTargetCount` still compares translated paths, so copying an index entry onto its own file is caught.

## Consequences

- Deleting or renaming on a Browse fileindex panel does not touch files; a removed record whose file still exists comes back on the next scan that covers it.
- Edit, run, the shell and terminal hand-offs on a Browse fileindex panel still use the real paths: they start an external tool the user chose.
- The fullscreen preview and compare view delete a `fileindex://` path from the index only, like the panel.
- Any new operation that reaches `ops` with a `fileindex://` path acts on the index; an operation that means the disk must say so explicitly (as `DeleteOnDisk` does).
