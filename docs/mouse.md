# Mouse

Optional mouse support for the twin-panel file browser, the menus, the footer, the jobs, commands and messages lists, and dialogs. Off by default; turn it on with **Mouse support** in the Configuration dialog (Options menu), or with `[ui].mouse = true` in `config.toml` (see [Configuration](config.md)). Either way it applies at once.

## What works

In the file browser:

| Action | Result |
|---|---|
| Left click on a row | Focuses that panel and moves its cursor to the row. |
| Left click elsewhere in a panel | Focuses the panel. |
| Double click on a row | Opens it, like Enter. |
| Drag with the left button | Tints the rows between the pressed row and the pointer, and adds them to the selection on release. See [Drag select](#drag-select). |
| Drag with the right button | The same with a red tint, and removes the rows from the selection on release. |
| Right click on a row | Focuses the panel and moves the cursor on press, and toggles the row's selection on release. |
| Wheel | Scrolls the panel under the pointer by a few rows. Focus does not change. |
| Pointer over a row | Highlights the row in the inactive panel's cursor colors. The cursor does not move. A key press hides the highlight until the mouse moves again. |
| Pointer over the path in a panel's top border | Inverts the colors of the path element under it (`~`, `/`, a directory name). |
| Left click on a path element | Focuses the panel and opens that directory, with the cursor on the directory you came from. |
| Left click on the last path element | Focuses the panel and refreshes its directory (the panel's own directory). |
| Left click on a column title (Name, Size, Modified, a meta column) | Focuses the panel and sorts by that column. Clicking the column already sorted on reverses the order. Name also counts as sorted while sorting by extension. |
| Left click elsewhere on the column header row | Focuses the panel and opens its Sort order dialog. |

A shortened element (`p` for `projects` in a narrow panel) opens the full directory it stands for. The path does not react while the panel's top border shows the quick filter or a quick-view directory.

Panels in tree view behave the same as flat ones. A panel in carousel mode only takes focus on click. Picking a column turns the sort dialog's disk-usage size ordering off, so it does not take over again; while that ordering is in effect, Size counts as the sorted column. A narrow name-only panel shows one header title, so a click anywhere on its header opens the dialog. While quick view or a file preview fills the inactive column, clicks and the wheel there are ignored.

### Drag select

Press the left button on a row and move the pointer to another row. The cursor follows the pointer and the rows in between are tinted. Releasing the button adds those rows to the selection, the same as range select from the keyboard: rows already selected stay selected, and `..` is skipped. The cursor stays on the row where you let go.

Dragging with the right button unselects instead. The rows in between get a red tint, and releasing removes them from the selection; rows that were not selected are left as they are. A right button released on the row it was pressed on is a plain right click.

- Moving past the top or bottom of the list scrolls one row per pointer movement. The wheel also extends the range.
- The drag stays with the panel it started in, even when the pointer crosses into the other panel. The menus and the footer do not react until you release.
- Any key press cancels the drag and selects nothing. Esc is the plain way out.
- A long list can scroll while you drag, so the cursor then follows the pointer's movement and may not sit exactly under it.
- A drag does not start in a panel in carousel mode, or from the path or the column header.

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
| Pointer over a list row | Highlights the row. The list cursor and focus stay where they were. A key press hides the highlight until the mouse moves again. |
| Left click on a list row | Moves the list cursor to the row. |
| Double click on a list row | Moves there and confirms, like Enter. |
| Wheel over a list | Moves the list cursor. |

Clicks on the dialog frame, on text labels and outside the dialog do nothing. The help dialog's entry list, the command output body, the delete and mass rename previews do not take clicks yet.

A key press hides every pointer highlight (rows, path segments, buttons, menu titles, status commands, footer entries) until the mouse moves again.

Menu titles and the status commands take clicks in the file browser and in the jobs, commands, messages, compare and duplicates views, but not while a dialog is open. Footer entries take clicks everywhere the footer shows them, dialogs included.

In the leader menu strip (and the user, copy-path, preview and git filter menus it hosts), a left click on an entry runs it, like its letter. A click outside the strip closes it.

In the jobs, commands, messages, compare and duplicates views, a left click on a list row selects that job, command, message, compare row or duplicates row (in the duplicates view it also focuses the main tree, the Copies pane or the browse panel under the pointer); the wheel over the list moves the selection. The row under the pointer is highlighted (a whole command or message, across its lines; in the compare view both columns of the row); a key press hides the highlight until the mouse moves again. The selection does not follow it, so moving across the list to the details or output does not switch what they show. In the messages view a click also copies the whole message to the clipboard and confirms it with a toast; a click on the second line of a wrapped message selects and copies the whole message.

In the fullscreen text preview, drag with the left button to select text. Double-click a word to select and copy it; a word is letters, digits and underscores, so dots, hyphens and slashes end it. Releasing the button copies the selection to the clipboard. The toast shows the copied text, or the number of copied lines when the selection is 70 characters or longer. The highlight stays until the next left press on the text or until the preview reloads. Dragging past the top or bottom edge scrolls one row per pointer report.

The selection leaves out the scrollbar and the line numbers, and soft-wrapped rows copy as one line. The copied text is what the preview shows: tabs arrive as the spaces they expand to, rendered markdown as rendered, and line numbers printed by an external `[preview].command` (such as `bat -n`) count as text.

Everything else ignores the mouse for now: the other panes of the jobs and commands views, the rest of the fullscreen preview view, the quick filter and the embedded terminal panel.

## Caveats

- Mouse support asks the terminal for pointer motion reports (for the path highlight), not only clicks.
- With mouse reporting on, the terminal no longer selects text on a plain drag (the fullscreen text preview has its own selection). Hold Shift while dragging (or your terminal's own modifier) to select and copy text anywhere.
- Inside tmux, mouse events only reach the application when tmux has mouse mode on: `set -g mouse on`.
- The mouse is switched off while another program owns the terminal (external editor, drop to shell, the persistent subshell) and switched on again when you return.
