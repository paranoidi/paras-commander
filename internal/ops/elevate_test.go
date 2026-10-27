package ops

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"

	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/privexec"
)

type elevHarness struct {
	asked   []PermissionRequest
	runs    [][]string
	pws     []string
	answers []PermissionAnswer
	// badPW makes run fail with ErrBadPassword for that password.
	badPW string
	noPW  bool
}

func (h *elevHarness) ctx(enabled bool) context.Context {
	e := NewElevator(enabled, func(_ context.Context, req PermissionRequest) PermissionAnswer {
		h.asked = append(h.asked, req)
		a := h.answers[0]
		if len(h.answers) > 1 {
			h.answers = h.answers[1:]
		}
		return a
	})
	if e == nil {
		return context.Background()
	}
	e.noPW = func(context.Context) bool { return h.noPW }
	e.run = func(_ context.Context, pw string, argv []string) error {
		h.runs = append(h.runs, argv)
		h.pws = append(h.pws, pw)
		if pw == h.badPW {
			return privexec.ErrBadPassword
		}
		return nil
	}
	e.runNoPW = func(_ context.Context, argv []string) error {
		h.runs = append(h.runs, argv)
		return nil
	}
	return WithElevator(context.Background(), e)
}

func denied(path string) func() error {
	return func() error { return &fs.PathError{Op: "open", Path: path, Err: syscall.EACCES} }
}

func TestWithElevationRunsExactArgvAfterAsking(t *testing.T) {
	h := &elevHarness{answers: []PermissionAnswer{{Decision: PermissionElevate, Password: "heron"}}}
	argv := []string{"mv", "-T", "--", "/srv/otter", "/srv/badger"}
	if err := withElevation(h.ctx(true), "rename", "/srv/otter", argv, denied("/srv/otter")); err != nil {
		t.Fatalf("withElevation() = %v, want nil", err)
	}
	if len(h.asked) != 1 || h.asked[0].Op != "rename" || h.asked[0].Path != "/srv/otter" || h.asked[0].NoPassword {
		t.Fatalf("asked = %+v", h.asked)
	}
	if !reflect.DeepEqual(h.runs, [][]string{argv}) || h.pws[0] != "heron" {
		t.Fatalf("runs = %v pws = %v", h.runs, h.pws)
	}
}

func TestWithElevationSkipAndAbort(t *testing.T) {
	h := &elevHarness{answers: []PermissionAnswer{{Decision: PermissionSkip}}}
	if err := withElevation(h.ctx(true), "delete", "/srv/x", []string{"rm"}, denied("/srv/x")); !errors.Is(err, ErrPermissionSkipped) {
		t.Fatalf("skip: err = %v", err)
	}
	h = &elevHarness{answers: []PermissionAnswer{{Decision: PermissionAbort}}}
	if err := withElevation(h.ctx(true), "delete", "/srv/x", []string{"rm"}, denied("/srv/x")); !errors.Is(err, ErrUserCanceled) {
		t.Fatalf("abort: err = %v", err)
	}
	if len(h.runs) != 0 {
		t.Fatalf("runner called on abort: %v", h.runs)
	}
}

func TestWithElevationBadPasswordAsksAgain(t *testing.T) {
	h := &elevHarness{badPW: "wrong", answers: []PermissionAnswer{
		{Decision: PermissionElevate, Password: "wrong"},
		{Decision: PermissionElevate, Password: "right"},
	}}
	if err := withElevation(h.ctx(true), "mkdir", "/srv/x", []string{"mkdir"}, denied("/srv/x")); err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(h.asked) != 2 || h.asked[0].BadPassword || !h.asked[1].BadPassword {
		t.Fatalf("asked = %+v", h.asked)
	}
}

func TestWithElevationPassesThroughOtherErrors(t *testing.T) {
	h := &elevHarness{answers: []PermissionAnswer{{Decision: PermissionElevate}}}
	ctx := h.ctx(true)
	other := errors.New("disk on fire")
	if err := withElevation(ctx, "rename", "/srv/x", []string{"mv"}, func() error { return other }); err != other {
		t.Fatalf("other error: %v", err)
	}
	erofs := &fs.PathError{Op: "rename", Path: "/srv/x", Err: syscall.EROFS}
	if err := withElevation(ctx, "rename", "/srv/x", []string{"mv"}, func() error { return erofs }); err != erofs {
		t.Fatalf("EROFS: %v", err)
	}
	if err := withElevation(ctx, "rename", "", []string{"mv"}, denied("")); !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("remote (no host path): %v", err)
	}
	if len(h.asked) != 0 {
		t.Fatalf("asked = %+v, want none", h.asked)
	}
}

func TestWithElevationDisabled(t *testing.T) {
	h := &elevHarness{answers: []PermissionAnswer{{Decision: PermissionElevate}}}
	if err := withElevation(h.ctx(false), "rename", "/srv/x", []string{"mv"}, denied("/srv/x")); !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("err = %v, want the original permission error", err)
	}
	if len(h.asked) != 0 || len(h.runs) != 0 {
		t.Fatalf("asked/ran with elevation disabled: %v %v", h.asked, h.runs)
	}
}

func TestWithElevationReusesPasswordWithinOneElevator(t *testing.T) {
	h := &elevHarness{answers: []PermissionAnswer{{Decision: PermissionElevate, Password: "heron"}}}
	ctx := h.ctx(true)
	for _, p := range []string{"/srv/a", "/srv/b", "/srv/c"} {
		if err := withElevation(ctx, "delete", p, []string{"rm", p}, denied(p)); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
	}
	if len(h.asked) != 1 || len(h.runs) != 3 {
		t.Fatalf("asked %d times, ran %d times, want 1 and 3", len(h.asked), len(h.runs))
	}
	for _, pw := range h.pws {
		if pw != "heron" {
			t.Fatalf("pws = %v", h.pws)
		}
	}
	// A new job gets a new Elevator and asks again.
	if err := withElevation(h.ctx(true), "delete", "/srv/d", []string{"rm"}, denied("/srv/d")); err != nil {
		t.Fatal(err)
	}
	if len(h.asked) != 2 {
		t.Fatalf("asked = %d, want 2", len(h.asked))
	}
}

func TestWithElevationNoPassword(t *testing.T) {
	h := &elevHarness{noPW: true, answers: []PermissionAnswer{{Decision: PermissionElevate}}}
	if err := withElevation(h.ctx(true), "chmod", "/srv/x", []string{"chmod"}, denied("/srv/x")); err != nil {
		t.Fatal(err)
	}
	if len(h.asked) != 1 || !h.asked[0].NoPassword || len(h.pws) != 0 || len(h.runs) != 1 {
		t.Fatalf("asked=%+v pws=%v runs=%v", h.asked, h.pws, h.runs)
	}
}

func TestArgHelpers(t *testing.T) {
	if got := chmodArg(0o755 | os.ModeSetuid | os.ModeSticky); got != "5755" {
		t.Fatalf("chmodArg = %q", got)
	}
	for _, c := range []struct {
		uid, gid int
		want     string
	}{{1000, 100, "1000:100"}, {1000, -1, "1000"}, {-1, 100, ":100"}} {
		if got := chownArg(c.uid, c.gid); got != c.want {
			t.Fatalf("chownArg(%d,%d) = %q, want %q", c.uid, c.gid, got, c.want)
		}
	}
}

// readOnlyDir returns a directory the current user cannot write into.
func readOnlyDir(t *testing.T) string {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := filepath.Join(t.TempDir(), "meadow")
	if err := os.Mkdir(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	return dir
}

func TestExecuteMkdirElevatesOnPermissionDenied(t *testing.T) {
	parent := readOnlyDir(t)
	target := filepath.Join(parent, "lantern")
	h := &elevHarness{answers: []PermissionAnswer{{Decision: PermissionElevate, Password: "heron"}}}
	err := ExecuteMkdir(h.ctx(true), MkdirPlan{Path: pathloc.FileMust(target).String(), Name: "lantern"})
	if err != nil {
		t.Fatal(err)
	}
	if want := [][]string{{"mkdir", "--", target}}; !reflect.DeepEqual(h.runs, want) {
		t.Fatalf("runs = %v, want %v", h.runs, want)
	}
	// Without an elevator the original error comes back.
	err = ExecuteMkdir(context.Background(), MkdirPlan{Path: pathloc.FileMust(target).String(), Name: "lantern"})
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("err = %v, want permission error", err)
	}
}

func TestExecuteDeletePathsSkipContinues(t *testing.T) {
	parent := readOnlyDir(t)
	// Files inside a read-only dir cannot be removed; create them first via a writable window.
	if err := os.Chmod(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join(parent, "walrus.txt")
	free := filepath.Join(t.TempDir(), "otter.txt")
	for _, p := range []string{locked, free} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(parent, 0o555); err != nil {
		t.Fatal(err)
	}
	h := &elevHarness{answers: []PermissionAnswer{{Decision: PermissionSkip}}}
	files, _, err := ExecuteDeletePaths(h.ctx(true), []string{
		pathloc.FileMust(locked).String(), pathloc.FileMust(free).String(),
	}, nil, nil, nil)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if files != 1 {
		t.Fatalf("deleted %d, want 1 (locked item skipped, free item deleted)", files)
	}
	if _, err := os.Stat(locked); err != nil {
		t.Fatalf("skipped file should remain: %v", err)
	}
}
