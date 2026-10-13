---
name: add-theme-key
description: Add a new theme color/icon key with fallback and theme updates
---

# Add a theme key

See `../theming.md` for the themes tree layout and UI-theme vs Chroma separation.

## Steps

1. **Schema + resolution**: add the key to `internal/theme` (`theme.go` or the matching per-area file: `panel_chrome.go`, `jobs_style.go`, `panel_git_style.go`, `folder_icon.go`, …). Provide a **runtime fallback** when the key is missing — imported/user themes won't have it (pattern: `Theme.Icon*` resolvers, or defaulting to related chrome styles like the bottom-indicator keys do).
2. **Default theme**: add the key with a sensible value to `themes/default.toml` (the only embedded theme). Repo CLAUDE.md: when adding colors, update existing themes to use them — for `themes/imports/*` prefer extending the generator (`import_alacritty.py`) over hand-editing 100+ files; the fallback from step 1 must keep un-regenerated themes rendering correctly.
3. **Placeholder pair rule**: input-related keys come in active/inactive pairs plus `.placeholder` variants (`dialog.input.active.placeholder` / `dialog.input.inactive.placeholder`) — add all variants when applicable.
4. **Option/row background rule**: `dialog.option.*` keys are foreground-only (row bg always `dialog.surface`); progress label keys (`*.label.on_fill` / `*.label.on_track`) are foreground-only and merged onto the bar background at render time. Follow whichever pattern the area already uses.
5. **Statuses**: if the key represents a status (ongoing, paused, failed, aborted, success), reuse the existing status icon/icon keys instead of inventing new ones.
6. **Tests**: regression tests for composed dialog chrome styling live under `internal/ui/dialog/internal/draw/` — extend when touching dialog title/accent composition.
7. **Verify**: `golangci-lint run ./...`, `go test ./...`; record the new key in `../theming.md`.
