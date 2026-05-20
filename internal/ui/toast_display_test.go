package ui

import "testing"

func TestFormatToastDisplay(t *testing.T) {
	t.Parallel()
	if got := FormatToastDisplay("hello"); got != " hello " {
		t.Fatalf("FormatToastDisplay = %q, want padded", got)
	}
	if FormatToastDisplay("  trimmed  ") != " trimmed " {
		t.Fatal("expected trim then pad")
	}
	if FormatToastDisplay("") != "" {
		t.Fatal("empty input should yield empty display")
	}
	if FormatToastDisplay("   ") != "" {
		t.Fatal("whitespace-only input should yield empty display")
	}
}
