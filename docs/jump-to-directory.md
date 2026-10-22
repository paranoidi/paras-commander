# Jump to directory

Press `Alt+M` (action `panel.jump-dialog`, rebindable in `keybindings.toml`), choose Left/Right -> "Jump to directory", or use the leader menu (`Esc`, then `t`) to change directory by typing part of its name.

It is the Find dialog (`Ctrl+F`) reduced to one job: fuzzy-type, press Enter, the panel changes to that directory.

## How it searches

Only directories are indexed, breadth-first: every directory directly under the panel's path appears first, then the next level down, and so on. Shallow, likely targets show up immediately, even in very large trees. Files are never indexed.

Type to filter; the best matches are listed first. Enter changes the panel to the highlighted directory. `Shift+Left` / `Shift+Right` point the left or right panel at it and keep the dialog open. `Alt+Insert` pins it. `Esc` cancels the scan and closes the dialog.

## Options

- **Stay on current volume** (`Alt+V`, default on) does not descend into other mounted file systems.
- **Include hidden** (`Alt+I`, default off) adds dot-directories and git-ignored directories.

See also [Navigation](navigation.md) and [Find files](find-files.md).

Marking, select all, group select, the view key, Only directories / Only files and the selected-directories scope of Find are not available here.
