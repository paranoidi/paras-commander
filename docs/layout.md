# Layout

How the screen is divided: menu bar, two panels, footer, and the options that change their size, order and scrolling.

## Panels

| Action | Default key |
|---|---|
| Toggle split orientation (side by side / stacked) | `C-Space` |
| Swap the panels | `C-M-Space` |
| Toggle zoom of the active panel | `M-z` |
| Hide the inactive panel | `S-Tab` |
| Toggle carousel view | `M-v` |

All are rebindable in `keybindings.toml` ([Keybindings](keybindings.md)). Split orientation and swap are also under the Command menu ("Toggle split orientation", "Swap panels").

- **Orientation**: side by side (left | right) or stacked (top / bottom). The saved default is `[ui.zoom] orientation`; the key overrides it for the session. Stacked mode needs enough terminal rows. When the terminal is too small to draw, `pc` shows "Terminal too small".
- **Swap** exchanges which panel is drawn first (left or top) for the session; the panels keep their contents and the Left/Right menus follow the screen position.
- **Zoom** gives the active panel more room (default 70% active, 30% inactive). It is suppressed in favour of an even split when every file name in the active panel already fits its name column at an even split, or when that column is wide (`disabled_above_name_width` 60 cells; file, git and meta columns all narrow it, so a wider terminal still zooms when they take the row) or, when stacked, on tall terminals (`disabled_above_height` 45). See [`[ui.zoom]`](config.md#uizoom).
- **Hide inactive panel** shows only the active panel at full width. In carousel view the inactive panel can hide automatically (`[carousel] autohide_inactive_panel`, default on).
- **Narrow panels** keep file names readable. As a panel loses width it first drops meta columns, rightmost first, then its [file columns](columns.md), rightmost first, then shows names only, and an inactive panel too narrow even for that is hidden until there is room again. While it is hidden, the hide inactive panel key only shows a message. Side-by-side only; sync follow, quick view and file preview keep both columns on screen.

## Carousel view

The carousel shows the parent directory, the current directory and a child column (a listing of the highlighted directory or a preview of the highlighted file) in one panel. Column widths, size columns and autohide are set in [`[carousel]`](config.md#carousel). Start in it with `--carousel`. Preview behaviour is in [Quick view](quick-view-preview.md).

## Scrolling

[`[ui.scroll]`](config.md#uiscroll) controls file lists, and also the Jobs, Messages, Compare and Duplicates lists:

- `mode`: `"edge"` (default; scroll when the cursor is within `edge_margin` rows of the border), `"center"` (keep the cursor centred) or `"minimal"`.
- `scrollbar`: `"thumb"` (default), `"bar"` or `"none"`; `scrollbar_inactive` also draws it on the inactive panel. The bar is hidden when everything fits.

## Menu bar

The top row is the menu bar ([Menus](menus.md)). Hide it with `show_menu_bar = false` in `[ui]`; panels then start at the top row. Between the menu labels and the right edge it shows job status groups and progress ([Jobs](jobs.md)).

### Status commands

The left end of the menu bar can show the output of small shell commands such as the load average or memory use, separated by a theme glyph. Choose them under Display → Status commands; define more in `status.toml`. See [Status commands](status-commands.md).

## Footer

The bottom row lists the function keys for the current view (`F1 Help`, `F3 View`, ...). Shift variants are shown alongside and are dropped first when the terminal is narrow. In vi-motion mode the footer shows the leader letters instead ([Menus](menus.md#vi-motion-mode)).
