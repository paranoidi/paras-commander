# Filesystem change events: ops report, one sink applies

The local filesystem primitives in `internal/ops` report what they did (`fschange.Renamed`, `Removed`, `Created`, `Dirty`) through an `fschange.Sink` carried on the context. The disk-usage engine is the one sink. It applies the reports on its own goroutine, in order, coalescing bursts into few cache sweeps. No entry point tells the cache about changes on its own.

## Context

- The disk-usage cache was told about changes per entry point: dialog renames called `RenamePaths` on the UI goroutine, move jobs fed a batched `OnRenamed` hook, and jobbridge guessed removals from the job type plus an `Lstat` per source. Undo, removed empty parents, everything a copy or extract added, overwrites and elevated retries were never reported, so scanned totals stayed wrong until a rescan.
- A walk caches every node (files included), so a removal or rename never needs a size. Only creations do.
- Sweeping the cache is O(cache) and only a *directory* rename or removal needs one. A merge move renames children one at a time, so per-change sweeps would be quadratic.
- UI-thread IO and long lock holds on the UI goroutine are forbidden (repo performance rule).

## Decision

- `internal/fschange` is a stdlib-only leaf: `Change`, `Sink`, `WithSink(ctx, sink)`, `Report(ctx, batch...)`, `Enabled(ctx)`. A context without a sink reports nothing. The changes of one batch happened together (a mass rename's net renames, with no temp hops); everything else reports one change per batch.
- Each `ops` operation reports each change once, at the step that makes it and after that step succeeds, through the `report*` helpers in `internal/ops/fsreport.go`. These are the one place that drops remote paths, and they `Lstat` the subject only when a sink listens. A failure that leaves an unknown amount behind (a delete stopped part way, a failed local copy, archive-tool output) reports `Dirty`.
- `diskusage.Engine.Apply` only appends to a pending list. `applyLoop` takes everything pending and applies it in one pass, so batches arriving during a pass coalesce into the next one. Renames and removals collect into a sweep group, which is flushed before any change that overlaps a path in it; only a group holding a directory sweeps. Creations add their bytes to cached ancestors at once. The rules are idempotent against a walk that already saw the change.
- `Dirty` drops the path now and queues a refresh scan, which moves the cached ancestors by the walked total's change as the root's key is written. Applied changes overlapping an active walk root mark it touched, and the planner walks it again before merging.
- The context carries the sink (like `ops.WithElevator`) rather than an `Options` field, because rename, mkdir, links and delete don't take `Options`.

## Consequences

- A new operation stays in step by calling the shared primitives; reporting is not opt-in per entry point.
- Cache updates are asynchronous everywhere, so a dialog rename's size may be missing for one apply pass.
- A dirty path shows as pending (in every panel) until its refresh scan lands.
- Other subscribers (panel reload targeting, git status, preview prefetch, find index) can use the same events later. Today the engine is the only sink.
