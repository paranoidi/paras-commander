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

func TestDecodeEntryInteractiveDetach(t *testing.T) {
	mf, err := Decode([]byte(`[[entry]]
key = "g"
title = "lazygit"
command = "lazygit"
interactive = 1

[[entry]]
key = "o"
title = "Open"
command = "xdg-open %d"
detach = true
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(mf.Entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(mf.Entries))
	}
	if !mf.Entries[0].Interactive || mf.Entries[0].Detach {
		t.Fatalf("entry 0: interactive=%v detach=%v", mf.Entries[0].Interactive, mf.Entries[0].Detach)
	}
	if mf.Entries[1].Interactive || !mf.Entries[1].Detach {
		t.Fatalf("entry 1: interactive=%v detach=%v", mf.Entries[1].Interactive, mf.Entries[1].Detach)
	}
}

func TestDecodeOptionalKey(t *testing.T) {
	mf, err := Decode([]byte(`[[entry]]
title = "Always"
command = "true"
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(mf.Entries) != 1 || mf.Entries[0].Key != "" {
		t.Fatalf("entries = %+v, want one entry with empty key", mf.Entries)
	}
}

func TestDecodeEntryInteractiveDetachMutuallyExclusive(t *testing.T) {
	_, err := Decode([]byte(`[[entry]]
key = "x"
title = "Bad"
command = "true"
interactive = true
detach = true
`))
	if err == nil {
		t.Fatal("expected mutual exclusion error")
	}
}
