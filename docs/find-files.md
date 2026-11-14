# Find files

Find is a fuzzy, recursive search across everything below a panel's directory. Results appear while the index is still being built, so shallow matches show immediately even in very large trees. [Jump to directory](navigation.md#jump-to-directory) is the same dialog reduced to directories only.

## Opening

`C-f`, Left/Right -> Find ..., or Esc then `f`. It searches below the active panel's path (the Left/Right menu uses that panel). Rebindable in `keybindings.toml`.

## Searching

Type to filter; by default matching is fuzzy against paths relative to the root, best matches first (see [Match modes](#match-modes) for shell patterns and regular expressions). The walk is breadth-first (level by level) and reads no file metadata unless an [advanced criterion](#advanced-criteria) needs it, so it stays responsive on millions of files. While indexing, the dialog title shows the count and the level being read, e.g. `Find (12.3K…) Depth 4 · 37%` plus a scanning icon (with Nerd Font icons on, a depth icon replaces the word `Depth`): the percentage is directories read at that depth out of all directories found for it (across all roots, the deepest level wins). When the whole tree is indexed and the query still matches nothing, the query turns red (while indexing it stays normal, as more results may come). Only the top `[ui.find].max_results` (default 200) ranked rows are displayed, but the whole index is searched, and select-all marks everything that matches.

Options in the dialog:

- **Depth** (input at the right end of the query row, reached with Tab from the query; Up and Down in the query move the result list): limit the index to this many path levels below the root (`a` is depth 1, `a/b` depth 2). Typing a digit 1-9 sets it, `0`, Backspace or Delete clear it for unlimited; either applies it at once and returns focus to the query. Leaving the field (Tab, S-Tab, Up, Down, Left) or pressing Enter, which returns focus to the query, applies it: a lower limit drops the deeper results at once and stops the running scan from going deeper; a higher limit re-indexes to reach the extra levels. The limit starts unlimited each time the dialog opens.
- **Include hidden** (`M-i`, default off): add dot-entries and git-ignored paths. This does not change the panel's own hidden-file setting.
- **Stay on current volume** (`M-v`, default on): do not descend into other mounted file systems.
- **Only directories** (`M-n`) / **Only files** (`M-y`): filter the result list without re-scanning.
- **Advanced** (checkbox at the end of the query row, or `M-a`): show the [advanced criteria](#advanced-criteria).
- **Search only from selected directories** (`M-s`, shown only when directories are selected in the panel, default on): the search is limited to those directories; uncheck it to widen to the whole panel path.

## Match modes

`M-m` cycles how the query is read, or pick it on the **Match** row of the advanced section. A non-fuzzy mode shows its name after the search icon.

| Mode | Query | Order |
|---|---|---|
| Fuzzy (default) | fzf-style terms: `^start`, `end$`, `'exact`, `!exclude`, space = AND | best match first |
| Glob | shell pattern (`*`, `?`, `[a-z]`) on the basename; with a `/` it matches the whole relative path and `*` crosses directories (`src/*_test.go`). A word without wildcards finds names containing it | breadth-first (shallow first) |
| Regex | Go regular expression on the relative path (`_test\.go$`, `^docs/`) | breadth-first |

Matching ignores case when `[filter] case_insensitive` is on, unless **Case sensitive** is checked. A glob or regex that does not compile turns the query red and shows no results.

## Advanced criteria

`M-a` or the **Advanced** checkbox opens a section under the options and moves focus to its first row; the results are greyed out while focus is in the section (their keys do not reach them) and the dialog keeps its size, the section taking rows from the results. While focus is on a check box or radio (not a text field), the highlighted letter of each option toggles it, with or without `Alt`. Every filled field must hold; results update as you change it. Check boxes apply at once; text fields apply on Enter or when focus leaves them. A text field that has had focus for a short pause shows a tooltip below it that explains what it matches and the syntax it takes (`[ui] tooltips = false` in `config.toml` turns tooltips off). A field that does not parse turns red and is ignored. While the section is closed, the criteria in effect are summarized on the line above the results, so nothing filters invisibly. `M-r` clears the mode and every criterion. The section and its values stay for the rest of the session.

| Field | Keeps | Input |
|---|---|---|
| Modified / Changed / Accessed `newer` | entries whose mtime / ctime / atime is at or after the instant | an age before now (`30m`, `2h`, `3d`, `2w`, `6mo`, `1y`, sums such as `1w3d`) or a local date (`2026-10-01`, `2026-10-01 14:30`) |
| Modified / Changed / Accessed `older` | entries whose time is before the instant | same |
| Size `min` / `max` | files of at least / at most this size (directories never match) | `512`, `10k`, `4M`, `1.5G`; binary units, `K`, `KB` and `KiB` are the same |
| Owner | entries owned by this user | user name or uid |
| Not mine | entries owned by someone else | |
| Exclude | everything except matches, which are not even scanned | shell patterns separated by commas or spaces (`node_modules, *.o`); a pattern with `/` matches the path relative to the root |
| Match | the [match mode](#match-modes) and **Case sensitive** | |
| Kind | **Empty** (zero-byte files and directories without entries), **Symlinks**, **Broken symlinks**, **Executables**; checked kinds are alternatives | |
| Types | files of the checked [Filter type groups](panel-filter.md) (`[filter.types]`) | |

Age, size, owner, executable and empty criteria need file metadata, so the first such criterion re-scans with it (Empty also looks inside every directory). Changing Exclude re-scans too. Accessed times are coarse on file systems mounted with `relatime`. Jump to directory has no advanced section.

## Working with results

| Key | Does |
|---|---|
| `Enter` | Navigate: a directory opens in the panel; a file opens its parent with the file highlighted. If rows are marked, they are added to the panel selection instead |
| `Insert` or `Space` | Mark / unmark the row |
| `F5` or `C-a` | Mark all matching results |
| `S-F5` | Unmark all |
| `F6` / `S-F6` | Mark / unmark by group pattern |
| `F2` | Replace the selection with the parent directories of the marked results |
| `F3` | View the highlighted file fullscreen; closing the viewer returns to Find as you left it |
| `F4` | Edit the highlighted file in the external editor and keep Find open |
| `M-a` | Show / hide the advanced criteria |
| `M-m` | Next match mode |
| `M-r` | Reset the match mode and criteria |
| `S-←` / `S-→` | Open the result in the left / right panel and keep Find open |
| `M-Insert` | Pin the row |
| `C-f` | Hide Find and keep it: scan, query, results and marks (`M-c` in Jump to directory). The panel shows a yellow search icon at its bottom right; the same key on that panel and directory brings Find back as you left it. Opening Find anywhere else asks: **Restore** brings the parked search back, **New search here** drops it and starts over, Esc leaves it parked |
| `Esc` | Cancel the scan and close |

Marked results show their combined size on the separator line. Marking a directory removes conflicting marks of its contents or parents and says so in a toast. The footer shows the keys that apply; the unmark keys appear as the grey Shift word after their mark key (`Select allUnselect`).

## Options

[`[ui.find]`](config.md#uifind): `max_results`, `query_debounce_ms`, `list_nav_idle_ms`. Mount-point behaviour follows `descend_into_mount_points` in [`[disk_usage]`](config.md#disk_usage).
