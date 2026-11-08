# The fileindex follows every job

Status: accepted. Supersedes the gate in [0026](0026-fileindex-operations-translate-to-disk-and-follow.md) and its use in [0027](0027-fileindex-browse-is-index-only.md).

Every job and dialog operation reports its filesystem changes to `Index.Apply`, whichever panel started it. `Job.FollowIndex` and `panel.FollowsIndex` are gone.

## Context

- A move or delete from an ordinary local panel left the index stale, so a moved file looked deleted and a deleted one came back as a ghost row until a prune.
- The gate tied a job to the panel it started from, and that flag was the only thing connecting the two. What the index does with a change is the index's concern.
- File tags now belong to content (fileindex ADR 0001), so a record dropped by a move across file systems no longer loses its tags.

## Decision

- `fschange` stays the one report of what changed on disk. `jobbridge.TransferFunc` and the dialog handler's `fsCtx` always fan out to the shared sink and the index. The panel knows nothing about it.
- `Index.Apply` decides what matters: with no database, or a batch with no `Removed` or `Renamed`, nothing is queued. It never prompts or creates a database.
- A `Renamed` whose source the index lacks still drops a file record at the target, as its content was replaced. A directory move that clashes with records at the target clears them, because `rename(2)` only lands on an empty directory. If it still fails, the source records go, so no ghost stays.
- Changes in a batch apply in arrival order.

## Consequences

- A user who never made a database pays one cached check per reported batch.
- A move across file systems stays `Created` plus `Removed`: the file has no record, and no tags showing, until a scan hashes the destination. A dedicated move event is deferred until that delay proves a problem.
- The index learns only what pc does. Changes made outside pc still wait for a scan.
