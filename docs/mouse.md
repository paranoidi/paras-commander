# Mouse

Optional mouse support for the twin-panel file browser, the menus, the footer, the jobs, commands and messages lists, and dialogs. Off by default; turn it on with **Mouse support** in the Configuration dialog (Options menu), or with `[ui].mouse = true` in `config.toml` (see [Configuration](config.md)). Either way it applies at once.

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

In dialogs:

| Action | Result |
|---|---|
| Left click on a button | Presses it. |
| Left click on a radio or checkbox | Selects the radio or toggles the checkbox, like Space. |
| Left click in a text field | Focuses the field and puts the cursor under the pointer. |
| Left click on a list row | Moves the list cursor to the row. |
| Double click on a list row | Moves there and confirms, like Enter. |
| Wheel over a list | Moves the list cursor. |

Clicks on the dialog frame, on text labels and outside the dialog do nothing. The help dialog's entry list, the command output body, the delete and mass rename previews do not take clicks yet.

Menu titles take clicks in the file browser and in the jobs, commands, messages, compare and duplicates views, but not while a dialog is open. Footer entries take clicks everywhere the footer shows them, dialogs included.

In the leader menu strip (and the user, copy-path, preview and git filter menus it hosts), a left click on an entry runs it, like its letter. A click outside the strip closes it.

In the jobs, commands and messages views, a left click on a list row selects that job, command or message and focuses the list; the wheel over the list moves the selection. In the messages view a click on the second line of a wrapped message selects the whole message.

Everything else ignores the mouse for now: the other panes of the jobs and commands views, the compare, duplicates and fullscreen preview views, the quick filter and the embedded terminal panel.

## Caveats

- With mouse reporting on, the terminal no longer selects text on a plain drag. Hold Shift while dragging (or your terminal's own modifier) to select and copy text.
- Inside tmux, mouse events only reach the application when tmux has mouse mode on: `set -g mouse on`.
- The mouse is switched off while another program owns the terminal (external editor, drop to shell, the persistent subshell) and switched on again when you return.
