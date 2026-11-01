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
| Pointer over a row | Highlights the row in the inactive panel's cursor colors. The cursor does not move. |
| Pointer over the path in a panel's top border | Inverts the colors of the path element under it (`~`, `/`, a directory name). |
| Left click on a path element | Focuses the panel and opens that directory, with the cursor on the directory you came from. |
| Left click on the last path element | Focuses the panel and refreshes its directory (the panel's own directory). |

A shortened element (`p` for `projects` in a narrow panel) opens the full directory it stands for. The path does not react while the panel's top border shows the quick filter or a quick-view directory.

Panels in tree view behave the same as flat ones. A panel in carousel mode only takes focus on click. While quick view or a file preview fills the inactive column, clicks and the wheel there are ignored.

In the menu bar and the footer:

| Action | Result |
|---|---|
| Pointer over a menu title | Highlights the title in the open menu's colors. Nothing opens. |
| Left click on a menu title | Opens that menu's pulldown. Clicking the title of the open menu closes it. |
| Pointer over a pulldown item | Selects the item, as the arrow keys would. Enter then runs it. |
| Left click on a pulldown item | Runs the item, like Enter on it. |
| Wheel over the pulldown | Moves the highlighted item. |
| Left click outside the pulldown | Closes the menu. The click does nothing else. |
| Pointer over the status commands (top-left) | Highlights them in the open menu's colors. |
| Left click on the status commands (top-left) | Opens the Status commands dialog. Closes an open menu first. |
| Pointer over a footer entry | Highlights the entry. The part a click would press stands out in brighter text: the key and its main label, or the grey Shift word after it (`Quick` in `F3 ViewQuick`). |
| Left click on a footer entry | Presses that entry's key: `F5 Copy` copies, `Esc Close` closes the open dialog. A click on the grey Shift word presses the Shift variant: `Ren` in `F6 MovRen` renames. |

In dialogs:

| Action | Result |
|---|---|
| Pointer over a button | Highlights the button in the focused button colors. Focus stays where it was. |
| Left click on a button | Presses it. |
| Left click on a radio or checkbox | Selects the radio or toggles the checkbox, like Space. |
| Left click in a text field | Focuses the field and puts the cursor under the pointer. |
| Pointer over a list row | Highlights the row. The list cursor and focus stay where they were. |
| Left click on a list row | Moves the list cursor to the row. |
| Double click on a list row | Moves there and confirms, like Enter. |
| Wheel over a list | Moves the list cursor. |

Clicks on the dialog frame, on text labels and outside the dialog do nothing. The help dialog's entry list, the command output body, the delete and mass rename previews do not take clicks yet.

Menu titles and the status commands take clicks in the file browser and in the jobs, commands, messages, compare and duplicates views, but not while a dialog is open. Footer entries take clicks everywhere the footer shows them, dialogs included.

In the leader menu strip (and the user, copy-path, preview and git filter menus it hosts), a left click on an entry runs it, like its letter. A click outside the strip closes it.

In the jobs, commands and messages views, a left click on a list row selects that job, command or message and focuses the list; the wheel over the list moves the selection. The row under the pointer is highlighted (a whole command or message, across its lines). The selection does not follow it, so moving across the list to the details or output does not switch what they show. In the messages view a click also copies the whole message to the clipboard and confirms it with a toast; a click on the second line of a wrapped message selects and copies the whole message.

Everything else ignores the mouse for now: the other panes of the jobs and commands views, the compare, duplicates and fullscreen preview views, the quick filter and the embedded terminal panel.

## Caveats

- Mouse support asks the terminal for pointer motion reports (for the path highlight), not only clicks.
- With mouse reporting on, the terminal no longer selects text on a plain drag. Hold Shift while dragging (or your terminal's own modifier) to select and copy text.
- Inside tmux, mouse events only reach the application when tmux has mouse mode on: `set -g mouse on`.
- The mouse is switched off while another program owns the terminal (external editor, drop to shell, the persistent subshell) and switched on again when you return.
