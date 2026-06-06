package metacmds_test

import (
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/metacmds"
)

func TestDecode_valid(t *testing.T) {
	toml := `
[[entry]]
name = "disk-size"
description = "Disk usage"
dirs = "du -sh %f | awk '{print $1}'"

[[entry]]
name = "line-count"
description = "Line count"
file = "wc -l < %f | tr -d ' '"
`
	mf, err := metacmds.Decode([]byte(toml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mf.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(mf.Entries))
	}
	if mf.Entries[0].Name != "disk-size" {
		t.Errorf("entry 0 name: got %q, want %q", mf.Entries[0].Name, "disk-size")
	}
	if mf.Entries[0].Dirs == "" {
		t.Error("entry 0 dirs should not be empty")
	}
	if mf.Entries[1].File == "" {
		t.Error("entry 1 file should not be empty")
	}
}

func TestDecode_missingName(t *testing.T) {
	toml := `
[[entry]]
description = "Missing name"
file = "echo test"
`
	_, err := metacmds.Decode([]byte(toml))
	if err == nil {
		t.Fatal("expected error for missing name, got nil")
	}
}

func TestDecode_missingDescription(t *testing.T) {
	toml := `
[[entry]]
name = "orphan"
file = "echo test"
`
	_, err := metacmds.Decode([]byte(toml))
	if err == nil {
		t.Fatal("expected error for missing description, got nil")
	}
}

func TestDecode_missingCommands(t *testing.T) {
	toml := `
[[entry]]
name = "empty"
description = "No commands"
`
	_, err := metacmds.Decode([]byte(toml))
	if err == nil {
		t.Fatal("expected error for missing file and dirs, got nil")
	}
}

func TestDecode_empty(t *testing.T) {
	mf, err := metacmds.Decode([]byte(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mf.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(mf.Entries))
	}
}

func TestMetaFile_EntryByName(t *testing.T) {
	toml := `
[[entry]]
name = "checksum"
description = "SHA256 checksum"
file = "sha256sum %f | awk '{print $1}'"
`
	mf, err := metacmds.Decode([]byte(toml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e, ok := mf.EntryByName("checksum")
	if !ok {
		t.Fatal("expected to find entry 'checksum'")
	}
	if e.Name != "checksum" {
		t.Errorf("got name %q, want %q", e.Name, "checksum")
	}
	_, ok = mf.EntryByName("missing")
	if ok {
		t.Error("expected not to find entry 'missing'")
	}
}

func TestDecode_onlyDirsIsValid(t *testing.T) {
	toml := `
[[entry]]
name = "folder-count"
description = "Count subfolders"
dirs = "find %f -maxdepth 1 -type d | wc -l"
`
	mf, err := metacmds.Decode([]byte(toml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mf.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(mf.Entries))
	}
	if mf.Entries[0].File != "" {
		t.Errorf("expected empty File field, got %q", mf.Entries[0].File)
	}
}

func TestDecode_when(t *testing.T) {
	toml := `
[[entry]]
name = "line-count"
description = "Line count"
when = ["*.py", "*.go"]
file = "wc -l < %f"
`
	mf, err := metacmds.Decode([]byte(toml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := mf.Entries[0]
	if len(e.When) != 2 {
		t.Fatalf("expected 2 when expressions, got %d", len(e.When))
	}
	if e.When[0] != "*.py" || e.When[1] != "*.go" {
		t.Errorf("unexpected when: %v", e.When)
	}
}

func TestMetaEntry_MatchesRow(t *testing.T) {
	toml := `
[[entry]]
name = "py-info"
description = "Python info"
when = ["*.py"]
file = "python3 -c 'print(1)'"
`
	mf, err := metacmds.Decode([]byte(toml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := &mf.Entries[0]

	tests := []struct {
		path string
		want bool
	}{
		{"/home/user/projects/solver.py", true},
		{"/home/user/projects/main.go", false},
		{"/home/user/projects/readme.txt", false},
		{"/home/user/projects/analyze.py", true},
	}
	for _, tt := range tests {
		ent := localfs.Entry{Name: filepath.Base(tt.path), Path: tt.path, Type: localfs.EntryFile}
		got, err := e.MatchesRow(ent, "/home/user/projects")
		if err != nil {
			t.Fatalf("MatchesRow(%q): %v", tt.path, err)
		}
		if got != tt.want {
			t.Errorf("MatchesRow(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestMetaEntry_MatchesRow_noFilter(t *testing.T) {
	toml := `
[[entry]]
name = "size"
description = "File size"
file = "stat -c '%s' %f"
`
	mf, err := metacmds.Decode([]byte(toml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := &mf.Entries[0]
	paths := []string{"/tmp/document.pdf", "/tmp/script.sh", "/tmp/archive.tar.gz", "/tmp/main.go"}
	for _, p := range paths {
		ent := localfs.Entry{Name: "x", Path: p, Type: localfs.EntryFile}
		ok, err := e.MatchesRow(ent, "/tmp")
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Errorf("MatchesRow(%q) = false, want true (no filter)", p)
		}
	}
}

func TestDecode_shellPatterns(t *testing.T) {
	mf, err := metacmds.Decode([]byte(`shell_patterns = 0
[[entry]]
name = "x"
description = "y"
file = "true"
`))
	if err != nil {
		t.Fatal(err)
	}
	if mf.ShellPatterns {
		t.Fatal("want file shell_patterns false")
	}
	if mf.Entries[0].ShellPatterns {
		t.Fatal("want entry to inherit file shell_patterns false")
	}
}

func TestDecode_shellPatternsEntryOverride(t *testing.T) {
	mf, err := metacmds.Decode([]byte(`shell_patterns = false
[[entry]]
name = "inherit"
description = "inherits regex"
file = "true"
[[entry]]
name = "glob"
description = "entry override"
shell_patterns = true
when = ["*.go"]
file = "true"
`))
	if err != nil {
		t.Fatal(err)
	}
	if mf.ShellPatterns {
		t.Fatal("want file shell_patterns false")
	}
	if mf.Entries[0].ShellPatterns {
		t.Fatal("want inherit entry shell_patterns false")
	}
	if !mf.Entries[1].ShellPatterns {
		t.Fatal("want override entry shell_patterns true")
	}
}

func TestMetaEntry_MatchesRow_regexOverride(t *testing.T) {
	mf, err := metacmds.Decode([]byte(`[[entry]]
name = "makefile"
description = "Makefile only"
shell_patterns = false
when = ["^Makefile$"]
file = "true"
`))
	if err != nil {
		t.Fatal(err)
	}
	e := &mf.Entries[0]
	tests := []struct {
		name string
		want bool
	}{
		{"Makefile", true},
		{"Makefile.in", false},
		{"GNUmakefile", false},
	}
	for _, tt := range tests {
		ent := localfs.Entry{Name: tt.name, Path: "/proj/" + tt.name, Type: localfs.EntryFile}
		got, err := e.MatchesRow(ent, "/proj")
		if err != nil {
			t.Fatalf("MatchesRow(%q): %v", tt.name, err)
		}
		if got != tt.want {
			t.Errorf("MatchesRow(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestDecode_cache(t *testing.T) {
	toml := `
[[entry]]
name = "slow-info"
description = "Expensive operation"
cache = true
file = "sha256sum %f | awk '{print $1}'"

[[entry]]
name = "fast-info"
description = "Cheap operation"
file = "stat -c '%s' %f"
`
	mf, err := metacmds.Decode([]byte(toml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mf.Entries[0].Cache {
		t.Error("entry 0: expected Cache=true")
	}
	if mf.Entries[1].Cache {
		t.Error("entry 1: expected Cache=false (default)")
	}
}

func TestDecode_workers(t *testing.T) {
	toml := `
[[entry]]
name = "slow-info"
description = "Expensive operation"
cache = true
workers = 8
file = "sha256sum %f | awk '{print $1}'"

[[entry]]
name = "fast-info"
description = "Cheap operation"
file = "stat -c '%s' %f"
`
	mf, err := metacmds.Decode([]byte(toml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mf.Entries[0].Workers != 8 {
		t.Errorf("entry 0: expected Workers=8, got %d", mf.Entries[0].Workers)
	}
	if mf.Entries[1].Workers != 0 {
		t.Errorf("entry 1: expected Workers=0 (default), got %d", mf.Entries[1].Workers)
	}
}

func TestDecode_workersNegative(t *testing.T) {
	toml := `
[[entry]]
name = "bad"
description = "Negative workers"
workers = -1
file = "echo test"
`
	_, err := metacmds.Decode([]byte(toml))
	if err == nil {
		t.Fatal("expected error for negative workers, got nil")
	}
}

func TestDecode_workersClamp(t *testing.T) {
	toml := `
[[entry]]
name = "lots"
description = "Too many workers"
workers = 999
file = "echo test"
`
	mf, err := metacmds.Decode([]byte(toml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mf.Entries[0].Workers > 64 {
		t.Errorf("entry 0: expected Workers clamped to max 64, got %d", mf.Entries[0].Workers)
	}
}
