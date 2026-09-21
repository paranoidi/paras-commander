# Shortcuts

## Design philosophy

Default mode uses Midnight Commander–inspired shortcuts. Arrows navigate, letters fuzzy-match the current list, and F-keys plus Ctrl (`C`), Alt (`M`), and Shift (`S`) chords issue commands and change views.

Alt (`M`) combinations generally alter the current view. Ctrl (`C`) combinations issue modifications. Letter mnemonics prefer common actions over rare ones (for example `f` for find, not flatten).

Leader-key commands are available via `:`. The same letters are used in the leader menu and in the Esc function menu where possible. Persistent vi/leader-key mode toggles with Esc and is indicated by a yellow border. This also applies inside the fullscreen file preview (F3): `j`/`k` scroll, `h` closes the view, and the footer shows the `[preview_menu]` letters (search stays `/`), which fire their action directly.

## Syntax

Bindings live in `keybindings.toml` as one chord per string. Spaces (multi-stroke sequences) are rejected.

Prefixes, in any order, before the key name:

| Prefix | Meaning |
|---|---|
| `C-` | Ctrl |
| `M-` | Alt / Meta |
| `S-` | Shift |

Examples: `C-f`, `M-v`, `S-F3`, `C-M-p`. Prefixes are case-insensitive (`c-f` = `C-f`). The remaining key is an F-key (`F1`…), a named key (`left`, `enter`, `insert`, `space`, `tab`, …), or a single printable rune.

## Precedence

`pc` builds a keymap **bundle**: a global `[main]` map plus overlay tables (`[jobs]`, `[commands]`, `[compare]`, `[dedup]`, `[terminal]`, `[dialog.input]`, `[dialog.transfer]`, and so on). While a view or dialog is focused, its overlay is consulted **before** `[main]`. Overlay tables only accept the action IDs that overlay is allowed to bind; see the generated stub header for the current list.

The in-app Help view (**F1** or `?`) shows the bindings that apply to the current view.

## Generate the default file

Do not treat this page as a complete encyclopedia of every action. The generated stub is the source of default chords:

```sh
pc --config-stub
```

That writes `keybindings.toml` (alongside `config.toml`) with every default binding and overlay table. Edit that file to override chords. Unknown action IDs and disallowed overlay IDs fail at load time.

`docs/config.md` describes `config.toml` keys. `keybindings.toml` is a separate file; it happens to reuse some table names (`[jobs]`, `[dialog.input]`) that are unrelated to the same-named `config.toml` tables.

## Feature pages

User docs under `docs/` cover large features, not every command:

| Page | Feature |
|---|---|
| [config.md](config.md) | `config.toml` keys and related files |
| [fileoperations.md](fileoperations.md) | Copy, move, delete, rename, and related dialogs |
| [jobs.md](jobs.md) | Background job queue |
| [graphics.md](graphics.md) | Terminal image preview |
| [background-scan-throttle.md](background-scan-throttle.md) | Disk-usage scan throttle |

There are **no** dedicated user pages for panel compare, SFTP browsing, or the persistent subshell. Use Help (**F1**) and the generated `keybindings.toml` / `config.toml` stubs for those. A few subshell notes that are easy to miss:

- zsh: `setopt HIST_IGNORE_SPACE` so injected `cd` lines stay out of history.
- bash ≥ 5.1: array-style `PROMPT_COMMAND` uses the string-append form for the cwd hook.
- Smoke: Ctrl+O, `cd` through a symlink, toggle out; Alt+Enter on a selection in fish.
