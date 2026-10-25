# Mouse

Optional mouse support for the twin-panel file browser. Off by default; enable it with `[ui].mouse = true` in `config.toml` (see [Configuration](config.md)). The change applies as soon as the configuration is reloaded.

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

Everything else ignores the mouse for now: other views (jobs, messages, compare, duplicates, commands, help, fullscreen preview), dialogs, menus, the quick filter and the embedded terminal panel.

## Caveats

- With mouse reporting on, the terminal no longer selects text on a plain drag. Hold Shift while dragging (or your terminal's own modifier) to select and copy text.
- Inside tmux, mouse events only reach the application when tmux has mouse mode on: `set -g mouse on`.
- The mouse is switched off while another program owns the terminal (external editor, drop to shell, the persistent subshell) and switched on again when you return.
