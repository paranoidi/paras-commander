package shell

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"

	"github.com/paranoidi/paras-commander/internal/cmdrun"
)

const defaultShell = "/bin/bash"

// ResolveShell returns $SHELL when set, otherwise the first bash on PATH, then /bin/bash.
func ResolveShell() string {
	if v := strings.TrimSpace(os.Getenv("SHELL")); v != "" {
		return v
	}
	if p, err := exec.LookPath("bash"); err == nil {
		return p
	}
	return defaultShell
}

// ShellArgv returns argv to run an interactive shell session.
func ShellArgv(shell string) []string {
	if strings.TrimSpace(shell) == "" {
		shell = ResolveShell()
	}
	return []string{shell}
}

// RunInteractive runs argv with stdin/stdout/stderr attached to the terminal.
// The process inherits the current working directory of the parent.
// A non-zero shell exit status is not reported as an error.
func RunInteractive(ctx context.Context, argv []string) error {
	if len(argv) == 0 {
		return errors.New("empty argv")
	}
	if err := cmdrun.RunInteractive(ctx, argv, ""); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil
		}
		return err
	}
	return nil
}
