package cmdrun

import (
	"fmt"
	"sync/atomic"
)

var runIDC atomic.Uint64

// NewRunID returns a unique command-run identifier for the session.
func NewRunID() string {
	n := runIDC.Add(1)
	return fmt.Sprintf("cmd-%d", n)
}
