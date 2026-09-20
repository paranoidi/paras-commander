//go:build linux

package preview

import (
	"context"
	"errors"
	"os/exec"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
	"golang.org/x/term"

	"github.com/paranoidi/paras-commander/internal/cmdrun"
)

// ptySlaveDrain is how long the PTY master stays open after the primary process
// exits so remaining output can be read. A descendant that inherited the slave
// cannot pin the capture goroutine past this window — same role as
// cmdrun.RunTracked's WaitDelay.
const ptySlaveDrain = 200 * time.Millisecond

// runRuleCommandCapture runs argv with a real PTY attached to its stdin/stdout/stderr instead
// of cmdrun.Run's plain pipes, and answers a small set of terminal capability queries
// (terminalQueryScanner) on its behalf. Some preview tools (observed: movie-info) probe the
// terminal with DA1 then CPR before deciding whether to draw a Sixel/Kitty image; against a
// plain pipe (stdin /dev/null, stdout unconnected to any terminal) nothing ever answers those
// queries and the tool silently falls back to text — this gives it a real, if minimal,
// terminal to talk to instead. A PTY has one combined stream, so Stderr is always empty in the
// result; capture is capped at maxBytes the same way cmdrun.Run caps its pipes (tail bytes kept,
// StdoutTrim set). After the primary process exits (or ctx cancel kills it), the master is
// closed past a short drain so a descendant that inherited the slave cannot pin the read.
func runRuleCommandCapture(ctx context.Context, argv []string, dir string, maxBytes int, sixelOK bool) cmdrun.RunResult {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir

	ptmx, tty, err := pty.Open()
	if err != nil {
		return cmdrun.RunResult{LaunchErr: err, ExitCode: -1}
	}
	defer func() { _ = ptmx.Close() }()
	if err := pty.Setsize(ptmx, &pty.Winsize{Rows: 24, Cols: 80}); err != nil {
		_ = tty.Close()
		return cmdrun.RunResult{LaunchErr: err, ExitCode: -1}
	}
	// A fresh pty starts in canonical (line-buffered, echoing) mode with output post-processing:
	// a byte-oriented read by the child (e.g. dd reading our DA1 reply) would never see it until
	// a newline shows up, echo would mirror our synthetic replies back into the very stream
	// we're capturing, and ONLCR would turn the child's "\n" into "\r\n". Raw mode fixes all
	// three, and must be set before the child starts so none of its output goes through the
	// cooked settings. Master and slave share one termios, so setting it via the master applies
	// to the child's end too; the prior state is never restored since this pty is discarded
	// (not reused) once the command exits.
	_, _ = term.MakeRaw(int(ptmx.Fd()))
	cmd.Stdin, cmd.Stdout, cmd.Stderr = tty, tty, tty
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}
	err = cmd.Start()
	_ = tty.Close()
	if err != nil {
		return cmdrun.RunResult{LaunchErr: err, ExitCode: -1}
	}

	scanner := &terminalQueryScanner{sixelOK: sixelOK}
	captured := cmdrun.CappedWriter{Max: maxBytes}

	// Wait in a goroutine so we can keep answering DA1/CPR while the primary runs,
	// then stop once it exits (or CommandContext kills it). A descendant that
	// inherited the slave keeps the master readable, so a blocking ptmx.Read after
	// primary exit never returns — unlike cmdrun.RunTracked, whose WaitDelay closes
	// pipes. Closing the master while Read is blocked also deadlocks (Go's os.File
	// serializes Close behind the outstanding read), so we poll with a timeout
	// instead, matching the subshell PTY readers.
	waitErrCh := make(chan error, 1)
	go func() { waitErrCh <- cmd.Wait() }()

	ptyFD := int(ptmx.Fd())
	_ = unix.SetNonblock(ptyFD, true)
	buf := make([]byte, 32*1024)
	pfd := []unix.PollFd{{Fd: int32(ptyFD), Events: unix.POLLIN}}

	var (
		waitErr    error
		waitDone   bool
		drainUntil time.Time
	)
	for {
		timeout := 100 * time.Millisecond
		if waitDone {
			rem := time.Until(drainUntil)
			if rem <= 0 {
				break
			}
			timeout = rem
		}
		timeoutMs := int(timeout / time.Millisecond)
		if timeoutMs < 1 {
			timeoutMs = 1
		}
		pfd[0].Revents = 0
		nready, perr := unix.Poll(pfd, timeoutMs)
		// File.Fd() and some poll paths flip the fd back to blocking; re-arm every
		// wakeup so an idle inherited slave cannot pin the next read.
		_ = unix.SetNonblock(ptyFD, true)
		if perr != nil && !errors.Is(perr, unix.EINTR) {
			break
		}
		if !waitDone {
			select {
			case waitErr = <-waitErrCh:
				waitDone = true
				drainUntil = time.Now().Add(ptySlaveDrain)
			default:
			}
		}
		if nready > 0 && pfd[0].Revents&(unix.POLLIN|unix.POLLHUP|unix.POLLERR) != 0 {
			n, rErr := unix.Read(ptyFD, buf)
			if n > 0 {
				clean, reply := scanner.Scan(buf[:n])
				if len(reply) > 0 {
					_, _ = unix.Write(ptyFD, reply)
				}
				_, _ = captured.Write(clean)
			}
			if rErr != nil && !errors.Is(rErr, unix.EAGAIN) && !errors.Is(rErr, unix.EINTR) {
				// Last slave closed: Linux returns EIO on the master, not io.EOF.
				break
			}
		}
	}
	if !waitDone {
		waitErr = <-waitErrCh
	}
	_, _ = captured.Write(scanner.Flush())
	// captured.Data, not .Bytes(): runRuleCommand discards the result outright whenever
	// StdoutTrim is set (a partial escape sequence can't be trusted), so there's no reason to pay
	// for .Bytes()'s truncation-marker suffix/copy here.
	res := cmdrun.RunResult{Stdout: captured.Data, StdoutTrim: captured.Trimmed, ExitCode: -1}
	var exitErr *exec.ExitError
	switch {
	case waitErr == nil:
		res.ExitCode = 0
	case errors.As(waitErr, &exitErr):
		res.ExitCode = exitErr.ExitCode()
	default:
		res.LaunchErr = waitErr
	}
	return res
}
