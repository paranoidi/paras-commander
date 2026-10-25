# Mouse

Optional mouse support for the twin-panel file browser, the menu bar and the footer. Off by default; enable it with `[ui].mouse = true` in `config.toml` (see [Configuration](config.md)). The change applies as soon as the configuration is reloaded.

## What works

In the file browser:

| Action | Result |
|---|---|
| Left click on a row | Focuses that panel and moves its cursor to the row. |
| Left click elsewhere in a panel | Focuses the panel. |
| Double click on a row | Opens it, like Enter. |
| Right click on a row | Focuses the panel, moves the cursor and toggles the row's selection. |
| Wheel | Scrolls the panel under the pointer by a few rows. Focus does not change. |

Panels in tree view behave the same as flat ones. A panel in carousel mode only takes focus on click. While quick view or a file preview fills the inactive column, clicks and the wheel there are ignored.

In the menu bar and the footer:

| Action | Result |
|---|---|
| Left click on a menu title | Opens that menu's pulldown. Clicking the title of the open menu closes it. |
| Left click on a pulldown item | Runs the item, like Enter on it. |
| Wheel over the pulldown | Moves the highlighted item. |
| Left click outside the pulldown | Closes the menu. The click does nothing else. |
| Left click on a footer entry | Presses that entry's key: `F5 Copy` copies, `Esc Close` closes the open dialog. |

In the leader menu strip (and the user, copy-path, preview and git filter menus it hosts), a left click on an entry runs it, like its letter. A click outside the strip closes it.

Menu titles take clicks in the file browser and in the jobs, commands, messages, compare and duplicates views, but not while a dialog is open. Footer entries take clicks everywhere the footer shows them, dialogs included.

Everything else ignores the mouse for now: the content of the other views (jobs, messages, compare, duplicates, commands, help, fullscreen preview), dialog contents, the quick filter and the embedded terminal panel.

## Caveats

- With mouse reporting on, the terminal no longer selects text on a plain drag. Hold Shift while dragging (or your terminal's own modifier) to select and copy text.
- Inside tmux, mouse events only reach the application when tmux has mouse mode on: `set -g mouse on`.
- The mouse is switched off while another program owns the terminal (external editor, drop to shell, the persistent subshell) and switched on again when you return.
