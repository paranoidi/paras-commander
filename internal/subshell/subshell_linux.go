//go:build linux

package subshell

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
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
	ptyFD int // cached int(pty.Fd()) — File.Fd() flips the fd back to blocking mode on every
	// call (see PanelFeed.run's re-arm comment), so Busy()/PTYFd() must not call it fresh
	// while a PanelFeed reader may be concurrently relying on the fd staying nonblocking.
	dead    chan struct{}
	mu      sync.Mutex
	alive   bool
	cwdPipe string // last $PWD reported by the shell's prompt hook (empty until first prompt)
	// cwdGen bumps on every line the prompt hook reports (even an unchanged $PWD, e.g. a
	// failed cd) — Chdir waits on cwdCond for a fresh generation instead of polling.
	cwdGen        int
	cwdCond       *sync.Cond
	hasPrecmdHook bool // true once Start actually wrote a prompt hook (see precmdInit)
	feed          *PanelFeed
	wait          sync.WaitGroup
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
	// ignoreboth keeps the injected leading-space cd/precmd lines out of bash history (MC does
	// the same). ponytail: zsh needs setopt HIST_IGNORE_SPACE from the user; fish skips natively.
	cmd.Env = append(os.Environ(), "HISTCONTROL=ignoreboth")

	// The shell's prompt hook reports $PWD on child fd 3 (MC's subshell_pipe mechanism).
	pipeR, pipeW, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("subshell: cwd pipe: %w", err)
	}
	cmd.ExtraFiles = []*os.File{pipeW}

	ptmx, err := pty.Start(cmd)
	_ = pipeW.Close() // child holds its dup; parent must not keep the write end open
	if err != nil {
		_ = pipeR.Close()
		return nil, fmt.Errorf("subshell: start pty: %w", err)
	}
	if term.IsTerminal(int(os.Stdout.Fd())) {
		_, _ = syncPTYSize(ptmx, os.Stdout)
	}

	s := &Subshell{
		cmd:   cmd,
		pty:   ptmx,
		ptyFD: int(ptmx.Fd()),
		dead:  make(chan struct{}),
		alive: true,
	}
	s.cwdCond = sync.NewCond(&s.mu)
	s.wait.Go(func() {
		defer close(s.dead)
		_ = cmd.Wait()
		s.mu.Lock()
		s.alive = false
		s.mu.Unlock()
	})
	s.wait.Go(func() {
		// Ends on EOF when the child (sole write-end holder) exits.
		defer func() { _ = pipeR.Close() }()
		scanner := bufio.NewScanner(pipeR)
		for scanner.Scan() {
			if line := strings.TrimSpace(scanner.Text()); line != "" {
				s.mu.Lock()
				s.cwdPipe = line
				s.cwdGen++
				s.cwdCond.Broadcast()
				s.mu.Unlock()
			}
		}
	})

	if init := precmdInit(shell); init != "" && opts.Command == "" {
		if _, err := ptmx.WriteString(init); err == nil {
			// Absorb the init line's echo so the first toggle shows a clean prompt, then
			// request a fresh one. ponytail: quiet-window heuristic; MC feeds until prompt.
			absorbPTYUntilQuiet(ptmx, 100*time.Millisecond, time.Second)
			_, _ = ptmx.WriteString("\n")
			s.hasPrecmdHook = true
		}
	}
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
	return s.ptyFD
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

	// Sole-reader invariant: park the panel feed's PTY reader for the whole visible
	// session; tee the feed loop's output into its emulator so panel state stays
	// current. The reader restarts on every exit path (defer runs last); the app
	// restores panel dims afterwards via PanelFeed.Resize.
	out := io.Writer(os.Stdout)
	if feed := s.panelFeed(); feed != nil {
		feed.Pause()
		defer feed.startReader()
		out = io.MultiWriter(os.Stdout, feed.teeWriter())
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

	resized, _ := syncPTYSize(s.pty, hostTTY)
	if feed := s.panelFeed(); feed != nil {
		feed.syncVTToPTY()
		// The emulator is the source of truth for the shell's screen (the real
		// terminal still shows whatever the previous visible session left).
		// Replay it in every case except busy+resized, where the size-change
		// WINCH already makes the full-screen app repaint itself.
		if !s.Busy() || !resized {
			_ = feed.WriteScreenTo(hostTTY)
		}
	} else if s.Busy() && !resized {
		forceFullRedraw(s.pty)
	}
	stopResize := watchWinchResize(s.pty, hostTTY)
	defer stopResize()

	toggledBack, err = runVisibleFeed(s.pty, hostTTY, out, s.dead)
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
	// Park the panel reader first — a reader racing the PTY close is a use-after-close.
	if feed := s.panelFeed(); feed != nil {
		feed.Close()
	}
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
