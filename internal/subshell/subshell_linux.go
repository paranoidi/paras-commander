//go:build linux

package subshell

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/gdamore/tcell/v2"
	"golang.org/x/term"
)

// Subshell is a persistent PTY-backed shell: one long-lived shell child and Suspend/Resume visible sessions.
type Subshell struct {
	cmd   *exec.Cmd
	pty   *os.File
	dead  chan struct{}
	mu    sync.Mutex
	alive bool
	wait  sync.WaitGroup
}

// StartOptions configures [Start].
type StartOptions struct {
	// Shell is the executable path; empty uses $SHELL, then /bin/sh.
	Shell string
	// Dir is the child working directory (empty leaves unset).
	Dir string
	// Command, when set, runs shell -c Command instead of an interactive shell -i.
	// Tests use this with a small stub script.
	Command string
}

// Start forks a shell on a PTY master returned internally via [Subshell.PTYFd].
func Start(opts StartOptions) (*Subshell, error) {
	shell := opts.Shell
	if shell == "" {
		shell = os.Getenv("SHELL")
	}
	if shell == "" {
		shell = "/bin/sh"
	}

	var cmd *exec.Cmd
	switch {
	case opts.Command != "":
		cmd = exec.Command(shell, "-c", opts.Command)
	default:
		cmd = exec.Command(shell, "-i")
	}
	if opts.Dir != "" {
		cmd.Dir = opts.Dir
	}
	cmd.Env = os.Environ()

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("subshell: start pty: %w", err)
	}
	if term.IsTerminal(int(os.Stdout.Fd())) {
		_ = syncPTYSize(ptmx, os.Stdout)
	}

	s := &Subshell{
		cmd:   cmd,
		pty:   ptmx,
		dead:  make(chan struct{}),
		alive: true,
	}
	s.wait.Go(func() {
		defer close(s.dead)
		_ = cmd.Wait()
		s.mu.Lock()
		s.alive = false
		s.mu.Unlock()
	})
	return s, nil
}

// Alive reports whether the shell child is still running.
func (s *Subshell) Alive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.alive
}

// PTYFd returns the master PTY fd for tests and later phases.
func (s *Subshell) PTYFd() int {
	return int(s.pty.Fd())
}

// WritePTY writes bytes to the shell (visible or not).
func (s *Subshell) WritePTY(b []byte) (int, error) {
	if !s.Alive() {
		return 0, ErrNotAlive
	}
	return s.pty.Write(b)
}

// RunVisible releases the TUI via Suspend, runs the PTY feed until Ctrl+O, then Resumes tcell.
func (s *Subshell) RunVisible(screen tcell.Screen) (toggledBack bool, err error) {
	if !s.Alive() {
		return false, ErrNotAlive
	}

	hostTTY, err := openControllingTTY()
	if err != nil {
		return false, fmt.Errorf("subshell: open /dev/tty: %w", err)
	}
	defer func() {
		if hostTTY != nil {
			_ = hostTTY.Close()
		}
	}()

	if err := screen.Suspend(); err != nil {
		return false, fmt.Errorf("subshell: suspend terminal: %w", err)
	}

	resumed := false
	hostRestored := false
	defer func() {
		if !hostRestored {
			if restore := takeVisibleRestore(); restore != nil {
				restore()
			}
		}
		if !resumed {
			_ = screen.Resume()
			enableKittyKeyboardProtocol()
		}
		screen.Sync()
	}()

	hostFD := int(hostTTY.Fd())
	restoreHost, rawErr := enterHostRawOn(hostFD)
	if rawErr != nil {
		return false, rawErr
	}
	registerVisibleRestore(restoreHost)

	_ = syncPTYSize(s.pty, hostTTY)
	stopResize := watchWinchResize(s.pty, hostTTY)
	defer stopResize()

	toggledBack, err = runVisibleFeed(s.pty, hostTTY, os.Stdout, s.dead)
	if err != nil {
		return toggledBack, err
	}

	// Restore cooked mode once, then Resume. Do not defer restoreHost — a second restore
	// after Resume leaves the terminal cooked while tcell is engaged (hang).
	restoreHost()
	hostRestored = true
	clearVisibleRestore()

	discardPendingStdin(hostFD)
	// Close before Resume: a second open /dev/tty blocks tcell engage on the same terminal.
	_ = hostTTY.Close()
	hostTTY = nil

	debugLog("resuming tcell after shell-visible")
	if resumeErr := screen.Resume(); resumeErr != nil {
		return toggledBack, fmt.Errorf("subshell: resume terminal: %w", resumeErr)
	}
	resumed = true
	enableKittyKeyboardProtocol()
	screen.Sync()
	debugLog("tcell resumed")
	return toggledBack, nil
}

// RunVisibleFeed is like [Subshell.RunVisible] but uses explicit streams (tests).
func (s *Subshell) RunVisibleFeed(in io.Reader, out io.Writer) (bool, error) {
	if !s.Alive() {
		return false, ErrNotAlive
	}
	return runVisibleFeed(s.pty, in, out, s.dead)
}

// Close terminates the shell child and releases the PTY. Closing only the master is not
// enough: an interactive shell may never see SIGHUP while a reader still holds the fd, and
// cmd.Wait would block forever. SIGHUP first (lets the shell save history), SIGKILL after a
// short grace. ponytail: fixed 500ms grace; make it configurable if a shell needs longer.
func (s *Subshell) Close() error {
	if s.Alive() {
		_ = s.cmd.Process.Signal(syscall.SIGHUP)
		select {
		case <-s.dead:
		case <-time.After(500 * time.Millisecond):
			_ = s.cmd.Process.Kill()
		}
	}
	s.wait.Wait()
	return s.pty.Close()
}
