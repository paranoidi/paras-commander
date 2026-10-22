# Getting started

Paras Commander (`pc`) is a twin-panel terminal file manager inspired by Midnight Commander. Two directory panels sit side by side; the active one takes your keys, and copy, move and similar commands use the other panel as the destination.

## Requirements

- Linux
- Go 1.26+ (to install or build)
- A Nerd Font or similar in the terminal. There is no icon fallback; with `use_nerdfont_icons = false` ([Config](config.md#ui)) dialogs use ASCII markers and file icons are hidden.

## Install

```sh
go install github.com/paranoidi/paras-commander/cmd/pc@latest
```

The `pc` binary lands in `$GOBIN`, or in `$(go env GOPATH)/bin` (often `~/go/bin`) when `GOBIN` is unset. Put that directory on `PATH`. Replace `@latest` with a tag or commit (`@v0.1.0`, `@main`) to pin a version.

To build from a checkout: `go build -o pc ./cmd/pc`.

## Running

```sh
pc                       # both panels at the current directory
pc ~/projects            # left panel opens the directory
pc ~/projects ~/Downloads  # left, then right
pc notes.txt             # opens the file in fullscreen preview
pc notes.txt ~/Downloads # one file and one directory
```

- With one file argument, the left panel opens the file's parent with the file selected and the file opens in fullscreen preview; `Esc` returns to the listing.
- With a file and a directory, each panel opens its path (the file's panel selects the file) and Quick View is turned on with the file's panel as the source. The argument order decides which panel is left.
- Two file arguments are an error, as is a missing path.
- Path arguments cannot be combined with `--chooser-file`.

## Flags

| Flag | Effect |
|---|---|
| `-v`, `--version` | Print the version and exit. |
| `--config-stub [DIR]` | Write example config files into `DIR` (default: the config directory) and exit. Existing files are kept. |
| `--chooser-file=PATH` | File-chooser mode: Enter on a file writes its path to `PATH` and exits. See [Chooser mode](chooser-mode.md). |
| `--select=PATH` | Chooser mode only: open at this file or directory. |
| `--no-carousel` | Chooser mode only: start without the carousel view. |
| `--carousel` | Start with the left panel in the carousel view (not with `--chooser-file`). See [Layout](layout.md#carousel-view). |
| `-qp` | Turn on Quick View at startup. Requires a path argument. See [Quick view](quick-view-preview.md). |
| `-dev` | Add a Dev pulldown with test toasts to the menu bar. |

At most two positional paths are accepted.

## Configuration files

Everything is optional: with no files, built-in defaults apply. Files live in `$XDG_CONFIG_HOME/pc/`, or `~/.config/pc/` when `XDG_CONFIG_HOME` is unset. `pc` does not create them on its own; generate a complete, commented set with:

```sh
pc --config-stub
```

This writes `config.toml`, `keybindings.toml`, `menu.toml`, `meta.toml`, `pools.toml` and `patterns.toml`, leaving existing files unchanged. Themes go in `themes/` in the same directory. Details: [Config](config.md), [Keybindings](keybindings.md), [Themes](themes.md).

Inside the app, Options -> Configuration edits settings, and `F9` there opens `config.toml` in your editor (writing the default file first if it does not exist yet).

## Orientation

- The top row is the [menu bar](menus.md) (`F9`); the bottom row shows function-key hints.
- `Tab` switches the active panel. Arrow keys move; typing letters fuzzy-filters the current listing.
- `F1` or `?` opens context-aware help that lists every key for the current view, searchable. See [Messages and help](messages.md#help).
- `:` opens the function menu, a letter-driven list of all commands. `Esc` toggles persistent vi-motion mode. See [Menus](menus.md).
- Long-running work (copy, move, delete) runs as background [jobs](jobs.md).
- Default shortcuts: [Shortcuts](shortcuts.md).
