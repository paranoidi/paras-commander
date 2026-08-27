package dialog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTransferTopLevelDestNamesSingleFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "willow.txt")
	dst := filepath.Join(dir, "harbor")
	if err := os.Mkdir(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := transferTopLevelDestNames([]string{src}, dst, true, false)
	if len(got) != 1 || got[0] != "willow.txt" {
		t.Fatalf("got %v, want [willow.txt]", got)
	}
}

func TestTransferTopLevelDestNamesStructuredBatch(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "orchard")
	b := filepath.Join(dir, "meadow")
	for _, d := range []string{a, b} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	f1 := filepath.Join(a, "cedar.txt")
	f2 := filepath.Join(b, "pine.txt")
	for _, f := range []string{f1, f2} {
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dst := filepath.Join(dir, "target")
	if err := os.Mkdir(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	got := transferTopLevelDestNames([]string{f1, f2}, dst, true, false)
	if len(got) != 2 || got[0] != "orchard" || got[1] != "meadow" {
		t.Fatalf("got %v, want [orchard meadow]", got)
	}
}

func TestTransferTopLevelDestNamesFlatten(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "orchard")
	if err := os.Mkdir(a, 0o755); err != nil {
		t.Fatal(err)
	}
	f1 := filepath.Join(a, "cedar.txt")
	if err := os.WriteFile(f1, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "target")
	if err := os.Mkdir(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	got := transferTopLevelDestNames([]string{f1}, dst, true, true)
	if len(got) != 1 || got[0] != "cedar.txt" {
		t.Fatalf("got %v, want [cedar.txt]", got)
	}
}

func TestTopLevelPathSegment(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"willow.txt", "willow.txt"},
		{"orchard/cedar.txt", "orchard"},
		{"a/b/c", "a"},
		{"/leading", "leading"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := topLevelPathSegment(tc.in); got != tc.want {
			t.Errorf("topLevelPathSegment(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
