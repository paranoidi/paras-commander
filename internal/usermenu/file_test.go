package usermenu

import (
	"strings"
	"testing"
)

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
	if mf.Entries[0].ShellPatterns {
		t.Fatalf("entry should inherit file shell_patterns false")
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
	if mf.Entries[0].ShellPatterns {
		t.Fatal("entry should inherit file shell_patterns false")
	}
}

func TestDecodeShellPatternsEntryOverride(t *testing.T) {
	mf, err := Decode([]byte(`shell_patterns = false

[[entry]]
title = "Regex default"
command = "true"

[[entry]]
title = "Glob override"
command = "true"
shell_patterns = true
`))
	if err != nil {
		t.Fatal(err)
	}
	if mf.Entries[0].ShellPatterns {
		t.Fatal("want inherit entry shell_patterns false")
	}
	if !mf.Entries[1].ShellPatterns {
		t.Fatal("want override entry shell_patterns true")
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
key = "p"
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

func TestDecodeEntryBackground(t *testing.T) {
	mf, err := Decode([]byte(`[[entry]]
key = "b"
title = "Background"
command = "true"
background = 1
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(mf.Entries) != 1 || !mf.Entries[0].Background {
		t.Fatalf("entries = %+v, want background=true", mf.Entries)
	}
	if mf.Entries[0].Interactive || mf.Entries[0].Detach {
		t.Fatalf("entry 0: interactive=%v detach=%v", mf.Entries[0].Interactive, mf.Entries[0].Detach)
	}
}

func TestDecodeEntryPool(t *testing.T) {
	mf, err := Decode([]byte(`[[entry]]
key = "p"
title = "Pooled"
command = "true"
pool = "build"
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(mf.Entries) != 1 || mf.Entries[0].Pool != "build" {
		t.Fatalf("entries = %+v, want pool=build", mf.Entries)
	}

	mf, err = Decode([]byte(`[[entry]]
key = "b"
title = "Pooled background"
command = "true"
pool = "build"
background = true
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(mf.Entries) != 1 || mf.Entries[0].Pool != "build" || !mf.Entries[0].Background {
		t.Fatalf("entries = %+v, want pool=build background=true", mf.Entries)
	}
}

func TestDecodeEntryExecutionModesMutuallyExclusive(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{
			name: "interactive and detach",
			body: `[[entry]]
key = "x"
title = "Bad"
command = "true"
interactive = true
detach = true
`,
		},
		{
			name: "interactive and background",
			body: `[[entry]]
key = "x"
title = "Bad"
command = "true"
interactive = true
background = true
`,
		},
		{
			name: "detach and background",
			body: `[[entry]]
key = "x"
title = "Bad"
command = "true"
detach = true
background = true
`,
		},
		{
			name: "pool and interactive",
			body: `[[entry]]
key = "x"
title = "Bad"
command = "true"
pool = "build"
interactive = true
`,
		},
		{
			name: "pool and detach",
			body: `[[entry]]
key = "x"
title = "Bad"
command = "true"
pool = "build"
detach = true
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Decode([]byte(tc.body))
			if err == nil {
				t.Fatal("expected mutual exclusion error")
			}
		})
	}
}

func TestDecodeWhenStringOrArray(t *testing.T) {
	mf, err := Decode([]byte(`[[entry]]
title = "One"
command = "true"
when = "*.go"
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(mf.Entries) != 1 || len(mf.Entries[0].When) != 1 || mf.Entries[0].When[0] != "*.go" {
		t.Fatalf("when string: %+v", mf.Entries)
	}

	mf, err = Decode([]byte(`[[entry]]
title = "Many"
command = "true"
when = ["*.py", "*.go"]
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(mf.Entries) != 1 || len(mf.Entries[0].When) != 2 {
		t.Fatalf("when array: %+v", mf.Entries)
	}
}

func TestDecodeRunForEachValidation(t *testing.T) {
	_, err := Decode([]byte(`[[entry]]
title = "Bad"
command = "true"
run_for_each = ["wat"]
`))
	if err == nil {
		t.Fatal("expected invalid run_for_each error")
	}

	_, err = Decode([]byte(`[[entry]]
title = "Bad"
command = "true"
run_for_each = ["files"]
interactive = true
`))
	if err == nil {
		t.Fatal("expected run_for_each + interactive error")
	}

	mf, err := Decode([]byte(`[[entry]]
title = "Good"
command = "echo %f"
run_for_each = ["files", "dirs"]
background = true
pool = "build"
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(mf.Entries) != 1 || len(mf.Entries[0].RunForEach) != 2 || !mf.Entries[0].Background || mf.Entries[0].Pool != "build" {
		t.Fatalf("decoded: %+v", mf.Entries)
	}
}

func TestDecodeUnknownTopLevelField(t *testing.T) {
	_, err := Decode([]byte(`tools = "x"

[[entry]]
title = "A"
command = "true"
`))
	if err == nil || !strings.Contains(err.Error(), `unknown top-level field "tools"`) {
		t.Fatalf("err = %v, want unknown top-level field", err)
	}
}

func TestDecodeUnknownEntryField(t *testing.T) {
	_, err := Decode([]byte(`[[entry]]
title = "Build project"
command = "true"
commnd = "oops"
`))
	if err == nil || !strings.Contains(err.Error(), `unknown field "commnd"`) {
		t.Fatalf("err = %v, want unknown entry field", err)
	}
}

func TestDecodeWrongEntryTableName(t *testing.T) {
	_, err := Decode([]byte(`[[toolname]]
title = "A"
command = "true"
`))
	if err == nil || !strings.Contains(err.Error(), `unknown top-level field "toolname"`) {
		t.Fatalf("err = %v, want unknown top-level field for wrong table", err)
	}
}

func TestDecodeEntryKeyValidation(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "multi-char key",
			body: `[[entry]]
key = "ab"
title = "A"
command = "true"
`,
			want: "single letter",
		},
		{
			name: "non-letter key",
			body: `[[entry]]
key = "1"
title = "A"
command = "true"
`,
			want: "single letter",
		},
		{
			name: "reserved cancel",
			body: `[[entry]]
key = "c"
title = "A"
command = "true"
`,
			want: "reserved",
		},
		{
			name: "duplicate keys",
			body: `[[entry]]
key = "a"
title = "One"
command = "true"

[[entry]]
key = "a"
title = "Two"
command = "true"
`,
			want: "duplicate key",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Decode([]byte(tc.body))
			if err == nil {
				t.Fatal("expected key validation error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestDecodeWhenValidation(t *testing.T) {
	_, err := Decode([]byte(`[[entry]]
title = "Bad when"
command = "true"
when = "("
`))
	if err == nil || !strings.Contains(err.Error(), "when:") {
		t.Fatalf("err = %v, want when syntax error", err)
	}

	_, err = Decode([]byte(`[[entry]]
title = "Bad glob"
command = "true"
when = "f ["
`))
	if err == nil || !strings.Contains(err.Error(), "when:") {
		t.Fatalf("err = %v, want when glob error", err)
	}

	mf, err := Decode([]byte(`shell_patterns = false

[[entry]]
title = "Good regex"
command = "true"
when = "f \\.go$"
`))
	if err != nil {
		t.Fatalf("valid regex when: %v", err)
	}
	if len(mf.Entries) != 1 || len(mf.Entries[0].When) != 1 {
		t.Fatalf("entries = %+v", mf.Entries)
	}
}

func TestDecodeRunForEachRequiresF(t *testing.T) {
	_, err := Decode([]byte(`[[entry]]
title = "Bad"
command = "true"
run_for_each = ["files"]
`))
	if err == nil || !strings.Contains(err.Error(), "requires %f") {
		t.Fatalf("err = %v, want run_for_each requires %%f", err)
	}
}

func TestDecodeMultipleDefaultEntries(t *testing.T) {
	_, err := Decode([]byte(`[[entry]]
title = "One"
command = "true"
default = true

[[entry]]
title = "Two"
command = "true"
default = true
`))
	if err == nil || !strings.Contains(err.Error(), "only one entry may set default") {
		t.Fatalf("err = %v, want duplicate default error", err)
	}
}

func TestValidatePoolRefs(t *testing.T) {
	mf := &MenuFile{
		Entries: []MenuEntry{{
			Title: "Pooled",
			Pool:  "build",
		}},
	}
	if err := mf.ValidatePoolRefs(PoolNameSet([]string{"build"})); err != nil {
		t.Fatalf("known pool: %v", err)
	}
	if err := mf.ValidatePoolRefs(PoolNameSet(nil)); err == nil || !strings.Contains(err.Error(), `unknown pool "build"`) {
		t.Fatalf("err = %v, want unknown pool", err)
	}
}
