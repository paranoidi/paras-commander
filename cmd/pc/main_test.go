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
