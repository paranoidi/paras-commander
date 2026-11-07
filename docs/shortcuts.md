# Shortcuts

## Design philosophy

- **Midnight Commander first.** F-keys do the classic file-manager jobs (`F3` view, `F5` copy, `F6` move, `F7` mkdir, `F8` delete). Shift on an F-key gives a close variant of the same job; the footer shows that variant dimmed next to each F-key.
- **Typing searches.** Arrows navigate and plain letters fuzzy-match the current list, so plain letters are never used as commands in panels.
- **Modifiers say what kind of action it is.** Alt (`M`) changes what you see: views, columns, display modes. Ctrl (`C`) makes changes, to files or selections for example.
- **Mnemonics favour common actions.** When two actions compete for a letter, the more common one gets it (`f` is find, not flatten).
- **Leader menu for discovery.** `:` opens the leader menu, which lists commands by group with a one-letter key and the direct chord next to each. You can learn the chords from it or never learn them at all. `Esc` toggles a persistent vi/leader mode, marked by a yellow border.
- **Vim keys where reading matters.** Read-only views such as the fullscreen preview accept vim-style motion keys.
- **Context decides.** A view or dialog can give a key its own meaning while it has focus, so the same chord can do different things in the jobs view, compare results or a dialog.
- **Dialogs behave the same everywhere.** OK, Cancel and the mnemonics of buttons and options work the same way in every dialog: Alt+letter always, a plain letter too while no text input is focused. The job conflict dialog, delete confirmations and the remove empty directories confirm accept Alt+letter only. Help is the one exception to OK and Cancel: there `M-o` and `M-c` show the command they are bound to, so close it with `Esc` or `F1`.
- **Terminal-safe chords.** Defaults avoid combinations that common terminals don't deliver reliably. A few shortcuts therefore have less obvious letters.
- **Everything is rebindable.** Every action can be bound in `keybindings.toml`; see [Keybindings](keybindings.md).

## Dialogs

These work in every dialog, whatever has focus (`[dialog.input]` in `keybindings.toml`):

| Key | Action |
|---|---|
| `M-o`, `M-Enter` | Press OK |
| `Esc`, `M-c`, `F9` | Press Cancel |

A dialog with an inner layer (a sub-confirm, a rename tool page, an open completion list) closes that layer first. Where Esc means something else it keeps that meaning: in a running command-output dialog it leaves the command running. Some window managers take `M-Enter`; `M-o` always works. The Configuration and meta-column dialogs use `F9` for "edit config" instead. The meta-column dialog also binds `F5` (`[dialog.meta]`) to save the checked columns as the fileindex defaults.

## Text input

Readline-style editing in every dialog text field. Kills go to one shared kill buffer that `C-y` pastes from.

| Key | Action |
|---|---|
| `C-a` / `C-e` | Start / end of line |
| `M-b` / `M-f` | Word left / right |
| `C-w`, `M-Backspace` | Delete previous word |
| `M-d` | Delete next word |
| `C-u` / `C-k` | Delete to line start / end |
| `C-l` | Delete the whole line |
| `C-y` | Paste the last deleted text |
| `M-u` / `M-l` | Upper- / lowercase to end of word |
| `M-t` | Capitalize word (`M-c` is Cancel) |
| `C-r`, `C-d` | Restore the field's default value |
| `*`, `\|` | In path fields: pick from bookmarks, history and pins |

Inside Help (`F1`), `F2` lists these too.

## Changing keys

See [Keybindings](keybindings.md) for `keybindings.toml`, chord syntax, per-view tables and `pc --config-stub`.
