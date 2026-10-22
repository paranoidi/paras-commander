# Layout

How the screen is divided: menu bar, two panels, footer, and the options that change their size, order and scrolling.

## Panels

| Action | Default key | Id |
|---|---|---|
| Toggle split orientation (side by side / stacked) | `Ctrl+Space` | `panel.toggle-split-orientation` |
| Swap the panels | `Ctrl+Alt+Space` | `panel.swap-panes` |
| Toggle zoom of the active panel | `Alt+Z` | `panel.toggle-zoom-active-panel` |
| Hide the inactive panel | `Shift+Tab` | `panel.toggle-hide-inactive` |
| Toggle carousel view | `Alt+V` | `panel.toggle-carousel` |

All are rebindable in `keybindings.toml` ([Keybindings](keybindings.md)). Split orientation and swap are also under the Command menu ("Toggle split orientation", "Swap panels").

- **Orientation**: side by side (left | right) or stacked (top / bottom). The saved default is `[ui.zoom] orientation`; the key overrides it for the session. Stacked mode needs enough terminal rows. When the terminal is too small to draw, `pc` shows "Terminal too small".
- **Swap** exchanges which panel is drawn first (left or top) for the session; the panels keep their contents and the Left/Right menus follow the screen position.
- **Zoom** gives the active panel more room (default 70% active, 30% inactive). It is suppressed in favour of an even split on large terminals (`disabled_above_width` 140, `disabled_above_height` 45). See [`[ui.zoom]`](config.md#uizoom).
- **Hide inactive panel** shows only the active panel at full width. In carousel view the inactive panel can hide automatically (`[carousel] autohide_inactive_panel`, default on).

## Carousel view

The carousel shows the parent directory, the current directory and a child column (a listing of the highlighted directory or a preview of the highlighted file) in one panel. Column widths, size columns and autohide are set in [`[carousel]`](config.md#carousel). Start in it with `--carousel`. Preview behaviour is in [Quick view](quick-view-preview.md).

## Scrolling

[`[ui.scroll]`](config.md#uiscroll) controls file lists, and also the Jobs, Messages, Compare and Duplicates lists:

- `mode`: `"edge"` (default; scroll when the cursor is within `edge_margin` rows of the border), `"center"` (keep the cursor centred) or `"minimal"`.
- `scrollbar`: `"thumb"` (default), `"bar"` or `"none"`; `scrollbar_inactive` also draws it on the inactive panel. The bar is hidden when everything fits.

## Menu bar

The top row is the menu bar ([Menus](menus.md)). Hide it with `show_menu_bar = false` in `[ui]`; panels then start at the top row. Between the menu labels and the right edge it shows job status groups and progress ([Jobs](jobs.md)).

### Status command

`[status_command]` runs a shell command periodically and shows the first line of its output at the left of the menu bar, for example a clock or a load average:

```toml
[status_command]
command = "date +%H:%M"
interval_ms = 3000
max_width = 15
```

Empty output, a failing command or a run over 10 s clears the text; an empty `command` disables it. The command runs via `sh -c` with no `%f`-style macro expansion. Style key: `menu.status` ([Themes](themes.md)). See [`[status_command]`](config.md#status_command).

## Footer

The bottom row lists the function keys for the current view (`F1 Help`, `F3 View`, ...). Shift variants are shown alongside and are dropped first when the terminal is narrow. In vi-motion mode the footer shows the leader letters instead ([Menus](menus.md#vi-motion-mode)).
