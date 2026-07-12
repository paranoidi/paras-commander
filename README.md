# Paras Commander

A Linux terminal twin-panel file manager inspired by Midnight Commander and fzf, written in Go using a TUI.

> ⚠️ **Not yet stable** Use at your own risk.

## Features

- Practically everything is based on fzf style searching. Filelist navigation, keybind help, etc.
- All jobs are queued into background by default.
- Jobs view with queue management and transfer graphs.
- Gitignore and git statuses using eza notation.
- Copied / moved files are indicated with an icon for visibility.
- Integrated disk usage scanning and reporting.
- Integrated directory comparison.
- Integrated find duplicates utility.
- Supports selections across multiple paths.
- Selections can be stashed and restored (1 slot).
- Integrated mass rename.
- Rename supports slugify and clean.
- Read bookmarks from ~/.fzf-marks and gnome gtk files. Writes bookmarks to ~/.fzf-marks.
- Find files / paths recursively. Can handle at least 2 million files.
- Meta column(s) can be used to provide data from external commands.
- Execute command for selected files.
- User menu for commands. Supports interactive, background and worker pools.
- SFTP remote panel browsing. Parses ~/.ssh/config for quick access.
- File chooser mode (`--chooser-file`) for editor integration (e.g. Helix).
- Subshell, ability to send selected filenames into the subshell.

### Outscoped compared to MC

- No EXT2 recovery.
- No shell link.

## Requirements

- Go 1.26+
- Linux
- NerdFont or something similar, no fallback implemented yet.

## Install

Install the `pc` binary into `$GOBIN` if that environment variable is set; otherwise into `$(go env GOPATH)/bin` (often `~/go/bin`). Put that directory on your `PATH`.

```bash
go install github.com/paranoidi/paras-commander/cmd/pc@latest
```

To install from a specific version tag or commit, replace `@latest` with that ref (for example `@v0.1.0` or `@main`).

## Build

```bash
go build ./cmd/pc
```

This produces a `pc` binary in the repo root.

## Run

After a local `go build` (binary in the current directory):

```bash
./pc
```

With test helpers (Dev pulldown menu for status toasts):

```bash
./pc -dev
```

After `go install`, with your Go `bin` directory on `PATH`:

```bash
pc
```

## File chooser (Helix)

Inspiration taken from yazi, the filemanager you should check out.

```bash
pc --chooser-file=/path/to/output-file [--select=PATH] [--no-carousel]
```

- **`--chooser-file`** — when you press **Enter** on a regular file, `pc` writes
  its absolute path (with a trailing newline) to this file and exits.
  Directories are opened as usual; only files complete the pick.
- **`--select`** — open at a file or directory and highlight it
  (e.g. Helix `%{buffer_name}` for the active buffer). For a file, `pc` opens
  its parent directory with that file selected. If the path does not exist yet,
  the parent directory is opened and the basename is selected when present in the listing.
- **Carousel view** on the left panel (parent | current | child columns)
  is **on by default** in chooser mode. Pass **`--no-carousel`** to use a plain
  single-column list instead.
- **Quick view** (Shift+F3) is **on by default** in chooser mode so the inactive
  panel previews the highlighted file.
- Quit without selecting leaves the output file unchanged — clear it
  before each invocation.

Example Helix keybinding (requires a Helix build with command extensions):

```toml
[keys.normal]
C-p = [
  ':sh rm -f /tmp/pc-chooser',
  ':insert-output pc -select "%{buffer_name}" -chooser-file=/tmp/pc-chooser',
  ':sh printf "\x1b[?1049h\x1b[?2004h" > /dev/tty',
  ':open %sh{cat /tmp/pc-chooser}',
  ':redraw',
]
```

## Test

```bash
go test ./...
```

## Lint

Install [golangci-lint](https://golangci-lint.run/welcome/install/) (v2), then:

```bash
golangci-lint run ./...
```

## Configuration

Paras Commander reads a TOML config file. On first run a default config is generated. Themes live in the `themes/` directory.
