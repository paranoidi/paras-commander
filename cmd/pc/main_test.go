package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
)

func TestRunConfigStubWritesDefaultsAndExits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	var stderr bytes.Buffer

	if err := run([]string{"--config-stub", path}, &stderr); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	cfg, err := config.LoadFromPaths(config.Paths{ConfigFile: path})
	if err != nil {
		t.Fatalf("LoadFromPaths() error = %v", err)
	}
	if !reflect.DeepEqual(cfg, config.Default()) {
		t.Fatalf("decoded config = %+v, want defaults %+v", cfg, config.Default())
	}
}

// TestRunConfigStubIncludesShortcutDefaults verifies that --config-stub
// emits a single self-contained file: general settings plus the full
// shortcut Bundle (global [action_keys] AND jobs-view-only
// [jobs_action_keys]). Both tables are sourced from the keymap package
// registries so adding a new ActionSpec or jobs-overlay default is
// automatically reflected here.
func TestRunConfigStubIncludesShortcutDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	var stderr bytes.Buffer
	if err := run([]string{"--config-stub", path}, &stderr); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	raw, err := readFile(path)
	if err != nil {
		t.Fatalf("read stub: %v", err)
	}
	for _, table := range []string{"[action_keys]", "[jobs_action_keys]"} {
		if !strings.Contains(raw, table) {
			t.Fatalf("stub missing %s table:\n%s", table, raw)
		}
	}

	for actionID, defaults := range keymap.DefaultActionKeys() {
		if len(defaults) == 0 {
			continue
		}
		needle := "\"" + actionID + "\""
		if !strings.Contains(raw, needle) {
			t.Fatalf("stub missing default-bound action %q:\n%s", actionID, raw)
		}
	}
	for actionID := range keymap.DefaultJobsOverlayKeys() {
		needle := "\"" + actionID + "\""
		if !strings.Contains(raw, needle) {
			t.Fatalf("stub missing jobs overlay action %q:\n%s", actionID, raw)
		}
	}

	// Section split: jobs.open defaults live under [action_keys]; every
	// other jobs.* default lives only under [jobs_action_keys].
	actionSection, jobsSection := splitShortcutSections(t, raw)
	if !strings.Contains(actionSection, `"`+keymap.ActionJobsOpen+`"`) {
		t.Fatalf("[action_keys] missing jobs.open:\n%s", actionSection)
	}
	for actionID := range keymap.DefaultJobsOverlayKeys() {
		needle := "\"" + actionID + "\""
		if strings.Contains(actionSection, needle) {
			t.Fatalf("[action_keys] must not list overlay-only action %q\n[action_keys]:\n%s", actionID, actionSection)
		}
		if !strings.Contains(jobsSection, needle) {
			t.Fatalf("[jobs_action_keys] missing %q\n[jobs_action_keys]:\n%s", actionID, jobsSection)
		}
	}

	keys, err := config.ReadActionKeys(path)
	if err != nil {
		t.Fatalf("ReadActionKeys: %v", err)
	}
	for actionID, defaults := range keymap.DefaultActionKeys() {
		if len(defaults) == 0 {
			continue
		}
		got, ok := keys[actionID]
		if !ok {
			t.Fatalf("ReadActionKeys missing %q", actionID)
		}
		if !equalStringSlices(got, defaults) {
			t.Fatalf("ReadActionKeys[%q] = %v, want %v", actionID, got, defaults)
		}
	}
	jobsKeys, err := config.ReadJobsActionKeys(path)
	if err != nil {
		t.Fatalf("ReadJobsActionKeys: %v", err)
	}
	for actionID, defaults := range keymap.DefaultJobsOverlayKeys() {
		got, ok := jobsKeys[actionID]
		if !ok {
			t.Fatalf("ReadJobsActionKeys missing %q", actionID)
		}
		if !equalStringSlices(got, defaults) {
			t.Fatalf("ReadJobsActionKeys[%q] = %v, want %v", actionID, got, defaults)
		}
	}

	bundle, err := keymap.LoadFromPaths(config.Paths{ConfigFile: path})
	if err != nil {
		t.Fatalf("keymap.LoadFromPaths: %v", err)
	}
	defBundle, err := keymap.DefaultBundle()
	if err != nil {
		t.Fatalf("keymap.DefaultBundle: %v", err)
	}
	for _, ev := range []*tcell.EventKey{
		tcell.NewEventKey(tcell.KeyF10, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyCtrlR, 0, tcell.ModNone),
	} {
		a1, ok1 := bundle.Global.Lookup(ev)
		a2, ok2 := defBundle.Global.Lookup(ev)
		if ok1 != ok2 || a1 != a2 {
			t.Fatalf("global stub vs default: %v -> %q %v vs %q %v", ev, a1, ok1, a2, ok2)
		}
	}
	f8 := tcell.NewEventKey(tcell.KeyF8, 0, tcell.ModNone)
	j1, ok1 := bundle.Jobs.Lookup(f8)
	j2, ok2 := defBundle.Jobs.Lookup(f8)
	if ok1 != ok2 || j1 != j2 {
		t.Fatalf("jobs overlay F8 stub vs default: %q %v vs %q %v", j1, ok1, j2, ok2)
	}
}

func TestRunKeybindingsStubWritesDefaultsAndExits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keybindings.toml")
	var stderr bytes.Buffer

	if err := run([]string{"--keybindings-stub", path}, &stderr); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	bundle, err := keymap.LoadFromPaths(config.Paths{KeybindingsFile: path})
	if err != nil {
		t.Fatalf("LoadFromPaths() error = %v", err)
	}
	def, err := keymap.Default()
	if err != nil {
		t.Fatalf("keymap.Default() error = %v", err)
	}
	ev := tcell.NewEventKey(tcell.KeyCtrlR, 0, tcell.ModNone)
	a1, ok1 := bundle.Global.Lookup(ev)
	a2, ok2 := def.Lookup(ev)
	if ok1 != ok2 || a1 != a2 {
		t.Fatalf("stub vs default: lookup ctrl-r = %q %v vs %q %v", a1, ok1, a2, ok2)
	}
}

func TestRunRejectsUnexpectedArgument(t *testing.T) {
	var stderr bytes.Buffer

	err := run([]string{"extra"}, &stderr)
	if err == nil {
		t.Fatal("run() error = nil, want unexpected argument error")
	}
	if !strings.Contains(err.Error(), "unexpected argument") {
		t.Fatalf("run() error = %v, want unexpected argument", err)
	}
}

func readFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// splitShortcutSections returns the raw text between [action_keys] and
// the next top-level table, and between [jobs_action_keys] and EOF (or
// the next table). It is a deliberately simple parser used only by the
// stub section-isolation assertion.
func splitShortcutSections(t *testing.T, raw string) (action, jobs string) {
	t.Helper()
	const akMarker = "[action_keys]\n"
	const jkMarker = "[jobs_action_keys]\n"
	akStart := strings.Index(raw, akMarker)
	jkStart := strings.Index(raw, jkMarker)
	if akStart < 0 || jkStart < 0 {
		t.Fatalf("stub missing one of the keybindings tables:\n%s", raw)
	}
	action = raw[akStart+len(akMarker) : jkStart]
	jobs = raw[jkStart+len(jkMarker):]
	return action, jobs
}
