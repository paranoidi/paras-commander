// Package privexec runs one fixed argv as root through sudo, feeding the password on stdin.
// It never uses a shell, never logs the password and never puts it in argv or the environment.
package privexec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ErrBadPassword means sudo rejected the supplied password.
var ErrBadPassword = errors.New("incorrect sudo password")

// command builds the sudo process; tests replace it so no real sudo runs.
var command = exec.CommandContext

// Argv returns the sudo invocation for argv. With a password sudo reads it from stdin (-S),
// ignores any cached credential (-k) and prints no prompt (-p ""); without one it must not ask (-n).
func Argv(withPassword bool, argv []string) []string {
	head := []string{"sudo", "-n", "--"}
	if withPassword {
		head = []string{"sudo", "-S", "-k", "-p", "", "--"}
	}
	return append(head, argv...)
}

// Run executes argv as root, authenticating with password. Wrong password -> ErrBadPassword.
func Run(ctx context.Context, password string, argv ...string) error {
	return run(ctx, true, password, argv)
}

// RunNoPassword executes argv as root without any password (sudo -n).
func RunNoPassword(ctx context.Context, argv ...string) error {
	return run(ctx, false, "", argv)
}

// NoPassword reports whether sudo would run commands without asking for a password.
func NoPassword(ctx context.Context) bool {
	return run(ctx, false, "", []string{"true"}) == nil
}

func run(ctx context.Context, withPassword bool, password string, argv []string) error {
	if len(argv) == 0 {
		return errors.New("privexec: empty argv")
	}
	full := Argv(withPassword, argv)
	cmd := command(ctx, full[0], full[1:]...)
	// C locale so the stderr phrases classify() looks for are not translated.
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANGUAGE=C")
	if withPassword {
		cmd.Stdin = strings.NewReader(password + "\n")
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return classify(err, stderr.String(), argv[0])
	}
	return nil
}

func classify(err error, stderr, tool string) error {
	low := strings.ToLower(stderr)
	if strings.Contains(low, "incorrect password") || strings.Contains(low, "sorry, try again") ||
		strings.Contains(low, "no password was provided") {
		return ErrBadPassword
	}
	if msg := strings.TrimSpace(stderr); msg != "" {
		return fmt.Errorf("%s as root: %s", tool, msg)
	}
	return fmt.Errorf("%s as root: %w", tool, err)
}
