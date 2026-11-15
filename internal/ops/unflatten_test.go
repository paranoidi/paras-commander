package ops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestUnflattenStem(t *testing.T) {
	t.Parallel()
	cases := []struct {
		base, want string
		ok         bool
	}{
		{"meadow.txt", "meadow", true},
		{"harbor.tar.gz", "harbor.tar", true},
		{".quiet.conf", ".quiet", true},
		{"lantern", "", false},
		{".pebble", "", false},
	}
	for _, c := range cases {
		got, ok := UnflattenStem(c.base)
		if got != c.want || ok != c.ok {
			t.Errorf("UnflattenStem(%q) = %q, %v; want %q, %v", c.base, got, ok, c.want, c.ok)
		}
	}
}

func runUnflatten(t *testing.T, paths ...string) (int, error) {
	t.Helper()
	locs := make([]pathloc.Path, len(paths))
	for i, p := range paths {
		locs[i] = pathloc.MustParse(p)
	}
	done, _, err := ExecuteUnflatten(context.Background(), locs, Options{CopyBufferKiB: 4}, ProgressEmitThrottle{}, nil, nil, nil)
	return done, err
}

func TestExecuteUnflattenGroupsByStem(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := filepath.Join(dir, "orchard.jpg")
	b := filepath.Join(dir, "orchard.txt")
	c := filepath.Join(dir, "violet.tar.gz")
	writeTestFile(t, a, "a")
	writeTestFile(t, b, "b")
	writeTestFile(t, c, "c")
	done, err := runUnflatten(t, a, b, c)
	if err != nil {
		t.Fatal(err)
	}
	if done != 3 {
		t.Fatalf("done = %d, want 3", done)
	}
	for _, p := range []string{"orchard/orchard.jpg", "orchard/orchard.txt", "violet.tar/violet.tar.gz"} {
		if _, err := os.Stat(filepath.Join(dir, p)); err != nil {
			t.Errorf("missing %s: %v", p, err)
		}
	}
	for _, p := range []string{a, b, c} {
		if _, err := os.Lstat(p); !os.IsNotExist(err) {
			t.Errorf("source %s still present", p)
		}
	}
}

func TestExecuteUnflattenReusesExistingDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "canyon", "notes.md"), "keep")
	src := filepath.Join(dir, "canyon.mp3")
	writeTestFile(t, src, "song")
	if _, err := runUnflatten(t, src); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"canyon/notes.md", "canyon/canyon.mp3"} {
		if _, err := os.Stat(filepath.Join(dir, p)); err != nil {
			t.Errorf("missing %s: %v", p, err)
		}
	}
}

func TestExecuteUnflattenTargetIsFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "thistle"), "blocker")
	src := filepath.Join(dir, "thistle.log")
	writeTestFile(t, src, "x")
	_, err := runUnflatten(t, src)
	var opsErr *Error
	if !errors.As(err, &opsErr) {
		t.Fatalf("err = %v, want ops error", err)
	}
	if _, err := os.Stat(src); err != nil {
		t.Errorf("source moved despite error: %v", err)
	}
}

func unflattenValidateText(t *testing.T, selected ...string) string {
	t.Helper()
	sel := map[string]bool{}
	for _, s := range selected {
		sel[filepath.Clean(s)] = true
	}
	_, err := ValidateUnflattenSource(&panel.State{SelectedPaths: sel})
	if err == nil {
		return ""
	}
	var opsErr *Error
	if !errors.As(err, &opsErr) {
		t.Fatalf("err = %v, want ops error", err)
	}
	return opsErr.Text
}

func TestValidateUnflattenSource(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	file1 := filepath.Join(dir, "willow.txt")
	file2 := filepath.Join(dir, "sparrow.png")
	bare := filepath.Join(dir, "glacier")
	sub := filepath.Join(dir, "meadow")
	writeTestFile(t, file1, "x")
	writeTestFile(t, file2, "x")
	writeTestFile(t, bare, "x")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := unflattenValidateText(t, file1, file2); got != "" {
		t.Errorf("files only: %q, want no error", got)
	}
	if got := unflattenValidateText(t, file1, sub); got != "cannot mix files and directories in selection" {
		t.Errorf("mixed: %q", got)
	}
	if got := unflattenValidateText(t, sub); got != "unflatten works on files only" {
		t.Errorf("dirs only: %q", got)
	}
	if got := unflattenValidateText(t, file1, bare); got != "glacier has no extension" {
		t.Errorf("no extension: %q", got)
	}
}
