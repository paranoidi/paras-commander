# General

This project is midnight commander inspired twin panel filesystem manager written in go using TUI.

Currently the project is in implementation phase and not all features are implemented.

Read all documantion in llm-docs/ before starting implementing new feature.

Once new feature is implemented update llm-docs/index.md for agents to later refer to.

Leave compiled binary after running tests. Build with `go build -o pc ./cmd/pc` — the binary must be named `pc`, not `paras-commander` (the module/repo directory name).

When user request shortcut to be added it needs to be configurable in `keybindings.toml`. Check if requested shortcut works in terminal. Many shortcut combinations do not. If user requests a shortcut that is already taken in that context, choose a reasonable next best option and notify the user.

When new colors are being added be sure to update any existing themes to use these new colors (including `dialog.input.active.placeholder` / `dialog.input.inactive.placeholder` for dimmed suggested input text when applicable).

Always aim to have ONE source for the truth. This applies to keybindings, theme usage, padding and margin values. Similarly operations should ideally have only one entry point.

# App package layout

- **`internal/app`**: `App` struct, `Run()` event loop, input dispatch, render/reconcile, and thin `*_bridge.go` / `*_host.go` files that wire feature handlers. Modal file-dialog orchestration lives in `dialog_*.go` (rendering/state remains in `internal/ui/dialog`).
- **`internal/apphandler/*`**: feature handlers (`jobs`, `find`; `dialog` reserved for future path-picker / transfer extraction). Each handler takes a `Deps` struct and a `Host` interface for cross-cutting app services.
- **`internal/apphandler/host/`**: decomposed host facets (`LayoutHost`, `MessageHost`, `PanelHost`, `ShellHost`) composed into each handler's `Host` interface.
- **`internal/app/shell_host.go`**: shared `appShellHost` embedded by `jobs_host.go` and `find_host.go`.
- **`internal/app/helpkeys`**, **`internal/app/pathpick`**, **`internal/app/jobbridge`**: stateless helpers extracted from the app layer (phase 1).

## Import rule (apphandler)

**`internal/apphandler/*` must not import `internal/app` (avoid import cycles). `internal/app` imports handlers only.**

Handlers call back into the shell via `Host` methods (layout, messages, panel navigation, etc.). Wake/event payload types used by `Run()` may live in the handler package (e.g. `jobsctrl.WakePayload`, `findctrl.WakePayload`).

# Backwards compatibility (CRITICAL)

The application is in prototyping phase. There is no need to make anything backwards compatible. Update all existing code to
new requirements. Remember to update theme (themes/default.toml).

# Menus

When adding new items into menu, use first letter of the text as a shortcut if available. If not use first letter of second word if available. If not available use second letter of the text if available. If not available use first letter of third word if available.

# Statuses

Theme file defines icons/glyphs for statuses such as ongoing, paused, failed, aborted, success etc. Use them where appropriate.

# Configuration

Default configuration values should be stored in a single place and imported from there instead of putting magic values around the codebase. Built-in defaults for jobs copy progress emit, file operations (copy buffer, fsync, disk checks, CoW cloning), and disk-usage walk concurrency are exported `const` in `internal/config/builtin.go` and wired through `config.Default()`; `internal/ops` reads them via `config` (it must not define parallel magic numbers). `internal/config` must not import `internal/ops` (import-cycle constraint).

# Dialog Standards

All dialogs (modal overlays) must follow these navigation and rendering rules:

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
- **Tab/Shift+Tab**: Same as Down/Up (cycles through all focusable items for convenience).
- **Enter**: Activates the focused button or confirms the dialog.
- **Esc**: Cancels/closes the dialog.

## Button Shortcuts

- Buttons have a highlighted shortcut letter in accent color (magenta by default).
- Shortcuts: `O` for OK, `C` for Cancel, `Y` for Yes, `N` for No.
- Shortcuts are triggered via **Alt+letter** (e.g., Alt+O executes OK, Alt+C cancels).

## Rendering / Styling

- All dialogs use `styles.DialogSurface` as the fill color, not `styles.DialogText`.
- The border uses `styles.DialogFrame` on the dialog background.
- The title row is built from `styles.DialogTitle` **attributes** (`bold`, etc.) with **glyph foreground** taken from `dialog.frame` and **glyph background** from `dialog.surface` (see `DrawDialogFrame` in `internal/ui/dialog/internal/draw/chrome.go`).
- All content rows use the dialog background as their background color.
- **Do not** add navigation help footers (e.g. `Left/Right  Enter confirm  Esc cancel`) at the bottom of dialogs. Standard dialog navigation and button Alt-shortcuts are enough; the user does not want keyboard cheat-sheet rows in the UI.
- Buttons use `drawDialogButton()` helper which renders ` [ label ] ` (outer spaces plus single spaces inside brackets) using `DialogButtonActive` / `DialogButtonInactive` (and `DialogButtonActiveDestructive` when `DialogButtonSpec.Destructive` is set) **as defined in the theme** (foreground and background; do not substitute `dialog.surface` for button fill).
  - Shortcut letter in `dialog.accent` foreground color (`Theme.DialogAccent`), bold
- Checkbox and radio rows use `Theme.DialogOptionRowStyle` (resolved from `dialog.option.inactive`, `dialog.option.active`, `dialog.option.selected`, and `dialog.option.active.selected` when focused and checked/marked; row background always matches `dialog.surface` — do not set `bg` on option keys).
- Do not use `styles.DialogText` for input row fill; input rows use `theme.Theme.DialogInputPair(focused)` (resolved from `dialog.input.active` / `dialog.input.inactive` and their `.placeholder` entries). Those styles carry both foreground and background for the input row—do not substitute `DialogText` for fill.

### Horizontal columns (do not confuse `rect.X+1` and `rect.X+2`)

Use `draw.DialogTextX(rect)`, `draw.DialogOptionX(rect)`, and `draw.DialogContentWidth(rect)` in `internal/ui/dialog/internal/draw/geom.go` instead of re-deriving offsets.

- `rect.X` — left border (`│`); `rect.X+1` — inner margin (blank dialog surface).
- **Labels, hints, and input rows** — `DialogTextX(rect)` (`rect.X+2`): first content glyph one space inside the border (e.g. `Destination:`, `/path`, command field).
- **Checkbox / radio rows** — pass `DialogOptionX(rect)` (`rect.X+1`) to `draw.DrawDialogCheckbox` / `draw.DrawDialogRadio` only. Markers are ` [ ] ` / ` ( ) ` with a **leading space** that fills the margin cell; the visible `[` or `(` must land on `DialogTextX`, not one column to the right.
- **Common bug:** `DrawDialogRadio(screen, rect.X+2, …)` paints an extra blank column before `( )`, so options look indented relative to labels (seen on Run-for-each pool rows). Copy/move checkboxes in `transfer_dialog_render.go` and Find checkboxes use `rect.X+1` — match that, not `rect.X+2`.
- **Inner width** — `DialogContentWidth(rect)` (`rect.Width-4`) for full-width text and inputs between margins.

## Code Structure

- Button rendering: `drawDialogButton()` in `internal/ui/dialog.go`.
- Focus management per dialog type is handled in `internal/app` (`input.go` and `dialog_*.go`).
- File dialogs (`FileDialogState`) use `FocusedField` for focus tracking; button focus indexes = len(Fields)+0 for OK and len(Fields)+1 for Cancel.

## Testing

- Do not use my local files or filenames as a basis for tests. Generate equivalent length of filenames using random english words.

## Input Area Rendering / Styling

All dialogs with labeled content blocks (text inputs, radio groups, checkbox groups under a section label) must follow these rules:

### Layout

- The label and the content below it must be on separate rows.
- A blank (empty) line must exist between the label row and the content row(s) below it (text input, first radio/checkbox option, preview value, etc.).
- Do not place the label and its content on the same line, even with visual separators.
- Section labels followed by radio or checkbox options use the same pattern as text fields: **label row → blank row → option rows** (e.g. Run-for-each **Worker pool (optional):** then pool radios). Standalone option rows without a section label (e.g. mkdir post-action radios directly under a separator) do not need an extra label row.

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

Logic lives in `internal/ui/dialog_field.go`. When opening a dialog with a suggested value, set `Value` and `Prefill` to the suggestion and set `PrefillPending` when the suggestion is non-empty.

- **First printable (`InsertRune`)**: clear `Value`, reset cursor, clear `PrefillPending`, then insert the rune (replace-from-scratch on top of the suggestion).
- **Backspace, Delete, cursor moves (`MoveCursor` / Home / End via handlers)** call `commitPrefill()` first: drop `PrefillPending` but **keep** `Value`, then apply the usual edit/move.
- **`Clear`** (file dialog `Ctrl+L`): empty `Value`, cursor `0`, `PrefillPending` false.
- **`RestorePrefill`** (defaults `Ctrl+R` and `Ctrl+D`, configurable under `[dialog.input]` as `ui.input.restore-default`): re-arm the suggested default after editing or clearing — `Value` becomes `Prefill`, cursor moves to the end, `PrefillPending` is set to true. No-op when `Prefill` is empty.
- **OK while still pending**: use current `Value` (still the full suggestion)—no extra commit step required.

### Helpers

- Use `drawInputField` in `internal/ui/file_dialog_render.go` for `FileDialogField`-based inputs (supports placeholder-pending rendering).
- Use `drawSimpleDialogInput` in `internal/ui/dialog_chrome.go` for simple string inputs (no placeholder-pending concept).

# Verification

Changes must pass `golangci-lint run ./...` with the repo `.golangci.yml` (see https://golangci-lint.run/).
Changes must pass `go test ./...`.

# File deletions

CRITICAL:

- Do not remove ANY files which are not being tracked by git.

