# Meta columns

Meta columns add computed columns to a panel by running a shell command per entry, for example video duration, image dimensions, line counts or a git author. The command output is shown between the name and size columns. Columns are defined in `meta.toml` and switched on per panel.

## Open

`M-,` (`panel.meta`, leader `,`, Left/Right menu, Meta) opens a checkbox picker of the defined entries; tick any subset. `M-;` (`panel.meta-edit`, also `M-S-,`) opens `meta.toml` in `$EDITOR`; `F9` in the picker does the same and reloads the list. Both are rebindable in `keybindings.toml`. A per-directory `meta.toml` (name from `[meta].local_names`) is preferred over the global one.

Because its commands run as you, a `meta.toml` is loaded only when it is a regular file owned by you (or root) that no one else can write: not world-writable, and group-writable only when its group is your own private group (named after your user, the usual setup with umask `0002`). An untrusted file is skipped with a warning. When the picker has nothing to offer it says why in place of the list: the file was skipped (with a `chmod` hint), it failed to parse, or no commands are defined yet.

## Saving default columns

`F5` in the picker (`panel.meta-save`, rebindable under `[dialog.meta]` in `keybindings.toml`) writes the currently ticked columns to `[fileindex].default_columns` in `config.toml`, so the choice survives restarts. Those columns switch on whenever a panel with no meta columns enters an index view (`fileindex://` or Browse with fileindex) and, unless changed from the picker meanwhile, switch off again when it leaves. Saving does not change the panel's columns (`OK` does that) and leaves the dialog open; with nothing ticked it saves an empty list (no defaults). With more ticked than a panel can show, only the first 8 are saved and the toast warns. A toast confirms the save or shows the error. The file is rewritten through the partial config writer, which keeps other settings but not comments. The action is unavailable while the picker lists no columns.

## Built-in columns

Some features add their own columns to the picker, each feature in its own section below your
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

Up to 8 columns can be active per panel, ordered by `order` then name. Output is filled in as commands finish; a pending cell shows an icon, but only for commands still running after `[panels].pending_marker_delay_ms` (default 800 ms), so fast commands do not flash it; until then the cell is blank. Set it to `0` to show the icon immediately, or to `-1` to never show it (a hung command then looks the same as an empty value; see [config.md](config.md)). If the output contains tabs or newlines it is split into aligned sub-columns. Numeric cells (sizes such as `5.6G` or `1.2 GiB` included) are right-aligned and text is left-aligned; wide columns shrink first and are clipped with an ellipsis. In the carousel view meta columns appear only in the centre column.

Columns never crowd out the file name. When a panel has too little width, the rightmost columns are not drawn until the name column has at least 20 cells again; they keep running and still sort, but a column that is not drawn shows no sort arrow. See [Layout](layout.md).

A failing command leaves the cell empty and shows one warning toast per run, not one per file.

`C-y` (`panel.refresh`) drops the cache and recomputes all active columns for that panel. Changing directory runs the active columns for the new listing. In [tree view](panels.md#tree-view) the columns also fill in for expanded directories: each directory's rows are computed as they load, with the command running in that directory.

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

The colors are `warn` and `critical` in the theme's `[panel.meta]` table. Under the active panel's cursor an alerting cell takes the cursor's icon color, like row icons and marks, so it stays readable on the cursor bar. Under a dialog the panel stays dimmed and shows no alerts.

Built-in columns can come with their own rules, such as the [fileindex](fileindex.md#columns) Copies column; these are fixed and cannot be changed from `meta.toml`.

## Sorting and filtering

Active meta columns appear in the Sort dialog (`M-s`, up to four). Values that parse as numbers sort numerically, and so do byte sizes with a binary unit (`512`, `800K`, `5.6G`, `1.2 GiB`, `3 MB`), so `du -h`-style output sorts by size; empty, missing and still-running cells sort last. An all-numeric column sorts high-first, others A to Z; the header arrow points toward larger values. Once every value has arrived the panel re-sorts after `[disk_usage].idle_sort_delay_ms` of cursor inactivity rather than reshuffling while results stream in.

The [panel Filter](panel-filter.md) can match meta values with "Include meta" or "Only meta".
