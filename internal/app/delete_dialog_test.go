package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func TestDeleteDialogDescendIntoMountPoints(t *testing.T) {
	t.Parallel()
	if !deleteDialogDescendIntoMountPoints {
		t.Fatal("delete dialog must scan across mount points (Samba, etc.)")
	}
}

func TestDeleteDialogSummaryRefreshesAfterDiskScanFlush(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "payload")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(sub, "one.dat"))
	writeFile(t, filepath.Join(sub, "two.dat"))

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	p := app.activePanel()
	source := ops.Source{
		Kind: ops.SourceCursor,
		Entries: []localfs.Entry{{
			Name: "payload", Path: sub, Type: localfs.EntryDirectory,
		}},
	}
	app.model.FileDialog = dialog.FileDialogState{
		Open:          true,
		DialogType:    dialog.FileDialogDelete,
		DeleteSummary: app.deleteDialogSummary(p, source),
		FocusedField:  1,
	}

	app.deleteDialogScanFP = ""
	app.diskUsage.StartScanFromListing([]string{sub}, app.diskUsageIgnore, app.model.ActivePanel, diskusage.ListingVolumeGate{})
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		app.pollDiskUsageUpdates()
		if !app.diskUsageScanBusy() {
			if _, ok := app.diskUsage.ByteSize(sub); ok {
				break
			}
		}
		time.Sleep(2 * time.Millisecond)
	}
	if _, ok := app.diskUsage.ByteSize(sub); !ok {
		t.Fatal("disk usage cache never indexed delete target")
	}

	summary := app.model.FileDialog.DeleteSummary
	if n, _ := app.diskUsage.FileCount(sub); n >= 2 {
		if strings.HasPrefix(summary, "0 files") {
			t.Fatalf("summary = %q after scan flush; cache has %d files", summary, n)
		}
	}
}

func TestDeleteDialogSummaryRefreshesWhenScanNoLongerNeeded(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "cached")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(sub, "a.dat"))

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	app.diskUsage.StartScanFromListing([]string{sub}, app.diskUsageIgnore, app.model.ActivePanel, diskusage.ListingVolumeGate{})
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		app.pollDiskUsageUpdates()
		if !app.diskUsageScanBusy() {
			if _, ok := app.diskUsage.ByteSize(sub); ok {
				break
			}
		}
		time.Sleep(2 * time.Millisecond)
	}

	p := app.activePanel()
	p.SelectedPaths = map[string]bool{sub: true}
	app.model.FileDialog = dialog.FileDialogState{
		Open:          true,
		DialogType:    dialog.FileDialogDelete,
		DeleteSummary: "0 files (0 B) stale",
		FocusedField:  1,
	}
	app.reconcileDeleteDialogScans()
	if got := app.model.FileDialog.DeleteSummary; got == "0 files (0 B) stale" {
		t.Fatalf("reconcile should refresh summary from cache, still %q", got)
	}
}

func TestDeleteDialogSummaryIgnoresStaleCacheAfterFilesMovedOut(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "payload")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	f1 := filepath.Join(sub, "one.dat")
	f2 := filepath.Join(sub, "two.dat")
	writeFile(t, f1)
	writeFile(t, f2)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	app.diskUsage.StartScanFromListing([]string{sub}, app.diskUsageIgnore, app.model.ActivePanel, diskusage.ListingVolumeGate{})
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		app.pollDiskUsageUpdates()
		if !app.diskUsageScanBusy() {
			if n, ok := app.diskUsage.FileCount(sub); ok && n >= 2 {
				break
			}
		}
		time.Sleep(2 * time.Millisecond)
	}
	if n, ok := app.diskUsage.FileCount(sub); !ok || n < 2 {
		t.Fatalf("expected cached file count >= 2, got ok=%v n=%d", ok, n)
	}

	dest := filepath.Join(root, "elsewhere")
	if err := os.Mkdir(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(f1, filepath.Join(dest, "one.dat")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(f2, filepath.Join(dest, "two.dat")); err != nil {
		t.Fatal(err)
	}
	if n, _ := app.diskUsage.FileCount(sub); n < 2 {
		t.Fatalf("precondition: cache should still report stale count, got %d", n)
	}

	p := app.activePanel()
	p.SelectedPaths = map[string]bool{sub: true}
	app.openDeleteDialog(p)

	if strings.HasPrefix(app.model.FileDialog.DeleteSummary, "2 files") {
		t.Fatalf("summary = %q; should not trust stale cache after files moved out", app.model.FileDialog.DeleteSummary)
	}

	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		app.pollDiskUsageUpdates()
		app.reconcileDeleteDialogScans()
		if !app.diskUsageScanBusy() {
			if n, ok := app.diskUsage.FileCount(sub); ok && n == 0 {
				break
			}
		}
		time.Sleep(2 * time.Millisecond)
	}
	if got := app.model.FileDialog.DeleteSummary; !strings.HasPrefix(got, "0 files") {
		t.Fatalf("summary = %q after rescan; want 0 files", got)
	}
}
