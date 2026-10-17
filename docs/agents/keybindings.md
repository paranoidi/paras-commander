# Keybindings and shortcut tables

How default shortcuts are declared, layered and validated. Add-a-shortcut procedure: `recipes/add-keybinding.md`. User-facing list: `docs/shortcuts.md`.

## Pieces

- Action ID consts: `internal/keymap/actions.go`.
- `DefaultActionSpecs()` (`internal/keymap/specs.go`): global `[main]` defaults plus help/menu metadata.
- Overlay default maps (`Default*OverlayKeys`) in `internal/keymap/*_overlay.go`, each with an allowed-identifier set (`AllowedIn*Overlay`) that validation enforces.
- `buildBundle` (`internal/keymap/load.go`) assembles `Bundle.Global`, `Jobs`, `Commands`, `Messages`, `Compare`, `Terminal`, and the dialog overlays (`DialogInput`, `RenameDialog`, `BookmarkDialog`, `FindDialog`, `HistoryDialog`, `FlattenDialog`, `TransferDialog`, `RunForEachDialog`, `PinDialog`). There is no promotion or copying between maps. `keymap.Default` is `Build(DefaultActionKeys())` (global map only).
- `Map.ActionBindings` pre-seeds canonical TOML strings (`M-j`, `M-c`, ...) from specs and overlay defaults so menu hints stay in TOML form.
- Table names: `internal/keymap/shortcut_tables.go` (single source, used by `keymap.overlayRegistry`); parsing: `shortcut_parse.go`.

## Layering and validation

- `keymap.LoadFromPaths` layers each table independently: defaults < `keybindings.toml`. Overlay maps resolve before the global map while their view/dialog is focused.
- Shortcut tables in `config.toml` fail `config.LoadFromPaths` (unknown field). `[jobs]` in `config.toml` is job settings; the jobs-view shortcuts are `[jobs]` in `keybindings.toml`.
- Namespace restrictions: `jobs.*` only under `[jobs]`; `commands.*` only under `[commands]`; `ui.input.*` only under `[dialog.input]`; `file.rename.open-sanitize`/`open-slugify` only under `[dialog.rename]`; other dialog actions only under their `[dialog.*]` table.
- `pc --config-stub` writes `config.toml` (`config.EncodeDefaultStub`), `keybindings.toml` (`keymap.EncodeDefaultStub`, all tables) and commented `menu.toml`/`meta.toml`/`pools.toml` into `~/.config/pc/` (or a given dir); existing files are kept. New global defaults and overlay defaults appear in the stub automatically.

## Where specific defaults live

- **Jobs**: `jobs.open` (Alt+J) and `jobs.answer-blocker` (Ctrl+Q; Ctrl-only, answering a blocker is a command, not a view switch) are `[main]` so they work from browser and dialogs, and over other primary modals for the blocker quick dialog. Other `jobs.*` live in `DefaultJobsOverlayKeys` (`DefaultKeys: nil`).
- **Commands**: `commands.open` (Alt+C) is `[main]`; `commands.close` (Left, plus hardcoded Esc), `commands.terminate` (F8, SIGTERM) and `commands.kill` (Shift+F8, SIGKILL) are in `DefaultCommandsOverlayKeys`. Terminate/kill target only rows with `Phase == CommandRunRunning` (else a transient warning). Process handles live in `commandsctrl.Handler`'s private `procs` map (`internal/apphandler/commands/proc.go`: `SetProcess`/`UnregisterProc`/`signalCommandRow`), filled through `cmdrun.RunTracked`'s `onStart` from `runFileExecuteCommand` (`internal/app/run_executable.go`), `runUserMenuCommand` (`user_menu.go`) and `runForEachUnifiedBatch`. Jobs deliberately has no kill: jobs own no signalable subprocess; it only has `jobs.cancel` (Ctrl+C in `[jobs]`; see `menus.md`). `file.run-for-each` is an ordinary `[main]` action.
- **Terminal panel**: `terminal.toggle-panel` (Alt+T, show/hide only) and `terminal.focus` (Alt+P, opens the panel first if hidden) are `[main]`, and are repeated in `[terminal]` so they still work with focus inside the panel. `terminal.grow` (Ctrl+K), `terminal.shrink` (Ctrl+J), `app.drop-to-shell` (Ctrl+O) are `[terminal]` only; only these five ids are allowed there (`AllowedInTerminalOverlay`). `internal/app/input.go` dispatches toggle/focus; `internal/app/terminal_panel.go` uses `App.keys.Terminal` for grow/shrink/PTY input (`subshell.md`). Alt+T and `panel.toggle-sync` (Alt+Y) are view-switch/command tier chords that avoid Ctrl+Alt combos, which are unreliable across terminals. `panel.refresh` (Ctrl+Y) does not conflict with Alt+Y.
- **Git filter**: `panel.git-filter-menu` (Alt+G) is `[main]`, dispatched through the normal action switch (`input.go`); see `menus.md` and `panels.md`.
- **Pin**: `panel.pin-dialog` (Alt+N, plus Navigation-section leader mnemonic `n`) and `panel.pin-toggle` (Alt+Insert) are `[main]`; toggle works from the main panel, Find, History, Bookmarks, Compare and Dedup (`navigation.md`). `panel.stash-toggle` is Ctrl+Alt+Insert, freeing Alt+Insert. `pin.view`/`pin.open-primary`/`pin.open-secondary`/`pin.remove`/`pin.remove-all` (F3 / S-left / S-right / F8 / S-F8) are `[dialog.pin]` only (`AllowedInPinDialogOverlay`).
- **Destination shortcuts**: `ui.destination-active` / `ui.destination-inactive` (S-left / S-right) are one action pair with one spec, bound independently in `[dialog.flatten]` and `[dialog.transfer]`.
- **Path picker host shortcut**: on copy/move destination and symlink/hardlink path rows the picker opens with the global `bookmark.open` chords (`App.tryPathPickerHostShortcut`, footer hints read the global map).
