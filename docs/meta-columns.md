# Meta columns

Meta columns add computed columns to a panel by running a shell command per entry, for example video duration, image dimensions, line counts or a git author. The command output is shown between the name and size columns. Columns are defined in `meta.toml` and switched on per panel.

## Open

`M-,` (leader `,`, Left/Right menu, Columns...) opens the [Columns dialog](columns.md), which lists the defined entries in its Meta section; tick any subset. `M-;` (also `M-S-,`) opens `meta.toml` in `$EDITOR`; `F9` in the dialog does the same and reloads the list. Both are rebindable in `keybindings.toml`. A per-directory `meta.toml` (name from `[meta].local_names`) is preferred over the global one.

Because its commands run as you, a `meta.toml` is loaded only when it is a regular file owned by you (or root) that no one else can write: not world-writable, and group-writable only when its group is your own private group (named after your user, the usual setup with umask `0002`). An untrusted file is skipped with a warning. When the picker has nothing to offer it says why in place of the list: the file was skipped (with a `chmod` hint), it failed to parse, or no commands are defined yet.

## Saving columns

A [column profile](columns.md#profiles) saves ticked meta columns together with file columns, and can be the startup default or the default for [index views](fileindex.md#columns).

## Built-in columns

Some features add their own columns to the dialog, each feature in its own section below your
`meta.toml` entries, under a separator headed by the feature's name (for example **Status**, **Tags** and
**Tree files** under **File Index**, the [fileindex](fileindex.md#columns) columns, listed only while the fileindex database
opens). They are computed inside pc once per directory listing instead of
running a command per entry, need no `meta.toml`, and otherwise behave like any meta column:
the same picker, column limit, sorting and filtering. A built-in column replaces a `meta.toml`
entry with the same name.

## Defining a column

Each `[[entry]]` in `meta.toml` has a unique `name`, a `description` for the picker, and a script per entry type: `file` for files, `dirs` for directories. `%f` expands to the entry's absolute path (do not quote it). Scripts run through `sh -c`. The generated file lists every field in its header; the main ones:

```toml
[[entry]]
name = "lines"
description = "Line count"
column = "Lines"
file = "wc -l < %f"
when = ["*.txt", "*.md"]
cache = true
```

| Field | Purpose |
|---|---|
| `column` | Header text (default: the entry name) |
| `order` | Left-to-right position, lowest first |
| `when` | Which rows the command runs for; same [predicate language](predicates.md) as preview commands. Applies to files and directories alike. |
| `cache` | Keep results in memory for the session |
| `workers` | Parallel commands for this column |
| `sort_on_activation`, `sort_reverse` | Sort the panel by this column when it is switched on |

`[meta]` in [config.md](config.md#meta) sets the file location, per-directory file names and default worker count. Meta commands and the user menu use the same predicate language and command runner but are separate files (`meta.toml` and `menu.toml`; see [Commands](commands.md)).

## Display

Up to 8 columns can be active per panel, ordered by `order` then name. Output is filled in as commands finish, in batches a few times a second: a panel first waits a quarter of a second so columns are sized from real values, and updates at once when the last command finishes. A column starts at the width it last had on that panel, so moving between similar directories or refreshing keeps the listing still; if that was too wide, it shrinks when the last command finishes. A pending cell shows an icon, but only for commands still running after `[panels].pending_marker_delay_ms` (default 800 ms), so fast commands do not flash it; until then the cell is blank. Set it to `0` to show the icon immediately, or to `-1` to never show it (a hung command then looks the same as an empty value; see [config.md](config.md)). If the output contains tabs or newlines it is split into aligned sub-columns. Numeric cells (sizes such as `5.6G` or `1.2 GiB` included) are right-aligned and text is left-aligned; wide columns shrink first and are clipped with an ellipsis. In the carousel view meta columns appear only in the centre column.

Columns never crowd out the file name. When a panel has too little width, the rightmost columns are not drawn until the name column has at least 20 cells again; they keep running and still sort, but a column that is not drawn shows no sort arrow. See [Layout](layout.md). If incoming results widen a column enough to push one out mid-run, a toast names it ("Column x no longer fits on available space"), once per run. Columns that never fit, ones gone within 500 ms of the run starting, and ones lost to a resize or zoom get no toast.

A failing command leaves the cell empty and shows one warning toast per run, not one per file.

Commands do not run for rows that exist only in the index ([ghost rows](fileindex.md)), since there is no file to run them on; their cells stay empty. File Index columns still fill in for them.

`C-y` drops the cache and recomputes all active columns for that panel. When pc itself changes something (delete, rename, copy over a file, and so on), only that item is recomputed and its cached value dropped; everything else keeps its value. Renaming a directory therefore recomputes that one row. Changes that affect a parent directory's value, such as deleting a file inside it, show after `C-y`. File Index columns are the exception: they update on every row when the index changes, because a change to one file can alter another's values, such as its copy count. Changing directory runs the active columns for the new listing. In [tree view](panels.md#tree-view) the columns also fill in for expanded directories: each directory's rows are computed as they load, with the command running in that directory.

## Alert colors

`warn` and `critical` on an `[[entry]]` color the cells worth noticing. Each takes a rule or a list of rules; a cell alerts when any rule in the list matches, and `critical` wins over `warn`.

```toml
[[entry]]
name = "lines"
description = "Line count"
file = "wc -l < %f"
warn = ">1000.5"
critical = [">5000", "error"]
```

| Rule | Matches |
|---|---|
| `>N`, `>=N`, `<N`, `<=N` | Numbers. `N` is a float or a byte size (`1.5`, `0.25`, `1G`), read the same way as for sorting. A cell that is not a number never matches. |
| `=X`, `!=X` | The whole value: as numbers when both sides are numbers (`=1.50` matches `1.5`), otherwise as text. |
| any other text | A substring of the value. |

Text matching ignores case. Anything not starting with an operator is a substring, so `warn = "2"` also matches `12`; use `=2` for the number. Text starting with an operator character can only be matched as a whole value (`=>5`).

Blank cells, still-running cells and output split into sub-columns never alert. An invalid rule, such as `>abc`, is reported as a `meta.toml` error in the picker.

The colors are `warn` and `critical` in the theme's `[panel.meta]` table. On the cursor row every column takes the file name's color (an unselected name's, so a selected file's columns are not drawn in the selection color), so an alerting cell under the active panel's cursor takes the cursor's icon color, like row icons and marks, and is bold to stand out from its neighbours. Under a dialog the panel stays dimmed and shows no alerts.

Built-in columns can come with their own rules, such as the [fileindex](fileindex.md#columns) Copies column; these are fixed and cannot be changed from `meta.toml`.

## Sorting and filtering

Active meta columns appear in the Sort dialog (`M-s`, up to four). Values that parse as numbers sort numerically, and so do byte sizes with a binary unit (`512`, `800K`, `5.6G`, `1.2 GiB`, `3 MB`), so `du -h`-style output sorts by size; empty, missing and still-running cells sort last, and so does a cell holding only `-`, which also keeps a numeric column numeric. An all-numeric column sorts high-first, others A to Z; the header arrow points toward larger values. Once every value has arrived the panel re-sorts after `[disk_usage].idle_sort_delay_ms` of cursor inactivity rather than reshuffling while results stream in.

The [panel Filter](panel-filter.md) can match meta values with "Include meta" or "Only meta".
