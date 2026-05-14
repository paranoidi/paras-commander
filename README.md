# Paras Commander

A Linux terminal twin-panel file manager inspired by Midnight Commander, written in Go using a TUI.

> ⚠️ **Not yet stable or usable.** Copy/move commands may cause data loss.

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
