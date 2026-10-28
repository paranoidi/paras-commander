# Paras Commander

A Linux terminal twin-panel file manager inspired by Midnight Commander and fzf, written in Go using a TUI.

⚠️ **Relatively stable** Use at your own risk.

📖 **Documentation: <https://paranoidi.github.io/paras-commander/>** (sources in `docs/`; `task docs:serve` browses it locally, needs `uv`).

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
- Meta column(s) can be used to provide data from external commands. Panels can sort by an active meta column (numeric-aware), up to 4 shown in the Sort dialog.
- Execute command for selected files.
- User menu for commands. Supports interactive, background and worker pools.
- SFTP remote panel browsing. Parses ~/.ssh/config for quick access.
- File chooser mode (`--chooser-file`) for editor integration (e.g. Helix).
- Subshell, ability to send selected filenames into the subshell.

## Requirements

- Go 1.26+
- Linux
- NerdFont or something similar, no fallback implemented yet across whole application.

## Install

Install the `pc` binary into `$GOBIN` if that environment variable is set; otherwise into `$(go env GOPATH)/bin` (often `~/go/bin`). Put that directory on your `PATH`.

```bash
go install github.com/paranoidi/paras-commander/cmd/pc@latest
```

To install from a specific version tag or commit, replace `@latest` with that ref (for example `@v0.1.0` or `@main`).

## Build

```bash
go build -o pc ./cmd/pc
```

This produces a `pc` binary in the repo root. The artifact must be named `pc`, not `paras-commander`.

## Run

```bash
pc [left-dir] [right-dir]
```

Command-line flags, file arguments and first-run setup: [Getting started](https://paranoidi.github.io/paras-commander/getting-started.html).
Editor integration (Helix file picker): [Chooser mode](https://paranoidi.github.io/paras-commander/chooser-mode.html).

## Test and lint

CI is **golangci-lint v2.12.1** plus **`go test -race ./...`**. Locally the race suite is opt-in (`task race`); the local gate runs plain `go test ./...`. A local `pc` artifact is **`go build -o pc ./cmd/pc`**. `govulncheck` is optional and is not a merge gate.

Fast inner loop (no race detector):

```bash
go test ./...
# or: task test
```

Pre-push (CI adds the race detector, plus the local `pc` binary):

```bash
golangci-lint run ./...
go test ./...
go build -o pc ./cmd/pc
# or: task verify
```

Requires [go-task](https://taskfile.dev) for the Taskfile targets. Install tools into `./bin`, then lint:

```bash
task setup   # install golangci-lint v2.12.1, errcheck, ineffassign, gocyclo into ./bin
task lint    # golangci-lint (includes errcheck, ineffassign, gocyclo)
task race    # go test -race ./...
task verify  # lint + test + go build -o pc ./cmd/pc
```

`task errcheck`, `task ineffassign`, and `task gocyclo` are optional debug helpers; `task lint` already covers them. `task govulncheck` is optional.

Or, if [golangci-lint](https://golangci-lint.run/welcome/install/) **v2.12.1** is already on your PATH:

```bash
golangci-lint run ./...
```

## Configuration

Paras Commander reads a TOML config file. On first run a default config is generated. Themes live in the `themes/` directory.

See [Configuration](https://paranoidi.github.io/paras-commander/config.html) for the full list of config.toml keys, defaults, and descriptions.
