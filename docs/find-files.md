# Find files

Find is a fuzzy, recursive search across everything below a panel's directory. Results appear while the index is still being built, so shallow matches show immediately even in very large trees. [Jump to directory](navigation.md#jump-to-directory) is the same dialog reduced to directories only.

## Opening

`Ctrl+F` (`panel.find-dialog`), Left/Right -> Find ..., or Esc then `f`. It searches below the active panel's path (the Left/Right menu uses that panel). Rebindable in `keybindings.toml`.

## Searching

Type to filter; matching is fuzzy against paths relative to the root, best matches first. The walk is breadth-first (level by level) and never reads sizes, so it stays responsive on millions of files. While indexing, the dialog title shows the count and the level being read, e.g. `Find (12.3K…) depth 4 · 37%` plus a scanning icon (with Nerd Font icons on, a depth icon replaces the word `depth`): the percentage is directories read at that depth out of all directories found for it (across all roots, the deepest level wins). Only the top `[ui.find].max_results` (default 200) ranked rows are displayed, but the whole index is searched, and select-all marks everything that matches.

Options in the dialog:

- **Stay on current volume** (`Alt+V`, default on): do not descend into other mounted file systems.
- **Include hidden** (`Alt+I`, default off): add dot-entries and git-ignored paths. This does not change the panel's own hidden-file setting.
- **Only directories** (`Alt+D`) / **Only files** (`Alt+L`): filter the result list without re-scanning.
- **Search only from selected directories** (`Alt+S`, shown only when directories are selected in the panel, default on): the search is limited to those directories; uncheck it to widen to the whole panel path.

## Working with results

| Key | Does |
|---|---|
| `Enter` | Navigate: a directory opens in the panel; a file opens its parent with the file highlighted. If rows are marked, they are added to the panel selection instead |
| `Insert` or `Space` | Mark / unmark the row |
| `F5` or `Alt+A` | Mark all matching results (`find.select-all`) |
| `F4` | Unmark all (`find.unselect-all`) |
| `F6` / `F7` | Mark / unmark by group pattern (`find.select-group`, `find.unselect-group`) |
| `F2` | Replace the selection with the parent directories of the marked results (`find.select-parent-dirs`) |
| `F3` | View the highlighted file fullscreen (`find.view`) |
| `Shift+Left` / `Shift+Right` | Open the result in the left / right panel and keep Find open |
| `Alt+Insert` | Pin the row |
| `Esc` | Cancel the scan and close |

Marked results show their combined size on the separator line. Marking a directory removes conflicting marks of its contents or parents and says so in a toast. The footer shows the keys that apply.

## Options

[`[ui.find]`](config.md#uifind): `max_results`, `query_debounce_ms`, `list_nav_idle_ms`. Mount-point behaviour follows `descend_into_mount_points` in [`[disk_usage]`](config.md#disk_usage).
