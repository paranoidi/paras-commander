package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/rsync"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

func needRsyncBinary(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync not in PATH")
	}
}

// openRsyncOn opens the Rsync dialog on a panel holding the named files (the cursor on the first)
// with the other panel in destDir.
func openRsyncOn(t *testing.T, srcDir, destDir string, names ...string) *App {
	t.Helper()
	for _, n := range names {
		writeFile(t, filepath.Join(srcDir, n))
	}
	app := newApp(t, newScreen(t, 100, 40), srcDir)
	app.model.Secondary.Path = pathloc.MustParse(destDir)
	_ = app.model.Secondary.Refresh(20)
	selectPanelEntryByName(t, app.activePanel(), names[0])
	app.dispatch(keymap.ActionFileRsync)
	if !app.model.FileDialog.Open || app.model.FileDialog.DialogType != dialog.FileDialogRsync {
		t.Fatal("rsync dialog not open")
	}
	return app
}

func TestFileMenuRsyncOpensRsyncDialog(t *testing.T) {
	needRsyncBinary(t)
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "lantern.txt"))
	app := newApp(t, newScreen(t, 100, 40), dir)
	selectPanelEntryByName(t, app.activePanel(), "lantern.txt")

	activateFileMenuItem(t, app, 'R')

	fd := app.model.FileDialog
	if !fd.Open || fd.DialogType != dialog.FileDialogRsync {
		t.Fatalf("dialog open=%v type=%d, want the Rsync dialog", fd.Open, fd.DialogType)
	}
	if !fd.RsyncOptions.Archive || fd.RsyncOptions.Mode != rsync.ModeUpdate {
		t.Fatalf("defaults = %+v, want Archive and Update", fd.RsyncOptions)
	}
	if fd.RsyncAllDirs {
		t.Fatal("a file source must not enable Copy dir contents")
	}
	if preview := strings.Join(fd.RsyncPreview, " "); !strings.HasPrefix(preview, "rsync -a ") || !strings.HasSuffix(preview, "/") {
		t.Fatalf("preview = %q", preview)
	}
}

func TestRsyncDialogWithoutRsyncShowsErrorAndOpensNothing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "lantern.txt"))
	app := newApp(t, newScreen(t, 100, 40), dir)
	app.dialogCtrl.SetLookPathForTest(func(string) (string, error) { return "", exec.ErrNotFound })
	selectPanelEntryByName(t, app.activePanel(), "lantern.txt")

	app.dispatch(keymap.ActionFileRsync)

	if app.model.FileDialog.Open {
		t.Fatal("the dialog must not open without rsync")
	}
	if !strings.Contains(app.model.Message, "rsync not found") {
		t.Fatalf("message = %q", app.model.Message)
	}
}

func TestRsyncDialogPreviewFollowsOptions(t *testing.T) {
	needRsyncBinary(t)
	srcDir, destDir := t.TempDir(), t.TempDir()
	app := openRsyncOn(t, srcDir, destDir, "lantern.txt")
	fd := &app.model.FileDialog

	for _, ev := range []dlg.Event{{ID: dialog.RsyncModeMirror}, {ID: dialog.RsyncChecksum}, {ID: dialog.RsyncDryRun}} {
		app.dialogCtrl.ApplyFileDialogEvent(ev)
	}
	fd.Fields[dialog.RsyncFieldExclude].Value = "*.tmp  node_modules"
	fd.Fields[dialog.RsyncFieldBandwidth].Value = "512"
	fd.Fields[dialog.RsyncFieldExtra].Value = "--hard-links --chmod='D755'"
	app.dialogCtrl.ApplyFileDialogEvent(dlg.Event{ID: dialog.RsyncCompress})

	want := "rsync -a -c -z -n --delete --bwlimit=512 '--exclude=*.tmp' --exclude=node_modules --hard-links --chmod=D755 " + filepath.Join(srcDir, "lantern.txt") + " " + destDir + "/"
	if fd.RsyncError != "" || strings.Join(fd.RsyncPreview, " ") != want {
		t.Fatalf("preview = %q (error %q)\nwant      %q", strings.Join(fd.RsyncPreview, " "), fd.RsyncError, want)
	}

	fd.Fields[dialog.RsyncFieldBandwidth].Value = "fast"
	app.dialogCtrl.ApplyFileDialogEvent(dlg.Event{ID: dialog.RsyncCompress})
	if !strings.Contains(fd.RsyncError, "Bandwidth limit") || len(fd.RsyncPreview) != 0 {
		t.Fatalf("error = %q preview = %q, want the bandwidth error in place of the preview", fd.RsyncError, fd.RsyncPreview)
	}
}

func TestRsyncDialogEnqueuesRsyncJobAndSyncs(t *testing.T) {
	needRsyncBinary(t)
	srcDir, destDir := t.TempDir(), t.TempDir()
	app := openRsyncOn(t, srcDir, destDir, "lantern.txt", "willow.txt")
	app.activePanel().AddSelection(filepath.Join(srcDir, "lantern.txt"))

	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	flushBackgroundJobs(t, app)

	all := app.jobState.AllJobs()
	if len(all) != 1 || all[0].Type != jobs.TypeRsync {
		t.Fatalf("jobs = %v, want one rsync job", all)
	}
	if all[0].Destination.String() != destDir || !all[0].DestIsDir {
		t.Fatalf("destination = %q isDir = %v", all[0].Destination, all[0].DestIsDir)
	}
	waitUntilAppJobsFinished(t, app, 10*time.Second)
	if all[0].Status != jobs.StatusCompleted {
		t.Fatalf("status = %s error = %q", all[0].Status, all[0].Error)
	}
	if _, err := os.Stat(filepath.Join(destDir, "lantern.txt")); err != nil {
		t.Fatalf("file not synced: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destDir, "willow.txt")); err == nil {
		t.Fatal("only the selection is synced, not the cursor entry")
	}
}

func TestRsyncDialogRemembersLastOptionsForTheSession(t *testing.T) {
	needRsyncBinary(t)
	srcDir, destDir := t.TempDir(), t.TempDir()
	app := openRsyncOn(t, srcDir, destDir, "lantern.txt", "orchard.txt")
	app.dialogCtrl.ApplyFileDialogEvent(dlg.Event{ID: dialog.RsyncModeMove})
	app.dialogCtrl.ApplyFileDialogEvent(dlg.Event{ID: dialog.RsyncArchive})
	app.model.FileDialog.Fields[dialog.RsyncFieldExclude].Value = "*.tmp"
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	flushBackgroundJobs(t, app)

	selectPanelEntryByName(t, app.activePanel(), "orchard.txt")
	app.dispatch(keymap.ActionFileRsync)

	fd := app.model.FileDialog
	if fd.RsyncOptions.Mode != rsync.ModeMove || fd.RsyncOptions.Archive || fd.Fields[dialog.RsyncFieldExclude].Value != "*.tmp" {
		t.Fatalf("options = %+v exclude = %q, want the last used ones", fd.RsyncOptions, fd.Fields[dialog.RsyncFieldExclude].Value)
	}
}

func TestRsyncDialogKeepsOpenOnInvalidOptions(t *testing.T) {
	needRsyncBinary(t)
	app := openRsyncOn(t, t.TempDir(), t.TempDir(), "lantern.txt")
	app.model.FileDialog.Fields[dialog.RsyncFieldBandwidth].Value = "-3"

	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if !app.model.FileDialog.Open || len(app.jobState.AllJobs()) != 0 {
		t.Fatal("an invalid bandwidth limit must keep the dialog open and queue nothing")
	}
	if !strings.Contains(app.model.Message, "Bandwidth limit") {
		t.Fatalf("message = %q", app.model.Message)
	}
}

func TestRsyncDryRunKeepsChangesAsJobResults(t *testing.T) {
	needRsyncBinary(t)
	srcDir, destDir := t.TempDir(), t.TempDir()
	app := openRsyncOn(t, srcDir, destDir, "lantern.txt")
	app.dialogCtrl.ApplyFileDialogEvent(dlg.Event{ID: dialog.RsyncDryRun})
	app.dialogCtrl.HandleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	var task *jobs.Task
	drainInterruptEventsUntil(t, app, app.screen.(tcell.SimulationScreen), 10*time.Second, func() bool {
		app.jobsCtrl.PollEvents()
		for _, j := range app.jobState.AllJobs() {
			task = j.Task
		}
		return task != nil && task.Results != nil
	})
	if task.Results.Label != "1 change" || task.Results.Show == nil {
		t.Fatalf("results = %+v, want 1 change with a Show", task.Results)
	}
	task.Results.Show()
	out := app.model.CommandOutputDialog
	if !out.Open || len(out.Lines) != 1 || !strings.HasSuffix(out.Lines[0], "lantern.txt") {
		t.Fatalf("output dialog = %+v", out)
	}
	if _, err := os.Stat(filepath.Join(destDir, "lantern.txt")); err == nil {
		t.Fatal("a dry run must not copy")
	}
}

func TestRsyncDialogTranslatesSFTPDestinationForSSH(t *testing.T) {
	needRsyncBinary(t)
	srcDir := t.TempDir()
	app := openRsyncOn(t, srcDir, t.TempDir(), "lantern.txt")
	app.model.FileDialog.Fields[dialog.RsyncFieldDest].Value = "sftp://ann@willow.example:2222/srv/meadow"
	app.dialogCtrl.ApplyFileDialogEvent(dlg.Event{ID: dialog.RsyncCompress})

	fd := app.model.FileDialog
	want := "rsync -a -z -e 'ssh -o BatchMode=yes -p 2222' " + filepath.Join(srcDir, "lantern.txt") + " ann@willow.example:/srv/meadow/"
	if fd.RsyncError != "" || strings.Join(fd.RsyncPreview, " ") != want {
		t.Fatalf("preview = %q (error %q)\nwant      %q", strings.Join(fd.RsyncPreview, " "), fd.RsyncError, want)
	}
}
