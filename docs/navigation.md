# Navigation

Besides typing paths and pressing Enter, Paras Commander has several ways to get back to places: directory history, pinned items, bookmarks, [jump to directory](#jump-to-directory), [Find](find-files.md) and a latched panel sync. All dialogs are fuzzy-filtered by typing.

## Directory history

| | |
|---|---|
| Back / forward | `Ctrl+Left` / `Ctrl+Right` (`nav.backward`, `nav.forward`) |
| History dialog | `Alt+H` (`panel.history-dialog`) or `Ctrl+H`; Left/Right -> History...; Esc, `h` |
| Both panels | `F5` inside the dialog (`panel.history-both-panels`) |

Each panel remembers every directory it visited (most recent first, up to 200). Back and forward move through that list without adding entries. A directory that no longer exists is skipped and a warning toast says so. Returning to a directory restores the previously highlighted entry.

The dialog lists the target panel's history, without its current directory. `F5` merges both panels (other panel first). Every row has a source prefix (history or zoxide) and rows whose directory has disappeared are shown in the invalid colour with a missing icon. Enter opens the highlighted directory in the panel; `Shift+Left` / `Shift+Right` open it in the left or right panel and keep the dialog open.

If [zoxide](https://github.com/ajeetdsouza/zoxide) is installed, an **Include zoxide** checkbox (`Alt+I`) appends its ranked directories after the panel history. The initial state is `[history].zoxide` (default on). Without zoxide the checkbox is hidden.

## Pinned items

`Alt+Insert` (`panel.pin-toggle`) pins or unpins the file or directory under the cursor; a pinned entry shows a pin icon and the menu bar shows a pin count. The same key pins the highlighted row in the Find, History, Bookmarks, Compare and Duplicates views. `Alt+N` (`panel.pin-dialog`, Esc then `N`) opens the pinned list.

Pins last for the session and are not saved to disk. In the pin dialog Enter opens the entry in the active panel, `Shift+Left` / `Shift+Right` open it in a panel and keep the dialog open, `F3` previews it fullscreen and returns to the dialog on close, `F8` unpins and `Shift+F8` clears all.

## Latched panel sync

`Alt+Y` (`panel.toggle-sync`) makes the active panel the driver: moving the cursor onto a directory shows that directory in the other panel. The status line shows ` Sync -> ` or ` <- Sync `. Only one panel drives; pressing `Alt+Y` on the other panel moves the role. Following does not touch the other panel's history. Cursor on a file does nothing.

Sync turns itself off when you use "open directory in other panel" (`Alt+O`) or "open current path in other panel" (`Alt+I`), when you enable quick view, and when you hide the inactive panel.

## Bookmarks and the path picker

`Ctrl+B` (also `Ctrl+G`, `Ctrl+E`; `bookmark.open`, Esc then `B`) opens the bookmarks list. `Ctrl+Alt+B` (`bookmark.add`, Esc then `b`) adds the current directory under a name you type (empty names are rejected).

Bookmarks are compatible with [fzf-marks](https://github.com/urbainvaes/fzf-marks): marks are read from `[bookmarks].file`, else `$FZF_MARKS_FILE`, else `~/.fzf-marks`, as `name : path` lines, and new marks are appended there. GTK 3 bookmarks (`~/.config/gtk-3.0/bookmarks`) are listed after them, read-only. The Source column shows which one each row came from. `F8` deletes a mark (fzf-marks only). Enter opens in the active panel; `Shift+Left` / `Shift+Right` open in a panel and keep the dialog open.

The same list serves as a **path picker** in path fields (copy and move destinations, flatten, extract, symlink): while such a field is focused, the footer offers All, Bookmarks, History and Pinned. `*` or `|` opens the combined list; `Ctrl+B`, `Alt+H` and `Alt+N` open the single-source ones. Typing a path in these fields has its own [autocomplete](path-completion.md).

## Jump to directory

`Alt+M` (`panel.jump-dialog`, Left/Right menu "Jump to directory", leader `t`) changes the panel's directory by typing part of its name. It is the [Find](find-files.md) dialog reduced to one job: fuzzy-type, press Enter, the panel changes to that directory.

Only directories are indexed, breadth-first: every directory directly under the panel's path first, then the next level down, and so on, so shallow targets show up immediately even in very large trees.

Enter changes the panel to the highlighted directory. `Shift+Left` / `Shift+Right` point the left or right panel at it and keep the dialog open. `Alt+Insert` pins it. `Esc` cancels the scan and closes the dialog.

- **Stay on current volume** (`Alt+V`, default on) does not descend into other mounted file systems.
- **Include hidden** (`Alt+I`, default off) adds dot-directories and git-ignored directories.

## Other ways to move

- [Find](find-files.md) (`Ctrl+F`): recursive search for files and directories.
- [SFTP](sftp.md) (`Alt+R`): remote hosts.
- `Alt+O` / `Alt+I`: open the highlighted directory, or the current path, in the other panel.

## Options

[`[history]`](config.md#history), [`[bookmarks]`](config.md#bookmarks), and key-repeat debounce in [`[ui]`](config.md#ui) (Options -> Calibrate Debounce) for smooth cursor-driven sync and previews. All keys are rebindable in `keybindings.toml`.
