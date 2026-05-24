package localfs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestModeIsExecutable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		mode os.FileMode
		want bool
	}{
		{"regular+user+x", 0o100755, true},
		{"regular+group+x", 0o100710, true},
		{"regular+other+x", 0o100701, true},
		{"regular no x", 0o100644, false},
		{"directory+x", os.ModeDir | 0o755, false},
		{"symlink", os.ModeSymlink | 0o777, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ModeIsExecutable(tt.mode); got != tt.want {
				t.Fatalf("ModeIsExecutable(%o) = %v, want %v", tt.mode, got, tt.want)
			}
		})
	}
}

func TestPathIsExecutable(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "run.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, err := PathIsExecutable(script)
	if err != nil {
		t.Fatalf("PathIsExecutable: %v", err)
	}
	if ok {
		t.Fatal("non-executable script should be false")
	}
	if err := os.Chmod(script, 0o755); err != nil {
		t.Fatal(err)
	}
	ok, err = PathIsExecutable(script)
	if err != nil {
		t.Fatalf("PathIsExecutable: %v", err)
	}
	if !ok {
		t.Fatal("executable script should be true")
	}
	sub := filepath.Join(dir, "child")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	ok, err = PathIsExecutable(sub)
	if err != nil {
		t.Fatalf("PathIsExecutable dir: %v", err)
	}
	if ok {
		t.Fatal("directory should not be executable file")
	}
}

func TestPathLooksRunnable(t *testing.T) {
	dir := t.TempDir()

	script := filepath.Join(dir, "run.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	ok, err := PathLooksRunnable(script)
	if err != nil {
		t.Fatalf("PathLooksRunnable script: %v", err)
	}
	if !ok {
		t.Fatal("shebang script with +x should be runnable")
	}

	plain := filepath.Join(dir, "plain.txt")
	if err := os.WriteFile(plain, []byte("not a script\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	ok, err = PathLooksRunnable(plain)
	if err != nil {
		t.Fatalf("PathLooksRunnable plain: %v", err)
	}
	if ok {
		t.Fatal("+x text without shebang/ELF should not be runnable")
	}

	fakeMedia := filepath.Join(dir, "clip.mkv")
	if err := os.WriteFile(fakeMedia, []byte{0x1a, 0x45, 0xdf, 0xa3}, 0o755); err != nil {
		t.Fatal(err)
	}
	ok, err = PathLooksRunnable(fakeMedia)
	if err != nil {
		t.Fatalf("PathLooksRunnable mkv: %v", err)
	}
	if ok {
		t.Fatal("+x mkv-like bytes should not be runnable")
	}

	elfStub := filepath.Join(dir, "binstub")
	if err := os.WriteFile(elfStub, []byte("\x7fELF\x02\x01"), 0o755); err != nil {
		t.Fatal(err)
	}
	ok, err = PathLooksRunnable(elfStub)
	if err != nil {
		t.Fatalf("PathLooksRunnable elf: %v", err)
	}
	if !ok {
		t.Fatal("ELF header with +x should be runnable")
	}

	noExec := filepath.Join(dir, "locked.sh")
	if err := os.WriteFile(noExec, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, err = PathLooksRunnable(noExec)
	if err != nil {
		t.Fatalf("PathLooksRunnable no x: %v", err)
	}
	if ok {
		t.Fatal("shebang without +x should not be runnable")
	}
}

func TestPathIsExecutableSymlinkToExecutable(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.sh")
	if err := os.WriteFile(target, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.sh")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	ok, err := PathIsExecutable(link)
	if err != nil {
		t.Fatalf("PathIsExecutable symlink: %v", err)
	}
	if !ok {
		t.Fatal("symlink to +x regular file should be executable via Stat")
	}
}
