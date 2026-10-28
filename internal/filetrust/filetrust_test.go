package filetrust

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestEvaluate(t *testing.T) {
	const me = 1000
	private := func(uid, gid uint32) bool { return gid == uid }
	cases := []struct {
		name string
		mode fs.FileMode
		uid  uint32
		gid  uint32
		want string
	}{
		{"own 0644", 0o644, me, 50, ""},
		{"root 0644", 0o644, 0, 0, ""},
		{"other owner", 0o644, 1001, 1001, ReasonOtherOwner},
		{"world-writable", 0o646, me, me, ReasonWorldWritable},
		{"group-writable private group", 0o664, me, me, ""},
		{"group-writable shared group", 0o664, me, 50, ReasonGroupWritable},
		{"directory", fs.ModeDir | 0o755, me, me, ReasonNotRegular},
		{"symlink", fs.ModeSymlink | 0o777, me, me, ReasonNotRegular},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := evaluate(tc.mode, tc.uid, tc.gid, me, private); got != tc.want {
				t.Fatalf("evaluate = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCheck_regularFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "walnut.toml")
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Check(p); got != "" {
		t.Fatalf("Check(0644 own file) = %q, want trusted", got)
	}
	if runtime.GOOS == "windows" {
		return
	}
	if err := os.Chmod(p, 0o646); err != nil {
		t.Fatal(err)
	}
	if got := Check(p); got != ReasonWorldWritable {
		t.Fatalf("Check(0646) = %q, want %q", got, ReasonWorldWritable)
	}
}

func TestCheck_missing(t *testing.T) {
	if Check(filepath.Join(t.TempDir(), "absent")) == "" {
		t.Fatal("missing file reported trusted")
	}
}
