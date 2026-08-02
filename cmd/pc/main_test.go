package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunVersionFlags(t *testing.T) {
	for _, flag := range []string{"-v", "-version", "--version"} {
		t.Run(flag, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if err := run([]string{flag}, &stderr, &stdout); err != nil {
				t.Fatalf("run(%q): %v", flag, err)
			}
			got := strings.TrimSpace(stdout.String())
			if !strings.HasPrefix(got, "pc ") {
				t.Fatalf("stdout = %q, want pc prefix", got)
			}
			if strings.TrimPrefix(got, "pc ") == "" {
				t.Fatalf("stdout = %q, want non-empty version", got)
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestRunRejectsPathArgumentsWithChooserFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	// Flags must precede positionals: the Go flag package stops at the first non-flag.
	err := run([]string{"--chooser-file=/tmp/out", "/tmp/example.go"}, &stderr, &stdout)
	if err == nil {
		t.Fatal("run: nil error, want rejection")
	}
	if !strings.Contains(err.Error(), "path arguments cannot be used with --chooser-file") {
		t.Fatalf("error = %v, want path arguments / chooser-file conflict", err)
	}
}

func TestRunRejectsSelectWithoutChooserFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"--select=/tmp/example.go"}, &stderr, &stdout)
	if err == nil {
		t.Fatal("run: nil error, want rejection")
	}
	if !strings.Contains(err.Error(), "--chooser-file") {
		t.Fatalf("error = %v, want mention of --chooser-file", err)
	}
}

func TestRunRejectsTooManyPositionals(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "alpha")
	b := filepath.Join(root, "bravo")
	c := filepath.Join(root, "charlie")
	for _, d := range []string{a, b, c} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatalf("Mkdir %s: %v", d, err)
		}
	}
	var stdout, stderr bytes.Buffer
	err := run([]string{a, b, c}, &stderr, &stdout)
	if err == nil {
		t.Fatal("run: nil error, want rejection")
	}
	if !strings.Contains(err.Error(), "unexpected argument") {
		t.Fatalf("error = %v, want unexpected argument", err)
	}
}

func TestRunRejectsNoCarouselWithoutChooserFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"--no-carousel"}, &stderr, &stdout)
	if err == nil {
		t.Fatal("run: nil error, want rejection")
	}
	if !strings.Contains(err.Error(), "--chooser-file") {
		t.Fatalf("error = %v, want mention of --chooser-file", err)
	}
}

func TestRunRejectsEmptySelect(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"--select="}, &stderr, &stdout)
	if err == nil {
		t.Fatal("run: nil error, want rejection")
	}
	if !strings.Contains(err.Error(), "non-empty") {
		t.Fatalf("error = %v, want non-empty path error", err)
	}
}

func TestRunRejectsEmptyChooserFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"--chooser-file="}, &stderr, &stdout)
	if err == nil {
		t.Fatal("run: nil error, want rejection")
	}
	if !strings.Contains(err.Error(), "non-empty") {
		t.Fatalf("error = %v, want non-empty path error", err)
	}
}

func TestRunRejectsQuickPreviewWithoutPath(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"-qp"}, &stderr, &stdout)
	if err == nil {
		t.Fatal("run: nil error, want rejection")
	}
	if !strings.Contains(err.Error(), "-qp requires a path argument") {
		t.Fatalf("error = %v, want -qp path requirement", err)
	}
}
