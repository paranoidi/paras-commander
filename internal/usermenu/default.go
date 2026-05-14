package usermenu

// DefaultMenuTOML is used when no menu.toml is found on disk.
const DefaultMenuTOML = `shell_patterns = 1

[[entry]]
key = "p"
title = "Print working directory (requires pwd on PATH)"
command = "pwd"
default = true

[[entry]]
key = "e"
title = "Echo active panel directory"
command = "echo %d"
`
