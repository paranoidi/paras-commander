//go:build linux

package subshell

import (
	"fmt"
	"os"
)

func debugLog(format string, args ...any) {
	if os.Getenv("SUBSHELL_DEBUG") == "" {
		return
	}
	fmt.Fprintf(os.Stderr, "subshell: "+format+"\n", args...)
}
