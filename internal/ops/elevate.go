package ops

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sync"

	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/privexec"
)

// ErrPermissionSkipped is returned by an elevated step the user chose to skip. Loops over several
// items treat it as "continue with the next item"; single-item callers treat it as a quiet cancel.
var ErrPermissionSkipped = errors.New("skipped after permission denied")

// PermissionDecision is the user's answer to a permission-denied prompt.
type PermissionDecision int

const (
	// PermissionElevate retries the failed step as root.
	PermissionElevate PermissionDecision = iota
	// PermissionSkip leaves this step undone and continues.
	PermissionSkip
	// PermissionAbort stops the whole operation.
	PermissionAbort
)

// PermissionRequest describes one permission-denied step for the prompt.
type PermissionRequest struct {
	Op   string // "rename", "delete", ...
	Path string // host path the step failed on
	Err  error
	// NoPassword is true when sudo needs no password; the prompt then omits the password field.
	NoPassword bool
	// BadPassword is true when the previous password was rejected.
	BadPassword bool
}

// PermissionAnswer is the reply to a PermissionRequest. Password is only meaningful for PermissionElevate.
type PermissionAnswer struct {
	Decision PermissionDecision
	Password string
}

// PermissionWaitFunc asks the user. It blocks until answered; a canceled ctx must return PermissionAbort.
type PermissionWaitFunc func(ctx context.Context, req PermissionRequest) PermissionAnswer

// Elevator retries permission-denied local steps as root. One Elevator lives for one job or one
// foreground operation; the password it learns is kept in memory only for that lifetime.
type Elevator struct {
	wait PermissionWaitFunc

	// Replaceable in tests.
	run     func(ctx context.Context, password string, argv []string) error
	runNoPW func(ctx context.Context, argv []string) error
	noPW    func(ctx context.Context) bool

	mu         sync.Mutex
	password   string
	authorized bool
	noPWSet    bool
	noPWVal    bool
}

// NewElevator returns nil (elevation off) unless enabled and a prompt is available.
func NewElevator(enabled bool, wait PermissionWaitFunc) *Elevator {
	if !enabled || wait == nil {
		return nil
	}
	return &Elevator{
		wait: wait,
		run: func(ctx context.Context, pw string, argv []string) error {
			return privexec.Run(ctx, pw, argv...)
		},
		runNoPW: func(ctx context.Context, argv []string) error {
			return privexec.RunNoPassword(ctx, argv...)
		},
		noPW: privexec.NoPassword,
	}
}

type elevatorKey struct{}

// WithElevator returns ctx carrying e; a nil e returns ctx unchanged.
func WithElevator(ctx context.Context, e *Elevator) context.Context {
	if e == nil {
		return ctx
	}
	return context.WithValue(ctx, elevatorKey{}, e)
}

// ElevationCandidate reports whether err is a permission failure worth offering sudo for.
// EROFS (read-only filesystem) is not an fs.ErrPermission, so it is never offered.
func ElevationCandidate(err error) bool {
	return err != nil && errors.Is(err, fs.ErrPermission)
}

// hostPathOf returns loc's host path, or "" for a non-local location (never elevated).
func hostPathOf(loc pathloc.Path) string {
	if loc.IsRemote() || loc.Scheme() != pathloc.SchemeFile {
		return ""
	}
	p, err := loc.FilePath()
	if err != nil {
		return ""
	}
	return p
}

// withElevation runs fn. When it fails with a permission error and ctx carries an Elevator, the
// user is asked whether to run argv as root instead. path is the host path shown in the prompt;
// "" disables elevation (remote locations).
func withElevation(ctx context.Context, op, path string, argv []string, fn func() error) error {
	err := fn()
	if !ElevationCandidate(err) || path == "" {
		return err
	}
	e, _ := ctx.Value(elevatorKey{}).(*Elevator)
	if e == nil {
		return err
	}
	return e.retry(ctx, op, path, argv, err)
}

func (e *Elevator) retry(ctx context.Context, op, path string, argv []string, cause error) error {
	e.mu.Lock()
	defer e.mu.Unlock() // one prompt at a time per job
	if !e.noPWSet {
		e.noPWVal, e.noPWSet = e.noPW(ctx), true
	}
	fail := func(err error) error { return fmt.Errorf("%w (as root: %v)", cause, err) }
	// After the user approved once, later steps of the same job run without asking again.
	if e.authorized {
		err := e.runAs(ctx, e.password, argv)
		if err == nil {
			return nil
		}
		if !errors.Is(err, privexec.ErrBadPassword) {
			return fail(err)
		}
		e.authorized, e.password = false, ""
	}
	bad := false
	for {
		ans := e.wait(ctx, PermissionRequest{Op: op, Path: path, Err: cause, NoPassword: e.noPWVal, BadPassword: bad})
		switch ans.Decision {
		case PermissionSkip:
			return ErrPermissionSkipped
		case PermissionElevate:
			err := e.runAs(ctx, ans.Password, argv)
			if err == nil {
				e.authorized, e.password = true, ans.Password
				return nil
			}
			if errors.Is(err, privexec.ErrBadPassword) {
				bad = true
				continue
			}
			return fail(err)
		default:
			return ErrUserCanceled
		}
	}
}

func (e *Elevator) runAs(ctx context.Context, password string, argv []string) error {
	if e.noPWVal {
		return e.runNoPW(ctx, argv)
	}
	return e.run(ctx, password, argv)
}

// IsSkipOrCancel reports whether err is a user skip/abort from the permission prompt (not a failure to report).
func IsSkipOrCancel(err error) bool {
	return errors.Is(err, ErrPermissionSkipped) || errors.Is(err, ErrUserCanceled)
}

// chmodArg renders mode like os.Chmod interprets it: permission bits plus setuid/setgid/sticky.
func chmodArg(mode os.FileMode) string {
	m := uint32(mode.Perm())
	if mode&os.ModeSetuid != 0 {
		m |= 0o4000
	}
	if mode&os.ModeSetgid != 0 {
		m |= 0o2000
	}
	if mode&os.ModeSticky != 0 {
		m |= 0o1000
	}
	return fmt.Sprintf("%04o", m)
}

// chownArg renders uid/gid (-1 = unchanged) as chown's OWNER[:GROUP] operand.
func chownArg(uid, gid int) string {
	switch {
	case uid >= 0 && gid >= 0:
		return fmt.Sprintf("%d:%d", uid, gid)
	case uid >= 0:
		return fmt.Sprintf("%d", uid)
	default:
		return fmt.Sprintf(":%d", gid)
	}
}
