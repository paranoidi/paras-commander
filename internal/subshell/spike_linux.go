//go:build linux

package subshell

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
	"github.com/gdamore/tcell/v2"
	"golang.org/x/term"
)

// Spike is a Phase 0 PTY prototype: one long-lived shell child and Suspend/Resume visible sessions.
type Spike struct {
	cmd   *exec.Cmd
	pty   *os.File
	dead  chan struct{}
	mu    sync.Mutex
	alive bool
	wait  sync.WaitGroup
}

// StartOptions configures [StartSpike].
type StartOptions struct {
	// Shell is the executable path; empty uses $SHELL, then /bin/sh.
	Shell string
	// Dir is the child working directory (empty leaves unset).
	Dir string
	// Command, when set, runs shell -c Command instead of an interactive shell -i.
	// Tests use this with a small stub script.
	Command string
}

// StartSpike forks a shell on a PTY master returned internally via [Spike.PTYFd].
func StartSpike(opts StartOptions) (*Spike, error) {
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

	s := &Spike{
		cmd:   cmd,
		pty:   ptmx,
		dead:  make(chan struct{}),
		alive: true,
	}
	s.wait.Add(1)
	go func() {
		defer s.wait.Done()
		defer close(s.dead)
		_ = cmd.Wait()
		s.mu.Lock()
		s.alive = false
		s.mu.Unlock()
	}()
	return s, nil
}

// Alive reports whether the shell child is still running.
func (s *Spike) Alive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.alive
}

// PTYFd returns the master PTY fd for tests and later phases.
func (s *Spike) PTYFd() int {
	return int(s.pty.Fd())
}

// WritePTY writes bytes to the shell (visible or not).
func (s *Spike) WritePTY(b []byte) (int, error) {
	if !s.Alive() {
		return 0, ErrNotAlive
	}
	return s.pty.Write(b)
}

// RunVisible releases the TUI via Suspend, runs the PTY feed until Ctrl+O, then Resumes tcell.
func (s *Spike) RunVisible(screen tcell.Screen) (toggledBack bool, err error) {
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

// RunVisibleFeed is like [Spike.RunVisible] but uses explicit streams (tests).
func (s *Spike) RunVisibleFeed(in io.Reader, out io.Writer) (bool, error) {
	if !s.Alive() {
		return false, ErrNotAlive
	}
	return runVisibleFeed(s.pty, in, out, s.dead)
}

// Close shuts the PTY and waits for the child exit waiter started in [StartSpike].
func (s *Spike) Close() error {
	if err := s.pty.Close(); err != nil {
		return err
	}
	s.wait.Wait()
	return nil
}
