package main

import (
	"bytes"
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

func TestRunRejectsUnexpectedPositional(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"/tmp/example.go"}, &stderr, &stdout)
	if err == nil {
		t.Fatal("run: nil error, want rejection")
	}
	if !strings.Contains(err.Error(), "unexpected argument") {
		t.Fatalf("error = %v, want unexpected argument", err)
	}
}

func TestRunRejectsPositionalWithChooserFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"/tmp/example.go", "--chooser-file=/tmp/out"}, &stderr, &stdout)
	if err == nil {
		t.Fatal("run: nil error, want rejection")
	}
	if !strings.Contains(err.Error(), "unexpected argument") {
		t.Fatalf("error = %v, want unexpected argument", err)
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
	var stdout, stderr bytes.Buffer
	err := run([]string{"/tmp/a", "/tmp/b", "--chooser-file=/tmp/out"}, &stderr, &stdout)
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
