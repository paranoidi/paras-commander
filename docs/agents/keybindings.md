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
- **Dialog OK / Cancel**: `ui.input.dialog-ok` and `ui.input.dialog-cancel` are `[dialog.input]` defaults and the only place their chords are bound. `App.handleKey` resolves them once (`dispatchDialogButton`, `internal/app/dialog_buttons.go`, reading the buttons of `App.activeDialog`); dialog key handlers must not look for them. `F9` stays "edit config" in the Configuration and meta dialogs.
- Namespace restrictions: `jobs.*` only under `[jobs]`; `commands.*` only under `[commands]`; `ui.input.*` only under `[dialog.input]`; `file.rename.open-sanitize`/`open-slugify` only under `[dialog.rename]`; other dialog actions only under their `[dialog.*]` table.
- `pc --config-stub` writes `config.toml` (`config.EncodeDefaultStub`), `keybindings.toml` (`keymap.EncodeDefaultStub`, all tables) and commented `menu.toml`/`meta.toml`/`pools.toml` into `~/.config/pc/` (or a given dir); existing files are kept. New global defaults and overlay defaults appear in the stub automatically.

## Where specific defaults live

- **Jobs**: `jobs.open` and `jobs.answer-blocker` (answering a blocker is a command, not a view switch) are `[main]` so they work from browser and dialogs, and over other primary modals for the blocker quick dialog. Other `jobs.*` (including `jobs.undo`, `jobs.result-filter`, `jobs.reset-filter`, `jobs.expand` and `jobs.collapse`) live in `DefaultJobsOverlayKeys` (`DefaultKeys: nil`).
- **Commands**: `commands.open` is `[main]`; `commands.close` (plus hardcoded Esc), `commands.terminate` (SIGTERM) and `commands.kill` (SIGKILL) are in `DefaultCommandsOverlayKeys`. Terminate/kill target only rows with `Phase == CommandRunRunning` (else a transient warning). Process handles live in `commandsctrl.Handler`'s private `procs` map (`internal/apphandler/commands/proc.go`: `SetProcess`/`UnregisterProc`/`signalCommandRow`), filled through `cmdrun.RunTracked`'s `onStart` from `runFileExecuteCommand` (`internal/app/run_executable.go`), `runUserMenuCommand` (`user_menu.go`) and `runForEachUnifiedBatch`. Jobs deliberately has no kill: jobs own no signalable subprocess; it only has `jobs.cancel` (`[jobs]`; see `menus.md`). `file.run-for-each` is an ordinary `[main]` action.
- **Terminal panel**: `terminal.toggle-panel` (show/hide only) and `terminal.focus` (opens the panel first if hidden) are `[main]`, and are repeated in `[terminal]` so they still work with focus inside the panel. `terminal.grow` and `terminal.shrink` are `[terminal]` only; `app.drop-to-shell` is `[main]` and repeated in `[terminal]`; only these five ids are allowed there (`AllowedInTerminalOverlay`). `internal/app/input.go` dispatches toggle/focus; `internal/app/terminal_panel.go` uses `App.keys.Terminal` for grow/shrink/PTY input (`subshell.md`).
- **Git filter**: `panel.git-filter-menu` is `[main]`, dispatched through the normal action switch (`input.go`); see `menus.md` and `panels.md`.
- **Pin**: `panel.pin-dialog` (plus Navigation-section leader mnemonic `n`) and `panel.pin-toggle` are `[main]`; toggle works from the main panel, Find, History, Bookmarks, Compare and Dedup (`navigation.md`). `pin.view`/`ui.open-primary`/`ui.open-secondary`/`pin.remove`/`pin.remove-all` are `[dialog.pin]` only (`AllowedInPinDialogOverlay`).
- **Command output**: `command-output.toggle-selection` is `[dialog.command_output]` only (`AllowedInCommandOutputDialogOverlay`); `Bundle.CommandOutputDialog` reaches `commands.Handler` as `Deps.KeysCommandOutputDialog`. The cursor keys themselves (arrows, hjkl via `keymap.RemapViMotionKey`) are fixed, not actions.
- **Destination shortcuts**: `ui.destination-active` / `ui.destination-inactive` are one action pair with one spec, bound independently in `[dialog.flatten]` and `[dialog.transfer]`.
- **Path picker host shortcut**: on copy/move destination and symlink/hardlink path rows the picker opens with the global `bookmark.open` chords (`App.tryPathPickerHostShortcut`, footer hints read the global map).
