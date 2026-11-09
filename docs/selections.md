# Selections

A panel keeps a set of selected paths. File operations (copy, move, delete, run for each, and so on) act on the selection, or on the cursor entry when nothing is selected. Selections survive directory changes, so you can collect files from several places before one operation.

## Selecting

| Key | Action |
|---|---|
| `Insert` / `Space` | Toggle the cursor entry |
| `+` | Select group dialog |
| `-` / `M-u` | Unselect group dialog |
| `*` / `C-i` | Invert the selection |
| `M-k` | First press sets the start, second press selects everything between it and the cursor |
| `C-u` | Clear the selection |

All are rebindable in `keybindings.toml`. Leader menu (`:`) Selection group: `k` range, `g` group, `i` invert, `U` clear. While a range start is pending, its row shows a marker, the cursor row turns yellow and the rows in between are tinted. With [mouse support](mouse.md) on, dragging with the left button selects a range too, and dragging with the right button unselects one. Range select also works in the carousel, [Compare](compare-directories.md) and [Find duplicates](find-duplicates.md).

Selecting a directory and one of its descendants cannot both stay selected: adding one removes the other and shows "Removed conflicting selections".

## Select and unselect group

The dialog has the same mode radios and type checkboxes as the [panel Filter](panel-filter.md): Simple / auto shell (the default: substring, or a shell pattern once it contains `*` or `?`), Shell patterns, Regular expression; Videos, Pictures, Archives, Audio, Documents (from `[filter.types]`); Files only, Directories only; Case sensitive. Option letters match Filter. Matching is on the file name. There is no Invert.

The pattern opens prefilled with the extension of the file under the cursor (e.g. `*.stl`), shown dimmed: Enter uses it, typing replaces it, editing keys keep it. Directories and files without an extension open empty.

A live count beside the Pattern label shows how many files and folders the pattern would newly select (or unselect). The input turns red when the pattern is invalid or matches nothing. With "Directories only" the type checkboxes are disabled. An empty pattern with types checked selects every entry of those types; an empty pattern and no types selects nothing. The dialog also works inside Find results (with "Match full path"), in the [Find duplicates](find-duplicates.md) results (marking duplicates, also with "Match full path") and on the paths detected in a user-menu output dialog.

## Selected strip

When selected paths lie outside the current directory, a Selected strip lists them under (or beside) the panel with paths relative to the common root. A bottom-border label shows `N item(s) (size)`; sizes of selected directories are computed in the background (see [Disk usage](disk-usage.md)), and an icon on the strip's border marks a selection that spans several locations.

| Key | Action |
|---|---|
| `C-s` | Focus the strip (shows it even when everything is local) |
| `C-M-s` | Change to the common root of the selection |
| `Enter` (in strip) | Go to the directory containing the highlighted path |
| `Insert` / `Delete` (in strip) | Unselect the row |
| typing (in strip) | Fuzzy filter the strip |
| `F2` (in strip) | Replace the selection with the parent directories |

Copying or moving a multi-location selection shows every source in the transfer dialog and keeps the directory structure below the common root ([File operations](fileoperations.md)). Selected items deleted in the background drop out of the selection.

## Stash

Each panel has one stash slot. `C-M-Insert`:

- selection and empty stash: save the selection and clear it;
- no selection and a stash: restore it;
- both: a dialog offers Replace, Merge, Drop stash, Drop all or Cancel.

Opening [Compare](compare-directories.md) with a selection offers to stash it; a panel that already has a stash gets the selection merged into it.

A stash count appears on the panel's bottom border.
