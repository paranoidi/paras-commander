# Themes

Colors, text styles and status glyphs come from a TOML theme file. One theme ships inside the binary (`default`); you can add your own and switch between them at runtime. Syntax highlighting in file previews is a separate system, covered at the end.

## Selecting a theme

- Options -> Theme opens the theme picker. Typing fuzzy-filters the list. Moving the cursor previews the highlighted theme live once you stop (holding an arrow key does not load every theme on the way), `Enter` applies it and saves `theme` in `config.toml`, `Esc` restores the previous theme, and the footer shows two entries:
  - **Reload** (`F5`, on when the picker opens): while it is on, the highlighted theme's file in the themes directory is checked twice a second and the preview reloads by itself when the file changes, so you can edit a theme and watch it. Switching it on reloads the highlighted theme at once. Built-in themes with no file on disk are not watched. A file that fails to load is reported once per saved change.
  - **Preview** (`F6` next page, `Shift+F6` previous page, wrapping; the footer shows the current page, such as "Preview 1/3"): the preview pane has several pages and the picker opens on page 1. Page 3 shows sample inputs (with a tooltip under the active one), checkboxes and two rows of sample buttons (selected, unselected and a red destructive one, as in the delete confirmation): the second row shows the shortcut letters as drawn when they need Alt. Pages 1 and 2 fill the whole right side of the dialog with the background the Jobs view uses, so they look as they do in the app. Page 2 shows app samples: a menu bar with an open pulldown, one job row per job status (with sample ETA and speed), and progress bars (active, inactive and completed). Page 1 shows a file panel with sample entries: a parent row, folders (one with a disk usage bar, one being scanned, one skipped), symlinks (one broken), files of several types, selected rows and the cursor, pinned, new, renamed and job marks, a no-permission file, and git statuses. The dialog keeps its size on every page, and on a short terminal the pane shows what fits.
  The page keys can be rebound in `[dialog.theme]` in `keybindings.toml`.
- Or set it directly:

```toml
theme = "default"
```

An unknown name falls back to the built-in default and reports an error.

## Where themes live

Put `*.toml` files in `themes/` inside the config directory (`~/.config/pc/themes/`, or `$XDG_CONFIG_HOME/pc/themes/`). A theme is identified by the `name` declared at the top of the file, not by its file name; a disk theme whose `name` matches a built-in one (including `default`) overrides it. Files that fail to parse are skipped when resolving the configured theme, but a broken file in the folder prevents the picker from listing themes, so fix or remove it.

## Structure

A theme file only needs what it changes: every palette color, role, style and icon it leaves out is inherited from the built-in default theme. A theme with just a `name` and a `[palette]` recolors the whole interface; add `[roles]` entries to retarget individual intents, and style or icon entries only where a style should look different. The built-in `themes/default.toml` in the repository lists every key, documented by position and comment, and is the reference for what can be set.

Inheritance works key by key:

- `[palette]`, `[roles]` and `[icons.*]` entries replace the same entry of the default theme.
- A style entry replaces the default's style whole: `{ fg = "fg" }` does not keep the default's bold or background.
- The progress light-bar gradients (`menu.progress.done.gfx.0` to `gfx.9`, and the `delete` and `remove` ones) are replaced as a set: if you define any step of a gradient, the default's steps for that gradient are dropped, so a shorter gradient stays short.
- Unknown keys, and colors that are not roles in a style, are still errors in your file.

```toml
name = "mine"

[palette]            # raw colors: exactly the 16 ANSI names; 0-255 ANSI/xterm index, "#RRGGBB", or "default"
black = 0
red = 1
# ... through bright_white = 15

[roles]              # intents: a [palette] name, "#RRGGBB" or "default"
bg = "#303446"       # background of the whole UI; "default" keeps the terminal's own
surface = "bright_black"   # dialog / chip fill
muted = "bright_black"     # dimmed text
# ... 37 roles in all, listed in themes/default.toml

[menu.bar]
active   = { fg = "fg" }
selected = { fg = "focus_fg", bg = "focus_bg", bold = true }

[panel.row]
directory = { fg = "directory", bold = true }
```

Colors come in two layers. `[palette]` holds only raw colors: the 16 ANSI slot names and nothing else (an unknown name is an error; a slot you leave out keeps its ANSI index). `[roles]` holds intents (body text, dialog fill, dimmed text, the current item, a success state, ...), each pointing at a palette name, a hex value or `"default"`. A style's `fg`, `bg` and `icon` may name **only a role**: a hue such as `"yellow"`, a hex value or `"default"` in a style is an error that lists the roles. To recolor the UI you edit `[palette]` and `[roles]` and leave the styles alone. A role you leave out takes its built-in default (the palette name shown below); `themes/default.toml` lists all of them.

| Role | Default | Used for |
|---|---|---|
| `bg` | `default` | background of every cell that has none of its own (so the whole file list and every plain area), input and list backgrounds, progress tracks; `default` keeps the terminal's own background |
| `surface` | `bright_black` | dialog fill, chips, bar tracks, speed pill |
| `header_bg` | `blue` | panel headers |
| `inactive_header_bg` | `bright_black` | header of the inactive panel |
| `menu_bg` | `blue` | menu dropdown fill and frame |
| `focus_bg` / `focus_fg` | `cyan` / `black` | the focused item: open menu-bar title, selected pulldown entry, selected completion entry, hovered footer entry; `focus_fg` is also the focused dialog button's text |
| `button_bg` | `black` | face of an unfocused dialog button |
| `button_focus_bg` | `cyan` | face of the focused dialog button |
| `file_cursor_bg` / `file_cursor_fg` | `cyan` / `black` | cursor row of an active file list (panels, duplicate finder) |
| `file_cursor_dim` | `black` | cursor row of an inactive or blocked panel |
| `highlight_bg` | `bright_blue` | clickable part of the hovered footer entry, disk usage cursor on a selected row, mount mark |
| `range_bg` / `range_unselect_bg` | `bright_black` / `red` | tints of rows covered by a pending range select / unselect |
| `inverse_bg` | `white` | light boxes: tooltips, info toast |
| `fg` / `fg_strong` | `white` / `bright_white` | body text / emphasised text |
| `muted` / `faint` | `bright_black` / `black` | dimmed text / barely visible text (disabled options) |
| `info_columns` | `bright_black` | file-list info columns: size, date, permissions, meta |
| `ink` | `black` | dark text on a colored fill |
| `accent` | `magenta` | shortcut letters, key hints, match highlights |
| `accent_alt` | `magenta` | dialog shortcut letters that need Alt |
| `primary` / `directory` / `symlink` / `selected` | `blue` / `blue` / `bright_cyan` / `yellow` | active panel frame, directories, symlinks, selected files |
| `inactive_frame` | `bright_black` | inactive panel frame |
| `info` / `success` / `success_strong` | `cyan` / `green` / `bright_green` | informational, completed, running |
| `warning` / `attention` / `danger` / `error` / `special` | `yellow` / `bright_yellow` / `red` / `bright_red` / `magenta` | paused, canceling, deleted, failed, queued |

Main sections:

| Section | Styles |
|---|---|
| `[palette]`, `[roles]` | The two color layers described above |
| `[status]` | One color per job state (scanning, queued, running, paused, decision, failed, canceling, canceled, completed), used by the menu bar, Jobs and Commands views and file-list marks; `scanning` also colors disk usage scans, fileindex scans and spinners |
| `[panel.*]` | Active, inactive and blocked (dimmed under dialogs) panel chrome; rows, marks, folder icons, git status, fileindex states (`[panel.fileindex]`, glyphs in `[icons.fileindex_state]`), [meta column alerts](meta-columns.md#alert-colors) (`[panel.meta]`), scrollbar, disk usage (`[panel.usage]`; an optional `icon` table recolors chosen icon groups where the bar is behind them, see [Icons on the disk usage bar](#icons-on-the-disk-usage-bar)) |
| `[menu.*]` | Menu bar, pulldowns, status commands (`menu.status`, `menu.status.separator`), job strip, progress bar, speed pill |
| `[dialog.*]` | Dialog frame, buttons, inputs, options, progress |
| `[preview]` | F3 fullscreen preview: `search_icon` (foreground only), the search bar's leading icon |
| `[jobs.*]`, `[compare]`, `terminal.text` | Jobs view, compare view, embedded terminal |
| `[icons.*]` | Glyphs |

## Icons on the disk usage bar

The disk usage bar styles (`[panel.usage]` `normal` and `selected`, and `usage.cursor` / `usage.cursor.selected` under `[panel.active]` and `[panel.inactive]`) take an optional `icon` table that recolors icon groups on the cells where the bar is behind them. Write it as dotted keys:

```toml
[panel.usage]
normal = { fg = "fg", bg = "surface", icon.file = "faint", icon.git = "faint" }
```

Each value is a role. The groups are:

| Group | Icons |
|---|---|
| `folder`, `folder_expanded`, `folder_scanning` | The plain, expanded (tree) and scanning folder icons |
| `folder_open`, `folder_mount`, `folder_excluded` | Folders open in the other panel, mount points and excluded folders |
| `file` | Every file icon in the icon strip (file types, broken links, image and video preview tints) |
| `preview_loading` | The icon shown while a file preview is loading |
| `fileindex` | The fileindex state icon |
| `git` | Git status cells |
| `suffix` | Marks after the file name (new, renamed, pinned, job and so on) |

A group you do not list keeps the color it has off the bar: the cursor row's icon color on the cursor row, otherwise the icon's own color. Past the end of the bar, icons always keep their own colors. An unknown group name is an error, and so is a plain color string on a bar style (a cursor row style, such as `row.cursor`, takes a plain `icon = "<role>"` string instead). Like any style, a bar style you redefine replaces the default's whole, groups included: list every group you want.

The default theme recolors `file`, `folder_open`, `folder_mount`, `folder_excluded`, `fileindex`, `git` and `suffix` on `normal` and `selected` and leaves the plain, expanded and scanning folder icons and the loading icon alone so they stay readable on the bar.

## Status glyphs

Job states each have an icon and a color: scanning, queued, running, paused, canceling, canceled, failed, decision and completed. Glyphs are under `[icons.jobs.list]` and colors under `[status]`; the menu-bar job strip uses `[icons.menu.job]`. The result icons of the jobs view Operations panel are `[icons.jobs.operations]` `success` / `failed` / `skipped`, colored by `[jobs.operations]` (the same colors mark the failed and skipped counts in the Details panel); without them the completed / failed / paused job colors and matching icons are used. Dialog buttons are drawn as `[icons.dialog.button]` `start`, the label and `end` (default: powerline half-circles that make each button a pill; without Nerd Font icons `"[ "` and `" ]"` in the button's own colors; spaces are kept and count toward the button width, so `start = " "` / `end = " "` drops the brackets). `[dialog.button]` `active.start`, `inactive.end` and so on color those caps per button state (`active`, `active_destructive`, `inactive`). A cap color's `fg` and `bg` each replace the button's own when set, and apply to the cap's glyphs only: spaces in a cap keep the button's colors. Examples: a light left edge and a dark right edge for a raised look, or powerline half-circles colored as the button on the dialog background for a pill. `active.accent` and `active_destructive.accent` color the shortcut letter on the focused button (foreground and bold), and `active.accent_alt` / `active_destructive.accent_alt` the same letter when it needs Alt, so a light button face can keep readable letters; unfocused buttons use the dialog's `accent` / `accent_alt`. Other icon groups: `[icons.menu.status]` (`separator` between status commands), `[icons.filelist]` (row suffixes such as new, renamed, selection, pinned), `[icons.dialog]` (missing, history), `[icons.tree]`, `[icons.scrollbar]`. Without Nerd Font icons (`use_nerdfont_icons = false`) ASCII markers are used instead.

Changing the theme also changes how the Messages toasts look (`message.info`, `message.warn`, `message.error`).

## Preview (syntax highlighting) themes

File previews use Chroma styles, chosen by `[preview] style` ([Config](config.md#preview)), default `catppuccin-frappe`. Pick one with Options -> Preview settings, or in the fullscreen preview (`F3`) through the `:` menu's theme picker (`t`). Besides Chroma's built-in styles, `pc` loads Chroma XML styles from `themes/preview/*.xml` in the config directory (the repository ships `themes/preview/jetbrains.xml` as an example). See [Quick view and preview](quick-view-preview.md).
