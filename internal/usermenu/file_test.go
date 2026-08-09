package usermenu

import (
	"strings"
	"testing"
)

func TestDecodeShellPatternsIntegerMCStyle(t *testing.T) {
	mf, err := Decode([]byte(`shell_patterns = 0

[a]
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

[a]
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

[a]
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

[regex_default]
title = "Regex default"
command = "true"

[glob_override]
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
	mf, err := Decode([]byte(`[lazygit]
key = "g"
title = "lazygit"
command = "lazygit"
interactive = 1

[open]
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
	mf, err := Decode([]byte(`[always]
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
	mf, err := Decode([]byte(`[background_entry]
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
	mf, err := Decode([]byte(`[pooled]
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

	mf, err = Decode([]byte(`[pooled_bg]
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
			body: `[bad]
key = "x"
title = "Bad"
command = "true"
interactive = true
detach = true
`,
		},
		{
			name: "interactive and background",
			body: `[bad]
key = "x"
title = "Bad"
command = "true"
interactive = true
background = true
`,
		},
		{
			name: "detach and background",
			body: `[bad]
key = "x"
title = "Bad"
command = "true"
detach = true
background = true
`,
		},
		{
			name: "pool and interactive",
			body: `[bad]
key = "x"
title = "Bad"
command = "true"
pool = "build"
interactive = true
`,
		},
		{
			name: "pool and detach",
			body: `[bad]
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
	mf, err := Decode([]byte(`[one]
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

	mf, err = Decode([]byte(`[many]
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
	_, err := Decode([]byte(`[bad]
title = "Bad"
command = "true"
run_for_each = ["wat"]
`))
	if err == nil {
		t.Fatal("expected invalid run_for_each error")
	}

	_, err = Decode([]byte(`[bad]
title = "Bad"
command = "true"
run_for_each = ["files"]
interactive = true
`))
	if err == nil {
		t.Fatal("expected run_for_each + interactive error")
	}

	mf, err := Decode([]byte(`[good]
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

func TestDecodeUnknownRootScalarField(t *testing.T) {
	_, err := Decode([]byte(`tools = "x"

[pwd]
title = "A"
command = "true"
`))
	if err == nil || !strings.Contains(err.Error(), "expected table") {
		t.Fatalf("err = %v, want type-mismatch error for a non-table root key", err)
	}
}

func TestDecodeUnknownEntryField(t *testing.T) {
	_, err := Decode([]byte(`[build]
title = "Build project"
command = "true"
commnd = "oops"
`))
	if err == nil || !strings.Contains(err.Error(), `unknown field "commnd"`) {
		t.Fatalf("err = %v, want unknown entry field", err)
	}
}

func TestDecodeArrayOfTablesMigrationError(t *testing.T) {
	_, err := Decode([]byte(`[[entry]]
title = "A"
command = "true"
`))
	if err == nil || !strings.Contains(err.Error(), "no longer supported") {
		t.Fatalf("err = %v, want [[entry]] migration error", err)
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
			body: `[a]
key = "ab"
title = "A"
command = "true"
`,
			want: "single letter",
		},
		{
			name: "non-letter key",
			body: `[a]
key = "1"
title = "A"
command = "true"
`,
			want: "single letter",
		},
		{
			name: "duplicate keys",
			body: `[one]
key = "a"
title = "One"
command = "true"

[two]
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
	_, err := Decode([]byte(`[bad_when]
title = "Bad when"
command = "true"
when = "("
`))
	if err == nil || !strings.Contains(err.Error(), "when:") {
		t.Fatalf("err = %v, want when syntax error", err)
	}

	_, err = Decode([]byte(`[bad_glob]
title = "Bad glob"
command = "true"
when = "f ["
`))
	if err == nil || !strings.Contains(err.Error(), "when:") {
		t.Fatalf("err = %v, want when glob error", err)
	}

	mf, err := Decode([]byte(`shell_patterns = false

[good_regex]
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
	_, err := Decode([]byte(`[bad]
title = "Bad"
command = "true"
run_for_each = ["files"]
`))
	if err == nil || !strings.Contains(err.Error(), "requires %f") {
		t.Fatalf("err = %v, want run_for_each requires %%f", err)
	}
}

func TestDecodeMultipleDefaultEntries(t *testing.T) {
	_, err := Decode([]byte(`[one]
title = "One"
command = "true"
default = true

[two]
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

func TestValidatePoolRefsNestedInSubmenu(t *testing.T) {
	mf := &MenuFile{
		Entries: []MenuEntry{{
			Title: "Tools",
			Entries: []MenuEntry{{
				Title: "Pooled",
				Pool:  "build",
			}},
		}},
	}
	if err := mf.ValidatePoolRefs(PoolNameSet(nil)); err == nil || !strings.Contains(err.Error(), `unknown pool "build"`) {
		t.Fatalf("err = %v, want unknown pool caught inside submenu", err)
	}
}

// --- Submenu decode coverage ---

func TestDecodeSubmenuOrderMatchesFileHeaderOrder(t *testing.T) {
	mf, err := Decode([]byte(`[pwd]
title = "Print working directory"
command = "pwd"

[tools]
title = "Tools"
key = "t"

[tools.disk_use]
title = "Show disk usage"
command = "du -sh %f"

[other]
title = "Other"
command = "true"

[tools.format]
title = "Format code"
command = "gofmt -w %f"
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(mf.Entries) != 3 {
		t.Fatalf("root entries = %d, want 3: %+v", len(mf.Entries), mf.Entries)
	}
	if mf.Entries[0].Title != "Print working directory" || mf.Entries[1].Title != "Tools" || mf.Entries[2].Title != "Other" {
		t.Fatalf("root order wrong: %+v", mf.Entries)
	}
	tools := mf.Entries[1]
	if !tools.IsSubmenu() {
		t.Fatalf("tools should be a submenu: %+v", tools)
	}
	if len(tools.Entries) != 2 || tools.Entries[0].Title != "Show disk usage" || tools.Entries[1].Title != "Format code" {
		t.Fatalf("submenu order wrong (interleaved root header shouldn't affect it): %+v", tools.Entries)
	}
	if mf.Entries[2].IsSubmenu() {
		t.Fatalf("other should be a leaf: %+v", mf.Entries[2])
	}
}

func TestDecodeSubmenuMutualExclusion(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"command", `[tools]
title = "Tools"
command = "true"

[tools.x]
title = "X"
command = "true"
`},
		{"run_for_each", `[tools]
title = "Tools"
run_for_each = ["files"]

[tools.x]
title = "X"
command = "true"
`},
		{"pool", `[tools]
title = "Tools"
pool = "build"

[tools.x]
title = "X"
command = "true"
`},
		{"toast", `[tools]
title = "Tools"
toast = "done"

[tools.x]
title = "X"
command = "true"
`},
		{"interactive", `[tools]
title = "Tools"
interactive = true

[tools.x]
title = "X"
command = "true"
`},
		{"detach", `[tools]
title = "Tools"
detach = true

[tools.x]
title = "X"
command = "true"
`},
		{"background", `[tools]
title = "Tools"
background = true

[tools.x]
title = "X"
command = "true"
`},
		{"dialog", `[tools]
title = "Tools"
dialog = true

[tools.x]
title = "X"
command = "true"
`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Decode([]byte(tc.body))
			if err == nil || !strings.Contains(err.Error(), "cannot be combined with a submenu") {
				t.Fatalf("err = %v, want submenu mutual-exclusion error", err)
			}
		})
	}
}

func TestDecodeEmptyTableRequiresCommand(t *testing.T) {
	_, err := Decode([]byte(`[a]
title = "A"
`))
	if err == nil || !strings.Contains(err.Error(), "command is required") {
		t.Fatalf("err = %v, want command is required for a table with no children and no command", err)
	}
}

func TestDecodeDuplicateKeyWithinSubmenuLevel(t *testing.T) {
	_, err := Decode([]byte(`[tools]
title = "Tools"

[tools.one]
title = "One"
command = "true"
key = "x"

[tools.two]
title = "Two"
command = "true"
key = "x"
`))
	if err == nil || !strings.Contains(err.Error(), "duplicate key") {
		t.Fatalf("err = %v, want duplicate key error within one submenu level", err)
	}
}

func TestDecodeSiblingSubmenuKeyReuseAllowed(t *testing.T) {
	mf, err := Decode([]byte(`[a]
title = "A"
key = "x"

[a.one]
title = "One"
command = "true"
key = "y"

[b]
title = "B"
key = "z"

[b.two]
title = "Two"
command = "true"
key = "y"
`))
	if err != nil {
		t.Fatalf("key reuse across sibling submenus should be fine: %v", err)
	}
	if len(mf.Entries) != 2 || len(mf.Entries[0].Entries) != 1 || len(mf.Entries[1].Entries) != 1 {
		t.Fatalf("entries = %+v", mf.Entries)
	}
}

func TestDecodeUnknownFieldInsideSubmenu(t *testing.T) {
	_, err := Decode([]byte(`[tools]
title = "Tools"

[tools.child]
title = "Child"
command = "true"
commnd = "oops"
`))
	if err == nil || !strings.Contains(err.Error(), `[tools.child]`) || !strings.Contains(err.Error(), `unknown field "commnd"`) {
		t.Fatalf("err = %v, want unknown field error scoped to tools.child", err)
	}
}

func TestDecodeChildTableNamedAfterReservedField(t *testing.T) {
	_, err := Decode([]byte(`[tools]
title = "Tools"

[tools.command]
title = "X"
command = "true"
`))
	if err == nil || !strings.Contains(err.Error(), "reserved field name") {
		t.Fatalf("err = %v, want reserved field name error", err)
	}
}
