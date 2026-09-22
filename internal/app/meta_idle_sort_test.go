package app

import (
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// TestMaybeScheduleMetaIdleSort_requiresColumnResolved confirms the idle re-sort timer only arms
// once every dispatched entry's meta command has finished (PendingCount back to 0) — a still-
// running column must not schedule a re-sort. The generic arm/apply/invalidate/epoch mechanism
// itself is exercised by the disk-usage idle sort tests (diskusage_sort_test.go); this only
// covers the meta-specific readiness gate.
func TestMaybeScheduleMetaIdleSort_requiresColumnResolved(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "aardvark.txt"))
	writeFile(t, filepath.Join(root, "bramble.txt"))

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	left := app.panelByID(ui.PrimaryPanel)
	left.Sort.Mode = panel.SortMeta
	left.Sort.MetaColumn = "info"

	const pending = "*"
	app.model.MetaResults[ui.PrimaryPanel] = []ui.MetaColumnState{
		{
			EntryName:    "info",
			Pending:      pending,
			PendingCount: 1,
			Results: map[string]string{
				filepath.Join(root, "aardvark.txt"): pending,
				filepath.Join(root, "bramble.txt"):  "1",
			},
		},
	}

	app.maybeScheduleMetaIdleSort(ui.PrimaryPanel)
	if app.metaSort.idleSort[ui.PrimaryPanel].timer != nil {
		t.Fatal("timer armed while a cell is still pending")
	}

	// Last cell resolves.
	app.model.MetaResults[ui.PrimaryPanel][0].Results[filepath.Join(root, "aardvark.txt")] = "2"
	app.model.MetaResults[ui.PrimaryPanel][0].PendingCount = 0
	app.maybeScheduleMetaIdleSort(ui.PrimaryPanel)
	if app.metaSort.idleSort[ui.PrimaryPanel].timer == nil {
		t.Fatal("expected timer armed once the column fully resolved")
	}
	app.invalidateMetaIdleSortPanel(ui.PrimaryPanel)
}

// TestApplyMetaIdleSort_reordersOnceResolved confirms the deferred apply actually reorders the
// panel by the meta column once it fires.
func TestApplyMetaIdleSort_reordersOnceResolved(t *testing.T) {
	root := t.TempDir()
	aPath := filepath.Join(root, "aardvark.txt")
	bPath := filepath.Join(root, "bramble.txt")
	writeFile(t, aPath)
	writeFile(t, bPath)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	left := app.panelByID(ui.PrimaryPanel)
	left.Sort.Mode = panel.SortMeta
	left.Sort.MetaColumn = "info"

	app.model.MetaResults[ui.PrimaryPanel] = []ui.MetaColumnState{
		{
			EntryName: "info",
			Pending:   "*",
			Results: map[string]string{
				aPath: "1",
				bPath: "9",
			},
		},
	}

	ep := app.metaSort.idleSort[ui.PrimaryPanel].epoch
	app.applyMetaIdleSort(ui.PrimaryPanel, ep)

	if len(left.Entries) < 2 {
		t.Fatalf("expected 2 entries, got %d", len(left.Entries))
	}
	if left.Entries[0].Name != "bramble.txt" {
		t.Fatalf("Entries[0] = %q, want bramble.txt (numeric meta sorts high-first: 9 before 1)", left.Entries[0].Name)
	}
}
