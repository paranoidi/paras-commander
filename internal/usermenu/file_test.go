package usermenu

import "testing"

func TestDecodeShellPatternsIntegerMCStyle(t *testing.T) {
	mf, err := Decode([]byte(`shell_patterns = 0

[[entry]]
key = "a"
title = "A"
command = "true"
`))
	if err != nil {
		t.Fatal(err)
	}
	if mf.ShellPatterns {
		t.Fatalf("shell_patterns = 0: got ShellPatterns true, want false")
	}

	mf, err = Decode([]byte(`shell_patterns = 1

[[entry]]
key = "a"
title = "A"
command = "true"
`))
	if err != nil {
		t.Fatal(err)
	}
	if !mf.ShellPatterns {
		t.Fatalf("shell_patterns = 1: got ShellPatterns false, want true")
	}
}

func TestDecodeShellPatternsBool(t *testing.T) {
	mf, err := Decode([]byte(`shell_patterns = false

[[entry]]
key = "a"
title = "A"
command = "true"
`))
	if err != nil {
		t.Fatal(err)
	}
	if mf.ShellPatterns {
		t.Fatal("want false")
	}
}

func TestDecodeMenuStubTOML(t *testing.T) {
	mf, err := Decode([]byte(MenuStubTOML))
	if err != nil {
		t.Fatalf("MenuStubTOML: %v", err)
	}
	if len(mf.Entries) != 0 {
		t.Fatalf("stub should decode to zero entries, got %d", len(mf.Entries))
	}
}
