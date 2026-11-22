package keymap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
)

func TestCopiesOverlayDefaultsAndRebinding(t *testing.T) {
	b, err := DefaultBundle()
	if err != nil {
		t.Fatal(err)
	}
	for ev, want := range map[*tcell.EventKey]string{
		tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone):  ActionCopiesClose,
		tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone): ActionCopiesPrevPane,
		tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModShift):   ActionOpenInPrimary,
		tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModShift):  ActionOpenInSecondary,
	} {
		if got, _ := b.Copies.Lookup(ev); got != want {
			t.Errorf("[copies] %v = %q, want %q", ev.Name(), got, want)
		}
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "keybindings.toml")
	write := func(body string) error {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := LoadFromPaths(config.Paths{ConfigDir: dir, KeybindingsFile: path})
		return err
	}
	if err := write("[copies]\ncopies.close = [\"q\"]\n"); err != nil {
		t.Fatalf("rebinding copies.close: %v", err)
	}
	if err := write("[copies]\nunique.close = [\"q\"]\n"); err == nil {
		t.Fatal("a unique.* action in [copies] must be rejected")
	}
}
