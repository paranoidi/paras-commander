package dialog

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	previewctrl "github.com/paranoidi/paras-commander/internal/apphandler/preview"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
	uidialog "github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/uitest"
)

type navRecordHost struct {
	identityTestHost
	navigated []string
}

func (f *navRecordHost) NavigatePanelToPath(_ int, path, _ string) error {
	f.navigated = append(f.navigated, path)
	return nil
}

// newTreeNestedHarness lists root with the tree layout, expands its single directory and parks the
// cursor on the nested file inside it.
func newTreeNestedHarness(t *testing.T) (*Handler, *navRecordHost, string, string) {
	t.Helper()
	root := t.TempDir()
	sub := filepath.Join(root, "orchard")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(sub, "willow.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	model := &ui.Model{}
	if err := model.Primary.Load(root); err != nil {
		t.Fatal(err)
	}
	if err := model.Secondary.Load(root); err != nil {
		t.Fatal(err)
	}
	if !model.Primary.SetListLayout(panel.ListLayoutTree, 20) {
		t.Fatal("SetListLayout(Tree) = false")
	}
	if err := model.Primary.ExpandTreeCursorRow(20); err != nil {
		t.Fatal(err)
	}
	if !model.Primary.SelectVisibleEntryPathCentered(file, 20) {
		t.Fatal("nested file not visible")
	}
	cfg := config.Default()
	cfg.Preview.Prefetch = false
	host := &navRecordHost{identityTestHost: identityTestHost{model: model, cfg: cfg}}
	var screen tcell.Screen = uitest.Screen(t, 80, 24)
	h := New(Deps{
		Host: host, Screen: screen, Model: model,
		Preview: previewctrl.New(previewctrl.Deps{
			Host: host, Screen: screen, Model: model, Ctx: context.Background(),
		}),
	})
	return h, host, sub, file
}

func TestMkdirInTreeCreatesNextToNestedCursorRow(t *testing.T) {
	h, host, sub, _ := newTreeNestedHarness(t)
	h.model.FileDialog = uidialog.FileDialogState{
		Open:       true,
		DialogType: uidialog.FileDialogMkdir,
		Fields:     []uidialog.FileDialogField{{Label: "Name", Value: "meadow"}},
	}
	h.executeMkdir()
	if len(host.errors) > 0 {
		t.Fatalf("errors: %v", host.errors)
	}
	want := filepath.Join(sub, "meadow")
	if fi, err := os.Stat(want); err != nil || !fi.IsDir() {
		t.Fatalf("expected directory %s: %v", want, err)
	}
	cur, ok := host.model.Primary.CurrentEntry()
	if !ok || filepath.Clean(cur.Path) != want {
		t.Fatalf("cursor on %q (ok=%v), want %q", cur.Path, ok, want)
	}
}

func TestRenameOpenInOtherUsesNestedNewPath(t *testing.T) {
	h, host, sub, _ := newTreeNestedHarness(t)
	inner := filepath.Join(sub, "brook")
	if err := os.Mkdir(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	h.model.FileDialog = uidialog.FileDialogState{
		Open:              true,
		DialogType:        uidialog.FileDialogRename,
		Fields:            []uidialog.FileDialogField{{Label: "Name", Value: "meadow"}},
		RenameSource:      inner,
		RenameSourceIsDir: true,
		RenameOpenInOther: true,
		RenameFocusAfter:  false,
	}
	h.executeRename()
	want := filepath.Join(sub, "meadow")
	if len(host.navigated) != 1 || filepath.Clean(host.navigated[0]) != want {
		t.Fatalf("navigated = %v, want [%s]", host.navigated, want)
	}
}

func TestPendingFocusSelectsNestedPath(t *testing.T) {
	h, host, sub, file := newTreeNestedHarness(t)
	p := &host.model.Primary
	p.Cursor = 0
	h.schedulePanelFocus(ui.PrimaryPanel, p.PathString(), "willow.txt", file)
	h.applyPendingPanelFocus()
	cur, ok := p.CurrentEntry()
	if !ok || filepath.Clean(cur.Path) != file {
		t.Fatalf("cursor on %q, want %q (sub=%s)", cur.Path, file, sub)
	}
}
