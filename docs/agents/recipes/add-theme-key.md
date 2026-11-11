---
name: add-theme-key
description: Add a new theme color/icon key to the default theme (all other themes inherit it)
---

# Add a theme key

See `../theming.md` for the themes tree layout and UI-theme vs Chroma separation.

## Steps

1. **Schema + resolution**: add the key to `internal/theme` (`theme.go` or the matching per-area file: `panel_chrome.go`, `jobs_style.go`, `panel_git_style.go`, `folder_icon.go`, …). A style key goes into `styleKeys` (or `extraStyleKeys`) and is read from the `styles` map in `build` (`theme.go`); a gfx gradient is a `gfxTrails` entry. An icon is an `IconKey*` constant read through `Theme.icon(key)` / `Theme.iconRune(key)` (no fallback argument: the value comes from the merged theme). Do not write a Go fallback value: every theme inherits missing keys from `themes/default.toml` (`../theming.md`, Inheritance).
2. **Default theme**: add the key to `themes/default.toml` (the only tracked theme and the base every other theme inherits). A style's `fg` / `bg` / `icon` may name only a **role** (the loader rejects hues, palette names, hex and `"default"`): pick the role by what the style means (`fg`, `bg`, `surface`, `muted`, `faint`, `ink`, `focus_bg` / `focus_fg`, `header_bg`, `accent`, `primary`, `selected`, `info`, `success`, `warning`, `danger`, ... — the full list is in `../theming.md`), not by the hue it has today. A key not in `default.toml` makes every load fail, and `TestDefaultDefinesEverySchemaKey` names it. If no role fits, add one: a row in `roleDefs` (`internal/theme/roles.go`, with its default palette name), a line in `[roles]` of `default.toml` (`TestDefaultThemeRolesEqualGoTable` keeps the two equal), a derivation in `themes/import/import_alacritty.py` (`build_roles`) when a scheme can supply a better value than the palette name, and the docs. There are no other themes to update: imported and user themes inherit the key. Set it in another theme only when that theme needs a different value; for imported themes extend the generator (`themes/import/import_alacritty.py`) over hand-editing 100+ files, run `import_alacritty.py --selftest`, regenerate with `themes/import/build.sh` and read the `--check` report (the new style is checked on the merged theme).
3. **Placeholder pair rule**: input-related keys come in active/inactive pairs plus `.placeholder` variants (`dialog.input.active.placeholder` / `dialog.input.inactive.placeholder`) — add all variants when applicable.
4. **Option/row background rule**: `dialog.option.*` keys are foreground-only (row bg always `dialog.surface`); progress label keys (`*.label.on_fill` / `*.label.on_track`) are foreground-only and merged onto the bar background at render time. Follow whichever pattern the area already uses.
5. **Statuses**: if the key represents a status (ongoing, paused, failed, aborted, success), reuse the existing status icon/icon keys instead of inventing new ones.
6. **Tests**: regression tests for composed dialog chrome styling live under `internal/ui/dialog/internal/draw/` — extend when touching dialog title/accent composition.
7. **Verify**: `golangci-lint run ./...`, `go test ./...`; record the new key in `../theming.md`.
