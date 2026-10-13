---
name: add-config-option
description: Add a new config.toml option with a single-source built-in default
---

# Add a config option

One source of truth for defaults: exported consts in `internal/config/builtin.go`, wired through `config.Default()`. Never scatter magic values.

## Steps

1. **Built-in default**: add an exported const to `internal/config/builtin.go` (with clamp consts if the value needs validation bounds). Other packages (`internal/ops`, `internal/diskusage`, …) read the value via `config` — they must not define parallel numbers. `internal/config` must never import `internal/ops` (import cycle).
2. **Config struct**: add the TOML field to the right table struct in `internal/config`, seed it in `config.Default()`, and clamp/validate in `Config.Validate` (follow existing patterns, e.g. `key_repeat_debounce_ms` clamps).
3. **Stub**: `pc --config-stub` writes `config.toml` via `config.EncodeDefaultStub` — confirm the new field appears (test `TestEncodeDefaultStubWritesLoadableDefaults` exercises round-tripping). Shortcut tables never go in `config.toml`; they live in `keybindings.toml` (unknown fields fail `config.LoadFromPaths`).
4. **Options dialog (only if user-facing toggle)**: `internal/ui/dialog/config_dialog_render.go` + handlers in `internal/app/app_settings_dialogs.go`; persist on OK via the existing partial-persist path. Alt mnemonics must stay unique per dialog (O/C reserved for OK/Cancel).
5. **Verify**: `golangci-lint run ./...`, `go test ./...`; document the option's behavior and default in the matching `docs/agents/` file and in `docs/config.md`.
