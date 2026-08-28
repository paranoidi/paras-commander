package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPatchPreviewTerminalKeys(t *testing.T) {
	cases := []struct {
		name    string
		initial string // "" means the file is not created at all
		want    []string
		notWant []string
	}{
		{
			name: "keys already present are rewritten in place",
			initial: strings.Join([]string{
				`theme = "default"`,
				``,
				`[preview]`,
				`mode = "internal"`,
				`terminal_sixel = "no"`,
				`terminal_kitty = "no"`,
				`terminal_kitty_placeholder = "no"`,
				`image_protocol = "sixel"`,
				`prefetch = true`,
				``,
				`[sftp]`,
				`idle_timeout_secs = 60`,
				``,
			}, "\n"),
			want: []string{
				`terminal_sixel = "yes"`,
				`terminal_kitty = "no"`,
				`terminal_kitty_placeholder = "yes"`,
				`image_protocol = "kitty"`,
				`mode = "internal"`, // untouched sibling key
				`prefetch = true`,   // untouched sibling key
				`theme = "default"`, // untouched unrelated table
				`idle_timeout_secs = 60`,
				`[sftp]`,
			},
		},
		{
			name: "keys missing from an existing table are appended inside it",
			initial: strings.Join([]string{
				`[preview]`,
				`mode = "internal"`,
				``,
				`[sftp]`,
				`idle_timeout_secs = 60`,
				``,
			}, "\n"),
			want: []string{
				`mode = "internal"`,
				`terminal_sixel = "yes"`,
				`terminal_kitty = "no"`,
				`terminal_kitty_placeholder = "yes"`,
				`image_protocol = "kitty"`,
				`idle_timeout_secs = 60`,
			},
		},
		{
			name: "preview table absent gets appended at end of file",
			initial: strings.Join([]string{
				`theme = "default"`,
				``,
				`[sftp]`,
				`idle_timeout_secs = 60`,
				``,
			}, "\n"),
			want: []string{
				`theme = "default"`,
				`[sftp]`,
				`idle_timeout_secs = 60`,
				`[preview]`,
				`terminal_sixel = "yes"`,
				`terminal_kitty = "no"`,
				`terminal_kitty_placeholder = "yes"`,
				`image_protocol = "kitty"`,
			},
		},
		{
			name:    "file absent entirely gets created",
			initial: "",
			want: []string{
				`[preview]`,
				`terminal_sixel = "yes"`,
				`terminal_kitty = "no"`,
				`terminal_kitty_placeholder = "yes"`,
				`image_protocol = "kitty"`,
			},
		},
		{
			name: "trailing inline comments on rewritten keys are preserved",
			initial: strings.Join([]string{
				`[preview]`,
				`terminal_sixel = "no"  # my old sixel note`,
				`terminal_kitty = "auto" # kitty note`,
				``,
			}, "\n"),
			want: []string{
				`terminal_sixel = "yes"  # my old sixel note`,
				`terminal_kitty = "no" # kitty note`,
			},
		},
		{
			name: "unrelated surrounding tables and content are untouched",
			initial: strings.Join([]string{
				`# top comment`,
				`theme = "default"`,
				``,
				`[jobs]`,
				`# a jobs comment`,
				`blocker_dialog_next_debounce_ms = 200`,
				``,
				`[preview]`,
				`# preview comment`,
				`mode = "internal"`,
				`style = "catppuccin-frappe"`,
				``,
				`[ui.zoom]`,
				`active_panel = true`,
				``,
			}, "\n"),
			want: []string{
				`# top comment`,
				`theme = "default"`,
				`[jobs]`,
				`# a jobs comment`,
				`blocker_dialog_next_debounce_ms = 200`,
				`# preview comment`,
				`mode = "internal"`,
				`style = "catppuccin-frappe"`,
				`terminal_sixel = "yes"`,
				`terminal_kitty = "no"`,
				`terminal_kitty_placeholder = "yes"`,
				`image_protocol = "kitty"`,
				`[ui.zoom]`,
				`active_panel = true`,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.toml")
			if tc.initial != "" {
				if err := os.WriteFile(path, []byte(tc.initial), 0o644); err != nil {
					t.Fatalf("seed file: %v", err)
				}
			}

			if err := PatchPreviewTerminalKeys(path, "yes", "no", "yes", "kitty"); err != nil {
				t.Fatalf("PatchPreviewTerminalKeys: %v", err)
			}

			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read patched file: %v", err)
			}
			gotStr := string(got)
			for _, want := range tc.want {
				if !strings.Contains(gotStr, want) {
					t.Errorf("output missing %q\n--- got ---\n%s", want, gotStr)
				}
			}
			for _, notWant := range tc.notWant {
				if strings.Contains(gotStr, notWant) {
					t.Errorf("output unexpectedly contains %q\n--- got ---\n%s", notWant, gotStr)
				}
			}
		})
	}
}

// TestPatchPreviewTerminalKeysIndentedKeys guards against the regex requiring flush-left keys:
// WriteDefaultStub/EncodeDefaultStub (the real config.toml generator) indents keys two spaces
// under their table header, e.g. `  image_protocol = "auto"`, not `image_protocol = "auto"`.
func TestPatchPreviewTerminalKeysIndentedKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	initial := strings.Join([]string{
		`[preview]`,
		`  mode = "internal"`,
		`  image_protocol = "auto"`,
		`  terminal_sixel = "auto"`,
		`  terminal_kitty = "auto"`,
		`  terminal_kitty_placeholder = "auto"`,
		``,
		`[sftp]`,
		`  idle_timeout_secs = 60`,
		``,
	}, "\n")
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	if err := PatchPreviewTerminalKeys(path, "yes", "no", "yes", "kitty"); err != nil {
		t.Fatalf("PatchPreviewTerminalKeys: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read patched file: %v", err)
	}
	gotStr := string(got)
	for _, want := range []string{
		`  terminal_sixel = "yes"`,
		`  terminal_kitty = "no"`,
		`  terminal_kitty_placeholder = "yes"`,
		`  image_protocol = "kitty"`,
		`  mode = "internal"`,
		`  idle_timeout_secs = 60`,
	} {
		if !strings.Contains(gotStr, want) {
			t.Errorf("output missing %q\n--- got ---\n%s", want, gotStr)
		}
	}
}

// TestPatchPreviewTerminalKeysAgainstGeneratedStub is an end-to-end smoke test against the real
// config.toml produced by EncodeDefaultStub (same generator WriteDefaultStub/-config-stub use),
// guarding against the patcher's line-scanning assumptions (indentation, key ordering, other
// [preview] keys like video_thumb_cols) drifting out of sync with the actual stub format.
func TestPatchPreviewTerminalKeysAgainstGeneratedStub(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := WriteDefaultStub(path); err != nil {
		t.Fatalf("WriteDefaultStub: %v", err)
	}

	if err := PatchPreviewTerminalKeys(path, "yes", "no", "yes", "kitty"); err != nil {
		t.Fatalf("PatchPreviewTerminalKeys: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read patched file: %v", err)
	}
	gotStr := string(got)
	for _, want := range []string{
		`terminal_sixel = "yes"`,
		`terminal_kitty = "no"`,
		`terminal_kitty_placeholder = "yes"`,
		`image_protocol = "kitty"`,
		`video_thumb_cols = 2`, // untouched sibling key, still present
	} {
		if !strings.Contains(gotStr, want) {
			t.Errorf("output missing %q\n--- got ---\n%s", want, gotStr)
		}
	}
	if n := strings.Count(gotStr, "[preview]"); n != 1 {
		t.Fatalf("[preview] appears %d times, want 1:\n%s", n, gotStr)
	}
	// The result must still be a loadable config.
	if _, err := LoadFromPaths(Paths{ConfigFile: path}); err != nil {
		t.Fatalf("patched config.toml failed to load: %v", err)
	}
}

// TestPatchPreviewTerminalKeysDoesNotDuplicateOnRepeatedCalls guards against a key being
// re-appended (duplicated) on a second patch after the file already has it in place.
func TestPatchPreviewTerminalKeysDoesNotDuplicateOnRepeatedCalls(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	if err := PatchPreviewTerminalKeys(path, "auto", "auto", "auto", "auto"); err != nil {
		t.Fatalf("first patch: %v", err)
	}
	if err := PatchPreviewTerminalKeys(path, "yes", "no", "yes", "kitty"); err != nil {
		t.Fatalf("second patch: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read patched file: %v", err)
	}
	gotStr := string(got)
	if n := strings.Count(gotStr, "terminal_sixel ="); n != 1 {
		t.Fatalf("terminal_sixel appears %d times, want 1:\n%s", n, gotStr)
	}
	if n := strings.Count(gotStr, "[preview]"); n != 1 {
		t.Fatalf("[preview] appears %d times, want 1:\n%s", n, gotStr)
	}
	if !strings.Contains(gotStr, `terminal_sixel = "yes"`) {
		t.Fatalf("second patch's value not applied:\n%s", gotStr)
	}
}

// TestPatchPreviewTerminalKeysAtomicWrite guards against a truncated file if something goes
// wrong mid-write: it must always write through a temp file + rename, never in place.
func TestPatchPreviewTerminalKeysAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("theme = \"default\"\n"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat before: %v", err)
	}

	if err := PatchPreviewTerminalKeys(path, "auto", "auto", "auto", "auto"); err != nil {
		t.Fatalf("patch: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Fatalf("leftover temp file: %s", e.Name())
		}
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat after: %v", err)
	}
	if before.Mode() != after.Mode() {
		t.Fatalf("mode changed: %v -> %v", before.Mode(), after.Mode())
	}
}

// TestPatchPreviewTerminalKeysSingleQuotedAndBareValues guards against the regex only matching
// double-quoted values: hand-edited configs may use single quotes or bare identifiers.
func TestPatchPreviewTerminalKeysSingleQuotedAndBareValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	initial := strings.Join([]string{
		`[preview]`,
		`  terminal_sixel = 'auto'`,
		`  terminal_kitty = auto`,
		`  terminal_kitty_placeholder = 'no'`,
		`  image_protocol = kitty`,
		``,
	}, "\n")
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	if err := PatchPreviewTerminalKeys(path, "yes", "yes", "yes", "kitty"); err != nil {
		t.Fatalf("PatchPreviewTerminalKeys: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read patched file: %v", err)
	}
	gotStr := string(got)
	for _, key := range []string{"terminal_sixel", "terminal_kitty", "terminal_kitty_placeholder", "image_protocol"} {
		if n := strings.Count(gotStr, key+" ="); n != 1 {
			t.Fatalf("%s appears %d times, want 1:\n%s", key, n, gotStr)
		}
	}
	for _, want := range []string{
		`terminal_sixel = "yes"`,
		`terminal_kitty = "yes"`,
		`terminal_kitty_placeholder = "yes"`,
		`image_protocol = "kitty"`,
	} {
		if !strings.Contains(gotStr, want) {
			t.Errorf("output missing %q\n--- got ---\n%s", want, gotStr)
		}
	}
}

func TestPatchPreviewTerminalKeysForPaths(t *testing.T) {
	dir := t.TempDir()
	paths := Paths{ConfigDir: dir}.WithResolvedLocations()
	if err := PatchPreviewTerminalKeysForPaths(paths, "yes", "no", "yes", "kitty"); err != nil {
		t.Fatalf("PatchPreviewTerminalKeysForPaths: %v", err)
	}
	got, err := os.ReadFile(paths.ConfigFile)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	gotStr := string(got)
	for _, want := range []string{
		`terminal_sixel = "yes"`,
		`terminal_kitty = "no"`,
		`terminal_kitty_placeholder = "yes"`,
		`image_protocol = "kitty"`,
	} {
		if !strings.Contains(gotStr, want) {
			t.Errorf("output missing %q\n--- got ---\n%s", want, gotStr)
		}
	}
}
