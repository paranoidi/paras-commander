package metacmds_test

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/metacmds"
)

func TestDecode_valid(t *testing.T) {
	toml := `
[[entry]]
name = "disk-size"
description = "Disk usage"
dirs = "du -sh \"$1\" | awk '{print $1}'"

[[entry]]
name = "line-count"
description = "Line count"
file = "wc -l < \"$1\" | tr -d ' '"
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
file = "sha256sum \"$1\" | awk '{print $1}'"
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
dirs = "find \"$1\" -maxdepth 1 -type d | wc -l"
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

func TestDecode_extensions(t *testing.T) {
	toml := `
[[entry]]
name = "line-count"
description = "Line count"
extensions = ["*.py", "*.go"]
file = "wc -l < \"$1\""
`
	mf, err := metacmds.Decode([]byte(toml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := mf.Entries[0]
	if len(e.Extensions) != 2 {
		t.Fatalf("expected 2 extensions, got %d", len(e.Extensions))
	}
	if e.Extensions[0] != "*.py" || e.Extensions[1] != "*.go" {
		t.Errorf("unexpected extensions: %v", e.Extensions)
	}
}

func TestMetaEntry_MatchesPath(t *testing.T) {
	toml := `
[[entry]]
name = "py-info"
description = "Python info"
extensions = ["*.py"]
file = "python3 -c 'import ast; print(len(open(\"$1\").readlines()))'"
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
		got := e.MatchesPath(tt.path)
		if got != tt.want {
			t.Errorf("MatchesPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestMetaEntry_MatchesPath_noFilter(t *testing.T) {
	toml := `
[[entry]]
name = "size"
description = "File size"
file = "stat -c '%s' \"$1\""
`
	mf, err := metacmds.Decode([]byte(toml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := &mf.Entries[0]
	// No extensions → all paths match.
	paths := []string{"/tmp/document.pdf", "/tmp/script.sh", "/tmp/archive.tar.gz", "/tmp/main.go"}
	for _, p := range paths {
		if !e.MatchesPath(p) {
			t.Errorf("MatchesPath(%q) = false, want true (no filter)", p)
		}
	}
}

func TestDecode_invalidGlob(t *testing.T) {
	toml := `
[[entry]]
name = "bad"
description = "Bad glob"
extensions = ["[invalid"]
file = "echo test"
`
	_, err := metacmds.Decode([]byte(toml))
	if err == nil {
		t.Fatal("expected error for invalid glob, got nil")
	}
}

func TestDecode_cache(t *testing.T) {
	toml := `
[[entry]]
name = "slow-info"
description = "Expensive operation"
cache = true
file = "sha256sum \"$1\" | awk '{print $1}'"

[[entry]]
name = "fast-info"
description = "Cheap operation"
file = "stat -c '%s' \"$1\""
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
