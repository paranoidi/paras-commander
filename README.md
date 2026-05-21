# Paras Commander

A Linux terminal twin-panel file manager inspired by Midnight Commander and fzf, written in Go using a TUI.

> ⚠️ **Not yet stable** Use at your own risk.

## Features

- Practically everything is based on fzf style searching. Filelist navigation, keybind help, etc.
- All jobs are queued into background by default.
- Jobs view with queue management and transfer graphs.
- Gitignore and git statuses using eza notation.
- Integrated disk usage scanning and reporting.
- Supports selections across multiple paths.
- Integrated mass rename.
- Rename supports slugify and clean.
- Read bookmarks from ~/.fzf-marks and gnome gtk files. Writes bookmarks to ~/.fzf-marks.
- Find files / paths recursively.
- Meta column can be used to provide data from external commands.
- Execute command for selected files.
- SFTP remote panel browsing.

### Outscoped compared to MC

- No EXT2 recovery.
- No embedded shell (FISH/subshell).
- No shell link.

## Requirements

- Go 1.26+
- Linux

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
