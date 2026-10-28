# General

This project is midnight commander inspired twin panel filesystem manager written in go using TUI.

Currently the project is in implementation phase and not all features are implemented.

Tracked architecture, import, and state rules live in this file. Per-area agent notes live in `docs/agents/`: read the file for the area you are about to change, and update every file a change touches. Those files describe the current state of the code: write what is true now, as if written fresh today (no "used to be", "previously", "renamed from", or changelog entries). Refer to commands by action id (`panel.toggle-carousel`), never by default key: default keys live only in `internal/keymap/specs.go` and the overlay defaults. Mention a literal key only for key *behaviour* (terminal quirks, in-widget keys that are not actions).

| Area | Read when touching | File |
|---|---|---|
| Glossary | naming any domain concept | `docs/agents/CONTEXT.md` |
| Decisions | architecture, transports, cross-package boundaries | `docs/agents/adr/` |
| Architecture | code map, package ownership, handler/Host wiring, CLI startup, layout, menu bar, render paths, post-event reconciler | `docs/agents/architecture.md` |
| Panels | listing, columns, tree view, filters, git, disk usage, selections, sort, quick filter | `docs/agents/panels.md` |
| Preview | quick view, F3, carousel, markdown, images, Chroma | `docs/agents/preview.md` |
| Terminal graphics | sixel/Kitty, tmux passthrough, terminal-specific image bugs | `docs/agents/terminal-graphics.md` |
| Jobs | job queue, progress, conflicts, rate limit, quit safety | `docs/agents/jobs.md` |
| Dialogs | any dialog's navigation, layout, styling; labels on borders; Filter ↔ Select group twins | `docs/agents/dialogs.md` |
| File operations | copy/move/delete/rename/mkdir dialogs, extract, flatten | `docs/agents/file-operations.md` |
| Navigation | history, pin, latched sync, find, bookmarks, path picker, SFTP | `docs/agents/navigation.md` |
| Menus | leader-menu strip, user menu, per-view menus, vi-motion | `docs/agents/menus.md` |
| Commands | Commands view, run-for-each, work pools, user-menu commands | `docs/agents/commands.md` |
| Views | messages, compare, dedup, help | `docs/agents/views.md` |
| Subshell | Ctrl+O subshell, embedded terminal panel | `docs/agents/subshell.md` |
| Keybindings | shortcut tables, overlays, validation | `docs/agents/keybindings.md` |
| Theming | theme TOML structure | `docs/agents/theming.md` |
| Technical debt | before proposing a cleanup | `docs/agents/technical-debt.md` |

Recipes: adding a config option, keybinding, menu item, theme key, or dialog → follow `docs/agents/recipes/<add-config-option|add-keybinding|add-menu-item|add-theme-key|new-dialog>.md`.

Leave compiled binary after running tests. Build with `CGO_ENABLED=0 go build -ldflags="-s -w" -o pc ./cmd/pc` — the binary must be named `pc`, not `paras-commander` (the module/repo directory name). Local verification is `golangci-lint run ./...` (v2.12.1) plus `go test ./...`; `task verify` runs lint + tests + that `pc` build. CI runs `go test -race ./...` (see Verification).

When user request shortcut to be added it needs to be configurable in `keybindings.toml`. Check if requested shortcut works in terminal. Many shortcut combinations do not. If user requests a shortcut that is already taken in that context, choose a reasonable next best option and notify the user.

When new colors are being added be sure to update any existing themes to use these new colors (including `dialog.input.active.placeholder` / `dialog.input.inactive.placeholder` for dimmed suggested input text when applicable).

Always aim to have ONE source for the truth. This applies to keybindings, theme usage, padding and margin values. Similarly operations should ideally have only one entry point.

Do not open files with GUI applications (`xdg-open`, `xed`, `gedit`, etc.) while working in this repo — it pops a window on the user's desktop. Use the Read tool or terminal output instead.

# Import rule (apphandler)

**`internal/apphandler/*` must not import `internal/app`**: handlers reach the app only through their `Host` interface (ADR 0001). Which package owns what is in `docs/agents/architecture.md` (Package layout).

# Backwards compatibility (CRITICAL)

The application is in prototyping phase. There is no need to make anything backwards compatible. Update all existing code to
new requirements. Remember to update theme (themes/default.toml).

# Menus

When adding new items into menu, use first letter of the text as a shortcut if available. If not use first letter of second word if available. If not available use second letter of the text if available. If not available use first letter of third word if available.

# Statuses

Theme file defines icons/glyphs for statuses such as ongoing, paused, failed, aborted, success etc. Use them where appropriate.

# Configuration

Default configuration values should be stored in a single place and imported from there instead of putting magic values around the codebase. Built-in defaults for jobs copy progress emit, file operations (copy buffer, fsync, disk checks, CoW cloning), and disk-usage walk concurrency are exported `const` in `internal/config/builtin.go` and wired through `config.Default()`; `internal/ops` reads them via `config` (it must not define parallel magic numbers). `internal/config` must not import `internal/ops` (import-cycle constraint).

Whenever a field is added, removed, renamed, or its default/behavior changes in the `internal/config` structs, update `docs/config.md` to match in the same change.

# Testing

- Do not use my local files or filenames as a basis for tests. Generate equivalent length of filenames using random english words.

# Verification

Local gate:

- `golangci-lint run ./...` with the repo `.golangci.yml` and **golangci-lint v2.12.1** (see https://golangci-lint.run/).
- `go test ./...`
- `CGO_ENABLED=0 go build -ldflags="-s -w" -o pc ./cmd/pc` (binary must be named `pc`)

`task verify` runs that sequence. `govulncheck` is optional and is not a merge requirement.

The race suite (`go test -race ./...`, `task race`) is slow and runs in CI on every push and PR. Do not run it locally unless the user asks, or the change touches concurrency (goroutines, channels, mutexes, atomics, state shared across goroutines). Then run it scoped to the touched packages (`go test -race ./internal/<pkg>/...`), not `./...`.

# File deletions

CRITICAL:

- Do not remove ANY files which are not being tracked by git.

# Feature documentation

- Features should be documented for end-user under ./docs/
- Each feature should live it's own file, update the file as feature evolves
- Feature is a large functionality, not a simple button or a toggle. Examples: (terminal graphics, job queue, panel comparison)

# Auto-commit

When a requested change to functionality is done, commit it without asking, provided all of these hold:

- The local gate (see Verification) passes.
- No open questions, unresolved design choices, or considerations remain that the user should weigh.
- Nothing significant from the request was skipped, stubbed, or deferred.

If any of these fails, stop, report, and wait for the user. Stage only the files the change touched: never untracked files you did not create, never unrelated pre-existing modifications. Follow the commit message rules below, do not push, and name the commit subject in your final reply.

# Commit messages

- Subject: Conventional Commits, `<type>(<scope>): <summary>` — lowercase imperative summary, no trailing period.
- Types: `feat`, `fix`, `refactor`, `perf`, `docs`, `test`. Scope is the area touched (`git`, `find`, `dialog`, `compare`, `jobs`, `panel`, `ui`, `agents` for `docs/agents/`, ...); omit it when a change spans many areas (`fix: batch of panel and dialog bugs`).
- One type per commit; a commit mixing a feature with a layout change still uses the dominant type.
- Body (optional): what and why, wrapped at 79 columns.

# Performance

- All IO/Disk/Network operations should be implemented in a separate goroutine/process in async manner UNLESS it is directly related to immediate UI feedback like path autocomplete where such heavy machinery is not warranted.
  

## Agent skills

### Issue tracker

Issues are tracked in GitHub Issues (`paranoidi/paras-commander`) via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Default canonical labels: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: `docs/agents/CONTEXT.md` plus `docs/agents/adr/`. See `docs/agents/domain.md`.
