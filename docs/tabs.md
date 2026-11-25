# Panel tabs

Each file panel has up to nine tabs, so one panel can hold several directories at once. With a single tab nothing extra is shown; the panel simply occupies slot 1.

| Action | Default key |
|---|---|
| Switch to tab 1 to 9, or open that slot as a copy of the current tab | `C-1` to `C-9` |
| Close the active tab | `C-w` |

Both act on the active panel. Close tab is also in the Command menu. All keys are rebindable in `keybindings.toml` ([Keybindings](keybindings.md)); run `pc --config-stub` for the action names.

- **Switching.** A key whose slot is in use shows that tab. A key whose slot is empty opens a new tab there, a copy of the current one, and shows it. The key of the current slot does nothing.
- **Closing.** The nearest lower used slot is shown, else the next higher one. Slots are not renumbered, so tabs 1 and 5 can exist with nothing between. Closing the only tab shows "No other tab". While you type a [quick filter](panel-filter.md) `C-w` is ignored; a tab key cancels the filter first, then switches.
- **Mouse.** A left click on a tab focuses that panel and switches to it ([Mouse](mouse.md)).
- **In dialogs.** In a path field of the copy, move, flatten, extract, rsync, symlink, hard link and Navigate to dialogs a tab key inserts the directory of that tab of the inactive panel, the one the operation targets. While such a dialog is open that panel's tab numbers keep their color instead of being dimmed. `*` lists the tabs of both panels. If the panel has no such tab the text stays as it is and a warning says so.
- **Both panels.** The panels have separate tabs. Opening, switching and closing tabs affects the active panel only.
- **Session only.** Tabs are not saved; `pc` starts with one tab per panel.
- **Views.** Tabs exist in the file browser. In the jobs, messages, compare and other views the tab keys do nothing.

## What a tab keeps

A new tab copies the current one, then the two diverge. Each tab has its own:

- directory, cursor and scroll position, and navigation history
- sort order, entry filter ([Panel filter](panel-filter.md)), tree view and carousel view
- browse with fileindex, file columns and meta columns ([Columns](columns.md))
- selection and pending range select (a new tab starts with neither, and without the quick filter)

These are shared by all tabs of a panel (or by the whole program): hidden-file visibility and scroll settings, the selection stash (so a stash can carry a selection from one tab to another, see [Selections](selections.md)), pins, zoom, split orientation, latched sync and quick view.

Switching to a tab re-reads its directory, so a tab always shows the current contents. If a directory was still loading when you left the tab, it loads again when you return.

## Tab bar

When a panel has more than one tab, a row of tabs appears above its frame:

```
◢ 1 projects ◣ ◢ 2 docs ◣ ◢ 5 / ◣
```

The slanted ends are Nerd Font glyphs (drawn here with similar plain triangles); without Nerd Font icons the tabs are square, ` 1 projects   2 docs   5 / `.

- **Side by side:** one row between the menu bar and the panels, shown whenever a visible panel has more than one tab. The tabs of each panel sit over its own column, and both panels move down one row so their frames stay aligned.
- **Stacked:** the row sits directly above each panel that has more than one tab.
- A tab is a start cap, its slot number and, with `tab_labels` on, its directory name (`/` at the root), and an end cap. The tab of a [tag view](fileindex.md) shows the database icon (with Nerd Font icons) and the tags it filters on, `beach | holiday` in the flat list and `beach & holiday /summer` in the directory structure, where the last directory follows the tags after a `/`. The active tab is highlighted. A click anywhere on a tab, caps included, selects it.
- When the labels do not fit, the longest are cut from the right with `…` until they share one width. Below four cells a label is dropped and only the numbers remain. If the numbers with their caps do not fit, the tabs become square (` 1 `); if even those do not fit, the tabs after the last one that fits are left out. The active tab always stays.

`tab_labels` in [`[ui]`](config.md#ui) turns the labels off. Colors are in the `[panel.tab]` theme section ([Themes](themes.md)).

### Tab shape

The caps come from the theme's `[icons.panel.tab]` `start` and `end`. Their spaces are part of the tab, so they are also its padding. The default theme draws slanted tabs; a theme can change that:

```toml
# Slanted (default): lower-right and lower-left triangles
[icons.panel.tab]
start = "\ue0ba "
end = " \ue0b8"

# Pill: half circles
# start = "\ue0b6 "
# end = " \ue0b4"

# Square
# start = " "
# end = " "
```

A cap glyph is drawn in the tab's own background color on the gap between tabs, so the tab's colors need no extra keys; the spaces in a cap keep the tab's colors. A tab color without a background of its own gets a blank instead of the glyph. With `use_nerdfont_icons = false` in [`[ui]`](config.md#ui) the caps are plain spaces.

## Terminal support

`C-1` to `C-9` are not control characters, so a terminal only reports them when it speaks an extended keyboard protocol. They work in xterm, WezTerm and other terminals with modifyOtherKeys or CSI-u support, and in tmux with extended keys turned on:

```
set -g extended-keys on
```

Terminals without extended keys send other keys for these chords (for example `C-2` arrives as `C-Space` and `C-3` as `Esc`), so the tab keys cannot work there; bind other chords in `keybindings.toml`. `C-w` is a plain control character and works everywhere, but some terminal setups map Ctrl+Backspace to it, which would close a tab in the file list.
