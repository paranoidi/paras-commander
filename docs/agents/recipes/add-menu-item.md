---
name: add-menu-item
description: Add a pulldown menu item or footer F-key hint
---

# Add a menu item

## Steps

1. **Definition**: menu structures live in `internal/ui/menu` — `top.go` (browser pulldowns), plus per-view menus (`jobs_menu.go`, `commands_menu.go`, `messages_menu.go`, `dedup_menu.go`, `display_menu.go`, `dev_menu.go`). Rendering is in `internal/ui`.
2. **Shortcut letter** (repo CLAUDE.md rule): first letter of the text; if taken, first letter of the second word; then second letter of the text; then first letter of the third word. Must be unique within the pulldown.
3. **Bind an action, not a chord**: item `KeyLabel` hints resolve from the loaded keymap via `menu.ApplyMenuKeyLabels` → `keymap.Map.MenuBindingLabel` (stable ordering for multi-binding actions). The item should dispatch a `keymap` action ID so user rebinds update the hint automatically. Menu-only actions (e.g. `dev.*`) exist but are the exception.
4. **Panel-scoped items** (Left/Right menus): use `menu.PanelScopePrimary` / `PanelScopeSecondary`; menu labels still say Left/Right while panel IDs are primary/secondary.
5. **Not implemented yet?** The item must close the menu and show a concise "not implemented" status message.
6. **Footer F-keys**: `menu.FunctionKey` entries in `internal/ui/menu/fkeys.go` (`Hint` + `HintShiftPrefix` for Shift-alternative suffixes); rendered by `drawFooter` in `internal/ui/status_footer_render.go`. Auxiliary views have their own restricted footer sets (`menu.FunctionKeys*View`).
7. **Verify**: `golangci-lint run ./...`, `go test ./...`; update the matching `docs/agents/` file.
