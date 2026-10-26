# Themes

Colors, text styles and status glyphs come from a TOML theme file. One theme ships inside the binary (`default`); you can add your own and switch between them at runtime. Syntax highlighting in file previews is a separate system, covered at the end.

## Selecting a theme

- Options -> Theme opens the theme picker. Moving the cursor previews each theme live, `Enter` applies it and saves `theme` in `config.toml`, `Esc` restores the previous theme, and `F5` reloads the highlighted theme from disk (handy while editing it).
- Or set it directly:

```toml
theme = "default"
```

An unknown name falls back to the built-in default and reports an error.

## Where themes live

Put `*.toml` files in `themes/` inside the config directory (`~/.config/pc/themes/`, or `$XDG_CONFIG_HOME/pc/themes/`). A theme is identified by the `name` declared at the top of the file, not by its file name; a disk theme whose `name` matches a built-in one (including `default`) overrides it. Files that fail to parse are skipped when resolving the configured theme, but a broken file in the folder prevents the picker from listing themes, so fix or remove it.

## Structure

The best starting point is a copy of the built-in `themes/default.toml` from the repository; every key in it is documented by position and comment. Some newer optional keys (for example the progress light-bar gradients) fall back to built-in values when absent, but start from a full copy rather than a partial file.

```toml
name = "mine"

[palette]            # 0-255 ANSI/xterm index, "#RRGGBB", or "default"
black = 0
red = 1
# ... through bright_white = 15

[menu.bar]
active   = { fg = "default" }
selected = { fg = "black", bg = "cyan", bold = true }

[panel.row]
directory = { fg = "bright_blue", bold = true }
```

A style is a table with `fg`, `bg` and flags such as `bold`; colors are palette names or hex values. Main sections:

| Section | Styles |
|---|---|
| `[palette]` | The 16 named colors the rest of the file refers to |
| `[panel.*]` | Active, inactive and blocked (dimmed under dialogs) panel chrome; rows, marks, folder icons, git status, scrollbar, disk usage |
| `[menu.*]` | Menu bar, pulldowns, status command, job strip, progress bar, speed pill |
| `[dialog.*]` | Dialog frame, buttons, inputs, options, progress |
| `[jobs.*]`, `[compare]`, `terminal.text` | Jobs view, compare view, embedded terminal |
| `[icons.*]` | Glyphs |

## Status glyphs

Job states each have an icon and a color: scanning, queued, running, paused, canceled, failed, decision and completed. Glyphs are under `[icons.jobs.list]` and colors under `[jobs.icons]`; the menu-bar job strip uses `[icons.menu.job]`. The result icons of the jobs view Operations panel are `[icons.jobs.operations]` `success` / `failed` / `skipped`, colored by `[jobs.operations]` (the same colors mark the failed and skipped counts in the Details panel); without them the completed / failed / paused job colors and matching icons are used. Other icon groups: `[icons.filelist]` (row suffixes such as new, renamed, selection, pinned), `[icons.dialog]` (missing, history), `[icons.tree]`, `[icons.scrollbar]`. Without Nerd Font icons (`use_nerdfont_icons = false`) ASCII markers are used instead.

Changing the theme also changes how the Messages toasts look (`message.info`, `message.warn`, `message.error`).

## Preview (syntax highlighting) themes

File previews use Chroma styles, chosen by `[preview] style` ([Config](config.md#preview)), default `catppuccin-frappe`. Pick one with Options -> Preview settings, or in the fullscreen preview (`F3`) through the `:` menu's theme picker (`t`, action `preview.theme-picker`). Besides Chroma's built-in styles, `pc` loads Chroma XML styles from `themes/preview/*.xml` in the config directory (the repository ships `themes/preview/jetbrains.xml` as an example). See [Quick view and preview](quick-view-preview.md).
