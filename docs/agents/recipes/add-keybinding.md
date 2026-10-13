---
name: add-keybinding
description: Add a new configurable keyboard shortcut (action) wired through keybindings.toml
---

# Add a configurable keybinding

Every user-requested shortcut must be configurable in `keybindings.toml` — never hardcode chords in dispatch. See `../keybindings.md` for the table layout.

## Steps

1. **Action ID**: add a const in `internal/keymap/actions.go`. Use the existing dotted namespaces (`app.*`, `nav.*`, `panel.*`, `file.*`, `jobs.*`, `commands.*`, `messages.*`, `compare.*`, `dedup.*`, `find.*`, `bookmark.*`, `flatten.*`, `remote.*`, `ui.input.*`).
2. **Default + metadata**: add an `ActionSpec` to `DefaultActionSpecs()` in `internal/keymap/specs.go` (`ID`, `Title`, `Section` for help grouping, `DefaultKeys`, `PreferredKey` for menu/footer labels, `Keywords` for help fuzzy search).
   - **Global action** (`[main]` table): set `DefaultKeys` on the spec. Done.
   - **View/dialog-scoped action**: set `DefaultKeys: nil` on the spec; put the default chord in the overlay map instead (`Default*OverlayKeys` in the matching `internal/keymap/*_overlay.go` — jobs, commands, messages, compare, dedup, file_preview, or a `[dialog.*]` overlay). Add the ID to that overlay's allowed-identifier set (`AllowedIn*Overlay`) or `keymap.LoadFromPaths` validation will reject user configs.
3. **Dispatch**: handle the action in `internal/app` — `input.go` for global/browser dispatch, or the view's key handler for overlay actions. Overlay maps resolve before the global map while their view/dialog is focused.
4. **Hints resolve themselves**: menu `KeyLabel`s, footer labels, help rows, and the `pc --config-stub` keybindings stub all derive from the specs/overlay defaults — no extra plumbing. If the action needs a menu entry, see `add-menu-item.md`.
5. **Verify**: `golangci-lint run ./...` and `go test ./...`; update the matching `docs/agents/` file.

## Choosing a chord (terminals lie)

Many combinations don't reach the app. Known traps:

- **Ctrl+H** arrives as Backspace in many terminals (bound to `nav.parent`).
- **Ctrl+Enter** is indistinguishable from Enter; **Shift+Enter** needs kitty-protocol terminals (works via tcell CSI-u).
- **Shift+Alt+comma** may arrive as **Alt+;** — provide both when relevant (see `panel.meta-edit`).
- **Ctrl+C / Ctrl+Space / Ctrl+O** may be intercepted by terminal or shell settings.
- Probe tests in `internal/keymap/*_probe_test.go` document tcell chord behavior — check them before inventing a chord.

If the requested chord is already taken in that context, pick a reasonable alternative and **tell the user** (repo CLAUDE.md rule).
