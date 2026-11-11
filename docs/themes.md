# Themes

Colors, text styles and status glyphs come from a TOML theme file. One theme ships inside the binary (`default`); you can add your own and switch between them at runtime. Syntax highlighting in file previews is a separate system, covered at the end.

## Selecting a theme

- Options -> Theme opens the theme picker. Typing fuzzy-filters the list. Moving the cursor previews the highlighted theme live once you stop (holding an arrow key does not load every theme on the way), `Enter` applies it and saves `theme` in `config.toml`, `Esc` restores the previous theme, and the footer shows two entries:
  - **Reload** (`F5`, on when the picker opens): while it is on, the highlighted theme's file in the themes directory is checked twice a second and the preview reloads by itself when the file changes, so you can edit a theme and watch it. Switching it on reloads the highlighted theme at once. Built-in themes with no file on disk are not watched. A file that fails to load is reported once per saved change.
  - **Preview** (`F6` next page, `Shift+F6` previous page, wrapping; the footer shows the current page, such as "Preview 1/3"): the preview pane has several pages and the picker opens on page 1. Page 1 shows sample inputs (with a tooltip under the active one), checkboxes and sample buttons (selected, unselected and a red destructive one, as in the delete confirmation). Pages 2 and 3 fill the whole right side of the dialog with the background the Jobs view uses, so they look as they do in the app. Page 2 shows app samples: a menu bar with an open pulldown, one job row per job status (with sample ETA and speed), and progress bars (active, inactive and completed). Page 3 shows a file panel with sample entries: a parent row, folders (one with a disk usage bar, one being scanned, one skipped), symlinks (one broken), files of several types, selected rows and the cursor, pinned, new, renamed and job marks, a no-permission file, and git statuses. The dialog keeps its size on every page, and on a short terminal the pane shows what fits.
  The page keys can be rebound in `[dialog.theme]` in `keybindings.toml`.
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
fg = 7               # semantic names: body text,
bg = 0               # dark well (input and list background),
surface = 8          # dialog / chip fill,
muted = 8            # dimmed text

[menu.bar]
active   = { fg = "default" }
selected = { fg = "black", bg = "cyan", bold = true }

[panel.row]
directory = { fg = "bright_blue", bold = true }
```

A style is a table with `fg`, `bg` and flags such as `bold`; colors are palette names or hex values. Main sections:

| Section | Styles |
|---|---|
| `[palette]` | The 16 named colors the rest of the file refers to, plus the semantic names `fg`, `bg`, `surface` and `muted` (and `range_bg` / `range_unselect_bg`, the tints of rows covered by a pending range select / unselect, and `cursor_dim`, the cursor row of an inactive panel) that styles use for body text, background wells, dialog fill and dimmed text, plus the roles `cursor_bg` / `cursor_fg` (the current item: the cursor row, the selected menu entry, the active button), `header_bg` (panel headers and the menu dropdown) and `accent` (shortcut letters and key hints); you can add any other name |
| `[status]` | One color per job state (scanning, queued, running, paused, decision, failed, canceling, canceled, completed), used by the menu bar, Jobs and Commands views and file-list marks; `scanning` also colors disk usage scans, fileindex scans and spinners |
| `[panel.*]` | Active, inactive and blocked (dimmed under dialogs) panel chrome; rows, marks, folder icons, git status, fileindex states (`[panel.fileindex]`, glyphs in `[icons.fileindex_state]`), [meta column alerts](meta-columns.md#alert-colors) (`[panel.meta]`), scrollbar, disk usage (`[panel.usage]`; an optional `icon` color replaces the color of icons where the bar is behind them) |
| `[menu.*]` | Menu bar, pulldowns, status commands (`menu.status`, `menu.status.separator`), job strip, progress bar, speed pill |
| `[dialog.*]` | Dialog frame, buttons, inputs, options, progress |
| `[preview]` | F3 fullscreen preview: `search_icon` (optional, foreground only), the search bar's leading icon |
| `[jobs.*]`, `[compare]`, `terminal.text` | Jobs view, compare view, embedded terminal |
| `[icons.*]` | Glyphs |

## Status glyphs

Job states each have an icon and a color: scanning, queued, running, paused, canceling, canceled, failed, decision and completed. Glyphs are under `[icons.jobs.list]` and colors under `[status]`; the menu-bar job strip uses `[icons.menu.job]`. The result icons of the jobs view Operations panel are `[icons.jobs.operations]` `success` / `failed` / `skipped`, colored by `[jobs.operations]` (the same colors mark the failed and skipped counts in the Details panel); without them the completed / failed / paused job colors and matching icons are used. Other icon groups: `[icons.menu.status]` (`separator` between status commands), `[icons.filelist]` (row suffixes such as new, renamed, selection, pinned), `[icons.dialog]` (missing, history), `[icons.tree]`, `[icons.scrollbar]`. Without Nerd Font icons (`use_nerdfont_icons = false`) ASCII markers are used instead.

Changing the theme also changes how the Messages toasts look (`message.info`, `message.warn`, `message.error`).

## Preview (syntax highlighting) themes

File previews use Chroma styles, chosen by `[preview] style` ([Config](config.md#preview)), default `catppuccin-frappe`. Pick one with Options -> Preview settings, or in the fullscreen preview (`F3`) through the `:` menu's theme picker (`t`). Besides Chroma's built-in styles, `pc` loads Chroma XML styles from `themes/preview/*.xml` in the config directory (the repository ships `themes/preview/jetbrains.xml` as an example). See [Quick view and preview](quick-view-preview.md).
