package usermenu

import (
	"errors"
	"strings"
	"testing"
)

func TestShortLoadErrorEntryValidation(t *testing.T) {
	err := errors.New("menu.toml: [tools.disk_use]: command is required")
	got := ShortLoadError(err)
	if got != "[tools.disk_use]: command is required" {
		t.Fatalf("got %q", got)
	}
}

func TestShortLoadErrorTomlTypeMismatch(t *testing.T) {
	err := errors.New(`menu.toml: toml: line 1 (last key "shell_patterns"): incompatible types: TOML value has type int64; destination has type boolean`)
	got := ShortLoadError(err)
	if !strings.Contains(got, "shell_patterns") {
		t.Fatalf("got %q, want key in message", got)
	}
	if !strings.Contains(got, "invalid value type") {
		t.Fatalf("got %q, want short type explanation", got)
	}
}

func TestShortLoadErrorTruncatesLongDetail(t *testing.T) {
	long := strings.Repeat("x", 200)
	err := errors.New("menu.toml: " + long)
	got := ShortLoadError(err)
	if len([]rune(got)) > shortLoadErrorMaxRunes+3 {
		t.Fatalf("got len %d, want at most %d+ellipsis", len([]rune(got)), shortLoadErrorMaxRunes)
	}
}
