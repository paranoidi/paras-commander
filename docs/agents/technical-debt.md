# Technical debt

Known duplication and complexity left in place on purpose; each item records why. Check here before proposing the same cleanup again.

## Compare reclassifies on every hash completion

`internal/compare/session.go`'s hash-worker loop calls `Classify(...)` (full reindex and sort of both trees) on every job completion. This is deliberate: a finished small file leaves `Pending` immediately instead of waiting for a batch, at the cost of an O(P+S) reclassify plus a render wake per file, which is measurable on trees with very many small files. If it becomes a problem, throttle the `Classify()`+publish call by time (`internal/sched` debouncer), keeping the cheap `MarkHashing`-only path for progress ticks between flushes. Do not reintroduce a fixed `flushInterval` or completion count: that caused the staleness this design fixed.

## Two hjkl-remap implementations

Vi-motion h/j/k/l mapping exists twice:
- `internal/keymap/chord.go`'s `RemapViMotionKey` rewrites the event key to Left/Down/Up/Right so each auxiliary view's arrow handling works unchanged (Compare, Dedup, Messages, Commands, Jobs).
- `internal/app/input.go`'s `InputModeNormal` (browser) resolves `resolvedAction` directly to `ActionNavParent`/`NavDown`/`NavUp`/`NavOpen` in a hand-written switch and tracks a `viMotionNav` bool to suppress quick-filter start for those runes.

The browser could call `RemapViMotionKey` (plain arrows already resolve to the same nav actions in `internal/keymap/specs.go`), but its switch is entangled with quick-filter suppression and `SubFocusSelectedStrip` handling inside `handleKey` (gocyclo 71). Consolidating needs care with those interactions, not a mechanical swap.

## Vi-motion common-key preamble duplicated across the app/apphandler boundary

`internal/app/leader_menu.go`'s `dispatchAuxiliaryViewCommonKeys` factors the quit / quit-immediate / F9 menu / leader toggle / direct-fire / menu-shortcut block for the views living in `internal/app` (Compare, Dedup, Messages). `internal/apphandler/commands/view.go` and `internal/apphandler/jobs/handler.go` carry their own copy via `h.host.ToggleLeaderMenu()` / `h.host.DispatchLeaderLetter(...)`, because `internal/apphandler/*` cannot import `internal/app`. Jobs also has a `blockerFocused` guard the others lack. Merging them means adding several `Host` methods (quit, quit-immediate, open-menu, menu-by-shortcut) to both handlers and deciding where `blockerFocused` fits. Do it if a sixth call site appears, not for two.
