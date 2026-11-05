package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestMetaDialogF5SavesDefaultColumns(t *testing.T) {
	cfgDir := filepath.Join(t.TempDir(), "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgFile := filepath.Join(cfgDir, "config.toml")
	if err := os.WriteFile(cfgFile, []byte("theme = \"lantern\"\n\n[panels]\nwatch_directories = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a, _, _, _ := fileIndexAppWith(t, func(o *Options) { o.Paths = config.Paths{ConfigDir: cfgDir} })

	a.metaCtrl.OpenDialog(ui.PrimaryPanel)
	st := &a.model.MetaDialog
	if !st.Open || len(st.Entries) < 3 {
		t.Fatalf("meta dialog open=%v with %d entries, want provider entries", st.Open, len(st.Entries))
	}
	assertFooterSave(t, a)

	press := func() {
		t.Helper()
		if quit, _ := a.handleKey(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone)); quit {
			t.Fatal("F5 quit the app")
		}
		if !a.model.MetaDialog.Open || a.inputMode() != InputModeMetaDialog {
			t.Fatal("meta dialog closed or lost focus on save")
		}
		if a.model.FileDialog.Open {
			t.Fatal("F5 started a copy dialog")
		}
		a.jobsCtrl.SyncJobsList()
		if n := len(a.model.JobsList); n != 0 {
			t.Fatalf("F5 queued %d jobs", n)
		}
	}
	load := func() []string {
		t.Helper()
		cfg, err := config.LoadFromPaths(config.Paths{ConfigDir: cfgDir})
		if err != nil {
			t.Fatal(err)
		}
		return cfg.FileIndex.DefaultColumns
	}

	st.Checked[0], st.Checked[2] = true, true
	want := []string{st.Entries[0].Name, st.Entries[2].Name}
	press()
	if got := load(); !slices.Equal(got, want) {
		t.Fatalf("saved default_columns = %v, want %v", got, want)
	}
	if !slices.Equal(a.config.FileIndex.DefaultColumns, want) {
		t.Fatalf("in-memory default_columns = %v, want %v", a.config.FileIndex.DefaultColumns, want)
	}
	if len(a.model.MetaResults[ui.PrimaryPanel]) != 0 {
		t.Fatal("save applied the selection to the panel")
	}
	raw, err := os.ReadFile(cfgFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "lantern") || !strings.Contains(string(raw), "watch_directories = false") {
		t.Fatalf("other keys lost:\n%s", raw)
	}

	st.Checked[0] = false
	want = []string{st.Entries[2].Name}
	press()
	if got := load(); !slices.Equal(got, want) {
		t.Fatalf("second save default_columns = %v, want %v", got, want)
	}

	st.Checked[2] = false
	press()
	raw, _ = os.ReadFile(cfgFile)
	if !strings.Contains(string(raw), "default_columns = []") {
		t.Fatalf("empty selection not written as an empty list:\n%s", raw)
	}
	if got := load(); got == nil || len(got) != 0 {
		t.Fatalf("empty save loaded as %#v, want empty non-nil", got)
	}
}

func TestMetaDialogSaveWithoutConfigPathReportsError(t *testing.T) {
	a, _, _, _ := fileIndexApp(t)
	a.metaCtrl.OpenDialog(ui.PrimaryPanel)
	before := slices.Clone(a.config.FileIndex.DefaultColumns)
	a.model.MetaDialog.Checked[0] = true
	a.handleKey(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone))
	if !slices.Equal(a.config.FileIndex.DefaultColumns, before) {
		t.Fatalf("config changed without a persist path: %v", a.config.FileIndex.DefaultColumns)
	}
}

func assertFooterSave(t *testing.T, a *App) {
	t.Helper()
	for _, k := range a.activeFooterKeys() {
		if k.Hint == "Save" {
			ev, ok := k.Event()
			if !ok || ev.Key() != tcell.KeyF5 {
				t.Fatalf("Save footer replays %v, want F5", ev)
			}
			return
		}
	}
	t.Fatal("meta dialog footer has no Save entry")
}
