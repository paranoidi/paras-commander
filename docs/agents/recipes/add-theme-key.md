---
name: add-theme-key
description: Add a new theme color/icon key with fallback and theme updates
---

# Add a theme key

See `../theming.md` for the themes tree layout and UI-theme vs Chroma separation.

## Steps

1. **Schema + resolution**: add the key to `internal/theme` (`theme.go` or the matching per-area file: `panel_chrome.go`, `jobs_style.go`, `panel_git_style.go`, `folder_icon.go`, …). Provide a **runtime fallback** when the key is missing — imported/user themes won't have it (pattern: `Theme.Icon*` resolvers, which read `[icons]` only through `Theme.iconOr(key, fallback)` / `Theme.iconRuneOr`, or defaulting to related chrome styles like the bottom-indicator keys do). A colour fallback is built with `roleStyle(roles, fgRole, bgRole)` from the role matching the key's meaning, never from a `tcell` colour constant, so an imported theme recolours it.
2. **Default theme**: add the key with a sensible value to `themes/default.toml` (the only embedded theme). A style's `fg` / `bg` / `icon` may name only a **role** (the loader rejects hues, palette names, hex and `"default"`): pick the role by what the style means (`fg`, `bg`, `surface`, `muted`, `faint`, `ink`, `cursor_bg` / `cursor_fg`, `header_bg`, `accent`, `primary`, `selected`, `info`, `success`, `warning`, `danger`, ... — the full list is in `../theming.md`), not by the hue it has today. If no role fits, add one: a row in `roleDefs` (`internal/theme/roles.go`, with its default palette name), a line in `[roles]` of `default.toml` (`TestDefaultThemeRolesEqualGoTable` keeps the two equal), a derivation in `themes/import/import_alacritty.py` (`build_roles`) when a scheme can supply a better value than the palette name, and the docs. Repo CLAUDE.md: when adding colors, update existing themes to use them — for imported themes extend the generator (`themes/import/import_alacritty.py`) over hand-editing 100+ files; run `import_alacritty.py --selftest`, regenerate with `themes/import/build.sh` and read the `--check` report after changing the template; the fallback from step 1 must keep un-regenerated themes rendering correctly.
3. **Placeholder pair rule**: input-related keys come in active/inactive pairs plus `.placeholder` variants (`dialog.input.active.placeholder` / `dialog.input.inactive.placeholder`) — add all variants when applicable.
4. **Option/row background rule**: `dialog.option.*` keys are foreground-only (row bg always `dialog.surface`); progress label keys (`*.label.on_fill` / `*.label.on_track`) are foreground-only and merged onto the bar background at render time. Follow whichever pattern the area already uses.
5. **Statuses**: if the key represents a status (ongoing, paused, failed, aborted, success), reuse the existing status icon/icon keys instead of inventing new ones.
6. **Tests**: regression tests for composed dialog chrome styling live under `internal/ui/dialog/internal/draw/` — extend when touching dialog title/accent composition.
7. **Verify**: `golangci-lint run ./...`, `go test ./...`; record the new key in `../theming.md`.
