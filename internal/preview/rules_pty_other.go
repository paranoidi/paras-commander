//go:build !linux

package preview

import (
	"context"

	"github.com/paranoidi/paras-commander/internal/cmdrun"
)

// runRuleCommandCapture falls back to cmdrun.Run's plain-pipe execution on platforms without
// the PTY-backed terminal-query-answering path (see rules_pty_linux.go): preview commands still
// run and text output still shows, they just can't self-detect graphics support here.
func runRuleCommandCapture(ctx context.Context, argv []string, dir string, maxBytes int, sixelOK bool) cmdrun.RunResult {
	_ = sixelOK
	return cmdrun.Run(ctx, argv, dir, maxBytes)
}
