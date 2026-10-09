package editor

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestCommandElevation(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "lantern.conf")
	if err := os.WriteFile(existing, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "meadow", "river.conf")
	if err := os.Mkdir(filepath.Dir(missing), 0o700); err != nil {
		t.Fatal(err)
	}
	prev := accessDenied
	t.Cleanup(func() { accessDenied = prev })

	tests := []struct {
		name    string
		path    string
		denied  []string // paths for which the probe reports denial
		elevate bool
		want    bool
	}{
		{"writable", existing, nil, true, false},
		{"unwritable file", existing, []string{existing}, true, true},
		{"erofs or other error is not denial", existing, nil, true, false},
		{"missing file, unwritable dir", missing, []string{filepath.Dir(missing)}, true, true},
		{"missing file, writable dir", missing, []string{missing}, true, false},
		{"elevate disabled", existing, []string{existing}, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			accessDenied = func(p string) bool { return slices.Contains(tc.denied, p) }
			argv, env, elevated, err := Command("vim -u NONE", tc.path, tc.elevate)
			if err != nil {
				t.Fatal(err)
			}
			if elevated != tc.want {
				t.Fatalf("elevated = %v, want %v", elevated, tc.want)
			}
			if tc.want {
				if !slices.Equal(argv, []string{"sudo", "-e", "--", tc.path}) || !slices.Equal(env, []string{"SUDO_EDITOR=vim -u NONE"}) {
					t.Fatalf("argv=%q env=%q", argv, env)
				}
				return
			}
			if !slices.Equal(argv, []string{"vim", "-u", "NONE", tc.path}) || env != nil {
				t.Fatalf("argv=%q env=%q", argv, env)
			}
		})
	}
}
