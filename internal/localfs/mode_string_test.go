package localfs

import (
	"io/fs"
	"testing"
)

func TestUnixModeStringDirectory754(t *testing.T) {
	mode := fs.FileMode(0o754) | fs.ModeDir
	got := UnixModeString(mode)
	if want := "drwxr-xr--"; got != want {
		t.Fatalf("UnixModeString(%v) = %q, want %q", mode, got, want)
	}
}
