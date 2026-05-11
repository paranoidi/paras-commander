package themes

import "embed"

// Files contains built-in TOML theme definitions.
//
//go:embed *.toml
var Files embed.FS
