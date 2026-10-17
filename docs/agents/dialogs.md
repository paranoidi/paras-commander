# Dialog standards

Navigation, button, layout and styling rules every modal dialog follows. For the wiring checklist when adding a dialog, see `recipes/new-dialog.md`.

## Navigation

- **Up/Down**: Navigate between content elements (radio items, checkboxes, input fields).
  - Down from the last content element moves to the first button (OK).
  - Down from last button does nothing (no wrap-around).
  - Up from first content element does nothing (no wrap-around).
  - Up from a button goes back to the last content element.
- **Left/Right**: Navigate between buttons only (OK, Cancel, etc.).
  - Left from first button does nothing.
  - Right from last button does nothing.
  - Does not wrap around.
- **Tab/Shift+Tab**: Jump between *groups* of controls, not between individual items — a radio
  block, a checkbox block, and the button strip are each one group, and Tab lands on the first
  item of the next group (wrapping around). Declared via `DialogTrailingButtonsForm.WithSegments`
  (`internal/ui/dialog/dialog_focus.go`).
  - **Every text input is its own group.** In a dialog with more than one text input, Tab steps
    from one input to the next before it leaves the block (mass rename Find → Replace, chown
    User → Group, symlink Target → Link path). Use Up/Down to step item-by-item inside a group.
- **Enter**: Activates the focused button or confirms the dialog.
- **Esc**: Cancels/closes the dialog.

## Button Shortcuts

- Buttons have a highlighted shortcut letter in accent color (magenta by default).
- Shortcuts: `O` for OK, `C` for Cancel, `Y` for Yes, `N` for No.
- Shortcuts are triggered via **Alt+letter** (e.g., Alt+O executes OK, Alt+C cancels).
- An Alt-letter that toggles a checkbox or picks a radio **never moves focus out of a focused
  text input** — the user keeps typing after flipping the option. From a non-input row it may
  move focus to the toggled row (group select, filter, mkdir actions follow this).

## Rendering / Styling

- All dialogs use `styles.DialogSurface` as the fill color, not `styles.DialogText`.
- The border uses `styles.DialogFrame` on the dialog background.
- The title row is built from `styles.DialogTitle` **attributes** (`bold`, etc.) with **glyph foreground** taken from `dialog.frame` and **glyph background** from `dialog.surface` (see `DrawDialogFrame` in `internal/ui/dialog/internal/draw/chrome.go`).
- All content rows use the dialog background as their background color.
- **Do not** add navigation help footers (e.g. `Left/Right  Enter confirm  Esc cancel`) at the bottom of dialogs. Standard dialog navigation and button Alt-shortcuts are enough; the user does not want keyboard cheat-sheet rows in the UI.
- Buttons use the `draw.DrawDialogButton` helper (`internal/ui/dialog/internal/draw/button.go`), which which renders ` [ label ] ` (outer spaces plus single spaces inside brackets) using `DialogButtonActive` / `DialogButtonInactive` (and `DialogButtonActiveDestructive` when `DialogButtonSpec.Destructive` is set) **as defined in the theme** (foreground and background; do not substitute `dialog.surface` for button fill).
  - Shortcut letter in `dialog.accent` foreground color (`Theme.DialogAccent`), bold
- Checkbox and radio rows use `Theme.DialogOptionRowStyle` (resolved from `dialog.option.inactive`, `dialog.option.active`, `dialog.option.selected`, and `dialog.option.active.selected` when focused and checked/marked; row background always matches `dialog.surface` — do not set `bg` on option keys).
- Do not use `styles.DialogText` for input row fill; input rows use `theme.Theme.DialogInputPair(focused)` (resolved from `dialog.input.active` / `dialog.input.inactive` and their `.placeholder` entries). Those styles carry both foreground and background for the input row—do not substitute `DialogText` for fill.

### Text overlaid directly on a border (not inside a dialog)

Any label painted on top of a panel/dialog border line (top-row end labels like the Queue
rate-limit indicator or compare/dedup filter indicators, bottom-row selection-size indicators,
etc.) must carry one space of padding on each side so it doesn't run flush into the border
dashes — `──100MB/s─` is a bug, `── 100MB/s ─` is correct. Use `panelSelectionSizePadded` /
`SelectionSizePadded` (`internal/ui/selection_size.go`) to pad the raw text; do not pass an
unpadded string to `drawAuxPanelChrome`'s `endLabel` or similar. This has been missed more than
once (e.g. the jobs rate-limit indicator) — check it whenever adding a new border-overlaid label.

## Horizontal columns (do not confuse `rect.X+1` and `rect.X+2`)

Use `draw.DialogTextX(rect)`, `draw.DialogOptionX(rect)`, and `draw.DialogContentWidth(rect)` in `internal/ui/dialog/internal/draw/geom.go` instead of re-deriving offsets.

- `rect.X` — left border (`│`); `rect.X+1` — inner margin (blank dialog surface).
- **Labels, hints, and input rows** — `DialogTextX(rect)` (`rect.X+2`): first content glyph one space inside the border (e.g. `Destination:`, `/path`, command field).
- **Checkbox / radio rows** — pass `DialogOptionX(rect)` (`rect.X+1`) to `draw.DrawDialogCheckbox` / `draw.DrawDialogRadio` only. Markers are ` [ ] ` / ` ( ) ` with a **leading space** that fills the margin cell; the visible `[` or `(` must land on `DialogTextX`, not one column to the right.
- **Common bug:** `DrawDialogRadio(screen, rect.X+2, …)` paints an extra blank column before `( )`, so options look indented relative to labels (seen on Run-for-each pool rows). Copy/move checkboxes in `transfer_dialog_render.go` and Find checkboxes use `rect.X+1` — match that, not `rect.X+2`.
- **Inner width** — `DialogContentWidth(rect)` (`rect.Width-4`) for full-width text and inputs between margins.

## Code Structure

- Button rendering: `draw.DrawDialogButton` (`internal/ui/dialog/internal/draw/button.go`); centered rows via `DrawDialogButtonRowCentered`.
- Focus management lives with the dialog's handler: `internal/apphandler/dialog` for most dialogs, `internal/app` for the ones that stay app-side (see `architecture.md`, Package layout). `internal/app/input.go` owns top-level key dispatch.
- File dialogs (`FileDialogState`) use `FocusedField` for focus tracking; button focus indexes = len(Fields)+0 for OK and len(Fields)+1 for Cancel.

## Input Area Rendering / Styling

All dialogs with labeled content blocks (text inputs, radio groups, checkbox groups under a section label) must follow these rules:

### Layout

- The label and the content below it must be on separate rows.
- **The content row sits directly beneath its label row — no blank row between them** (text input, first radio/checkbox option, preview value, etc.). This is the compact layout, and it is the convention everywhere; a dialog that spaces a label away from the thing it labels is a bug, not a style choice.
- Do not place the label and its content on the same line, even with visual separators.
- Section labels followed by radio or checkbox options use the same pattern as text fields: **label row → option rows** (e.g. Run-for-each **Worker pool (optional):** then pool radios). Standalone option rows without a section label (e.g. mkdir post-action radios directly under a separator) do not need an extra label row.
- Blank rows still do section work: one blank row above the first button row (see below), and a blank row between a section's last content row and the next section's label where no separator already does that job. What went away is only the gap between a label and its own content.
- Guard: `TestDialogLabelsAreFollowedByContentRow` (`internal/ui/dialog/dialog_label_spacing_test.go`) renders dialogs and asserts the row under each label is the content. Add a case for every new labelled row.

- Elements should be spaced and aligned following these examples:

NO:

┌─ Copy ──────────────────────────┐
│Destination:/home/user/projects  │
│[x] Preserve permissions         │
│[x] Preserve timestamps          │
│  <area for buttons>             │
└─────────────────────────────────┘

YES:

┌────────────── Copy ──────────────┐
│ Destination:                     │
│ /home/user/projects              │
├──────────────────────────────────┤
│ [x] Preserve permissions         │
│ [x] Preserve timestamps          │
├──────────────────────────────────┤
│       <area for buttons>         │
└──────────────────────────────────┘

NO:

┌──────────────────────────────────────────────────────────┐
│  [x] Focus after rename                          [x] Fail│
└──────────────────────────────────────────────────────────┘

YES:

┌──────────────────────────────────────────────────────────┐
│ [x] Focus after rename                          [x] Fail │
└──────────────────────────────────────────────────────────┘

There should be ONE space after (and before) border.


Pay attention to following details:
- Title Copy is centered. Surrounded with empty spaces.
- ONE space margin at the left (`rect.X+1`); first label/input glyph at `DialogTextX` (`rect.X+2`). Checkbox/radio helpers at `DialogOptionX` (`rect.X+1`) — see **Horizontal columns** above. Dialog is scaled so that there is ONE space margin at the right side.
- Input text area is in it's own line. It should cover the all avaialable space in the dialog (respecting 1 space margin).
- Buttons should be visually centered.
- Separation lines between sections.
- **Exactly one empty row above the first button row** (not zero, not two or more). That blank row is dialog surface only—do not paint text on it. Content (or a section separator + prompt) ends on the row immediately above that blank row.

#### Button row placement (read this before sizing `height`)

- **Flow `y` downward** while drawing content, then `y++` once for the mandatory blank row above buttons, then paint button row(s). Single-button dialogs: `buttonY = rect.Y + rect.Height - 2`, blank row at `buttonY - 1`, separator/content ending on `buttonY - 2` when a separator precedes the button block (see `DrawQuitConfirmDialog` — add the blank row between separator and buttons when adapting that pattern).
- **Size `height` to fit** — do not pick an oversized fixed `height` and anchor buttons with `rect.Y + rect.Height - N`; that leaves extra blank rows beyond the single required one.
- **Two button rows:** second row at `rect.Y + rect.Height - 2`, first at `rect.Y + rect.Height - 3`, blank row at `rect.Y + rect.Height - 4`, content ending above that (separator/prompt on preceding rows).
- **Precompute row count** before `CenteredDialogRect`, or advance `y` while drawing (include `y++` for the button blank row) and set `height = y - rect.Y + 2` (last painted row + one inner bottom margin + bottom border).
- **Common bugs:** (1) `height := 17` with buttons at `height - 4` / `height - 3` plus a help footer — three or more blank rows above buttons. (2) Drawing buttons on the row immediately after the prompt/separator — zero blank rows above buttons.

### Styling

- For any dialog text input row, use `theme.Theme.DialogInputPair(focused)` (`internal/theme/theme.go`). It returns:
  - **base**: row fill + normal (committed) text
  - **placeholder**: dimmed suggested/default glyphs
- Do not override the input row background with `DialogSurface` or chrome-derived `.Background(...)`; the theme’s input row background is intentional.
- Labels stay on the dialog surface (`DialogText` on `DialogSurface` background).
- When adding or correcting alignment avoid using empty spaces to do it. Like string " value" or " value ". Instead adjust rendering position or formatting. Spaces are good only when the background matters.

### Suggested default / prefill (`FileDialogField`)

Logic lives in `internal/ui/dialog/dialog_field.go`. When opening a dialog with a suggested value, set `Value` and `Prefill` to the suggestion and set `PrefillPending` when the suggestion is non-empty.

- **First printable (`InsertRune`)**: clear `Value`, reset cursor, clear `PrefillPending`, then insert the rune (replace-from-scratch on top of the suggestion).
- **Backspace, Delete, cursor moves (`MoveCursor` / Home / End via handlers)** call `commitPrefill()` first: drop `PrefillPending` but **keep** `Value`, then apply the usual edit/move.
- **`Clear`** (file dialog `Ctrl+L`): empty `Value`, cursor `0`, `PrefillPending` false.
- **`RestorePrefill`** (defaults `Ctrl+R` and `Ctrl+D`, configurable under `[dialog.input]` as `ui.input.restore-default`): re-arm the suggested default after editing or clearing — `Value` becomes `Prefill`, cursor moves to the end, `PrefillPending` is set to true. No-op when `Prefill` is empty.
- **OK while still pending**: use current `Value` (still the full suggestion)—no extra commit step required.

### Helpers

- Use `drawInputField` in `internal/ui/dialog/file_dialog_render.go` for `FileDialogField`-based inputs (supports placeholder-pending rendering).
- Use `draw.DrawSimpleDialogInput` in `internal/ui/dialog/internal/draw/chrome.go` for simple string inputs (no placeholder-pending concept).
