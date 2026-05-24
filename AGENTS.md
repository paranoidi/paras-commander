# General

This project is midnight commander inspired twin panel filesystem manager written in go using TUI.

Currently the project is in implementation phase and not all features are implemented.

Read all documantion in docs/ before starting implementing new feature.

Once new feature is implemented update docs/features-done.md

Leave compiled binary after running tests.

When user request shortcut to be added it needs to be configurable in the configuration file (toml).

When new colors are being added be sure to update any existing themes to use these new colors (including `dialog.input.active.placeholder` / `dialog.input.inactive.placeholder` for dimmed suggested input text when applicable).

Always aim to have ONE source for the truth. This applies to keybindings, theme usage, padding and margin values.

# App package layout

- **`internal/app`**: `App` struct, `Run()` event loop, input dispatch, render/reconcile, and thin `*_bridge.go` / `*_host.go` files that wire feature handlers. Modal file-dialog orchestration lives in `dialog_*.go` (rendering/state remains in `internal/ui/dialog`).
- **`internal/apphandler/*`**: feature handlers (`jobs`, `find`; `dialog` reserved for future path-picker / transfer extraction). Each handler takes a `Deps` struct and a `Host` interface for cross-cutting app services.
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
- All content rows and help text use the dialog background as their background color.
- Buttons use `drawDialogButton()` helper which renders ` [ label ] ` (outer spaces plus single spaces inside brackets) using `DialogButtonActive` / `DialogButtonInactive` (and `DialogButtonActiveDestructive` when `DialogButtonSpec.Destructive` is set) **as defined in the theme** (foreground and background; do not substitute `dialog.surface` for button fill).
  - Shortcut letter in `dialog.accent` foreground color (`Theme.DialogAccent`), bold
- Checkbox and radio rows use `dialog.option.inactive`, `dialog.option.active` (focused row), and `dialog.option.selected` (checked radio or checked checkbox when the row is not focused).
- Do not use `styles.DialogText` for input row fill; input rows use `theme.Theme.DialogInputPair(focused)` (resolved from `dialog.input.active` / `dialog.input.inactive` and their `.placeholder` entries). Those styles carry both foreground and background for the input row—do not substitute `DialogText` for fill.

## Code Structure

- Button rendering: `drawDialogButton()` in `internal/ui/dialog.go`.
- Focus management per dialog type is handled in `internal/app` (`input.go` and `dialog_*.go`).
- File dialogs (`FileDialogState`) use `FocusedField` for focus tracking; button focus indexes = len(Fields)+0 for OK and len(Fields)+1 for Cancel.

## Testing

- Do not use my local files or filenames as a basis for tests. Generate equivalent length of filenames using random english words.

## Input Area Rendering / Styling

All dialogs with user text input fields must follow these rules:

### Layout

- The label and input field must be on separate rows.
- A blank (empty) line must exist between the label row and the input field row.
- Do not place the label and input on the same line, even with visual separators.

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

Pay attention to following details:
- Title Copy is centered. Surrounded with empty spaces.
- ONE space margin at the left. Dialog is scaled so that there is ONE space margin at the right side.
- Input text area is in it's own line. It should cover the all avaialable space in the dialog (respecting 1 space margin).
- Buttons should be visually centered.
- Separation lines between sections.
- Do not add empty lines above buttons.

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
- **`RestorePrefill`** (defaults `Ctrl+R` and `Ctrl+D`, configurable under `[dialog_input_action_keys]` as `ui.input.restore-default`): re-arm the suggested default after editing or clearing — `Value` becomes `Prefill`, cursor moves to the end, `PrefillPending` is set to true. No-op when `Prefill` is empty.
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

