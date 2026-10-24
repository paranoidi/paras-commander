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
- **Esc**: Cancels/closes the dialog (see Button Shortcuts: OK/Cancel are resolved centrally).

## Button Shortcuts

- Buttons have a highlighted shortcut letter in accent color (magenta by default).
- Shortcuts: `O` for OK, `C` for Cancel, `Y` for Yes, `N` for No.
- Shortcuts are triggered via **Alt+letter**.
- **OK and Cancel are resolved once, centrally.** `ui.input.dialog-ok` and `ui.input.dialog-cancel` are `[dialog.input]` actions dispatched in `App.handleKey` through `App.activeDialogButtons` (`internal/app/dialog_buttons.go`), which returns the top-most open dialog's `dialogButtons{OK, Cancel, Esc}`. A new dialog adds one case there; its key handler must **not** handle the OK / Cancel chords itself. A nil `OK` or `Cancel` means the dialog has no such button (the chord is swallowed); `Esc` is set only where Esc deliberately differs from Cancel (running command-output dialog keeps the command running).
  - Layered dialogs resolve to their innermost layer: handlers with sub-phases, sub-confirms or a path-completion dropdown expose `OK`/`Cancel` funcs that pick the layer (`FileDialogOK`/`FileDialogCancel`, `TransferDialogCancel`, `jobs.Handler.DialogOK`/`DialogCancel`, the Configuration dialog's Yes/No sub-confirms).
  - Dialogs whose OK / Cancel letters are letter choices (the job blocker's Overwrite / Cancel) return zero for rune events; the non-letter `ui.input.dialog-ok` / `ui.input.dialog-cancel` chords still map. Configuration and meta dialogs return zero for the `F9` chord (edit config).
  - The undo confirmation (`UndoConfirmState`, `UndoConfirmView`, `App.handleUndoConfirmKey`; buttons `UndoYes` / `UndoNo`, No focused first) is a `PrimaryModalUndo` modal opened by `jobs.Handler.undoSelectedJob`; its `dialogButtons` case calls `Handler.ConfirmUndo` for Yes and answers No for Cancel.
  - Dialog-specific extras (Alt+Y/Alt+N on delete, Alt+P on transfer, Alt+I on history) go through `dialog.TryDialogMnemonics(ev, extras)`.
- An Alt-letter that toggles a checkbox or picks a radio **never moves focus out of a focused
  text input** — the user keeps typing after flipping the option. From a non-input row it may
  move focus to the toggled row (group select, filter, mkdir actions follow this).
- Mnemonic letters always require Alt. A plain letter never toggles a checkbox or picks a radio,
  even when that row is focused; the focused option toggles with Space or Enter.

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
unpadded string to `drawAuxPanelChrome`'s `endLabel` or similar. Exception:
`drawAuxPanelBottomCenterLabel` (`internal/ui/panel_chrome_draw.go`, bottom-border centered
legends such as the compare and dedup cursor-row icon legends) pads internally, so pass it raw
text, never pre-padded. This has been missed repeatedly (the jobs rate-limit indicator, the
dedup icon legend) — when adding any border-overlaid label, check the rendered row shows
`─ text ─`, and assert the surrounding spaces in its render test.

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

## Declarative dialogs (`dlg` engine)

`internal/ui/dialog/dlg` (ADR 0020) draws and drives dialogs from a plain-data `dlg.Spec`: groups of radio / checkbox / label items (one or more columns each) followed by a button strip. Each dialog is a pure `XView(state) dlg.Spec`, its state's `Focus` is a `dlg.ID`, its `DrawX` is one `dlg.Draw` call, and its key handler calls `dlg.HandleKey` and maps the returned `dlg.Event` onto the state (`Apply`) or onto OK / Cancel.

| Dialog | View | Notes |
|---|---|---|
| Sort | `SortDialogView`, `SortDialogState.Apply`, `App.handleSortDialogKey` | two-column radio grid + checkboxes |
| Listing format | `ListingFormatView`, `ListingFormatDialogState.Apply` | radios; plain letters press OK / Cancel |
| Compare category / select / unselect | `CompareFilterView`, `CompareFilterDialogState.Apply` | checkboxes, `All` is the master toggle |
| Meta columns | `MetaDialogView`, `MetaDialogState.Apply` | checkboxes from `MetaEntryID(i)`; F9 (edit config) is handled before the engine |
| Undo confirm | `UndoConfirmView` | message labels, `UndoYes` / `UndoNo`, unclamped |
| Quit confirm | `QuitConfirmView` | message labels, `QuitStay` / `QuitQuit`, unclamped |
| Host key | `HostKeyView` | detail labels, `HostKeyAccept` / `HostKeySave` / `HostKeyReject`, plain letters press buttons, unclamped |
| Stash restore | `StashRestoreView` | message labels, five buttons (`StashReplace`, `StashMerge`, `StashDropStash`, `StashDropAll`, `dlg.Cancel`), unclamped |
| Dedup kept results / scan scope | `DedupReturnView` | message labels, `DedupReturnFirst` / `DedupReturnSecond` / `dlg.Cancel`, unclamped |

Every other dialog (anything with a text input, a scrollable or filterable list, live-updating content, section separators or inline-labelled option rows) still uses the hand-written render plus `dialogform` / `DialogTrailingButtonsForm` path.

- `dlg.Place` is the only geometry pass. `Draw`, `HitTest` and `Click` read its `Placement`; nothing re-derives rects. Button x positions come from `draw.ButtonRowXs`, shared with `DrawDialogButtonRowCentered`. The button strip is drawn directly under a separator, with no blank row between them.
- **Sizing**: `Spec.Width` is the preferred width and `Place` widens it to fit the button strip between the borders (`draw.ButtonRowWidth`). By default the dialog is clamped to the screen (`draw.ClampCenteredDialogRect`) and `Place` refuses when it cannot reach `MinWidth` / `MinHeight`. `Spec.Unclamped` sizes like `draw.CenteredDialogRect` instead and always draws: width is `min(Width, screen-4)`, widened to `min(MinWidth, screen-2)` when below `MinWidth`; the height only shrinks to the screen. Height always fits the rows (`MinHeight` raises it); never pick an oversized height.
- **Labels**: an item of `Kind: dlg.KindLabel` is a static text row in the dialog text style (`DialogText` on `DialogSurface`), starting at `DialogTextX` and truncated with an ellipsis to the content width. An empty `Label` is a blank spacer row. Labels have no ID, mnemonic or state, are skipped by every focus move, and are not hit-test or click targets. A group made only of labels is not a Tab segment. Labels may sit between items of a column.
- Navigation rules are implemented generically by the engine, so a spec-based dialog gets them without per-dialog code. Focus order is group by group, column-major inside a group, then the buttons. Up/Down move within a column; Down from the bottom of a column enters the first item of the next group (or the first button); Up from the top of column 0 returns to the last item of column 0 of the previous group; Up from a button returns to the last item of the last group. Left/Right move to the same row of the adjacent column (staying put when there is no focusable item there) and along the button strip. Tab/Shift+Tab jump between groups and the button strip with wrap; in a dialog that is only a button strip they land on the first button. Space and Enter activate the focused widget (buttons included); Alt+mnemonic focuses and activates; with `Spec.PlainButtonKeys` a plain button letter presses the button (the `AllowPlainOKCancel` option of `dialogform.Handlers`).
- `HandleKey` returns an `Event`; the caller applies it to its state. OK / Cancel / Esc chords are not handled by the engine (see Button Shortcuts). Buttons that are not OK / Cancel (Yes / No, Stay / Quit Anyway, Accept / Save / Reject, Replace / Merge ...) get their own IDs; the caller acts on them, and the `activeDialogButtons` case maps the central OK / Cancel chords onto the right ones.
- The frame, option markers, separator and buttons are drawn with the same `draw.*` helpers as the other dialogs, so styling rules above apply unchanged. Labels in columns after the first are truncated to their column.
- Tests: `dlg_test.go` covers placement, hit rects, focus rules, labels and sizing. Each migrated dialog has render goldens (`internal/ui/dialog/testdata/`, `sort_dialog_golden_test.go`, `simple_dialog_golden_test.go`) and key-script goldens driven through `App.handleKey` (`internal/app/testdata/`, `sort_dialog_keys_golden_test.go`, `simple_dialog_keys_golden_test.go`), both dumped with `tcelltest.DumpRect`. Regenerate with `go test -run <Test> -update` and review the diff.

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

- **Flow `y` downward** while drawing content, then `y++` once for the mandatory blank row above buttons, then paint button row(s). Single-button dialogs: `buttonY = rect.Y + rect.Height - 2`, blank row at `buttonY - 1`, separator/content ending on `buttonY - 2` when a separator precedes the button block. Dialogs built on the `dlg` engine instead put the button strip directly under the separator; the separator is the divider, so they have no blank row.
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
- **`Clear`** (file dialog `ui.input.kill-line`): empty `Value`, cursor `0`, `PrefillPending` false.
- **`RestorePrefill`** (`ui.input.restore-default`, configurable under `[dialog.input]`): re-arm the suggested default after editing or clearing — `Value` becomes `Prefill`, cursor moves to the end, `PrefillPending` is set to true. No-op when `Prefill` is empty.
- **OK while still pending**: use current `Value` (still the full suggestion)—no extra commit step required.

### Helpers

- Use `drawInputField` in `internal/ui/dialog/file_dialog_render.go` for `FileDialogField`-based inputs (supports placeholder-pending rendering).
- Use `draw.DrawSimpleDialogInput` in `internal/ui/dialog/internal/draw/chrome.go` for simple string inputs (no placeholder-pending concept).

## Twin dialogs: Filter and Select group

Filter (`panel.filter-dialog`) and Select/Unselect group are twins: same mode radios, file-type checkboxes, Files/Dirs only, Case sensitive, meta row, live preview, reserved hint row, width and grid navigation. **A change to one (layout, options, mnemonics, matching, preview, hint/error behaviour, navigation) should generally be mirrored to the other in the same change.** When a difference is intentional, record it in `docs/agents/panels.md`.

- **File pairs**: `internal/ui/dialog/filter_hint.go` ↔ `groupselect_hint.go`, `filter_dialog_render.go` ↔ `groupselect_dialog_render.go`, `internal/apphandler/dialog/filter.go` ↔ group-select handlers in `internal/app/dialog_settings.go` (find context: `internal/apphandler/find/group_select.go`, `group_count.go`). User docs for both: `docs/panel-filter.md`.
- **Shared code (extend, don't duplicate)**: `dialog.PatternDialogWidth`, `dialog.FilterTypesRegexp` (takes the dirs-only flag), `dialog.PatternTypesDisabled` (type checkboxes disabled while Directories only is selected; renderers and toggle functions both use it), `FilterTypeLabels`/`FilterTypeMnemonics`, `dialogGridMove`/`patternDialogGridHead` (`filter_hint.go`), `panel.FilterSpec.MatchFunc`.
- **Intentional differences**: Filter has Invert and also targets Compare/dedup rows; Select group has Match full path (find context, Alt+H) and its preview counts only selection-state flips. Select group also has a third context, `output` (command-output dialog detected paths, Match full path shown like find, no meta columns); Filter has no equivalent.
- **Mnemonics**: one letter per option across both dialogs, defined once as the `PatternMnemonic*` constants in `internal/ui/dialog/state_filter.go` (Shell S, Regex R, Simple L, FilesOnly Y, DirsOnly D, Case E, IncludeMeta M, OnlyMeta N, plus Filter-only Invert I and Select group-only FullPath H, and `FilterTypeMnemonics`). Both renderers and both key handlers use them, and `dialog.PatternAltIsMnemonic` derives the pattern-input pass-through from them.
