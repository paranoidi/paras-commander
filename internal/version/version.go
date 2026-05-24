package version

import (
	"fmt"
	"runtime/debug"
)

const devVersion = "dev"

// String returns the module version baked into the binary (go build / go install),
// or "dev" when build info is missing or reports (devel).
func String() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return devVersion
}

// Line returns a single-line version string suitable for -v / --version output.
func Line() string {
	return fmt.Sprintf("pc %s", String())
}
