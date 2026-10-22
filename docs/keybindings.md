# Keybindings

Every shortcut is configurable. Defaults are built in; `keybindings.toml` overrides only what you list. This page covers how to customise it; the default keys are in [Shortcuts](shortcuts.md) and in the in-app help (`F1`).

## File

`keybindings.toml` lives next to `config.toml`: `$XDG_CONFIG_HOME/pc/` or `~/.config/pc/`. It is optional. Generate a file containing every default binding with:

```sh
pc --config-stub
```

Existing files are left untouched. The generated header lists the actions each table accepts.

## Syntax

Each action maps to a list of chords. A chord is an optional modifier prefix plus a key:

| Prefix | Meaning |
|---|---|
| `C-` | Ctrl |
| `M-` | Alt |
| `S-` | Shift |

Examples: `C-f`, `M-v`, `S-F3`, `C-M-p`, `F5`, `left`, `space`. Prefixes combine in any order. Multi-key sequences are not supported.

```toml
[main]
"file.copy" = ["F5", "C-c"]     # replaces the default list for this action
"panel.jump-dialog" = ["M-m"]
```

An action you list replaces its default chords entirely; list every chord you want. Actions you leave out keep their defaults.

## Tables

Bindings are grouped by context. While a view or dialog has focus, its table is consulted before `[main]`.

| Table | Applies to |
|---|---|
| `[main]` | Global and file browser actions |
| `[jobs]`, `[commands]`, `[messages]`, `[compare]`, `[dedup]` | The matching view |
| `[preview]` | Fullscreen preview (F3) |
| `[terminal]` | Embedded terminal panel |
| `[dialog.input]` | Text inputs and the OK/Cancel chords of every dialog |
| `[dialog.rename]`, `[dialog.mass_rename]`, `[dialog.mkdir]`, `[dialog.bookmark]`, `[dialog.find]`, `[dialog.history]`, `[dialog.pin]`, `[dialog.flatten]`, `[dialog.transfer]`, `[dialog.run_for_each]`, `[dialog.command_output]`, `[dialog.help]`, `[dialog.user_menu]` | The matching dialog |
| `[leader_key]`, `[copy_menu]`, `[preview_menu]` | Single-letter keys inside the `:`, `"` and preview menus ([Menus](menus.md)) |

`[jobs]` and `[dialog.input]` here are unrelated to the same-named tables in `config.toml`. Shortcut tables in `config.toml` are rejected.

## Validation

Problems abort startup with a message naming the problem:

- unknown action id, or an action placed in a table that does not allow it (`jobs.*` only under `[jobs]`, `commands.*` only under `[commands]`, `ui.input.*` only under `[dialog.input]`, dialog actions under their own `[dialog.*]` table)
- one chord bound to two actions within a table, or listed twice for one action
- an unknown key name, an empty key, or a multi-stroke value
- values that are not lists of strings
- `[leader_key]`, `[copy_menu]` and `[preview_menu]` values that are not valid letters, or are duplicated within one menu

Fix the file or move it aside and run `pc` again.

## Terminal limits

Terminals do not deliver every combination. Known traps: `Ctrl+H` arrives as Backspace, `Ctrl+Enter` is the same as Enter, `Shift+Enter` needs a terminal with the kitty keyboard protocol, `Ctrl+C`, `Ctrl+Space` and `Ctrl+O` can be intercepted by the terminal or shell, and `Alt+Enter` is taken by some window managers (use `M-o` for dialog OK). Ctrl+Alt combinations are unreliable. If a chord does nothing, try another or check what your terminal sends.

## Related

- Press `F1` in any view for the bindings that apply there; `F2` inside help lists text-input editing keys.
- [Shortcuts](shortcuts.md) for the default scheme and chord conventions.
- [Menus](menus.md) for leader-menu letters.
