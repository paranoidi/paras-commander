# Built-in defaults live in internal/config, which never imports ops

All numeric/operational defaults are exported consts in `internal/config/builtin.go` wired through `config.Default()`; `internal/ops` and others read them from `config`. `internal/config` must not import `internal/ops` (import cycle), so ops-specific defaults live on the config side rather than next to the code that uses them.
