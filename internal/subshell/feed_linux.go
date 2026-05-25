//go:build linux

package subshell

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// runVisibleFeed multiplexes in→PTY and PTY→out until Ctrl+O or child exit.
func runVisibleFeed(ptyMaster *os.File, in io.Reader, out io.Writer, dead <-chan struct{}) (bool, error) {
	inFD, ok := in.(*os.File)
	if !ok {
		return runVisibleFeedFallback(ptyMaster, in, out, dead)
	}
	return runVisiblePoll(ptyMaster, inFD, out, dead)
}

func runVisiblePoll(ptyMaster *os.File, in *os.File, out io.Writer, dead <-chan struct{}) (bool, error) {
	pumpPTYAvailable(ptyMaster, out)

	inFD := int(in.Fd())
	ptyFD := int(ptyMaster.Fd())
	_ = unix.SetNonblock(inFD, true)
	defer func() { _ = unix.SetNonblock(inFD, false) }()
	_ = unix.SetNonblock(ptyFD, true)
	defer func() { _ = unix.SetNonblock(ptyFD, false) }()

	pending := make([]byte, 0, 256)
	var pendingAt time.Time
	inBuf := make([]byte, 4096)
	ptyBuf := make([]byte, 4096)
	pfds := []unix.PollFd{
		{Fd: int32(inFD), Events: unix.POLLIN},
		{Fd: int32(ptyFD), Events: unix.POLLIN},
	}

	for {
		select {
		case <-dead:
			drainPTYToWriter(ptyMaster, out, 100*time.Millisecond)
			return false, nil
		default:
		}

		if len(pending) > 0 && !pendingAt.IsZero() && time.Since(pendingAt) > 50*time.Millisecond {
			if _, err := ptyMaster.Write(pending); err != nil {
				return false, err
			}
			pending = nil
			pendingAt = time.Time{}
		}

		for i := range pfds {
			pfds[i].Revents = 0
		}
		_, err := unix.Poll(pfds, 100)
		if err != nil && !errors.Is(err, unix.EINTR) {
			return false, err
		}

		if pfds[1].Revents&(unix.POLLIN|unix.POLLHUP) != 0 {
			for {
				n, readErr := unix.Read(ptyFD, ptyBuf)
				if n > 0 {
					if _, werr := out.Write(ptyBuf[:n]); werr != nil {
						return false, werr
					}
				}
				if n <= 0 || errors.Is(readErr, unix.EAGAIN) {
					break
				}
			}
		}

		if pfds[0].Revents&unix.POLLIN != 0 {
			for {
				n, readErr := unix.Read(inFD, inBuf)
				if n > 0 {
					var toggle bool
					var procErr error
					pending, toggle, procErr = processInputToPTY(pending, inBuf[:n], ptyMaster)
					if procErr != nil {
						return false, procErr
					}
					if len(pending) > 0 && pendingAt.IsZero() {
						pendingAt = time.Now()
					}
					if len(pending) == 0 {
						pendingAt = time.Time{}
					}
					if toggle {
						debugLog("toggle detected, leaving shell-visible")
						drainPTYToWriter(ptyMaster, out, 150*time.Millisecond)
						discardPendingStdin(inFD)
						return true, nil
					}
				}
				if n <= 0 || errors.Is(readErr, unix.EAGAIN) {
					break
				}
				if readErr != nil {
					return false, readErr
				}
			}
		}
	}
}

func processInputToPTY(pending, chunk []byte, ptyMaster *os.File) (newPending []byte, toggled bool, err error) {
	pending = append(pending, chunk...)
	for len(pending) > 0 {
		if at, _, ok := FindToggle(pending); ok {
			if at > 0 {
				if _, err := ptyMaster.Write(pending[:at]); err != nil {
					return pending, false, err
				}
			}
			return nil, true, nil
		}
		flush := SafePTYFlushLen(pending)
		if flush == 0 {
			return pending, false, nil
		}
		if _, err := ptyMaster.Write(pending[:flush]); err != nil {
			return pending, false, err
		}
		pending = pending[flush:]
	}
	return pending, false, nil
}

func drainPTYToWriter(ptyMaster *os.File, out io.Writer, maxWait time.Duration) {
	if out == io.Discard {
		return
	}
	deadline := time.Now().Add(maxWait)
	buf := make([]byte, 4096)
	ptyFD := int(ptyMaster.Fd())
	for time.Now().Before(deadline) {
		n, err := unix.Read(ptyFD, buf)
		if n > 0 {
			_, _ = out.Write(buf[:n])
		}
		if n <= 0 || errors.Is(err, unix.EAGAIN) {
			return
		}
	}
}

func pumpPTYAvailable(ptyMaster *os.File, out io.Writer) {
	if out == io.Discard {
		return
	}
	ptyFD := int(ptyMaster.Fd())
	_ = unix.SetNonblock(ptyFD, true)
	defer func() { _ = unix.SetNonblock(ptyFD, false) }()
	buf := make([]byte, 4096)
	for {
		n, err := unix.Read(ptyFD, buf)
		if n > 0 {
			_, _ = out.Write(buf[:n])
		}
		if n <= 0 || errors.Is(err, unix.EAGAIN) {
			return
		}
	}
}

// runVisibleFeedFallback is used when stdin is not an *os.File (unit tests with bytes.Reader).
func runVisibleFeedFallback(ptyMaster *os.File, in io.Reader, out io.Writer, dead <-chan struct{}) (bool, error) {
	if buf, ok, err := readStaticInput(in); ok {
		if err != nil {
			return false, err
		}
		_, toggled, err := processInputToPTY(nil, buf, ptyMaster)
		if err != nil {
			return false, err
		}
		if toggled {
			drainPTYToWriter(ptyMaster, out, 100*time.Millisecond)
			return true, nil
		}
		return false, nil
	}

	pending := make([]byte, 0, 256)
	inBuf := make([]byte, 4096)
	ptyBuf := make([]byte, 4096)

	for {
		if dead != nil {
			select {
			case <-dead:
				drainPTYToWriter(ptyMaster, out, 200*time.Millisecond)
				return false, nil
			default:
			}
		}

		_ = ptyMaster.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
		n, ptyErr := ptyMaster.Read(ptyBuf)
		_ = ptyMaster.SetReadDeadline(time.Time{})
		if n > 0 {
			if _, werr := out.Write(ptyBuf[:n]); werr != nil {
				return false, werr
			}
		}
		if ptyErr != nil && !errors.Is(ptyErr, os.ErrDeadlineExceeded) {
			return false, nil
		}

		inFDSetReadDeadline(in, 50*time.Millisecond)
		n, readErr := in.Read(inBuf)
		inFDClearReadDeadline(in)
		if n > 0 {
			var toggled bool
			var err error
			pending, toggled, err = processInputToPTY(pending, inBuf[:n], ptyMaster)
			if err != nil {
				return false, err
			}
			if toggled {
				drainPTYToWriter(ptyMaster, out, 100*time.Millisecond)
				return true, nil
			}
		}
		if readErr != nil && !errors.Is(readErr, os.ErrDeadlineExceeded) {
			if errors.Is(readErr, io.EOF) {
				return false, nil
			}
			return false, readErr
		}
	}
}

func inFDSetReadDeadline(in io.Reader, d time.Duration) {
	if f, ok := in.(*os.File); ok {
		_ = f.SetReadDeadline(time.Now().Add(d))
	}
}

func inFDClearReadDeadline(in io.Reader) {
	if f, ok := in.(*os.File); ok {
		_ = f.SetReadDeadline(time.Time{})
	}
}

func readStaticInput(in io.Reader) (data []byte, ok bool, err error) {
	switch in.(type) {
	case *bytes.Reader, *bytes.Buffer, *strings.Reader:
	default:
		return nil, false, nil
	}
	data, err = io.ReadAll(in)
	return data, true, err
}

func watchWinchResize(ptyMaster, sizeFrom *os.File) (stop func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			_ = syncPTYSize(ptyMaster, sizeFrom)
		}
	}()
	return func() { signal.Stop(ch) }
}
