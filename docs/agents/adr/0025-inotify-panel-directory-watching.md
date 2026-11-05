# Inotify directory watching: wakeups for the panel refresh, nothing more

`internal/dirwatch` tells the app that a local directory changed. The app answers by scheduling the same listing refresh the periodic ticker already runs. The watcher carries no listing data, and the periodic refresh keeps running.

## Context

- Panels re-read their directory every `refresh_interval_ms`, so a change made by another program shows up seconds late, and a change in an expanded tree directory that is off screen can take a round of ticks (one off-screen directory per tick).
- Polling cannot go away: SFTP and `fileindex://` have no notification source, a network mount does not report changes made by other machines, inotify's queue can overflow, and the watch limit can run out.
- The refresh path already has the hard parts: start-to-start back-off, an in-flight flag, epoch checks against newer in-memory mutations, diff-before-apply, and the gate that makes background I/O yield to a job on the same volume. A second listing path would have to repeat all of them.
- `inotify_add_watch` resolves a path and can block on a stalled network mount; `Set` is called from the UI thread, which must not do I/O.
- tcell's event channel holds 10 events and `PostEvent` drops silently when it is full, so a wake must not be posted with a plain `PostEvent`.

## Decision

- The watcher is a trigger. `onChange(dir)` feeds a `coalescingQueue[string]`; a throttle window (`config.DefaultDirWatchDebounce`, started by the first change of a window, not restarted by later ones) posts one guaranteed wake. On the UI thread each panel whose root or expanded tree directory is among the changed directories is re-listed through `schedulePanelListingRefresh`. Changed tree directories are added to that call's refresh plan, so a nested change is fetched whether or not it is on screen.
- `schedulePanelListingRefresh` reports transient skips (pending load, back-off deadline, read in flight) and the wake handler queues the directory again for one more window. A volume-contention skip is dropped: the job's own reloads and the next tick cover it.
- The wake is handled under the periodic tick's gate (browser view, no modal dialog). Changes that arrive while gated are dropped; the next tick heals them.
- `Set` stores the desired set and wakes a worker goroutine, which adds and removes watches; a newer set replaces one not yet applied. The reader uses a non-blocking inotify descriptor wrapped in `os.File`, so `Close` unblocks it through the runtime poller. `Close` never waits for the worker, which may be inside a syscall on a dead mount. Every failure (no inotify, watch limit, permissions, vanished directory) leaves that directory unwatched and is not reported.
- The mask is create, delete, move in/out, close-after-write, attribute change and the directory's own delete/move. `IN_MODIFY` floods during writes (`IN_CLOSE_WRITE` reports the final size and mtime), and `IN_ACCESS` / `IN_OPEN` would make a listing trigger itself. A queue overflow reports every watched directory.
- The watch set is the root of each local panel plus its expanded tree directories, capped per panel (`config.DefaultDirWatchMaxPerPanel`, the root first). It is recomputed only when a per-panel signature changes, from the post-event reconciler. `Path.IsLocal()` selects it: a panel with Browse with fileindex on is a local listing with the overlay merged in, so it is watched; direct `fileindex://` browsing and SFTP are not.
- `EntriesListingEqual` compares the fileindex overlay state, so a file that became a ghost row (same indexed size and mtime) counts as a change. The four states the drift pass writes onto indexed directories after a listing (`indexed`, `tagged`, `unchecked`, `differs`) compare as one, so an idle panel does not re-apply on every refresh.
- `[panels].watch_directories` is the only user setting; the throttle window and the cap are built-in constants. The periodic refresh interval is independent of it.

## Why not

- **fsnotify**: an extra dependency for one syscall family. The stdlib covers Linux; elsewhere the periodic refresh is the behaviour that existed before, so a no-op is enough.
- **An `fschange` sink**: `fschange` reports what pc's own operations did, and the disk-usage engine is its consumer. It does not see other programs, which is the point of watching, and inotify reports no sizes or entries to apply. Feeding the watcher through it would add a path that bypasses the refresh's guards.

## Consequences

- A change between a listing and the moment its watch is armed waits for the next tick.
- A watched directory removed and recreated is re-armed by the wake its removal caused, since `handleDirWatchWake` re-sends the set. If it is recreated after that wake, the next `Set` re-arms it; until then the periodic refresh covers it.
- Changes made to a network mount by other machines are not seen. Off Linux, nothing is watched.
