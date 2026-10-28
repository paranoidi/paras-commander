# Meta columns

Meta columns add computed columns to a panel by running a shell command per entry, for example video duration, image dimensions, line counts or a git author. The command output is shown between the name and size columns. Columns are defined in `meta.toml` and switched on per panel.

## Open

`Alt+,` (`panel.meta`, leader `,`, Left/Right menu, Meta) opens a checkbox picker of the defined entries; tick any subset. `Alt+;` (`panel.meta-edit`, also `Shift+Alt+,`) opens `meta.toml` in `$EDITOR`; `F9` in the picker does the same and reloads the list. Both are rebindable in `keybindings.toml`. A per-directory `meta.toml` (name from `[meta].local_names`) is preferred over the global one.

Because its commands run as you, a `meta.toml` is loaded only when it is a regular file owned by you (or root) that no one else can write: not world-writable, and group-writable only when its group is your own private group (named after your user, the usual setup with umask `0002`). An untrusted file is skipped with a warning. When the picker has nothing to offer it says why in place of the list: the file was skipped (with a `chmod` hint), it failed to parse, or no commands are defined yet.

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

Up to 8 columns can be active per panel, ordered by `order` then name. Output is filled in as commands finish; a pending cell shows an icon. If the output contains tabs or newlines it is split into aligned sub-columns. Numeric cells are right-aligned and text is left-aligned; wide columns shrink first and are clipped with an ellipsis. In the carousel view meta columns appear only in the centre column.

A failing command leaves the cell empty and shows one warning toast per run, not one per file.

`Ctrl+Y` (`panel.refresh`) drops the cache and recomputes all active columns for that panel. Changing directory runs the active columns for the new listing.

## Sorting and filtering

Active meta columns appear in the Sort dialog (`Alt+S`, up to four). Values that parse as numbers sort numerically; empty, missing and still-running cells sort last. An all-numeric column sorts high-first, others A to Z; the header arrow points toward larger values. Once every value has arrived the panel re-sorts after `[disk_usage].idle_sort_delay_ms` of cursor inactivity rather than reshuffling while results stream in.

The [panel Filter](panel-filter.md) can match meta values with "Include meta" or "Only meta".
