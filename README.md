# Paras Commander

A Linux terminal twin-panel file manager inspired by Midnight Commander, written in Go using a TUI.

> ⚠️ **Not yet stable or usable.** Copy/move commands may cause data loss.

## Requirements

- Go 1.26+
- Linux

## Build

```bash
go build ./cmd/pc
```

This produces a `pc` binary in the repo root.

## Run

```bash
./pc
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
