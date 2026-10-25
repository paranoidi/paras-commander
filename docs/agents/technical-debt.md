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

`internal/app/leader_menu.go`'s `dispatchAuxiliaryViewCommonKeys` factors the quit / quit-immediate / `app.open-menu` / leader toggle / direct-fire / menu-shortcut block for the views living in `internal/app` (Compare, Dedup, Messages). `internal/apphandler/commands/view.go` and `internal/apphandler/jobs/handler.go` carry their own copy via `h.host.ToggleLeaderMenu()` / `h.host.DispatchLeaderLetter(...)`, because `internal/apphandler/*` cannot import `internal/app`. Jobs also has a `blockerFocused` guard the others lack. Merging them means adding several `Host` methods (quit, quit-immediate, open-menu, menu-by-shortcut) to both handlers and deciding where `blockerFocused` fits. Do it if a sixth call site appears, not for two.

## Dedup wrapper copy shows the content icon, not the full-dir stamp

`dedupDirMarks` (`internal/ui/dedup_view_types.go`) gives each dir-group member its group's kind, and an exact group wins over a content group. When `maple/` holds only `sub/`, and `sub/` is an exact copy of `walnut/`, then `walnut` and `maple/sub` get the exact stamp (`dedup.full_dir`) but `maple` gets only the content icon. Mixed tiers behave the same way: with exact {A,B} and content {A,B,C}, C shows the content icon. For: the icons are accurate, because the wrapper really is not a layout clone. Against: users read two copies with different markers as an asymmetry bug ("all copies should have the icon"). The options are to stamp every member of a content set when any member is exact, to also mark the wrapper's inner dir, or to keep this and explain both icons in the legend. Relevance: high, because a user reported it. It is blocked on a product decision, not on effort.

## List scroll policy is package-level state

Jobs, Messages, Compare and Dedup read `[ui.scroll]` mode and edge margin from a package global, `atomic.Pointer[listScrollPolicy]` in `internal/ui/jobs_view_state.go`, through `ensureSelectionVisible`. `NewWithOptions` (`internal/app/app.go`) and `syncScrollFromConfig` (`internal/app/panels.go`) write it. Both writes are needed, because `syncScrollFromConfig` runs only when settings change, not at startup. For: it is tiny, about 74 `EnsureSelectionVisible` call sites needed no change, one reader keeps the semantics identical across the four views, and it is race-safe. Against:
- It is hidden coupling, and with nothing set it silently falls back to "edge, margin 0".
- Tests must reset it with `t.Cleanup` and cannot run `t.Parallel`.
- Panels carry the same pair per `panel.State`, so one config value is plumbed two ways.
- Generic list scrolling lives in a jobs file.

The fix is a policy value on `ui.Model` or on each view state, passed into `EnsureSelectionVisible`, with the helper in its own file. Relevance: medium. It is a mechanical refactor to do before the next scroll change.

## Minimal-scroll logic duplicated

The minimal (`default:`) branch of `ensureSelectionVisible` reimplements `panel.State.EnsureCursorVisible` (`internal/panel/state.go`). `internal/ui/geom/scroll.go` has `ScrollOffset` and `ScrollOffsetEdge` but no minimal variant. `SetListScroll` also re-applies `panel.EffectiveScrollMode`, which `scrollModeFromConfig` has already applied. For: about five readable lines, and the double normalisation is idempotent. Against: two copies of the scroll semantics can drift when a mode or edge case changes. A third copy is `ensurePathVisible` (`internal/apphandler/commands/output_dialog.go`), which scrolls the output dialog's path cursor with the minimal rule only, so it ignores `[ui.scroll]` center/edge. It cannot call `ensureSelectionVisible`, because that helper is unexported in `internal/ui`. The fix is `geom.ScrollOffsetMinimal`, shared by all three, with the output dialog honouring the scroll policy too. Relevance: low. Fold it into the scroll-policy refactor.

## Dedup unfocused pane only clamps; `visibleRows = 0` means "clamp only"

`Handler.ensureSelectionVisible` (`internal/apphandler/dedup/handler.go`) scrolls only the focused pane with its own height (Copies uses `DedupSecondaryRects`). The other pane gets `0`, which only clamps. Many other handler calls pass `0` with the same meaning. For: each pane is measured correctly, and an unfocused pane rarely needs scrolling. Against: after a resize or a `syncCopies` rebuild, the unfocused pane can sit outside the edge margin until it is focused, and the bare `0` hides intent. The fix is to compute both pane heights in the app caller, or to add an explicit `Clamp(total)`. Relevance: low–medium.

## Other-match mark special-cased on the disk-usage bar, with hard-coded black

`ListingSuffixSpans` (`internal/panellist/indicators.go`) forces `tcell.ColorBlack` only for the other-match glyph, and only when its `onBar` callback says the cell is on the bar. For: it fixes exactly the reported collision (bright_black on bright_black) without touching other marks, and black is what was asked for. Against:
- Other marks can collide too; `no_permission` is also bright_black.
- The function now has ten positional parameters, and the carousel passes a bare `nil`.
- The `onBar` closure is allocated per row, per frame. The cheap `suffix.OtherMatch` check runs first, but the allocation stays.
- The hard-coded colour bypasses the theme rule, so a light bar theme cannot override it.

The fix is a theme key (e.g. `panel.usage.mark`, see `recipes/add-theme-key.md`) applied to every suffix icon whose cell lies on the bar, with `barEnd int` passed instead of a closure, inside a params struct. Relevance: medium. It is cheap now, while the code is fresh, and otherwise the collision gets rediscovered one mark at a time.

## File-op dialogs other than delete assume the active panel

`OpenDeleteDialog(panelID int)` keeps its source panel, which Compare needs when deleting from a column that is not the active panel. Copy, move, rename and mkdir still resolve through `host.ActivePanel()`. For: in the browser view the active panel is always correct, and the delete-only fix kept the change focused. Against: these dialogs have the same latent bug for any Compare or Dedup entry point, and it will repeat when one is added. The fix is for every `Open*Dialog` to take an explicit panel ID kept in dialog state. Relevance: medium. It becomes high as soon as any of those dialogs is reachable from Compare or Dedup.

## Delete-dialog source fields are a data clump

`deleteDialogSelGen`, `deleteDialogPanelPath`, `deleteDialogPanelID` and `deleteDialogPrunedPaths` (`internal/apphandler/dialog/deps.go`) describe one concept and are reset field by field in `ClearDeleteDialogReconcileCache`. For: `deleteDialogPanel()` already centralises the open/no-panel guard, and the fields are plain. Against: a new field must be added in three places, and a missed reset leaks stale state into the next dialog. The fix is one `deleteDialogSource` struct, reset as a value. Relevance: low. Do it on the next delete-dialog change.

## Bare-path delete shows a directory's own size

`OpenDeleteDialogForPath` (`internal/apphandler/dialog/file_ops.go`), used by Compare for a cursor row with no selection, shows the entry's `Size`. For a directory that is the inode size, not the subtree size. It starts no scan and is marked `deleteDialogNoPanel`. For: the dialog is instant and never stuck pending, and it is correct for files. Against: on a destructive confirmation it understates what will be deleted, and it differs from the selection path. The fix is a single-pruned-path source fed through the existing scan, reconcile and refresh pipeline, which also removes `deleteDialogNoPanel`. Relevance: medium–high, because a user can see it on a destructive operation.

## Single rename keeps a typed trailing space

`pathloc.Parse` does not trim, and `executeRename` passes the field unchanged, so renaming to `"bar "` keeps the space. Mkdir, duplicate, self-copy and remote rename trim typed names. For: an existing `"foo "` survives a rename that edits something else, and names round-trip. Against: it is inconsistent with the other name dialogs, and a stray space can create a name that is hard to spot. The options are to trim only when the original had no trailing whitespace, or to show trailing whitespace visibly (e.g. a dimmed `·`) in panel and dialog. Relevance: low–medium. It is a UX choice, not a defect.

## Jump mode is a flag checked across the Find dialog

`FindDialogState.JumpMode` (jump to directory) reuses the Find dialog and its scan, and is guarded by `if JumpMode` checks in `internal/apphandler/find/handler.go` (`applyFindToggle`, a hard-coded action list in `RunAction`, `panel.select-toggle` and Insert handling), in `internal/ui/dialog` (`findDialogTitle`, and `FindDialogView`, which leaves the radios out so focus never reaches them), and in the footer keys in `internal/app/input.go`. `OnlyDirectories` is set alongside it, so the state is partly redundant. For: one dialog and one index, and each guard is small. Against: every new Find action must remember a jump-mode guard. The fix is one capability source on `FindDialogState` (e.g. `Marking()`) that `RunAction`, the footer and the view all read. If the modes keep diverging, make jump a separate small dialog sharing only the scan and index. Relevance: medium. Do it before adding the next jump-only or find-only control.

## Group select context is a string switch

`GroupSelectState.Context` is `"find"` or `"output"`, and `"output"` is special-cased in the preview, the apply switch, the full-path flag, input-mode priority (`internal/app/input.go`), and draw order (`GroupSelectOverOutput` in `internal/ui/render.go`, which stacks it above the output dialog). For: two hosts, and each branch is short. Against: a third host repeats every switch, a typo in a context string fails silently, and the stacking rule is a one-off rather than a general modal parent. The fix is for the opener to supply the behaviour (`Preview`, `Apply`, `ShowsFullPath`) as a small interface or typed value, plus a generic dialog stack for "opened over another modal". Relevance: medium. It becomes high when a third group-select host appears.

## Disk-usage cache is told about renames per entry point

Single rename and mass rename (`applyMassRenameRows` in `internal/apphandler/dialog/mass_rename.go`) call `diskUsageRenamed` (`internal/apphandler/dialog/file_ops.go`), while move jobs reach `diskusage.Engine.RenamePaths` through an `OnRenamed` hook that `internal/app/app.go` wires through `jobbridge.TransferFunc`. Each entry point opts in separately, and the local/remote guard is written in more than one place. For: three explicit call sites, and the batching in `jobbridge` avoids a cache sweep per renamed child. Against: a new rename path (undo, a Compare rename) leaves the cache stale until someone remembers the call, and `TransferFunc` grew another positional parameter. The fix is for the shared local rename primitive to emit one "paths renamed" event, with a single app subscriber feeding the engine. Relevance: low–medium.

## Disk-usage rename rebase walks ancestors twice per key

`rebaseKeys` (`internal/diskusage/engine.go`) calls `rebase`, then `stale` when `rebase` misses. Each one is a `nearestAncestor` walk of `filepath.Dir` to the root, repeated for `cache`, `fileCounts` and `excluded` while holding `e.mu`. For: it runs once per job on the job goroutine, not the UI goroutine, and the code is simple. Against: on a very large size cache, scans and readers block on `e.mu` for the whole sweep. The fix is one walk per key that answers both questions, or a cheap prefix check that skips keys under no rename root. Relevance: low. Do it only if profiling shows it.

## Find walker reads directories sorted and serially

`runBFS` (`internal/scan/walk.go`) uses `os.ReadDir`, which sorts each directory, one directory at a time, and in `DirsOnly` mode still reads the file entries it discards. For: the sort keeps the order within each depth level stable, and the previous `WalkDir` sorted too. Against: on big or slow trees the walk is bound by serial `getdents` latency. The fix is `f.ReadDir(-1)` with no sort, if a stable order is not needed, and optionally a small worker pool per level. Relevance: low.

## "Removed conflicting selections" warning repeated

`setTransientMessage("Removed conflicting selections", ui.MessageUrgencyWarn)` after a conflicting selection change appears at about a dozen sites across `internal/app` and `internal/apphandler/commands`. The output dialog's invert and group-unselect also hand-roll `TogglePathSelection` loops, and `internal/panel` has `BulkAddSelections` but no bulk-remove counterpart. For: each site is one line. Against: the wording and urgency can drift, and the toggle loops are near-copies. The fix is one helper (`warnSelectionConflicts(bool)`, reachable from both `app` and the handler `Host`) and a `panel.BulkRemoveSelections`. Relevance: low.
