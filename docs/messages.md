# Messages and help

## Messages view

Status toasts (the line above the footer: "Copied 3 files", errors, warnings) disappear after a few seconds. The Messages view keeps a log of them so you can read what you missed.

Open it with `M-l` (also Display -> Messages and leader `L`); the same key closes it. It fills both panels, newest message first.

- Each message carries a local `hh:mm:ss` on its first line; long messages wrap onto indented continuation lines. Up/Down move by message.
- `"` (leader `y`, Actions -> Copy message) copies the selected message's whole text to the clipboard, unwrapped. With [mouse support](mouse.md) on, a click on a message does the same. In this view `"` copies the message instead of opening the copy menu.
- `F8` (leader `c`) empties the log and the banner.
- `Left` or `Esc` (leader `x`) returns to the file view.
- `:` opens a menu scoped to this view; `F9` opens its menu bar ([Menus](menus.md)).

How long toasts stay and how many log lines are kept: `message_ttl_seconds` and `log_max_entries` in [`[ui.status]`](config.md#uistatus). Colors: `message.info`, `message.warn`, `message.error` ([Themes](themes.md)). Keys are rebindable under `[messages]` in `keybindings.toml`.

## Help

Press `F1` or `?` in any view to list the key bindings for that view. The list reflects your `keybindings.toml`, grouped by section (App, Navigation, View, ...), and the title names the view (`Help`, `Help - Jobs`, ...). Menu mnemonics are not listed.

- Type to filter. Matching is the same fuzzy search used elsewhere and looks at the keys, section, title and action name, with synonyms (for example "dotfiles" finds the hidden-files toggle). Section headers stay visible above their matches.
- Up/Down, PgUp/PgDn, Home/End move through rows.
- `Enter` runs the highlighted action, as if you pressed its key. An action that does not apply to the current view, such as a text-editing key, stays unrun and a warning says so.
- `F4` (`[dialog.help]`) finds actions by keypress. A small dialog waits for one key, and help then shows only what that key does. Every key counts, including `Esc`, `F1` and `F10`, so nothing is run or closed by mistake. The action the key runs in the current view comes first, and anything it does in other views and dialogs is listed under *Other contexts*. `Esc` returns to the full list, and you can type to narrow the result. Single-letter keys inside menus are not included. To back out of the waiting dialog, press any key and then `Esc`.
- `F2` (`[dialog.help]`) switches to the text-input editing keys; `F2` again returns.
- `F3` (`[dialog.help]`) opens the icon legend, one icon per line; `F3`, `F1` or `Esc` closes it.
- `F1` or `Esc` closes help. In text fields a literal `?` is typed, not treated as help.

See [Keybindings](keybindings.md) for changing the keys.
