package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

func TestSetErrorMessageOpsErrorNoDuplicatePrefix(t *testing.T) {
	t.Parallel()
	app := testAppMinimal(t)
	err := &ops.Error{Op: "mkdir", Text: "target already exists"}
	app.setErrorMessage("Mkdir", err)
	if app.model.Message != "mkdir: target already exists" {
		t.Fatalf("message = %q, want mkdir: target already exists", app.model.Message)
	}
}

func TestSetErrorMessageExecuteFailureNoDuplicatePrefix(t *testing.T) {
	t.Parallel()
	cases := []struct {
		prefix string
		err    error
		want   string
	}{
		{
			prefix: "Mkdir failed",
			err:    fmt.Errorf(`mkdir "/tmp/x": file exists`),
			want:   `mkdir "/tmp/x": file exists`,
		},
		{
			prefix: "Rename failed",
			err:    fmt.Errorf(`rename "/a" to "/b": permission denied`),
			want:   `rename "/a" to "/b": permission denied`,
		},
		{
			prefix: "Hardlink failed",
			err:    fmt.Errorf(`link "/dst" -> "/src": file exists`),
			want:   `link "/dst" -> "/src": file exists`,
		},
		{
			prefix: "Mass rename failed",
			err:    fmt.Errorf(`mass rename stage1 "/a": permission denied`),
			want:   `mass rename stage1 "/a": permission denied`,
		},
		{
			prefix: "Chmod failed",
			err:    &ops.Error{Op: "chmod", Text: "failed to change mode for foo", Err: errors.New(`chmod "/p": permission denied`)},
			want:   `chmod: failed to change mode for foo (chmod "/p": permission denied)`,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.prefix, func(t *testing.T) {
			t.Parallel()
			app := testAppMinimal(t)
			app.setErrorMessage(tc.prefix, tc.err)
			if app.model.Message != tc.want {
				t.Fatalf("message = %q, want %q", app.model.Message, tc.want)
			}
		})
	}
}

func TestSetErrorMessageKeepsUnrelatedPrefix(t *testing.T) {
	t.Parallel()
	app := testAppMinimal(t)
	err := fmt.Errorf(`read directory "/tmp": permission denied`)
	app.setErrorMessage("Enter failed", err)
	want := `Enter failed: read directory "/tmp": permission denied`
	if app.model.Message != want {
		t.Fatalf("message = %q, want %q", app.model.Message, want)
	}
}

func TestShouldOmitErrorPrefix(t *testing.T) {
	t.Parallel()
	if !shouldOmitErrorPrefix("Mkdir failed", fmt.Errorf(`mkdir "/x": exists`)) {
		t.Fatal("expected omit for mkdir execute error")
	}
	if shouldOmitErrorPrefix("Enter failed", fmt.Errorf(`read directory "/x": denied`)) {
		t.Fatal("expected keep prefix for unrelated enter error")
	}
}

func TestTransientErrorTextPermissionDenied(t *testing.T) {
	wrapped := fmt.Errorf(`read directory "/home/nella": %w`, fs.ErrPermission)
	if got := transientErrorText(wrapped); got != "permission denied" {
		t.Fatalf("transientErrorText() = %q, want permission denied", got)
	}
	if got := transientErrorText(errors.New("other")); got != "other" {
		t.Fatalf("transientErrorText() = %q, want literal message when not ErrPermission", got)
	}
}

func TestJobFailureBannerDetail(t *testing.T) {
	t.Parallel()
	wrapped := fmt.Errorf(`create directory "/very/long/path/nested": %w`, fs.ErrPermission)
	if got := jobFailureBannerDetail(wrapped, wrapped.Error()); got != "permission denied" {
		t.Fatalf("jobFailureBannerDetail(wrapped) = %q, want permission denied", got)
	}
	if got := jobFailureBannerDetail(nil, `open "/tmp/x": permission denied`); got != "permission denied" {
		t.Fatalf("jobFailureBannerDetail(nil, ...) = %q, want permission denied", got)
	}
	long := strings.Repeat("a", 120)
	got := jobFailureBannerDetail(errors.New(long), "")
	if got == "" || strings.TrimSuffix(got, "…") == got {
		t.Fatalf("expected truncated banner with ellipsis, got %q", got)
	}
	if utf8.RuneCountInString(got) != jobFailureBannerMaxRunes+1 {
		t.Fatalf("len runes = %d, want %d", utf8.RuneCountInString(got), jobFailureBannerMaxRunes+1)
	}
}

func TestJobFailureLogDetailKeepsFullError(t *testing.T) {
	t.Parallel()
	err := &ops.Error{
		Op:   "delete",
		Text: "failed to delete stale-node",
		Err:  errors.New("directory not empty"),
	}
	errText := err.Error()
	if got := jobFailureLogDetail(err, errText); got != errText {
		t.Fatalf("jobFailureLogDetail() = %q, want %q", got, errText)
	}
}

func TestMessageLogNewestFirst(t *testing.T) {
	t.Parallel()
	app := testAppMinimal(t)
	app.setTransientMessage("first", ui.MessageUrgencyInfo)
	app.setTransientMessage("second", ui.MessageUrgencyInfo)
	if len(app.model.MessageLog) < 2 {
		t.Fatalf("log len = %d, want at least 2", len(app.model.MessageLog))
	}
	if app.model.MessageLog[0].Text != "second" {
		t.Fatalf("log[0] = %q, want newest message first", app.model.MessageLog[0].Text)
	}
	if app.model.MessageLog[len(app.model.MessageLog)-1].Text != "first" {
		t.Fatalf("log[last] = %q, want oldest message last", app.model.MessageLog[len(app.model.MessageLog)-1].Text)
	}
}

func TestClearMessageLog(t *testing.T) {
	t.Parallel()
	app := testAppMinimal(t)
	app.setTransientMessage("hello", ui.MessageUrgencyInfo)
	app.clearMessageLog()
	if len(app.model.MessageLog) != 0 {
		t.Fatalf("MessageLog len = %d, want 0", len(app.model.MessageLog))
	}
	if app.model.Message != "" {
		t.Fatalf("banner = %q, want empty", app.model.Message)
	}
}

func TestSetJobFailedTransientMessageLogsFullError(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 120, 24)
	app := newApp(t, screen, dir)

	err := &ops.Error{
		Op:   "delete",
		Text: "failed to delete stale-node",
		Err:  errors.New("directory not empty"),
	}
	errText := err.Error()
	app.setTransientMessageBanner(
		fmt.Sprintf("Job failed: %s", jobFailureLogDetail(err, errText)),
		fmt.Sprintf("Job failed: %s", jobFailureBannerDetail(err, errText)),
		ui.MessageUrgencyError,
	)
	if len(app.model.MessageLog) == 0 {
		t.Fatal("expected message log entry")
	}
	full := strings.Join(func() []string {
		var parts []string
		for _, e := range app.model.MessageLog {
			parts = append(parts, e.Text)
		}
		return parts
	}(), " ")
	if !strings.Contains(full, "directory not empty") {
		t.Fatalf("log = %q, want full error including reason", full)
	}
	if strings.Contains(app.model.Message, "remove \"") {
		t.Fatalf("banner should omit repeated remove paths, got %q", app.model.Message)
	}
	if utf8.RuneCountInString(app.model.Message) > jobFailureBannerMaxRunes+len("Job failed: ") {
		t.Fatalf("banner too long, got %q", app.model.Message)
	}
}

func TestFilePanelPlusMinusStarSelectionShortcuts(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	t.Run("minus opens unselect dialog without quick filter", func(t *testing.T) {
		if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, '-', tcell.ModNone)); quit {
			t.Fatal("handleKey('-') quit = true")
		}
		if !app.model.GroupSelect.Open || app.model.GroupSelect.Mode != "unselect" {
			t.Fatalf("want unselect group dialog, got %+v", app.model.GroupSelect)
		}
		f := app.activePanel().Filter
		if f.Active || f.Editing || f.Query != "" {
			t.Fatalf("quick filter should stay off, got %+v", f)
		}
		app.handleKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
		if app.model.GroupSelect.Open {
			t.Fatal("dialog should close on Esc")
		}
	})

	t.Run("plus opens select dialog with or without shift", func(t *testing.T) {
		for _, ev := range []*tcell.EventKey{
			tcell.NewEventKey(tcell.KeyRune, '+', tcell.ModNone),
			tcell.NewEventKey(tcell.KeyRune, '+', tcell.ModShift),
		} {
			if quit, _ := app.handleKey(ev); quit {
				t.Fatalf("handleKey(%+v) quit", ev)
			}
			if !app.model.GroupSelect.Open || app.model.GroupSelect.Mode != "select" {
				t.Fatalf("ev %+v: want select dialog, got %+v", ev, app.model.GroupSelect)
			}
			app.handleKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
		}
	})

	t.Run("star inverts selection", func(t *testing.T) {
		if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, '*', tcell.ModShift)); quit {
			t.Fatal("handleKey('*') quit = true")
		}
		if !strings.Contains(app.model.Message, "Selection inverted") {
			t.Fatalf("status message = %q, want selection inverted", app.model.Message)
		}
		f := app.activePanel().Filter
		if f.Active || f.Editing {
			t.Fatalf("invert must not open quick filter, got %+v", f)
		}
	})
}

func TestGroupSelectPatternCtrlLAndWordNav(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openGroupSelect("select")

	for _, r := range "ab cd" {
		app.handleGroupSelectKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	gs := &app.model.GroupSelect
	if gs.Text != "ab cd" || gs.TextCursor != 5 {
		t.Fatalf("after type: text=%q cursor=%d", gs.Text, gs.TextCursor)
	}

	app.handleGroupSelectKey(tcell.NewEventKey(tcell.KeyRune, 'b', tcell.ModAlt))
	if gs.TextCursor != 3 {
		t.Fatalf("after Alt+b: cursor=%d want 3", gs.TextCursor)
	}

	app.handleGroupSelectKey(tcell.NewEventKey(tcell.KeyCtrlL, 0, tcell.ModNone))
	if gs.Text != "" || gs.TextCursor != 0 || gs.TextScroll != 0 {
		t.Fatalf("after Ctrl+L: text=%q cursor=%d scroll=%d", gs.Text, gs.TextCursor, gs.TextScroll)
	}
}

func TestGroupSelectEnterOnPatternInputConfirms(t *testing.T) {
	dir := t.TempDir()
	foo := filepath.Join(dir, "foo.txt")
	bar := filepath.Join(dir, "bar.txt")
	writeFile(t, foo)
	writeFile(t, bar)
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openGroupSelect("select")
	for _, r := range "*.txt" {
		app.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	if app.model.GroupSelect.Focus != 0 {
		t.Fatalf("focus = %d, want 0 (pattern input)", app.model.GroupSelect.Focus)
	}
	app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if app.model.GroupSelect.Open {
		t.Fatal("Enter on pattern input should close dialog")
	}
	p := app.activePanel()
	if p.SelectedPaths == nil || !p.SelectedPaths[foo] || !p.SelectedPaths[bar] {
		t.Fatalf("selection after Enter = %v, want foo.txt and bar.txt", p.SelectedPaths)
	}
}

func TestGroupSelectPlainTypingDoesNotTriggerShortcuts(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.openGroupSelect("select")

	for _, r := range "focus" {
		app.handleGroupSelectKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	if got := app.model.GroupSelect.Text; got != "focus" {
		t.Fatalf("pattern = %q, want focus", got)
	}
	if app.model.GroupSelect.FilesOnly || app.model.GroupSelect.CaseSensitive || !app.model.GroupSelect.UseShellPatterns {
		t.Fatalf("checkbox state changed unexpectedly: %+v", app.model.GroupSelect)
	}

	app.handleGroupSelectKey(tcell.NewEventKey(tcell.KeyRune, 'F', tcell.ModShift))
	if got := app.model.GroupSelect.Text; got != "focusF" {
		t.Fatalf("pattern after shifted letter = %q, want focusF", got)
	}

	app.handleGroupSelectKey(tcell.NewEventKey(tcell.KeyRune, 'f', tcell.ModAlt))
	if !app.model.GroupSelect.FilesOnly || app.model.GroupSelect.Focus != 1 {
		t.Fatalf("Alt+F should toggle Files only and focus row; got FilesOnly=%v focus=%d",
			app.model.GroupSelect.FilesOnly, app.model.GroupSelect.Focus)
	}
}

func TestMenuBarPermissionHiddenInJobsView(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "x.txt"))
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.model.ViewMode = ui.ViewJobs
	if got := app.menuBarPermissionText(); got != "" {
		t.Fatalf("jobs view: menuBarPermissionText() = %q, want empty", got)
	}

	app.model.ViewMode = ui.ViewBrowser
	if got := app.menuBarPermissionText(); got == "" {
		t.Fatal("browser view: menuBarPermissionText should show mode for selected entry")
	}
}

func TestHelpViewEnterRunsCopyLikeKeyboardShortcut(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.openHelpDialog()
	if !app.model.HelpView.Open {
		t.Fatal("HelpView should open")
	}

	copyEntryIdx := -1
	for i, e := range app.model.HelpView.Entries {
		if e.ActionID == keymap.ActionCopy {
			copyEntryIdx = i
			break
		}
	}
	if copyEntryIdx < 0 {
		t.Fatal("help entries should include Copy action")
	}
	sel := -1
	for i, idx := range app.model.HelpView.Ranked {
		if idx == copyEntryIdx {
			sel = i
			break
		}
	}
	if sel < 0 {
		t.Fatal("ranked list should include Copy entry")
	}
	app.model.HelpView.Selected = sel
	app.model.HelpView.Focus = 0

	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); quit {
		t.Fatal("Enter on Copy should not quit")
	}
	if app.model.HelpView.Open {
		t.Fatal("HelpView should close after activating action")
	}
	if !app.model.TransferDialog.Open || app.model.TransferDialog.Kind != ui.TransferKindCopy {
		t.Fatal("Copy dialog should open (keyboard parity)")
	}
}

func TestCopyMoveClearsSelectionOnlyWhenQueued(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))
	dstDir := filepath.Join(dir, "dest")
	if err := os.Mkdir(dstDir, 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	if err := app.inactivePanel().Load(dstDir); err != nil {
		t.Fatalf("inactive Load: %v", err)
	}
	defer app.stopWorker()
	defer flushBackgroundJobs(t, app)

	p := app.activePanel()
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone)); quit {
		t.Fatal("unexpected quit")
	}
	if len(p.SelectedPaths) == 0 {
		t.Fatal("expected current entry tagged after Insert")
	}

	app.openCopyDialog()
	if !app.model.TransferDialog.Open || app.model.TransferDialog.Kind != ui.TransferKindCopy {
		t.Fatal("copy dialog should open")
	}
	if len(p.SelectedPaths) == 0 {
		t.Fatal("opening copy dialog must not clear selection")
	}
	app.handleTransferDialogKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if app.model.TransferDialog.Open {
		t.Fatal("copy dialog should close on Esc")
	}
	if len(p.SelectedPaths) == 0 {
		t.Fatal("canceling copy dialog must not clear selection")
	}

	app.openCopyDialog()
	app.handleTransferDialogKey(tcell.NewEventKey(tcell.KeyRune, 'C', tcell.ModAlt))
	if app.model.TransferDialog.Open {
		t.Fatal("copy dialog should close on Alt+C")
	}
	if len(p.SelectedPaths) == 0 {
		t.Fatal("canceling copy dialog with Alt+C must not clear selection")
	}

	app.openMoveDialog()
	if !app.model.TransferDialog.Open || app.model.TransferDialog.Kind != ui.TransferKindMove {
		t.Fatal("move dialog should open")
	}
	if len(p.SelectedPaths) == 0 {
		t.Fatal("opening move dialog must not clear selection")
	}
	app.handleTransferDialogKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if app.model.TransferDialog.Open {
		t.Fatal("move dialog should close on Esc")
	}
	if len(p.SelectedPaths) == 0 {
		t.Fatal("canceling move dialog must not clear selection")
	}

	app.openMoveDialog()
	app.handleTransferDialogKey(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModAlt))
	if app.model.TransferDialog.Open {
		t.Fatal("move dialog should close on Alt+c")
	}
	if len(p.SelectedPaths) == 0 {
		t.Fatal("canceling move dialog with Alt+C must not clear selection")
	}

	app.openCopyDialog()
	app.confirmCopy()
	if len(p.SelectedPaths) != 0 {
		t.Fatal("confirming copy should clear current-directory selection")
	}
	if len(app.jobState.AllJobs()) != 1 {
		t.Fatalf("expected one job after confirmCopy, got %d", len(app.jobState.AllJobs()))
	}

	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone)); quit {
		t.Fatal("unexpected quit")
	}
	nJobs := len(app.jobState.AllJobs())
	app.enqueueCopyJob()
	if len(p.SelectedPaths) != 0 {
		t.Fatal("enqueueCopyJob should clear current-directory selection")
	}
	if len(app.jobState.AllJobs()) != nJobs+1 {
		t.Fatalf("expected one new job from enqueueCopyJob")
	}
}

func TestTransferDialogEnterFromDestinationConfirms(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))
	dstDir := filepath.Join(dir, "dest")
	if err := os.Mkdir(dstDir, 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	if err := app.inactivePanel().Load(dstDir); err != nil {
		t.Fatalf("inactive Load: %v", err)
	}
	defer app.stopWorker()
	defer flushBackgroundJobs(t, app)

	p := app.activePanel()
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone)); quit {
		t.Fatal("unexpected quit")
	}
	if len(p.SelectedPaths) == 0 {
		t.Fatal("expected current entry tagged after Insert")
	}

	app.openCopyDialog()
	if app.model.TransferDialog.FocusField != 0 {
		t.Fatalf("FocusField = %d, want destination row", app.model.TransferDialog.FocusField)
	}
	app.handleTransferDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if app.model.TransferDialog.Open {
		t.Fatal("copy dialog should close after Enter from destination")
	}
	if len(app.jobState.AllJobs()) != 1 {
		t.Fatalf("expected one job after Enter, got %d", len(app.jobState.AllJobs()))
	}
}

func TestTransferDialogDestinationLeftRightMoveCursor(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))
	dstDir := filepath.Join(dir, "dest")
	if err := os.Mkdir(dstDir, 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	if err := app.inactivePanel().Load(dstDir); err != nil {
		t.Fatalf("inactive Load: %v", err)
	}
	defer app.stopWorker()
	defer flushBackgroundJobs(t, app)

	p := app.activePanel()
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone)); quit {
		t.Fatal("unexpected quit")
	}
	if len(p.SelectedPaths) == 0 {
		t.Fatal("expected current entry tagged after Insert")
	}

	app.openCopyDialog()
	d := &app.model.TransferDialog
	if d.FocusField != 0 || d.Phase != ui.TransferPhaseDestination {
		t.Fatalf("unexpected initial dialog state: focus=%d phase=%v", d.FocusField, d.Phase)
	}
	startCursor := d.Destination.Cursor
	if startCursor == 0 {
		t.Fatalf("expected destination prefill cursor > 0, got %d", startCursor)
	}

	app.handleTransferDialogKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	if d.Destination.Cursor != startCursor-1 {
		t.Fatalf("Left: cursor = %d, want %d", d.Destination.Cursor, startCursor-1)
	}
	if d.DestSubFocus != ui.TransferDestSubFocusText {
		t.Fatalf("Left changed sub-focus to %v, want text", d.DestSubFocus)
	}

	app.handleTransferDialogKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	if d.Destination.Cursor != startCursor-2 {
		t.Fatalf("Left again: cursor = %d, want %d", d.Destination.Cursor, startCursor-2)
	}

	app.handleTransferDialogKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if d.Destination.Cursor != startCursor-1 {
		t.Fatalf("Right: cursor = %d, want %d", d.Destination.Cursor, startCursor-1)
	}

	app.handleTransferDialogKey(tcell.NewEventKey(tcell.KeyRune, 'X', tcell.ModNone))
	wantInsertPos := startCursor - 1
	runes := []rune(d.Destination.Value)
	if wantInsertPos < 0 || wantInsertPos >= len(runes) || runes[wantInsertPos] != 'X' {
		t.Fatalf("expected 'X' inserted at rune %d in %q", wantInsertPos, d.Destination.Value)
	}
	if d.Destination.Cursor != wantInsertPos+1 {
		t.Fatalf("after insert: cursor = %d, want %d", d.Destination.Cursor, wantInsertPos+1)
	}
}

func TestTransferSelfCopyRenameFlow(t *testing.T) {
	t.Run("dialog OK enters rename phase without queueing", func(t *testing.T) {
		dir := t.TempDir()
		aaa := filepath.Join(dir, "aaa")
		if err := os.Mkdir(aaa, 0o755); err != nil {
			t.Fatal(err)
		}
		screen := newScreen(t, 80, 24)
		app := newApp(t, screen, dir)
		defer app.stopWorker()

		p := app.activePanel()
		p.SelectedPaths = map[string]bool{aaa: true}

		app.openCopyDialog()
		app.confirmCopy()
		if !app.model.TransferDialog.Open {
			t.Fatal("dialog should stay open")
		}
		if app.model.TransferDialog.Phase != ui.TransferPhaseSelfCopyRename {
			t.Fatalf("phase = %v, want SelfCopyRename", app.model.TransferDialog.Phase)
		}
		if len(app.jobState.AllJobs()) != 0 {
			t.Fatal("no job should be queued yet")
		}
		if len(p.SelectedPaths) == 0 {
			t.Fatal("selection should remain until the job is queued")
		}
	})

	t.Run("enqueueCopyJob opens rename phase", func(t *testing.T) {
		dir := t.TempDir()
		aaa := filepath.Join(dir, "aaa")
		if err := os.Mkdir(aaa, 0o755); err != nil {
			t.Fatal(err)
		}
		screen := newScreen(t, 80, 24)
		app := newApp(t, screen, dir)
		defer app.stopWorker()

		p := app.activePanel()
		p.SelectedPaths = map[string]bool{aaa: true}
		app.enqueueCopyJob()
		if !app.model.TransferDialog.Open || app.model.TransferDialog.Phase != ui.TransferPhaseSelfCopyRename {
			t.Fatalf("want self-copy rename dialog, got %+v", app.model.TransferDialog)
		}
		if len(p.SelectedPaths) == 0 {
			t.Fatal("enqueue should not clear selection until confirm")
		}
	})

	t.Run("same new name shows error", func(t *testing.T) {
		dir := t.TempDir()
		aaa := filepath.Join(dir, "aaa")
		if err := os.Mkdir(aaa, 0o755); err != nil {
			t.Fatal(err)
		}
		screen := newScreen(t, 80, 24)
		app := newApp(t, screen, dir)
		defer app.stopWorker()

		p := app.activePanel()
		p.SelectedPaths = map[string]bool{aaa: true}
		app.openCopyDialog()
		app.confirmCopy()
		app.model.TransferDialog.SelfCopyNewName = ui.FileDialogField{
			Value:          "aaa",
			Prefill:        "aaa",
			Cursor:         len([]rune("aaa")),
			PrefillPending: false,
		}
		app.confirmCopy()
		if !strings.Contains(app.model.Message, "New name must differ") {
			t.Fatalf("message = %q", app.model.Message)
		}
		if len(app.jobState.AllJobs()) != 0 {
			t.Fatal("job should not enqueue")
		}
	})

	t.Run("distinct new name queues job", func(t *testing.T) {
		dir := t.TempDir()
		aaa := filepath.Join(dir, "aaa")
		if err := os.Mkdir(aaa, 0o755); err != nil {
			t.Fatal(err)
		}
		screen := newScreen(t, 80, 24)
		app := newApp(t, screen, dir)
		defer app.stopWorker()

		p := app.activePanel()
		p.SelectedPaths = map[string]bool{aaa: true}
		app.openCopyDialog()
		app.confirmCopy()
		app.model.TransferDialog.SelfCopyNewName = ui.FileDialogField{
			Value:          "aaa2",
			Prefill:        "aaa2",
			Cursor:         len([]rune("aaa2")),
			PrefillPending: false,
		}
		app.confirmCopy()
		if len(app.jobState.AllJobs()) != 1 {
			t.Fatalf("expected 1 job, got %d", len(app.jobState.AllJobs()))
		}
		j := app.jobState.AllJobs()[0]
		wantDest := filepath.Join(dir, "aaa2")
		if filepath.Clean(j.Destination.String()) != filepath.Clean(wantDest) {
			t.Fatalf("Destination = %q, want %q", j.Destination, wantDest)
		}
		if len(p.SelectedPaths) != 0 {
			t.Fatal("selection cleared after queue")
		}
		waitUntilAppJobsFinished(t, app, 5*time.Second)
	})
}

func TestTransferSelfCopyMultipleSourcesRejected(t *testing.T) {
	dir := t.TempDir()
	aaa := filepath.Join(dir, "aaa")
	bbb := filepath.Join(dir, "bbb.txt")
	if err := os.Mkdir(aaa, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, bbb)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	defer app.stopWorker()

	p := app.activePanel()
	p.SelectedPaths = map[string]bool{aaa: true, bbb: true}
	app.openCopyDialog()
	app.confirmCopy()
	if app.model.TransferDialog.Phase != ui.TransferPhaseDestination {
		t.Fatalf("phase = %v, want Destination", app.model.TransferDialog.Phase)
	}
	if !strings.Contains(app.model.Message, "multiple items") {
		t.Fatalf("message = %q", app.model.Message)
	}
}

func TestEnqueueMoveJobSameDirUnsupportedDoesNotClearSelection(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "only.txt"))
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	p := app.activePanel()
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone)); quit {
		t.Fatal("unexpected quit")
	}
	app.enqueueMoveJob()
	if len(p.SelectedPaths) == 0 {
		t.Fatal("unsupported same-directory move should leave selection intact")
	}
}

func TestEnqueueCopyJobClearsCrossDirectorySelections(t *testing.T) {
	dir := t.TempDir()
	here := filepath.Join(dir, "here")
	other := filepath.Join(dir, "other")
	dst := filepath.Join(dir, "dest")
	if err := os.MkdirAll(here, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(here, "here.txt"))
	writeFile(t, filepath.Join(other, "other.txt"))

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	defer app.stopWorker()
	defer flushBackgroundJobs(t, app)

	if err := app.activePanel().Load(here); err != nil {
		t.Fatalf("Load here: %v", err)
	}
	if err := app.inactivePanel().Load(dst); err != nil {
		t.Fatalf("Load dest: %v", err)
	}

	p := app.activePanel()
	hereTxt := filepath.Join(here, "here.txt")
	otherTxt := filepath.Join(other, "other.txt")
	p.SelectedPaths = map[string]bool{hereTxt: true, otherTxt: true}
	p.SelectionsStripOrder = []string{otherTxt}

	app.enqueueCopyJob()

	if p.SelectedPaths != nil {
		t.Fatalf("expected all queued sources removed from selection, got %#v", p.SelectedPaths)
	}
	if len(p.SelectionsStripOrder) != 0 {
		t.Fatalf("expected selections strip order cleared, got %v", p.SelectionsStripOrder)
	}
	if len(app.jobState.AllJobs()) != 1 {
		t.Fatalf("expected one job, got %d", len(app.jobState.AllJobs()))
	}
}

func TestEnqueueMoveJobClearsCrossDirectorySelections(t *testing.T) {
	dir := t.TempDir()
	here := filepath.Join(dir, "here")
	other := filepath.Join(dir, "other")
	dst := filepath.Join(dir, "dest")
	if err := os.MkdirAll(here, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(here, "here.txt"))
	writeFile(t, filepath.Join(other, "other.txt"))

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	defer app.stopWorker()
	defer flushBackgroundJobs(t, app)

	if err := app.activePanel().Load(here); err != nil {
		t.Fatalf("Load here: %v", err)
	}
	if err := app.inactivePanel().Load(dst); err != nil {
		t.Fatalf("Load dest: %v", err)
	}

	p := app.activePanel()
	hereTxt := filepath.Join(here, "here.txt")
	otherTxt := filepath.Join(other, "other.txt")
	p.SelectedPaths = map[string]bool{hereTxt: true, otherTxt: true}
	p.SelectionsStripOrder = []string{otherTxt}

	app.enqueueMoveJob()

	if p.SelectedPaths != nil {
		t.Fatalf("expected all queued sources removed from selection, got %#v", p.SelectedPaths)
	}
	if len(p.SelectionsStripOrder) != 0 {
		t.Fatalf("expected selections strip order cleared, got %v", p.SelectionsStripOrder)
	}
	if len(app.jobState.AllJobs()) != 1 {
		t.Fatalf("expected one job, got %d", len(app.jobState.AllJobs()))
	}
}

func TestHelpViewEnterJobsCancelNoOpWhileBrowser(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.openHelpDialog()

	jobsIdx := -1
	for i, e := range app.model.HelpView.Entries {
		if e.ActionID == keymap.ActionJobsCancel {
			jobsIdx = i
			break
		}
	}
	if jobsIdx < 0 {
		t.Fatal("help entries should include Cancel job action")
	}
	sel := -1
	for i, idx := range app.model.HelpView.Ranked {
		if idx == jobsIdx {
			sel = i
			break
		}
	}
	if sel < 0 {
		t.Fatal("ranked list should include Cancel job entry")
	}
	app.model.HelpView.Selected = sel
	app.model.HelpView.Focus = 0

	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); quit {
		t.Fatal("Enter on jobs.cancel should not quit")
	}
	if !app.model.HelpView.Open {
		t.Fatal("HelpView should stay open when action is invalid for browser")
	}
}

func TestActiveFooterKeysBrowserShowsF7JobsViewUsesJobsLegend(t *testing.T) {
	dir := t.TempDir()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := New(screen, func() (string, error) {
		return dir, nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	browserKeys := menu.FunctionKeys
	if got := app.activeFooterKeys(); len(got) != len(browserKeys) {
		t.Fatalf("browser footer len = %d, want %d", len(got), len(browserKeys))
	}
	var f7Hint string
	for _, fk := range app.activeFooterKeys() {
		if fk.Key == tcell.KeyF7 {
			f7Hint = fk.Hint
			break
		}
	}
	if f7Hint == "" {
		t.Fatal("browser footer: F7 should have a hint (Mkdir)")
	}

	jobsKeys := menu.FunctionKeysJobsView()
	app.model.ViewMode = ui.ViewJobs
	gotJobs := app.activeFooterKeys()
	if len(gotJobs) != len(jobsKeys) {
		t.Fatalf("jobs footer len = %d, want %d", len(gotJobs), len(jobsKeys))
	}
	for i := range jobsKeys {
		if gotJobs[i].Key != jobsKeys[i].Key || gotJobs[i].KeyLabel != jobsKeys[i].KeyLabel || gotJobs[i].Hint != jobsKeys[i].Hint {
			t.Fatalf("jobs footer key %d = %+v, want %+v", i, gotJobs[i], jobsKeys[i])
		}
	}
	for _, fk := range gotJobs {
		if fk.Key == tcell.KeyF7 {
			t.Fatal("jobs footer should not list F7 (mkdir is for file panels)")
		}
	}
}

func TestJobsViewEscClosesViewDoesNotQuit(t *testing.T) {
	testJobsViewDismissKeyClosesViewDoesNotQuit(t, tcell.KeyEsc)
}

func TestJobsViewLeftClosesViewDoesNotQuit(t *testing.T) {
	testJobsViewDismissKeyClosesViewDoesNotQuit(t, tcell.KeyLeft)
}

func testJobsViewDismissKeyClosesViewDoesNotQuit(t *testing.T, key tcell.Key) {
	t.Helper()
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.openJobsView()
	if app.model.ViewMode != ui.ViewJobs {
		t.Fatalf("ViewMode = %v, want ViewJobs", app.model.ViewMode)
	}

	quit, _ := app.handleKey(tcell.NewEventKey(key, 0, tcell.ModNone))
	if quit {
		t.Fatalf("%v in jobs view must not quit the application", key)
	}
	if app.model.ViewMode != ui.ViewBrowser {
		t.Fatalf("ViewMode = %v, want browser after %v", app.model.ViewMode, key)
	}
}

func TestMessagesViewLeftClosesView(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.openMessagesView()
	if app.model.ViewMode != ui.ViewMessages {
		t.Fatalf("ViewMode = %v, want ViewMessages", app.model.ViewMode)
	}

	_, _ = app.handleKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	if app.model.ViewMode != ui.ViewBrowser {
		t.Fatalf("ViewMode = %v, want browser after Left", app.model.ViewMode)
	}
}

func TestCommandsViewLeftClosesView(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.openCommandsView()
	if app.model.ViewMode != ui.ViewCommands {
		t.Fatalf("ViewMode = %v, want ViewCommands", app.model.ViewMode)
	}

	_, _ = app.handleKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	if app.model.ViewMode != ui.ViewBrowser {
		t.Fatalf("ViewMode = %v, want browser after Left", app.model.ViewMode)
	}
}

func TestDispatchMovesOnlyActivePanel(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))
	writeFile(t, filepath.Join(dir, "b.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := New(screen, func() (string, error) {
		return dir, nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	app.dispatch(keymap.ActionNavDown)
	if app.model.Left.Cursor != 1 {
		t.Fatalf("left cursor = %d, want 1", app.model.Left.Cursor)
	}
	if app.model.Right.Cursor != 0 {
		t.Fatalf("right cursor = %d, want 0", app.model.Right.Cursor)
	}

	app.dispatch(keymap.ActionPanelSwitch)
	app.dispatch(keymap.ActionNavDown)
	if app.model.ActivePanel != ui.RightPanel {
		t.Fatalf("active panel = %d, want right panel", app.model.ActivePanel)
	}
	if app.model.Left.Cursor != 1 {
		t.Fatalf("left cursor = %d, want unchanged 1", app.model.Left.Cursor)
	}
	if app.model.Right.Cursor != 1 {
		t.Fatalf("right cursor = %d, want 1", app.model.Right.Cursor)
	}
}

func TestHideInactivePanelToggleAndTabShow(t *testing.T) {
	dir := t.TempDir()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := New(screen, func() (string, error) {
		return dir, nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	app.model.SyncFollowEnabled = true
	app.model.SyncFollowPanel = ui.LeftPanel
	app.model.QuickViewEnabled = true

	app.dispatch(keymap.ActionPanelToggleHideInactive)
	if !app.model.HideInactivePanel {
		t.Fatal("HideInactivePanel = false, want true")
	}
	if app.model.SyncFollowEnabled {
		t.Fatal("sync still enabled after hiding inactive panel")
	}
	if app.model.QuickViewEnabled {
		t.Fatal("quick view still enabled after hiding inactive panel")
	}

	app.dispatch(keymap.ActionPanelSwitch)
	if app.model.HideInactivePanel {
		t.Fatal("HideInactivePanel = true after Tab, want shown")
	}
	if app.model.ActivePanel != ui.RightPanel {
		t.Fatalf("ActivePanel = %d, want right", app.model.ActivePanel)
	}
}

func TestDispatchTogglesSelectionOnlyInActivePanel(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))
	writeFile(t, filepath.Join(dir, "b.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := New(screen, func() (string, error) {
		return dir, nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	leftEntry, ok := app.model.Left.CurrentEntry()
	if !ok {
		t.Fatal("left CurrentEntry() ok = false, want true")
	}
	rightEntry, ok := app.model.Right.CurrentEntry()
	if !ok {
		t.Fatal("right CurrentEntry() ok = false, want true")
	}

	app.dispatch(keymap.ActionPanelSelectToggle)
	if !app.model.Left.IsSelected(leftEntry) {
		t.Fatal("left active entry is not selected")
	}
	if app.model.Left.Cursor != 1 {
		t.Fatalf("left cursor = %d, want 1 after selection advances", app.model.Left.Cursor)
	}
	if app.model.Right.IsSelected(rightEntry) {
		t.Fatal("right entry is selected, want inactive panel unchanged")
	}

	app.dispatch(keymap.ActionPanelSwitch)
	app.dispatch(keymap.ActionPanelSelectToggle)
	if !app.model.Right.IsSelected(rightEntry) {
		t.Fatal("right active entry is not selected after switching panels")
	}
}

func TestMenuInputUsesMenuStateInsteadOfPanelNavigation(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))
	writeFile(t, filepath.Join(dir, "b.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := New(screen, func() (string, error) {
		return dir, nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	app.dispatch(keymap.ActionAppOpenMenu)
	if !app.model.Menu.Open {
		t.Fatal("menu open = false, want true")
	}
	if app.model.Menu.ActiveMenu != menu.DefaultIndex() {
		t.Fatalf("active menu = %d, want file menu", app.model.Menu.ActiveMenu)
	}

	// F9 now opens menu bar only (no pulldown). Press Down to open pulldown.
	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if app.model.Left.Cursor != 0 {
		t.Fatalf("left cursor = %d, want unchanged 0 while menu is open", app.model.Left.Cursor)
	}
	if !app.model.Menu.PulldownOpen {
		t.Fatalf("pulldown open = false after Down")
	}
	// First selectable menu item (View at index 0) should be selected.
	if app.model.Menu.SelectedItem != 0 {
		t.Fatalf("selected menu item = %d, want 0", app.model.Menu.SelectedItem)
	}
	// Press Down again to move to second item.
	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if app.model.Menu.SelectedItem != 1 {
		t.Fatalf("selected menu item = %d, want 1", app.model.Menu.SelectedItem)
	}

	// Esc when pulldown open: closes pulldown, keeps menu bar active.
	app.handleKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if app.model.Menu.PulldownOpen {
		t.Fatal("pulldown open = true, want false after Esc")
	}
	if !app.model.Menu.Open {
		t.Fatal("menu open = false, want true after Esc (menu bar stays active)")
	}
	// Second Esc closes the menu bar entirely.
	app.handleKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if app.model.Menu.Open {
		t.Fatal("menu open = true, want false after second Esc")
	}

	app.dispatch(keymap.ActionAppOpenMenu)
	app.handleKey(tcell.NewEventKey(tcell.KeyCtrlLeftSq, 0, tcell.ModNone))
	if app.model.Menu.Open {
		t.Fatal("menu open = true, want false after Ctrl-[ (Esc alias)")
	}

	app.dispatch(keymap.ActionAppOpenMenu)
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, '\x1b', tcell.ModNone))
	if app.model.Menu.Open {
		t.Fatal("menu open = true, want false after escape rune")
	}
}

func TestLeftMenuToggleHiddenTargetsLeftPanel(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".hidden"))
	writeFile(t, filepath.Join(dir, "visible.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := New(screen, func() (string, error) {
		return dir, nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	app.model.ActivePanel = ui.RightPanel

	app.dispatch(keymap.ActionAppOpenMenu)
	app.moveMenu(-1)
	// Open pulldown for Left menu.
	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	app.moveMenuItem(2)
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if quit {
		t.Fatal("handleKey() quit = true, want false")
	}
	if app.model.Menu.Open {
		t.Fatal("menu open = true, want closed")
	}
	if !app.model.Left.ShowHidden {
		t.Fatal("left ShowHidden = false, want true")
	}
	if app.model.Right.ShowHidden {
		t.Fatal("right ShowHidden = true, want false")
	}
	if len(app.model.Left.Entries) != 2 {
		t.Fatalf("left len(Entries) = %d, want hidden and visible entries", len(app.model.Left.Entries))
	}
	if app.model.Message != "Left panel hidden and ignored files shown" {
		t.Fatalf("Message = %q, want left panel hidden visibility message", app.model.Message)
	}
}

func TestBookmarkDialogOpensAndNavigates(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "deep", "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	marksPath := filepath.Join(root, "marks")
	line := fmt.Sprintf("markone : %s\n", target)
	if err := os.WriteFile(marksPath, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	cfg := config.Default()
	cfg.Bookmarks.File = marksPath
	app, err := NewWithOptions(screen, Options{
		CWD:    func() (string, error) { return root, nil },
		Config: cfg,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	app.openBookmarkDialog()
	if !app.model.PathPicker.Open {
		t.Fatal("expected path picker open")
	}
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); quit {
		t.Fatal("unexpected quit")
	}
	if app.model.PathPicker.Open {
		t.Fatal("expected dialog closed")
	}
	if got := app.activePanel().Path.String(); got != filepath.Clean(target) {
		t.Fatalf("panel path = %q want %q", got, filepath.Clean(target))
	}
}

func TestBookmarkDialogF8DeletesFZFMark(t *testing.T) {
	root := t.TempDir()
	xdg := filepath.Join(root, "xdg")
	if err := os.MkdirAll(filepath.Join(xdg, "gtk-3.0"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", xdg)
	target := filepath.Join(root, "marked")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	marksPath := filepath.Join(root, "marks")
	line := fmt.Sprintf("markone : %s\n", target)
	if err := os.WriteFile(marksPath, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	cfg := config.Default()
	cfg.Bookmarks.File = marksPath
	bundle, err := keymap.DefaultBundle()
	if err != nil {
		t.Fatalf("DefaultBundle: %v", err)
	}
	app, err := NewWithOptions(screen, Options{
		CWD:          func() (string, error) { return root, nil },
		Config:       cfg,
		KeymapBundle: bundle,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	app.openBookmarkDialog()
	if len(app.model.PathPicker.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(app.model.PathPicker.Items))
	}
	if !app.bookmarkDialogDeleteFooterEligible() {
		t.Fatal("expected delete footer for fzf-marks row")
	}
	if !app.tryBookmarkDialogShortcut(tcell.NewEventKey(tcell.KeyF8, 0, tcell.ModNone)) {
		t.Fatal("F8 should delete selected fzf-marks bookmark")
	}
	if len(app.model.PathPicker.Items) != 0 {
		t.Fatalf("expected empty picker items, got %d", len(app.model.PathPicker.Items))
	}
	data, err := os.ReadFile(marksPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 0 {
		t.Fatalf("marks file = %q, want empty", string(data))
	}
	if !strings.Contains(app.model.Message, "Bookmark removed") {
		t.Fatalf("message = %q, want removal confirmation", app.model.Message)
	}
}

func TestBookmarkDialogDeleteFooterSkippedForGnomeMark(t *testing.T) {
	root := t.TempDir()
	xdg := filepath.Join(root, "xdg")
	gtkDir := filepath.Join(xdg, "gtk-3.0")
	if err := os.MkdirAll(gtkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	gnomePath := filepath.Join(root, "gnome-only")
	if err := os.MkdirAll(gnomePath, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", xdg)
	marksPath := filepath.Join(root, "marks")
	if err := os.WriteFile(marksPath, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	gtkMarks := filepath.Join(gtkDir, "bookmarks")
	if err := os.WriteFile(gtkMarks, []byte("file://"+gnomePath+" gnomeproj\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	cfg := config.Default()
	cfg.Bookmarks.File = marksPath
	bundle, err := keymap.DefaultBundle()
	if err != nil {
		t.Fatalf("DefaultBundle: %v", err)
	}
	app, err := NewWithOptions(screen, Options{
		CWD:          func() (string, error) { return root, nil },
		Config:       cfg,
		KeymapBundle: bundle,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	app.openBookmarkDialog()
	if len(app.model.PathPicker.Items) != 1 {
		t.Fatalf("items = %d, want 1 gnome bookmark", len(app.model.PathPicker.Items))
	}
	if app.model.PathPicker.Items[0].Source != "gnome" {
		t.Fatalf("source = %q, want gnome", app.model.PathPicker.Items[0].Source)
	}
	if app.bookmarkDialogDeleteFooterEligible() {
		t.Fatal("delete footer should not show for gnome bookmark")
	}
	before, err := os.ReadFile(gtkMarks)
	if err != nil {
		t.Fatal(err)
	}
	if app.tryBookmarkDialogShortcut(tcell.NewEventKey(tcell.KeyF8, 0, tcell.ModNone)) {
		t.Fatal("F8 should not delete gnome bookmark")
	}
	after, err := os.ReadFile(gtkMarks)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("gtk bookmarks changed: before %q after %q", before, after)
	}
}

func TestHistoryDialogAltHUsesActivePanel(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.model.ActivePanel = ui.RightPanel
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'h', tcell.ModAlt)); quit {
		t.Fatal("unexpected quit")
	}
	if !app.model.HistoryDialog.Open {
		t.Fatal("expected history dialog open")
	}
	if app.model.HistoryDialog.PanelID != ui.RightPanel {
		t.Fatalf("History panel = %d want right (%d)", app.model.HistoryDialog.PanelID, ui.RightPanel)
	}
}

func TestOpenSelectedDirectoryInInactivePanel(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	gamma := filepath.Join(alpha, "gamma")
	if err := os.MkdirAll(gamma, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	left := app.panelByID(ui.LeftPanel)
	for i := 0; i < left.VisibleEntryCount(); i++ {
		entry, _, ok := left.VisibleEntry(i)
		if ok && entry.Name == "alpha" {
			left.Cursor = i
			break
		}
	}
	app.model.ActivePanel = ui.LeftPanel
	app.dispatch(keymap.ActionPanelOpenDirInOther)

	wantRoot := filepath.Clean(root)
	wantAlpha := filepath.Clean(alpha)
	if got := filepath.Clean(app.panelByID(ui.LeftPanel).Path.String()); got != wantRoot {
		t.Fatalf("left panel path = %q want %q", got, wantRoot)
	}
	if got := filepath.Clean(app.panelByID(ui.RightPanel).Path.String()); got != wantAlpha {
		t.Fatalf("right panel path = %q want %q", got, wantAlpha)
	}

	right := app.panelByID(ui.RightPanel)
	for i := 0; i < right.VisibleEntryCount(); i++ {
		entry, _, ok := right.VisibleEntry(i)
		if ok && entry.Name == "gamma" {
			right.Cursor = i
			break
		}
	}
	app.model.ActivePanel = ui.RightPanel
	app.dispatch(keymap.ActionPanelOpenDirInOther)

	if got := filepath.Clean(app.panelByID(ui.RightPanel).Path.String()); got != wantAlpha {
		t.Fatalf("right panel path = %q want %q after second open", got, wantAlpha)
	}
	wantGamma := filepath.Clean(gamma)
	if got := filepath.Clean(app.panelByID(ui.LeftPanel).Path.String()); got != wantGamma {
		t.Fatalf("left panel path = %q want %q", got, wantGamma)
	}
}

func TestOpenActivePathInInactivePanel(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	gamma := filepath.Join(alpha, "gamma")
	if err := os.MkdirAll(gamma, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	left := app.panelByID(ui.LeftPanel)
	for i := 0; i < left.VisibleEntryCount(); i++ {
		entry, _, ok := left.VisibleEntry(i)
		if ok && entry.Name == "alpha" {
			left.Cursor = i
			break
		}
	}
	app.model.ActivePanel = ui.LeftPanel
	app.dispatch(keymap.ActionNavOpen)

	for i := 0; i < left.VisibleEntryCount(); i++ {
		entry, _, ok := left.VisibleEntry(i)
		if ok && entry.Name == "gamma" {
			left.Cursor = i
			break
		}
	}
	wantAlpha := filepath.Clean(alpha)
	if got := filepath.Clean(left.PathString()); got != wantAlpha {
		t.Fatalf("left cwd = %q want %q", got, wantAlpha)
	}

	app.dispatch(keymap.ActionPanelOpenActivePathInOther)

	if got := filepath.Clean(app.panelByID(ui.RightPanel).Path.String()); got != wantAlpha {
		t.Fatalf("right panel path = %q want active cwd %q", got, wantAlpha)
	}
}

func TestHistoryDialogFilterNavigatesToMatch(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	beta := filepath.Join(root, "beta")
	if err := os.Mkdir(alpha, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(beta, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	app, err := New(screen, func() (string, error) { return root, nil })
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	p := app.panelByID(ui.LeftPanel)
	for i := 0; i < p.VisibleEntryCount(); i++ {
		entry, _, ok := p.VisibleEntry(i)
		if ok && entry.Name == "alpha" {
			p.Cursor = i
			break
		}
	}
	app.dispatch(keymap.ActionNavOpen)
	for i := 0; i < p.VisibleEntryCount(); i++ {
		entry, _, ok := p.VisibleEntry(i)
		if ok && entry.Name == "beta" {
			p.Cursor = i
			break
		}
	}
	app.dispatch(keymap.ActionNavOpen)

	app.openHistoryDialog(ui.LeftPanel)
	if !app.model.HistoryDialog.Open {
		t.Fatal("expected history dialog open")
	}
	for _, r := range "alpha" {
		if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone)); quit {
			t.Fatal("unexpected quit")
		}
	}
	if len(app.model.HistoryDialog.Ranked) == 0 {
		t.Fatal("expected fuzzy matches for alpha")
	}
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); quit {
		t.Fatal("unexpected quit")
	}
	if app.model.HistoryDialog.Open {
		t.Fatal("expected dialog closed")
	}
	want := filepath.Clean(alpha)
	if got := filepath.Clean(app.activePanel().Path.String()); got != want {
		t.Fatalf("panel path = %q want %q", got, want)
	}
}

func TestBookmarkDialogFilterSelectsRankedFirst(t *testing.T) {
	root := t.TempDir()
	tAlpha := filepath.Join(root, "alpha")
	tBeta := filepath.Join(root, "beta")
	if err := os.MkdirAll(tAlpha, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tBeta, 0o755); err != nil {
		t.Fatal(err)
	}
	marksPath := filepath.Join(root, "marks")
	content := fmt.Sprintf("aaa : %s\nbbb : %s\n", tAlpha, tBeta)
	if err := os.WriteFile(marksPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	cfg := config.Default()
	cfg.Bookmarks.File = marksPath
	app, err := NewWithOptions(screen, Options{
		CWD:    func() (string, error) { return root, nil },
		Config: cfg,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	app.openBookmarkDialog()
	for _, r := range "b" {
		if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone)); quit {
			t.Fatal("unexpected quit")
		}
	}
	if len(app.model.PathPicker.Ranked) == 0 {
		t.Fatal("expected at least one fuzzy match for query b")
	}
	if app.model.PathPicker.Ranked[0] != 1 {
		t.Fatalf("first ranked index = %d want 1 (bbb line), ranked=%v", app.model.PathPicker.Ranked[0], app.model.PathPicker.Ranked)
	}
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); quit {
		t.Fatal("unexpected quit")
	}
	if got := app.activePanel().Path.String(); got != filepath.Clean(tBeta) {
		t.Fatalf("panel path = %q want %q", got, filepath.Clean(tBeta))
	}
}

func TestBookmarkDialogTypingODoesNotActivateWithoutEnter(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "t")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	marksPath := filepath.Join(root, "marks")
	if err := os.WriteFile(marksPath, []byte("x : "+target+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	cfg := config.Default()
	cfg.Bookmarks.File = marksPath
	app, err := NewWithOptions(screen, Options{
		CWD:    func() (string, error) { return root, nil },
		Config: cfg,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	startPath := app.activePanel().Path
	app.openBookmarkDialog()
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModNone)); quit {
		t.Fatal("unexpected quit")
	}
	if !app.model.PathPicker.Open {
		t.Fatal("typing o in filter must not close or navigate")
	}
	if app.model.PathPicker.Query != "o" {
		t.Fatalf("query = %q want o", app.model.PathPicker.Query)
	}
	if app.activePanel().Path != startPath {
		t.Fatalf("panel path changed without Enter: %q", app.activePanel().Path.String())
	}
}

func TestNewWithOptionsAppliesConfiguredHiddenFilesToBothPanels(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".hidden"))
	writeFile(t, filepath.Join(dir, "visible.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	cfg := config.Default()
	cfg.ShowHidden = true
	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	if !app.model.Left.ShowHidden || !app.model.Right.ShowHidden {
		t.Fatalf("ShowHidden left=%v right=%v, want both true", app.model.Left.ShowHidden, app.model.Right.ShowHidden)
	}
	if len(app.model.Left.Entries) != 2 || len(app.model.Right.Entries) != 2 {
		t.Fatalf("entry counts left=%d right=%d, want hidden and visible entries", len(app.model.Left.Entries), len(app.model.Right.Entries))
	}
}

func TestNewWithOptionsAppliesProvidedTheme(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	styles := theme.Default()
	styles.Name = "custom"
	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: config.Default(),
		Theme:  styles,
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	if app.styles.Name != "custom" {
		t.Fatalf("app theme name = %q, want custom", app.styles.Name)
	}
}

func TestNewWithOptionsSetsShowFileIconsFromConfig(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	cfg := config.Default()
	cfg.UI.ShowFileIcons = false
	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Theme:  theme.Default(),
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}
	if app.model.ShowFileIcons {
		t.Fatal("ShowFileIcons = true, want false from config")
	}
}

func TestNewWithOptionsAppliesDefaultListingFormatFromConfig(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	cfg := config.Default()
	cfg.DefaultListingFormat = config.ListingFormatBrief
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Theme:  theme.Default(),
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}
	if app.model.Left.ListFormat != panel.ListFormatBrief || app.model.Right.ListFormat != panel.ListFormatBrief {
		t.Fatalf("panels ListFormat = %v/%v, want brief", app.model.Left.ListFormat, app.model.Right.ListFormat)
	}
}

func TestNewWithOptionsAppliesFilterCycleMatchesToPanels(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	cfg := config.Default()
	cfg.Filter.CycleMatches = config.FilterCycleMatchesRanked
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Theme:  theme.Default(),
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}
	if app.model.Left.Filter.CycleMatches != config.FilterCycleMatchesRanked {
		t.Fatalf("Left.Filter.CycleMatches = %q, want ranked", app.model.Left.Filter.CycleMatches)
	}
	if app.model.Right.Filter.CycleMatches != config.FilterCycleMatchesRanked {
		t.Fatalf("Right.Filter.CycleMatches = %q, want ranked", app.model.Right.Filter.CycleMatches)
	}
}

func TestOptionsMenuOpensConfigurationDialog(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	cfg := config.Default()
	cfg.DefaultListingFormat = config.ListingFormatPerm
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Theme:  theme.Default(),
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	app.dispatch(keymap.ActionAppOpenMenu)
	app.moveMenu(3) // File → Command → Display → Options
	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModNone))

	if quit {
		t.Fatal("handleKey() quit = true, want false")
	}
	if app.model.Menu.Open {
		t.Fatal("menu open = true, want closed")
	}
	if !app.model.ConfigDialog.Open {
		t.Fatal("configuration dialog open = false, want true")
	}
	if !app.model.ConfigDialog.ShowFileIcons {
		t.Fatal("working copy ShowFileIcons = false, want default true")
	}
	if app.model.ConfigDialog.ListFormat != panel.ListFormatPerm {
		t.Fatalf("ConfigDialog.ListFormat = %v, want perm", app.model.ConfigDialog.ListFormat)
	}
}

func TestConfigDialogApplyPersistsShowFileIcons(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	appPaths := config.Paths{ConfigDir: filepath.Join(t.TempDir(), "persist-cfg-icons")}.WithResolvedLocations()
	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: config.Default(),
		Paths:  appPaths,
		Theme:  theme.Default(),
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	app.openConfigDialog()
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'f', tcell.ModNone))
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if quit {
		t.Fatal("handleKey() quit = true, want false")
	}
	if app.model.ConfigDialog.Open {
		t.Fatal("config dialog should close after apply")
	}
	if app.model.ShowFileIcons {
		t.Fatal("ShowFileIcons = true, want false after toggle")
	}
	if app.config.UI.ShowFileIcons {
		t.Fatal("config UI ShowFileIcons = true, want false")
	}
	reloaded, err := config.LoadFromPaths(appPaths)
	if err != nil {
		t.Fatalf("LoadFromPaths after persist: %v", err)
	}
	if reloaded.UI.ShowFileIcons {
		t.Fatalf("persisted show_file_icons = true, want false")
	}
}

func TestConfigDialogApplyPersistsZoomActivePanel(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	appPaths := config.Paths{ConfigDir: filepath.Join(t.TempDir(), "persist-cfg-zoom")}.WithResolvedLocations()
	cfg := config.Default()
	cfg.UI.ZoomActivePanel = false
	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Paths:  appPaths,
		Theme:  theme.Default(),
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	app.openConfigDialog()
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'z', tcell.ModNone))
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if quit {
		t.Fatal("handleKey() quit = true, want false")
	}
	if !app.config.UI.ZoomActivePanel {
		t.Fatal("ZoomActivePanel = false, want true after toggle")
	}
	reloaded, err := config.LoadFromPaths(appPaths)
	if err != nil {
		t.Fatalf("LoadFromPaths after persist: %v", err)
	}
	if !reloaded.UI.ZoomActivePanel {
		t.Fatalf("persisted zoom_active_panel = false, want true")
	}
}

func TestConfigDialogApplyPersistsCenterScrolling(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	appPaths := config.Paths{ConfigDir: filepath.Join(t.TempDir(), "persist-cfg-center")}.WithResolvedLocations()
	cfg := config.Default()
	cfg.UI.CenterScrolling = false
	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Paths:  appPaths,
		Theme:  theme.Default(),
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}
	if app.model.Left.CenterScrolling {
		t.Fatal("Left.CenterScrolling = true, want false from config")
	}

	app.openConfigDialog()
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'e', tcell.ModNone))
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if quit {
		t.Fatal("handleKey() quit = true, want false")
	}
	if !app.config.UI.CenterScrolling {
		t.Fatal("CenterScrolling = false, want true after toggle")
	}
	if !app.model.Left.CenterScrolling || !app.model.Right.CenterScrolling {
		t.Fatal("panel CenterScrolling not synced after apply")
	}
	reloaded, err := config.LoadFromPaths(appPaths)
	if err != nil {
		t.Fatalf("LoadFromPaths after persist: %v", err)
	}
	if !reloaded.UI.CenterScrolling {
		t.Fatalf("persisted center_scrolling = false, want true")
	}
}

func TestRuntimeZoomToggleChangesLayoutAndDoesNotPersist(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(100, 30)

	cfg := config.Default()
	cfg.UI.ZoomActivePanel = false
	cfg.UI.PanelZoomActivePercent = 70
	cfg.UI.PanelZoomInactivePercent = 30

	appPaths := config.Paths{ConfigDir: filepath.Join(t.TempDir(), "runtime-zoom-persist")}.WithResolvedLocations()
	if err := os.MkdirAll(appPaths.ConfigDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	initToml := `[ui]
zoom_active_panel = false
panel_zoom_active_percent = 70
panel_zoom_inactive_percent = 30
`
	if err := os.WriteFile(appPaths.ConfigFile, []byte(initToml), 0o644); err != nil {
		t.Fatalf("WriteFile config: %v", err)
	}

	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Paths:  appPaths,
		Theme:  theme.Default(),
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	layBefore := app.layoutForTerminalSize(100, 30)
	if layBefore.Left.Width != 50 || layBefore.Right.Width != 50 {
		t.Fatalf("before toggle Left=%d Right=%d want 50/50", layBefore.Left.Width, layBefore.Right.Width)
	}

	app.dispatch(keymap.ActionPanelToggleZoomActivePanel)
	if app.zoomActivePanelOverride == nil || !*app.zoomActivePanelOverride {
		t.Fatal("expected runtime override zoom on")
	}
	if app.config.UI.ZoomActivePanel {
		t.Fatal("saved ZoomActivePanel must stay false")
	}

	layAfter := app.layoutForTerminalSize(100, 30)
	if layAfter.Left.Width != 70 || layAfter.Right.Width != 30 {
		t.Fatalf("after toggle Left=%d Right=%d want 70/30", layAfter.Left.Width, layAfter.Right.Width)
	}

	app.openConfigDialog()
	if quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); quit {
		t.Fatal("handleKey quit")
	}
	if app.zoomActivePanelOverride != nil {
		t.Fatal("override should clear after Configuration OK")
	}

	reloaded, err := config.LoadFromPaths(appPaths)
	if err != nil {
		t.Fatalf("LoadFromPaths: %v", err)
	}
	if reloaded.UI.ZoomActivePanel {
		t.Fatalf("persisted zoom_active_panel leaked true, want false")
	}
}

func TestLayoutForTerminalSizeIgnoresZoomInAuxiliaryViews(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(100, 30)

	cfg := config.Default()
	cfg.UI.ZoomActivePanel = true
	cfg.UI.PanelZoomActivePercent = 70
	cfg.UI.PanelZoomInactivePercent = 30

	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Paths:  config.Paths{}.WithResolvedLocations(),
		Theme:  theme.Default(),
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	layBrowser := app.layoutForTerminalSize(100, 30)
	if layBrowser.Left.Width != 70 || layBrowser.Right.Width != 30 {
		t.Fatalf("browser Left=%d Right=%d want 70/30", layBrowser.Left.Width, layBrowser.Right.Width)
	}

	for _, vm := range []ui.ViewMode{ui.ViewJobs, ui.ViewCommands, ui.ViewMessages, ui.ViewFilePreview} {
		app.model.ViewMode = vm
		lay := app.layoutForTerminalSize(100, 30)
		if lay.Left.Width != 50 || lay.Right.Width != 50 {
			t.Fatalf("view %v with zoom on: Left=%d Right=%d want 50/50", vm, lay.Left.Width, lay.Right.Width)
		}
	}
}

func TestLayoutForTerminalSizeIgnoresHideInactivePanelInAuxiliaryViews(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(100, 30)

	app, err := New(screen, func() (string, error) {
		return t.TempDir(), nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	app.model.HideInactivePanel = true
	app.model.ActivePanel = ui.LeftPanel

	layBrowser := app.layoutForTerminalSize(100, 30)
	if layBrowser.Left.Width != 100 || layBrowser.Right.Width != 0 {
		t.Fatalf("browser with hide: Left=%d Right=%d want 100/0", layBrowser.Left.Width, layBrowser.Right.Width)
	}

	for _, vm := range []ui.ViewMode{ui.ViewJobs, ui.ViewCommands, ui.ViewMessages} {
		app.model.ViewMode = vm
		lay := app.layoutForTerminalSize(100, 30)
		if lay.Left.Width != 50 || lay.Right.Width != 50 {
			t.Fatalf("view %v with hide inactive: Left=%d Right=%d want 50/50", vm, lay.Left.Width, lay.Right.Width)
		}
	}
	if !app.model.HideInactivePanel {
		t.Fatal("HideInactivePanel cleared when switching auxiliary views")
	}
}

func TestLayoutForTerminalSizeDisablesZoomWhileFilePreviewOpen(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(100, 30)

	cfg := config.Default()
	cfg.UI.ZoomActivePanel = true
	cfg.UI.PanelZoomActivePercent = 70
	cfg.UI.PanelZoomInactivePercent = 30

	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Paths:  config.Paths{}.WithResolvedLocations(),
		Theme:  theme.Default(),
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	layZoomed := app.layoutForTerminalSize(100, 30)
	if layZoomed.Left.Width != 70 || layZoomed.Right.Width != 30 {
		t.Fatalf("without preview Left=%d Right=%d want 70/30", layZoomed.Left.Width, layZoomed.Right.Width)
	}

	app.commandsMu.Lock()
	app.model.FilePreview.Open = true
	app.model.FilePreview.Phase = ui.FilePreviewPhaseDone
	app.model.FilePreview.CombinedText = "hello"
	app.commandsMu.Unlock()

	layEven := app.layoutForTerminalSize(100, 30)
	if layEven.Left.Width != 50 || layEven.Right.Width != 50 {
		t.Fatalf("with preview Left=%d Right=%d want 50/50", layEven.Left.Width, layEven.Right.Width)
	}

	app.commandsMu.Lock()
	app.model.FilePreview = ui.FilePreviewState{}
	app.commandsMu.Unlock()
	app.model.QuickViewEnabled = true
	layQV := app.layoutForTerminalSize(100, 30)
	if layQV.Left.Width != 50 || layQV.Right.Width != 50 {
		t.Fatalf("with quick view armed Left=%d Right=%d want 50/50", layQV.Left.Width, layQV.Right.Width)
	}
}

func TestLayoutForTerminalSizeDisablesZoomAtOrAboveDisabledAboveWidth(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(160, 30)

	cfg := config.Default()
	cfg.UI.ZoomActivePanel = true
	cfg.UI.ZoomActivePanelDisabledAboveWidth = 155
	cfg.UI.PanelZoomActivePercent = 70
	cfg.UI.PanelZoomInactivePercent = 30

	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Paths:  config.Paths{}.WithResolvedLocations(),
		Theme:  theme.Default(),
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	lay := app.layoutForTerminalSize(160, 30)
	if lay.Left.Width != 80 || lay.Right.Width != 80 {
		t.Fatalf("wide terminal Left=%d Right=%d want 50/50", lay.Left.Width, lay.Right.Width)
	}

	layNarrow := app.layoutForTerminalSize(154, 30)
	if layNarrow.Left.Width != 107 || layNarrow.Right.Width != 47 {
		t.Fatalf("below gate Left=%d Right=%d want 70%%/30%% of width", layNarrow.Left.Width, layNarrow.Right.Width)
	}

	layBoundary := app.layoutForTerminalSize(155, 30)
	if layBoundary.Left.Width != 77 || layBoundary.Right.Width != 78 {
		t.Fatalf("width == gate Left=%d Right=%d want ~50/50", layBoundary.Left.Width, layBoundary.Right.Width)
	}
}

func TestLayoutForTerminalSizeZoomNotSuppressedWhenDisabledAboveWidthIsZero(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(300, 30)

	cfg := config.Default()
	cfg.UI.ZoomActivePanel = true
	cfg.UI.ZoomActivePanelDisabledAboveWidth = 0
	cfg.UI.PanelZoomActivePercent = 70
	cfg.UI.PanelZoomInactivePercent = 30

	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Paths:  config.Paths{}.WithResolvedLocations(),
		Theme:  theme.Default(),
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	lay := app.layoutForTerminalSize(300, 30)
	if lay.Left.Width != 210 || lay.Right.Width != 90 {
		t.Fatalf("Left=%d Right=%d want 70/30 split", lay.Left.Width, lay.Right.Width)
	}
}

func TestPanelToggleZoomNoOpOnWideTerminal(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(160, 30)

	cfg := config.Default()
	cfg.UI.ZoomActivePanelDisabledAboveWidth = 155

	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Paths:  config.Paths{}.WithResolvedLocations(),
		Theme:  theme.Default(),
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	app.dispatch(keymap.ActionPanelToggleZoomActivePanel)
	if app.zoomActivePanelOverride != nil {
		t.Fatalf("zoom override = %v, want nil (toggle ignored)", app.zoomActivePanelOverride)
	}
	if !strings.Contains(app.model.Message, "≥ 155") {
		t.Fatalf("transient message = %q, want threshold mention", app.model.Message)
	}
}

func TestFilePreviewFocusScrollAndTabReturnsToActivePanelFileList(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(100, 30)

	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: config.Default(),
		Paths:  config.Paths{}.WithResolvedLocations(),
		Theme:  theme.Default(),
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	app.patchFilePreview(func(st *ui.FilePreviewState) {
		st.Open = true
		st.Phase = ui.FilePreviewPhaseDone
		st.CombinedText = strings.Repeat("line\n", 40)
		st.Scroll = 0
	})
	app.model.ActiveSubFocus = ui.SubFocusInactivePreview

	app.dispatch(keymap.ActionNavDown)
	if app.model.FilePreview.Scroll != 1 {
		t.Fatalf("FilePreview.Scroll = %d, want 1", app.model.FilePreview.Scroll)
	}

	prevActive := app.model.ActivePanel
	app.dispatch(keymap.ActionPanelSwitch)
	if app.model.ActivePanel != prevActive {
		t.Fatalf("ActivePanel = %d, want unchanged %d after Tab from preview", app.model.ActivePanel, prevActive)
	}
	if app.model.ActiveSubFocus != ui.SubFocusFileList {
		t.Fatalf("ActiveSubFocus = %v, want SubFocusFileList", app.model.ActiveSubFocus)
	}
}

func TestPanelToggleZoomNoOpWhileFilePreviewOpen(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(100, 30)

	cfg := config.Default()
	cfg.UI.ZoomActivePanel = false

	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: cfg,
		Paths:  config.Paths{}.WithResolvedLocations(),
		Theme:  theme.Default(),
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	app.commandsMu.Lock()
	app.model.FilePreview.Open = true
	app.model.FilePreview.Phase = ui.FilePreviewPhaseDone
	app.model.FilePreview.CombinedText = "x"
	app.commandsMu.Unlock()

	app.dispatch(keymap.ActionPanelToggleZoomActivePanel)
	if app.zoomActivePanelOverride != nil {
		t.Fatalf("zoom override = %v, want nil (toggle ignored)", app.zoomActivePanelOverride)
	}
	if !strings.Contains(app.model.Message, "Zoom disabled") {
		t.Fatalf("transient message = %q, want mention of zoom disabled", app.model.Message)
	}
}

func TestConfigDialogApplyPersistsDefaultListingFormat(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	appPaths := config.Paths{ConfigDir: filepath.Join(t.TempDir(), "persist-cfg-lf")}.WithResolvedLocations()
	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: config.Default(),
		Paths:  appPaths,
		Theme:  theme.Default(),
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	app.openConfigDialog()
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'p', tcell.ModNone))
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if quit {
		t.Fatal("handleKey() quit = true, want false")
	}
	if app.model.Left.ListFormat != panel.ListFormatPerm || app.model.Right.ListFormat != panel.ListFormatPerm {
		t.Fatalf("panels ListFormat = %v/%v, want perm", app.model.Left.ListFormat, app.model.Right.ListFormat)
	}
	reloaded, err := config.LoadFromPaths(appPaths)
	if err != nil {
		t.Fatalf("LoadFromPaths after persist: %v", err)
	}
	if reloaded.DefaultListingFormat != config.ListingFormatPerm {
		t.Fatalf("persisted default_listing_format = %q, want %q", reloaded.DefaultListingFormat, config.ListingFormatPerm)
	}
}

func TestOptionsMenuOpensThemeDialog(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	defaultTheme := theme.Default()
	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: config.Default(),
		Theme:  defaultTheme,
		ThemeChoices: []theme.NamedTheme{
			{Name: defaultTheme.Name, Label: "Default", Theme: defaultTheme},
		},
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	app.dispatch(keymap.ActionAppOpenMenu)
	app.moveMenu(3) // File → Command → Display → Options
	// Open pulldown for Options menu, then press shortcut.
	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 't', tcell.ModNone))

	if quit {
		t.Fatal("handleKey() quit = true, want false")
	}
	if app.model.Menu.Open {
		t.Fatal("menu open = true, want closed")
	}
	if !app.model.ThemeDialog.Open {
		t.Fatal("theme dialog open = false, want true")
	}
	if app.model.ThemeDialog.Selected != 0 {
		t.Fatalf("theme dialog selected = %d, want current theme index 0", app.model.ThemeDialog.Selected)
	}
}

func TestThemeDialogAppliesThemeImmediately(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	defaultTheme := theme.Default()
	secondTheme, themePaths := loadTestTheme(t)
	appPaths := config.Paths{
		ConfigDir: filepath.Join(t.TempDir(), "persist-theme"),
		ThemesDir: themePaths.ThemesDir,
	}.WithResolvedLocations()
	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: config.Default(),
		Theme:  defaultTheme,
		Paths:  appPaths,
		ThemeChoices: []theme.NamedTheme{
			{Name: defaultTheme.Name, Label: "Default", Theme: defaultTheme},
			{Name: secondTheme.Name, Label: "Test Theme", Theme: secondTheme},
		},
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	app.openThemeDialog()
	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if quit {
		t.Fatal("handleKey() quit = true, want false")
	}
	if app.model.ThemeDialog.Open {
		t.Fatal("theme dialog open = true, want closed")
	}
	if app.styles.Name != "test-theme" {
		t.Fatalf("theme name = %q, want test-theme", app.styles.Name)
	}
	if app.config.Theme != "test-theme" {
		t.Fatalf("config theme = %q, want test-theme", app.config.Theme)
	}
	if app.model.ThemeDialog.CurrentName != "test-theme" {
		t.Fatalf("theme dialog current name = %q, want test-theme", app.model.ThemeDialog.CurrentName)
	}
	if app.model.Message != "Theme changed to test-theme" {
		t.Fatalf("Message = %q, want theme changed message", app.model.Message)
	}
	reloaded, err := config.LoadFromPaths(appPaths)
	if err != nil {
		t.Fatalf("LoadFromPaths after persist: %v", err)
	}
	if reloaded.Theme != "test-theme" {
		t.Fatalf("persisted Theme = %q, want test-theme", reloaded.Theme)
	}
}

func TestThemeDialogNavigatePreviewsWithoutPersist(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	defaultTheme := theme.Default()
	secondTheme, themePaths := loadTestTheme(t)
	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: config.Default(),
		Theme:  defaultTheme,
		Paths:  themePaths,
		ThemeChoices: []theme.NamedTheme{
			{Name: defaultTheme.Name, Label: "Default", Theme: defaultTheme},
			{Name: secondTheme.Name, Label: "Test Theme", Theme: secondTheme},
		},
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	app.openThemeDialog()
	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))

	if !app.model.ThemeDialog.Open {
		t.Fatal("theme dialog open = false, want true")
	}
	if app.styles.Name != "test-theme" {
		t.Fatalf("preview theme name = %q, want test-theme", app.styles.Name)
	}
	if app.config.Theme != defaultTheme.Name {
		t.Fatalf("config theme = %q, want persisted default %q", app.config.Theme, defaultTheme.Name)
	}
	if app.model.ThemeDialog.CurrentName != defaultTheme.Name {
		t.Fatalf("ThemeDialog.CurrentName = %q, want %q (marker for saved theme)", app.model.ThemeDialog.CurrentName, defaultTheme.Name)
	}
}

func TestThemeDialogEscRevertsPreview(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	defaultTheme := theme.Default()
	secondTheme, themePaths := loadTestTheme(t)
	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: config.Default(),
		Theme:  defaultTheme,
		Paths:  themePaths,
		ThemeChoices: []theme.NamedTheme{
			{Name: defaultTheme.Name, Label: "Default", Theme: defaultTheme},
			{Name: secondTheme.Name, Label: "Test Theme", Theme: secondTheme},
		},
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	app.openThemeDialog()
	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	app.handleKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))

	if app.model.ThemeDialog.Open {
		t.Fatal("theme dialog open = true, want closed")
	}
	if app.styles.Name != defaultTheme.Name {
		t.Fatalf("theme name after cancel = %q, want restored %q", app.styles.Name, defaultTheme.Name)
	}
	if app.config.Theme != defaultTheme.Name {
		t.Fatalf("config theme after cancel = %q, want %q", app.config.Theme, defaultTheme.Name)
	}
	if app.model.ThemeDialog.CurrentName != defaultTheme.Name {
		t.Fatalf("ThemeDialog.CurrentName = %q, want %q", app.model.ThemeDialog.CurrentName, defaultTheme.Name)
	}
}

func TestActiveFooterKeysThemeDialogShowsF5Reload(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	defaultTheme := theme.Default()
	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: config.Default(),
		Theme:  defaultTheme,
		ThemeChoices: []theme.NamedTheme{
			{Name: defaultTheme.Name, Label: "Default", Theme: defaultTheme},
		},
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	app.openThemeDialog()
	if !app.model.ThemeDialog.Open {
		t.Fatal("theme dialog open = false, want true")
	}
	keys := app.activeFooterKeys()
	if len(keys) != 3 {
		t.Fatalf("theme dialog footer len = %d, want Esc + F5 + F10", len(keys))
	}
	if keys[0].Key != tcell.KeyEsc || keys[0].Hint != "Close" {
		t.Fatalf("first footer key = %+v, want Esc Close", keys[0])
	}
	if keys[1].Key != tcell.KeyF5 || keys[1].Hint != "Reload" {
		t.Fatalf("second footer key = %+v, want F5 Reload", keys[1])
	}
	if keys[2].Key != tcell.KeyF10 {
		t.Fatalf("third footer key = %+v, want F10", keys[2])
	}
}

func TestHandleKeyOpeningFileDialogRendersDialogFooterImmediately(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyF7, 0, tcell.ModNone))
	if quit {
		t.Fatal("handleKey(F7) quit = true, want false")
	}
	if !app.model.FileDialog.Open || app.model.FileDialog.DialogType != ui.FileDialogMkdir {
		t.Fatalf("file dialog = %+v, want open mkdir dialog", app.model.FileDialog)
	}

	footer := screenLine(screen, 19, 80)
	if strings.Contains(footer, "Mkdir") {
		t.Fatalf("footer = %q, should hide browser F7 hint immediately", footer)
	}
	if !strings.Contains(footer, "Esc") || !strings.Contains(footer, "Close") {
		t.Fatalf("footer = %q, want Esc Close first", footer)
	}
	if !strings.Contains(footer, "F10") || !strings.Contains(footer, "Quit") {
		t.Fatalf("footer = %q, want dialog footer with F10 Quit", footer)
	}
}

func TestThemeDialogF5ReloadsCurrentPreviewFromDisk(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))
	themesDir := t.TempDir()

	base, err := os.ReadFile(filepath.Join("..", "..", "themes", "default.toml"))
	if err != nil {
		t.Fatalf("read default theme fixture: %v", err)
	}
	writeDiskDefault := func(hex string) {
		content := strings.Replace(string(base),
			`bar = { fg = "default" }`,
			fmt.Sprintf(`bar = { fg = "white", bg = %q }`, hex), 1)
		if err := os.WriteFile(filepath.Join(themesDir, "override.toml"), []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	writeDiskDefault("#111111")

	paths := config.Paths{ThemesDir: themesDir}.WithResolvedLocations()
	styles, err := theme.Resolve("default", paths.ThemesDir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	choices, err := theme.ThemeChoices(paths.ThemesDir)
	if err != nil {
		t.Fatalf("ThemeChoices: %v", err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config:       config.Default(),
		Theme:        styles,
		ThemeChoices: choices,
		Paths:        paths,
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	app.openThemeDialog()
	_, bg1, _ := app.styles.MenuBar.Decompose()
	if want := tcell.NewRGBColor(0x11, 0x11, 0x11); bg1 != want {
		t.Fatalf("initial preview bg = %v, want %v", bg1, want)
	}

	writeDiskDefault("#222222")
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone))
	if quit {
		t.Fatal("handleKey(F5) quit = true, want false")
	}
	_, bg2, _ := app.styles.MenuBar.Decompose()
	if want := tcell.NewRGBColor(0x22, 0x22, 0x22); bg2 != want {
		t.Fatalf("after F5 bg = %v, want updated disk theme %v", bg2, want)
	}
}

func TestThemeDialogF5ReloadsMenuDropdownAccentFromDisk(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))
	themesDir := t.TempDir()

	base, err := os.ReadFile(filepath.Join("..", "..", "themes", "default.toml"))
	if err != nil {
		t.Fatalf("read default theme fixture: %v", err)
	}
	writeAccentFG := func(paletteName string) {
		content := strings.Replace(string(base),
			`dropdown.accent = { fg = "white"}`,
			fmt.Sprintf(`dropdown.accent = { fg = %q, bold = true }`, paletteName), 1)
		if err := os.WriteFile(filepath.Join(themesDir, "override.toml"), []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}
	writeAccentFG("bright_red")

	paths := config.Paths{ThemesDir: themesDir}.WithResolvedLocations()
	styles, err := theme.Resolve("default", paths.ThemesDir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	choices, err := theme.ThemeChoices(paths.ThemesDir)
	if err != nil {
		t.Fatalf("ThemeChoices: %v", err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config:       config.Default(),
		Theme:        styles,
		ThemeChoices: choices,
		Paths:        paths,
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	app.openThemeDialog()
	fg1, _, _ := app.styles.MenuDropdownAccent.Decompose()
	if fg1 != tcell.PaletteColor(9) {
		t.Fatalf("initial preview menu.dropdown.accent fg = %v, want bright_red (ANSI 9)", fg1)
	}

	writeAccentFG("bright_green")
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone))
	if quit {
		t.Fatal("handleKey(F5) quit = true, want false")
	}
	fg2, _, _ := app.styles.MenuDropdownAccent.Decompose()
	if fg2 != tcell.PaletteColor(10) {
		t.Fatalf("after F5 menu.dropdown.accent fg = %v, want bright_green (ANSI 10)", fg2)
	}
}

func TestThemePreviewReloadErrorSetsCriticalStatusMessage(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX directory permissions")
	}
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))
	themesDir := t.TempDir()

	base, err := os.ReadFile(filepath.Join("..", "..", "themes", "default.toml"))
	if err != nil {
		t.Fatalf("read default theme fixture: %v", err)
	}
	content := strings.Replace(string(base),
		`bar = { fg = "default" }`,
		`bar = { fg = "white", bg = "#111111" }`, 1)
	if err := os.WriteFile(filepath.Join(themesDir, "override.toml"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	paths := config.Paths{ThemesDir: themesDir}.WithResolvedLocations()
	styles, err := theme.Resolve("default", paths.ThemesDir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	choices, err := theme.ThemeChoices(paths.ThemesDir)
	if err != nil {
		t.Fatalf("ThemeChoices: %v", err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := NewWithOptions(screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config:       config.Default(),
		Theme:        styles,
		ThemeChoices: choices,
		Paths:        paths,
	})
	if err != nil {
		t.Fatalf("NewWithOptions() error = %v", err)
	}

	app.openThemeDialog()
	if err := os.Chmod(themesDir, 0); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(themesDir, 0700) })

	app.handleKey(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone))
	if app.model.MessageDialog.Open {
		t.Fatal("expected no message dialog after reload error (transient message instead)")
	}
	if strings.TrimSpace(app.model.Message) == "" {
		t.Fatal("expected non-empty transient message after reload error")
	}
	if app.model.MessageUrgency != ui.MessageUrgencyCritical {
		t.Fatalf("MessageUrgency = %v, want MessageUrgencyCritical", app.model.MessageUrgency)
	}
	if !app.model.ThemeDialog.Open {
		t.Fatal("theme dialog should remain open")
	}
}

func TestFirstMessageLine(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"single", "single"},
		{"first\nsecond", "first"},
		{"\n\nmiddle\n", "middle"},
		{"  spaced  ", "spaced"},
	}
	for _, tt := range tests {
		if got := firstMessageLine(tt.in); got != tt.want {
			t.Errorf("firstMessageLine(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
	joined := errors.Join(errors.New("alpha"), errors.New("beta"))
	if got := firstMessageLine(joined.Error()); got != "alpha" {
		t.Fatalf("first line of joined error = %q, want alpha", got)
	}
}

func TestMenuShortcutActivatesFullscreenFileView(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := New(screen, func() (string, error) {
		return dir, nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	app.dispatch(keymap.ActionFileView)
	if app.model.ViewMode != ui.ViewFilePreview {
		t.Fatalf("ViewMode = %v, want ViewFilePreview after file.view", app.model.ViewMode)
	}
	app.commandsMu.RLock()
	open := app.model.FullscreenFilePreview.Open
	app.commandsMu.RUnlock()
	if !open {
		t.Fatal("FullscreenFilePreview.Open = false, want true after file.view")
	}
}

func TestFullscreenFilePreviewArrowDownScrollsWithoutNavigatingList(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		writeFile(t, filepath.Join(dir, name))
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := New(screen, func() (string, error) {
		return dir, nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	app.model.ViewMode = ui.ViewFilePreview
	app.patchFullscreenFilePreview(func(st *ui.FilePreviewState) {
		st.Open = true
		st.Phase = ui.FilePreviewPhaseDone
		st.CombinedText = strings.Repeat("x\n", 200)
		st.Scroll = 0
	})

	cursorBefore := app.activePanel().Cursor
	app.handleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if got := app.activePanel().Cursor; got != cursorBefore {
		t.Fatalf("list cursor moved %d -> %d; Down must scroll preview, not nav.down", cursorBefore, got)
	}
	app.commandsMu.RLock()
	scroll := app.model.FullscreenFilePreview.Scroll
	app.commandsMu.RUnlock()
	if scroll != 1 {
		t.Fatalf("FullscreenFilePreview.Scroll = %d, want 1 after first Down", scroll)
	}
}

func TestFullscreenFilePreviewLeftBackspaceDoNotChangePanelPath(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	writeFile(t, filepath.Join(sub, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := New(screen, func() (string, error) {
		return dir, nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := app.activePanel().Load(sub); err != nil {
		t.Fatalf("Load(sub): %v", err)
	}
	pathBefore := app.activePanel().Path

	app.model.ViewMode = ui.ViewFilePreview
	app.patchFullscreenFilePreview(func(st *ui.FilePreviewState) {
		st.Open = true
		st.Phase = ui.FilePreviewPhaseDone
		st.CombinedText = "x\n"
		st.Scroll = 0
	})

	app.handleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	if got := app.activePanel().Path; !got.Equal(pathBefore) {
		t.Fatalf("KeyLeft changed path %q -> %q", pathBefore, got)
	}
	app.handleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone))
	if got := app.activePanel().Path; !got.Equal(pathBefore) {
		t.Fatalf("Backspace changed path %q -> %q", pathBefore, got)
	}
}

func TestFullscreenFilePreviewRightDoesNotMoveListCursor(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		writeFile(t, filepath.Join(dir, name))
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := New(screen, func() (string, error) {
		return dir, nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	app.model.ViewMode = ui.ViewFilePreview
	app.patchFullscreenFilePreview(func(st *ui.FilePreviewState) {
		st.Open = true
		st.Phase = ui.FilePreviewPhaseDone
		st.CombinedText = strings.Repeat("x\n", 200)
		st.Scroll = 0
	})

	cursorBefore := app.activePanel().Cursor
	app.handleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if got := app.activePanel().Cursor; got != cursorBefore {
		t.Fatalf("list cursor moved %d -> %d; Right must not nav.open", cursorBefore, got)
	}
}

func TestFullscreenFilePreviewDoesNotOpenMenuFromDispatchOrF9(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := New(screen, func() (string, error) {
		return dir, nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	app.model.ViewMode = ui.ViewFilePreview
	app.patchFullscreenFilePreview(func(st *ui.FilePreviewState) {
		st.Open = true
		st.Phase = ui.FilePreviewPhaseDone
		st.CombinedText = "x\n"
		st.Scroll = 0
	})

	app.dispatch(keymap.ActionAppOpenMenu)
	if app.model.Menu.Open {
		t.Fatal("ActionAppOpenMenu must not open menu during fullscreen file preview")
	}
	app.handleFilePreviewViewKey(tcell.NewEventKey(tcell.KeyF9, 0, tcell.ModNone))
	if app.model.Menu.Open {
		t.Fatal("F9 must not open menu during fullscreen file preview")
	}
	for _, fk := range app.activeFooterKeys() {
		if fk.Key == tcell.KeyF9 {
			t.Fatalf("footer must not list F9, got %+v", app.activeFooterKeys())
		}
	}
}

func TestMenuShortcutCopyOpensTransferDialog(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))
	dstDir := filepath.Join(dir, "dest")
	if err := os.Mkdir(dstDir, 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := New(screen, func() (string, error) {
		return dir, nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := app.inactivePanel().Load(dstDir); err != nil {
		t.Fatalf("inactive Load: %v", err)
	}

	app.dispatch(keymap.ActionAppOpenMenu)
	// Open pulldown first, then press shortcut.
	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModNone))

	if quit {
		t.Fatal("handleKey() quit = true, want false")
	}
	if app.model.Menu.Open {
		t.Fatal("menu open = true, want closed")
	}
	if !app.model.TransferDialog.Open || app.model.TransferDialog.Kind != ui.TransferKindCopy {
		t.Fatal("File menu Copy shortcut should open transfer dialog")
	}
	if len(app.jobState.AllJobs()) != 0 {
		t.Fatalf("expected no job before confirm, got %d", len(app.jobState.AllJobs()))
	}
}

func TestFileMenuExitQuits(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := New(screen, func() (string, error) {
		return dir, nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	app.dispatch(keymap.ActionAppOpenMenu)
	// Open pulldown first, then press shortcut.
	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'i', tcell.ModNone))

	if !quit {
		t.Fatal("handleKey() quit = false, want true")
	}
}

func TestQuickFilterFunctionKeyClosesFuzzyAndRunsFullscreenFileView(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := New(screen, func() (string, error) {
		return dir, nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	app.activePanel().OpenFilter(app.activeViewportRows())
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone))

	// F3 runs file.view (full-screen file view) after the bound action clears the filter.
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyF3, 0, tcell.ModNone))
	if quit {
		t.Fatal("handleKey() quit = true, want false")
	}
	if app.model.Left.Filter.Editing || app.model.Left.Filter.Active || app.model.Left.Filter.Query != "" {
		t.Fatalf("filter should be cleared, got editing=%v active=%v query=%q",
			app.model.Left.Filter.Editing, app.model.Left.Filter.Active, app.model.Left.Filter.Query)
	}
	if app.model.ViewMode != ui.ViewFilePreview {
		t.Fatalf("ViewMode = %v, want ViewFilePreview after F3 from quick filter", app.model.ViewMode)
	}
	app.commandsMu.RLock()
	open := app.model.FullscreenFilePreview.Open
	app.commandsMu.RUnlock()
	if !open {
		t.Fatal("FullscreenFilePreview.Open = false, want true after F3 from quick filter")
	}
}

func TestQuickFilterF9ClosesFuzzyAndOpensMenu(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := New(screen, func() (string, error) {
		return dir, nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	app.activePanel().OpenFilter(app.activeViewportRows())
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyF9, 0, tcell.ModNone))
	if quit {
		t.Fatal("handleKey() quit = true, want false")
	}
	if app.model.Left.Filter.Editing || app.model.Left.Filter.Active {
		t.Fatal("filter should be cleared after F9")
	}
	if !app.model.Menu.Open {
		t.Fatal("menu open = false, want true after F9 from quick filter")
	}
}

func TestQuickFilterF10Quits(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := New(screen, func() (string, error) {
		return dir, nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	app.activePanel().OpenFilter(app.activeViewportRows())
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyF10, 0, tcell.ModNone))
	if !quit {
		t.Fatal("handleKey() quit = false, want true for F10 from quick filter")
	}
	if app.model.Left.Filter.Editing || app.model.Left.Filter.Active {
		t.Fatal("filter should be cleared before quit")
	}
}

func TestQuitImmediateSkipsConfirmation(t *testing.T) {
	t.Parallel()
	app := testAppMinimal(t)
	root := t.TempDir()
	job := &jobs.Job{ID: "j", Type: jobs.TypeCopy, Status: jobs.StatusRunning, Sources: pathloc.PathsForTest(root)}
	app.jobState.Queue().Enqueue(job)

	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyF10, 0, tcell.ModNone))
	if quit {
		t.Fatal("F10 with active job should not quit immediately")
	}
	if !app.model.QuitConfirm.Open {
		t.Fatal("expected quit confirmation dialog")
	}

	quit2, _ := app.handleKey(tcell.NewEventKey(tcell.KeyF10, 0, tcell.ModShift))
	if !quit2 {
		t.Fatal("Shift+F10 should quit immediately")
	}
	if app.model.QuitConfirm.Open {
		t.Fatal("quit confirm should be cleared on immediate quit")
	}
}

func TestQuitConfirmDialogAltShortcuts(t *testing.T) {
	t.Parallel()
	app := testAppMinimal(t)
	root := t.TempDir()
	job := &jobs.Job{ID: "j", Type: jobs.TypeCopy, Status: jobs.StatusRunning, Sources: pathloc.PathsForTest(root)}
	app.jobState.Queue().Enqueue(job)

	_, _ = app.handleKey(tcell.NewEventKey(tcell.KeyF10, 0, tcell.ModNone))
	if !app.model.QuitConfirm.Open {
		t.Fatal("expected quit confirmation dialog")
	}

	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModAlt))
	if quit {
		t.Fatal("Alt+S should stay in the app")
	}
	if app.model.QuitConfirm.Open {
		t.Fatal("Alt+S should close quit confirmation")
	}

	_, _ = app.handleKey(tcell.NewEventKey(tcell.KeyF10, 0, tcell.ModNone))
	if !app.model.QuitConfirm.Open {
		t.Fatal("expected quit confirmation dialog again")
	}

	quit, _ = app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'Q', tcell.ModAlt))
	if !quit {
		t.Fatal("Alt+Q should quit")
	}
	if app.model.QuitConfirm.Open {
		t.Fatal("Alt+Q should close quit confirmation")
	}
}

func TestQuickFilterEmptyOverlayThenTypingEnterOnFileClearsFuzzy(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "alpha.txt"))
	writeFile(t, filepath.Join(dir, "beta.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := New(screen, func() (string, error) {
		return dir, nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	app.activePanel().OpenFilter(app.activeViewportRows())
	if !app.model.Left.Filter.Editing {
		t.Fatal("filter editing = false, want true after OpenFilter")
	}
	for _, r := range "beta" {
		app.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if app.model.Left.Filter.Editing || app.model.Left.Filter.Active || app.model.Left.Filter.Query != "" {
		t.Fatalf("filter editing=%v active=%v query=%q, want fuzzy cleared after Enter on a file",
			app.model.Left.Filter.Editing, app.model.Left.Filter.Active, app.model.Left.Filter.Query)
	}
	if app.model.Left.VisibleEntryCount() != 2 {
		t.Fatalf("visible=%d, want both files visible", app.model.Left.VisibleEntryCount())
	}
	entry, ok := app.model.Left.CurrentEntry()
	if !ok || entry.Name != "beta.txt" {
		t.Fatalf("CurrentEntry() = %q ok=%v, want beta.txt", entry.Name, ok)
	}
}

func TestPlainTypingStartsQuickFilterAndMovesToFirstVisibleMatch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "notes.txt"))
	writeFile(t, filepath.Join(dir, "src"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := New(screen, func() (string, error) {
		return dir, nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModNone))

	if !app.model.Left.Filter.Editing || app.model.Left.Filter.Query != "s" {
		t.Fatalf("filter editing=%v query=%q, want typing to start query s", app.model.Left.Filter.Editing, app.model.Left.Filter.Query)
	}
	entry, ok := app.model.Left.CurrentEntry()
	if !ok || entry.Name != "notes.txt" {
		t.Fatalf("CurrentEntry() = %q ok=%v, want first visible match notes.txt", entry.Name, ok)
	}
}

func TestPlainTypingMultiLetterSelectsBestRankedMatch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "abzzc.txt"))
	writeFile(t, filepath.Join(dir, "abc.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := New(screen, func() (string, error) {
		return dir, nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	for _, r := range "abc" {
		app.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	entry, ok := app.model.Left.CurrentEntry()
	if !ok || entry.Name != "abc.txt" {
		t.Fatalf("CurrentEntry() = %q ok=%v, want best ranked abc.txt", entry.Name, ok)
	}
}

func TestQuickFilterEnterOpensDirectoryAndClearsQuery(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "subdir")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("Mkdir(sub): %v", err)
	}
	writeFile(t, filepath.Join(dir, "other.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := New(screen, func() (string, error) {
		return dir, nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	for _, r := range "sub" {
		app.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}

	app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	wantPath := filepath.Clean(sub)
	if got := filepath.Clean(app.model.Left.Path.String()); got != wantPath {
		t.Fatalf("left path=%q after Enter want %q", got, wantPath)
	}
	if app.model.Left.Filter.Active || app.model.Left.Filter.Query != "" || app.model.Left.Filter.Editing {
		t.Fatalf("filter cleared: active=%v query=%q editing=%v want all off",
			app.model.Left.Filter.Active, app.model.Left.Filter.Query, app.model.Left.Filter.Editing)
	}
}

func TestQuickFilterInsertSelectsAndAdvancesCursor(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "alpha.txt"))
	writeFile(t, filepath.Join(dir, "alpine.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := New(screen, func() (string, error) {
		return dir, nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone))
	if app.model.Left.Filter.Query != "a" {
		t.Fatalf("query=%q want a after typing", app.model.Left.Filter.Query)
	}

	entryPath := filepath.Join(dir, "alpha.txt")
	if app.model.Left.SelectedPaths[entryPath] {
		t.Fatal("alpha.txt selected before Insert, want not selected")
	}

	app.handleKey(tcell.NewEventKey(tcell.KeyInsert, 0, tcell.ModNone))
	if !app.model.Left.Filter.Active {
		t.Fatal("filter closed after Insert, want open for multi-select")
	}
	if !app.model.Left.SelectedPaths[entryPath] {
		t.Fatal("alpha.txt not selected after Insert, want selected")
	}
	if app.model.Left.Cursor != 1 {
		t.Fatalf("cursor=%d after Insert, want 1 (moved down past filtered entry)", app.model.Left.Cursor)
	}
}

func TestQuickFilterEmptyQueryEnterExitsEditing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "alpha.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := New(screen, func() (string, error) {
		return dir, nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	app.activePanel().OpenFilter(app.activeViewportRows())
	if !app.model.Left.Filter.Editing {
		t.Fatal("want editing after OpenFilter")
	}
	app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if app.model.Left.Filter.Editing {
		t.Fatal("want editing=false after Enter with empty query")
	}
	if app.model.Left.Filter.Active || app.model.Left.Filter.Query != "" {
		t.Fatalf("want no active query, got active=%v query=%q", app.model.Left.Filter.Active, app.model.Left.Filter.Query)
	}
}

func TestFilterModeEscCancelsInsteadOfQuitting(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "alpha.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := New(screen, func() (string, error) {
		return dir, nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	app.activePanel().OpenFilter(app.activeViewportRows())
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if quit {
		t.Fatal("handleKey(Esc) quit = true, want filter cancel")
	}
	if app.model.Left.Filter.Editing || app.model.Left.Filter.Active {
		t.Fatalf("filter editing=%v active=%v, want canceled", app.model.Left.Filter.Editing, app.model.Left.Filter.Active)
	}
}

func TestQuickFilterKeymapActionClosesFilterAndOpensDirInOtherPanel(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "subdir")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	writeFile(t, filepath.Join(dir, "other.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	ev, ok := app.keys.FirstEventKeyForAction(keymap.ActionPanelOpenDirInOther)
	if !ok {
		t.Fatal("no key bound to ActionPanelOpenDirInOther")
	}

	app.activePanel().OpenFilter(app.activeViewportRows())
	for _, r := range "sub" {
		app.handleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	if app.model.Left.Filter.Query != "sub" {
		t.Fatalf("query=%q want sub", app.model.Left.Filter.Query)
	}
	entry, okEntry := app.model.Left.CurrentEntry()
	if !okEntry || entry.Name != "subdir" {
		t.Fatalf("CurrentEntry() = %q ok=%v, want subdir under cursor before shortcut", entry.Name, okEntry)
	}

	quit, _ := app.handleKey(ev)
	if quit {
		t.Fatal("handleKey() quit = true, want false")
	}
	if app.model.Left.Filter.Editing || app.model.Left.Filter.Active || app.model.Left.Filter.Query != "" {
		t.Fatalf("filter should be cleared, got editing=%v active=%v query=%q",
			app.model.Left.Filter.Editing, app.model.Left.Filter.Active, app.model.Left.Filter.Query)
	}
	want := filepath.Clean(sub)
	if got := filepath.Clean(app.model.Right.Path.String()); got != want {
		t.Fatalf("right panel path=%q want %q", got, want)
	}
}

func TestQuickFilterUpDownCyclesMatches(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "assets"))
	writeFile(t, filepath.Join(dir, "notes.txt"))
	writeFile(t, filepath.Join(dir, "src"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app, err := New(screen, func() (string, error) {
		return dir, nil
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	app.activePanel().OpenFilter(app.activeViewportRows())
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModNone))
	if app.model.Left.Filter.Query != "s" {
		t.Fatalf("query=%q want s", app.model.Left.Filter.Query)
	}
	if !app.model.Left.Filter.Editing || !app.model.Left.Filter.Active {
		t.Fatalf("want filter editing and active, got editing=%v active=%v",
			app.model.Left.Filter.Editing, app.model.Left.Filter.Active)
	}

	first, ok := app.model.Left.CurrentEntry()
	if !ok {
		t.Fatal("CurrentEntry() = false")
	}
	if first.Name != "assets" {
		t.Fatalf("CurrentEntry() = %q, want first visible match assets", first.Name)
	}
	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	second, ok := app.model.Left.CurrentEntry()
	if !ok {
		t.Fatal("CurrentEntry() = false after Down")
	}
	if second.Name != "notes.txt" {
		t.Fatalf("after Down want next visible match notes.txt, got %q", second.Name)
	}
	if app.model.Left.Filter.Query != "s" {
		t.Fatalf("query should stay set, got %q", app.model.Left.Filter.Query)
	}

	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	third, ok := app.model.Left.CurrentEntry()
	if !ok {
		t.Fatal("CurrentEntry() = false after second Down")
	}
	if third.Name != "src" {
		t.Fatalf("after second Down want next visible match src, got %q", third.Name)
	}

	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	wrapped, ok := app.model.Left.CurrentEntry()
	if !ok {
		t.Fatal("CurrentEntry() = false after third Down")
	}
	if wrapped.Name != first.Name {
		t.Fatalf("after Down from last match want wrap to %q, got %q", first.Name, wrapped.Name)
	}

	app.handleKey(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))
	backToLast, ok := app.model.Left.CurrentEntry()
	if !ok {
		t.Fatal("CurrentEntry() = false after Up from first")
	}
	if backToLast.Name != "src" {
		t.Fatalf("after Up from first match want wrap to src, got %q", backToLast.Name)
	}
}

func TestQuickFilterCtrlBackspaceClearsQuery(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "alpha.txt"))
	writeFile(t, filepath.Join(dir, "beta.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	// Type something into filter
	app.activePanel().OpenFilter(app.activeViewportRows())
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone))
	app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'l', tcell.ModNone))

	if app.model.Left.Filter.Query != "al" {
		t.Fatalf("query=%q want al", app.model.Left.Filter.Query)
	}
	if !app.model.Left.Filter.Editing {
		t.Fatal("want filter editing")
	}

	// Ctrl+Backspace should clear query but keep editing
	app.handleKey(tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModCtrl))

	if app.model.Left.Filter.Query != "" {
		t.Fatalf("query=%q want empty after Ctrl+Backspace", app.model.Left.Filter.Query)
	}
	if !app.model.Left.Filter.Editing {
		t.Fatal("Ctrl+Backspace should keep filter in editing mode")
	}
	if app.model.Left.Filter.Active {
		t.Fatal("Ctrl+Backspace should deactivate filter")
	}
}

func TestFileMenuRenameOpensRenameDialog(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	activateFileMenuItem(t, app, 'r')

	if !app.model.FileDialog.Open {
		t.Fatal("File dialog not open")
	}
	if app.model.FileDialog.DialogType != ui.FileDialogRename {
		t.Fatalf("dialog type = %d, want FileDialogRename", app.model.FileDialog.DialogType)
	}
	if len(app.model.FileDialog.Fields) != 1 || app.model.FileDialog.Fields[0].Value != "test.txt" {
		t.Fatalf("expected prefilled rename field, got %+v", app.model.FileDialog.Fields)
	}
}

func TestFileMenuMkdirOpensMkdirDialog(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	activateFileMenuItem(t, app, 'm')

	if !app.model.FileDialog.Open {
		t.Fatal("File dialog not open")
	}
	if app.model.FileDialog.DialogType != ui.FileDialogMkdir {
		t.Fatalf("dialog type = %d, want FileDialogMkdir", app.model.FileDialog.DialogType)
	}
}

func TestFileMenuDeleteOpensDeleteConfirmation(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	activateFileMenuItem(t, app, 'd')

	if !app.model.FileDialog.Open {
		t.Fatal("File dialog not open")
	}
	if app.model.FileDialog.DialogType != ui.FileDialogDelete {
		t.Fatalf("dialog type = %d, want FileDialogDelete", app.model.FileDialog.DialogType)
	}
	if got, want := app.model.FileDialog.DeleteSummary, "Delete file?"; got != want {
		t.Fatalf("DeleteSummary = %q, want %q", got, want)
	}
	if len(app.model.FileDialog.DeleteEntries) != 1 || app.model.FileDialog.DeleteEntries[0].Name != "test.txt" {
		t.Fatalf("DeleteEntries = %v, want [test.txt]", app.model.FileDialog.DeleteEntries)
	}
	if app.model.FileDialog.FocusedField != 1 {
		t.Fatalf("FocusedField = %d, want 1 (No)", app.model.FileDialog.FocusedField)
	}
}

func TestFileMenuExtractOpensExtractDialog(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "archive.zip"))

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	activateFileMenuItem(t, app, 'x')

	if !app.model.FileDialog.Open {
		t.Fatal("File dialog not open")
	}
	if app.model.FileDialog.DialogType != ui.FileDialogExtract {
		t.Fatalf("dialog type = %d, want FileDialogExtract", app.model.FileDialog.DialogType)
	}
	if len(app.model.FileDialog.ExtractSources) != 1 {
		t.Fatalf("ExtractSources len = %d, want 1", len(app.model.FileDialog.ExtractSources))
	}
}

func TestExtractDialogEnqueuesJob(t *testing.T) {
	if _, err := exec.LookPath("tar"); err != nil {
		t.Skip("tar not in PATH")
	}
	srcDir := t.TempDir()
	destDir := t.TempDir()
	inner := filepath.Join(srcDir, "hello.txt")
	writeFile(t, inner)
	archivePath := filepath.Join(srcDir, "pack.tar.gz")
	cmd := exec.Command("tar", "-czf", archivePath, "-C", srcDir, "hello.txt")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, srcDir)
	app.model.Right.Path = pathloc.MustParse(destDir)
	_ = app.model.Right.Refresh(20)
	if !app.activePanel().SelectVisibleEntry("pack.tar.gz") {
		t.Fatal("pack.tar.gz not visible in panel")
	}

	app.dispatch(keymap.ActionFileExtract)
	if !app.model.FileDialog.Open {
		t.Fatal("extract dialog not open")
	}
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	flushBackgroundJobs(t, app)

	all := app.jobState.AllJobs()
	if len(all) != 1 {
		t.Fatalf("jobs len = %d, want 1", len(all))
	}
	if all[0].Type != jobs.TypeExtract {
		t.Fatalf("job type = %v, want TypeExtract", all[0].Type)
	}
	if all[0].Destination.String() != destDir {
		t.Fatalf("destination = %q, want %q", all[0].Destination, destDir)
	}
	waitUntilAppJobsFinished(t, app, 5*time.Second)
	if all[0].Status != jobs.StatusCompleted {
		t.Fatalf("status = %q, want completed (%s)", all[0].Status, all[0].Error)
	}
	if _, err := os.Stat(filepath.Join(destDir, "hello.txt")); err != nil {
		t.Fatalf("extracted file missing: %v", err)
	}
}

func TestFileMenuChmodOpensChmodDialog(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	activateFileMenuItem(t, app, 'h')

	if !app.model.FileDialog.Open {
		t.Fatal("File dialog not open")
	}
	if app.model.FileDialog.DialogType != ui.FileDialogChmod {
		t.Fatalf("dialog type = %d, want FileDialogChmod", app.model.FileDialog.DialogType)
	}
}

func TestFileMenuChownOpensChownDialog(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	activateFileMenuItem(t, app, 'o')

	if !app.model.FileDialog.Open {
		t.Fatal("File dialog not open")
	}
	if app.model.FileDialog.DialogType != ui.FileDialogChown {
		t.Fatalf("dialog type = %d, want FileDialogChown", app.model.FileDialog.DialogType)
	}
	if len(app.model.FileDialog.Fields) != 2 {
		t.Fatalf("chown has %d fields, want 2", len(app.model.FileDialog.Fields))
	}
}

func TestFileMenuSymlinkOpensSymlinkDialog(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	activateFileMenuItem(t, app, 's')

	if !app.model.FileDialog.Open {
		t.Fatal("File dialog not open")
	}
	if app.model.FileDialog.DialogType != ui.FileDialogSymlink {
		t.Fatalf("dialog type = %d, want FileDialogSymlink", app.model.FileDialog.DialogType)
	}
}

func TestPathPickerHostFooterShowsPathsOnCopyAndSymlinkDialogs(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.txt"))
	dst := filepath.Join(root, "dst")
	if err := os.Mkdir(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	marksPath := filepath.Join(root, "marks")
	line := fmt.Sprintf("m : %s\n", dst)
	if err := os.WriteFile(marksPath, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Bookmarks.File = marksPath
	screen := newScreen(t, 80, 24)
	app, err := NewWithOptions(screen, Options{
		CWD:    func() (string, error) { return root, nil },
		Config: cfg,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	p := app.activePanel()
	for i := 0; i < p.VisibleEntryCount(); i++ {
		entry, _, ok := p.VisibleEntry(i)
		if ok && entry.Name == "a.txt" {
			p.Cursor = i
			break
		}
	}

	app.openCopyDialog()
	if !app.model.TransferDialog.Open {
		t.Fatal("copy dialog should open")
	}
	if app.model.TransferDialog.FocusField != 0 {
		t.Fatalf("FocusField = %d, want 0 (destination)", app.model.TransferDialog.FocusField)
	}
	keys := app.activeFooterKeys()
	if len(keys) != 4 {
		t.Fatalf("footer len = %d, want Esc + Default + Paths + F10", len(keys))
	}
	if keys[1].Hint != "Default" || keys[1].KeyLabel != "C-r" {
		t.Fatalf("restore footer = %+v, want C-r Default", keys[1])
	}
	if keys[2].Hint != "Bookmarks" || keys[2].KeyLabel != "C-g" {
		t.Fatalf("bookmarks footer = %+v, want C-g Bookmarks", keys[2])
	}

	app.handleTransferDialogKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	app.dispatch(keymap.ActionFileSymlink)
	if !app.model.FileDialog.Open || app.model.FileDialog.DialogType != ui.FileDialogSymlink {
		t.Fatal("symlink dialog should be open")
	}
	keys = app.activeFooterKeys()
	if len(keys) != 3 {
		t.Fatalf("symlink footer len = %d, want Esc + Paths + F10", len(keys))
	}
	if keys[1].Hint != "Bookmarks" {
		t.Fatalf("symlink footer middle = %+v, want Bookmarks hint", keys[1])
	}
}

func TestPathPickerHostBookmarkOpenOpensPickerFromCopyAndSymlinkDialogs(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.txt"))
	dst := filepath.Join(root, "dst")
	if err := os.Mkdir(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	marksPath := filepath.Join(root, "marks")
	line := fmt.Sprintf("m : %s\n", dst)
	if err := os.WriteFile(marksPath, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Bookmarks.File = marksPath
	screen := newScreen(t, 80, 24)
	app, err := NewWithOptions(screen, Options{
		CWD:    func() (string, error) { return root, nil },
		Config: cfg,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	p := app.activePanel()
	for i := 0; i < p.VisibleEntryCount(); i++ {
		entry, _, ok := p.VisibleEntry(i)
		if ok && entry.Name == "a.txt" {
			p.Cursor = i
			break
		}
	}

	app.openCopyDialog()
	if app.model.TransferDialog.FocusField != 0 {
		t.Fatalf("FocusField = %d, want 0", app.model.TransferDialog.FocusField)
	}
	app.handleKey(tcell.NewEventKey(tcell.KeyCtrlG, 0, tcell.ModNone))
	if !app.model.PathPicker.Open || app.model.PathPicker.Purpose != ui.PathPickerPurposeApplyTransferDestination {
		t.Fatalf("path picker = open %v purpose %v, want ApplyTransferDestination",
			app.model.PathPicker.Open, app.model.PathPicker.Purpose)
	}
	app.handlePathPickerKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if app.model.PathPicker.Open {
		t.Fatal("path picker should close")
	}
	app.handleTransferDialogKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))

	app.dispatch(keymap.ActionFileSymlink)
	if !app.model.FileDialog.Open || app.model.FileDialog.DialogType != ui.FileDialogSymlink {
		t.Fatal("symlink dialog should be open")
	}
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyCtrlG, 0, tcell.ModNone))
	if !app.model.PathPicker.Open || app.model.PathPicker.Purpose != ui.PathPickerPurposeApplyFileDialogField {
		t.Fatalf("path picker = open %v purpose %v, want ApplyFileDialogField",
			app.model.PathPicker.Open, app.model.PathPicker.Purpose)
	}
}

func TestFileMenuHardlinkOpensHardlinkDialog(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	activateFileMenuItem(t, app, 'l')

	if !app.model.FileDialog.Open {
		t.Fatal("File dialog not open")
	}
	if app.model.FileDialog.DialogType != ui.FileDialogHardlink {
		t.Fatalf("dialog type = %d, want FileDialogHardlink", app.model.FileDialog.DialogType)
	}
}

func TestFileDialogEscCancelsAndNoMessage(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	activateFileMenuItem(t, app, 'r')
	if !app.model.FileDialog.Open {
		t.Fatal("dialog should be open")
	}

	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if app.model.FileDialog.Open {
		t.Fatal("dialog should be closed after Esc")
	}
}

func TestFileDialogEnterExecutesRename(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.txt")
	writeFile(t, oldPath)

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	// Dispatch rename via keybinding (Shift+F6).
	action := keymap.ActionFileRename
	app.dispatch(action)

	if !app.model.FileDialog.Open {
		t.Fatal("dialog should be open")
	}

	// Clear prefill by typing a character, enter new name.
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone))
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'e', tcell.ModNone))
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'w', tcell.ModNone))

	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if app.model.FileDialog.Open {
		t.Fatal("dialog should be closed after Enter")
	}
	if _, err := os.Stat(filepath.Join(dir, "new")); err != nil {
		t.Fatalf("renamed file not found: %v", err)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatal("old file should not exist")
	}
	if app.model.Message == "" {
		t.Fatal("expected success message after rename")
	}
}

func TestRenameDialogFocusCheckboxDefaultOff(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "old.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)
	app.dispatch(keymap.ActionFileRename)

	if !app.model.FileDialog.Open {
		t.Fatal("rename dialog should be open")
	}
	if app.model.FileDialog.RenameFocusAfter {
		t.Fatal("RenameFocusAfter = true, want false (default)")
	}
	if got, want := ui.FileDialogFocusForm(app.model.FileDialog).TotalFocus(), 4; got != want {
		t.Fatalf("focus count = %d, want %d (field + checkbox + OK + Cancel)", got, want)
	}
}

func TestRenameDialogConfigFocusAfterDefaultOn(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "old.txt"))

	screen := newScreen(t, 80, 20)
	cfg := config.Default()
	cfg.Operations.RenameFocusAfter = true
	app, err := NewWithOptions(screen, Options{
		CWD:    func() (string, error) { return dir, nil },
		Config: cfg,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	t.Cleanup(app.stopWorker)
	app.config.UI.PanelSyncFollowNavDebounceMS = 0

	app.dispatch(keymap.ActionFileRename)
	if !app.model.FileDialog.RenameFocusAfter {
		t.Fatal("RenameFocusAfter = false, want true from config")
	}
}

func TestRenameWithoutFocusAfterDoesNotCenterOnNewFile(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 20; i++ {
		writeFile(t, filepath.Join(dir, fmt.Sprintf("%02d.txt", i)))
	}
	target := filepath.Join(dir, "10.txt")

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	for i := 0; i < p.VisibleEntryCount(); i++ {
		entry, _, ok := p.VisibleEntry(i)
		if ok && entry.Path == target {
			p.Cursor = i
			break
		}
	}

	newName := "99.txt"
	app.dispatch(keymap.ActionFileRename)
	for _, r := range newName {
		app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	p = app.activePanel()
	entry, ok := p.CurrentEntry()
	if !ok {
		t.Fatal("CurrentEntry() ok = false after rename")
	}
	if entry.Name == newName {
		t.Fatalf("cursor entry = %q, want index fallback not focus-after selection", entry.Name)
	}
	if entry.Name != "11.txt" {
		t.Fatalf("cursor entry = %q, want 11.txt at prior index", entry.Name)
	}
}

func TestRenameWithFocusAfterSelectsAndCentersNewFile(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 20; i++ {
		writeFile(t, filepath.Join(dir, fmt.Sprintf("%02d.txt", i)))
	}
	target := filepath.Join(dir, "10.txt")

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	for i := 0; i < p.VisibleEntryCount(); i++ {
		entry, _, ok := p.VisibleEntry(i)
		if ok && entry.Path == target {
			p.Cursor = i
			break
		}
	}

	newName := "99.txt"
	app.dispatch(keymap.ActionFileRename)
	for _, r := range newName {
		app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'f', tcell.ModAlt))
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	p = app.activePanel()
	entry, ok := p.CurrentEntry()
	if !ok {
		t.Fatal("CurrentEntry() ok = false after rename")
	}
	if entry.Name != newName {
		t.Fatalf("cursor entry = %q, want %s", entry.Name, newName)
	}
	vp := app.activeViewportRows()
	wantScroll := p.Cursor - vp/2
	if wantScroll < 0 {
		wantScroll = 0
	}
	maxOffset := p.VisibleEntryCount() - vp
	if maxOffset < 0 {
		maxOffset = 0
	}
	if wantScroll > maxOffset {
		wantScroll = maxOffset
	}
	if p.ScrollOffset != wantScroll {
		t.Fatalf("ScrollOffset = %d, want %d (centered on renamed entry)", p.ScrollOffset, wantScroll)
	}
}

func TestMassRenameTwoSelectedFiles(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "foo_a.txt")
	bPath := filepath.Join(dir, "foo_b.txt")
	writeFile(t, aPath)
	writeFile(t, bPath)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{aPath: true, bPath: true}

	app.dispatch(keymap.ActionFileRename)
	if !app.model.FileDialog.Open || app.model.FileDialog.DialogType != ui.FileDialogMassRename {
		t.Fatalf("expected mass rename dialog, open=%v type=%v", app.model.FileDialog.Open, app.model.FileDialog.DialogType)
	}
	d := &app.model.FileDialog
	if d.FocusedField != massRenameFindFieldFocus {
		t.Fatalf("FocusedField = %d, want %d (Find)", d.FocusedField, massRenameFindFieldFocus)
	}
	for _, r := range "foo_" {
		app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	d.FocusedField = 3
	for _, r := range "bar_" {
		app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	d.FocusedField = ui.FileDialogOKFocusIndex(*d)
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if app.model.FileDialog.Open {
		t.Fatal("dialog should be closed after Enter")
	}
	if _, err := os.Stat(filepath.Join(dir, "bar_a.txt")); err != nil {
		t.Fatalf("bar_a.txt: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "bar_b.txt")); err != nil {
		t.Fatalf("bar_b.txt: %v", err)
	}
	if _, err := os.Stat(aPath); !os.IsNotExist(err) {
		t.Fatal("old path foo_a.txt should not exist")
	}
}

func TestMassRenameNoMatchMarksFindInvalid(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "foo_a.txt")
	writeFile(t, aPath)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{aPath: true}

	app.dispatch(keymap.ActionFileRename)
	d := &app.model.FileDialog
	for _, r := range "nomatch" {
		app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	if !d.Fields[0].InputInvalid {
		t.Fatal("expected Find input invalid when nothing matches")
	}
}

func TestMassRenameModeShortcutKeepsFindFocus(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "x.txt")
	writeFile(t, aPath)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{aPath: true}

	app.dispatch(keymap.ActionFileRename)
	d := &app.model.FileDialog
	if !d.Open || d.DialogType != ui.FileDialogMassRename {
		t.Fatalf("expected mass rename dialog")
	}
	if d.FocusedField != massRenameFindFieldFocus {
		t.Fatalf("FocusedField = %d, want %d (Find)", d.FocusedField, massRenameFindFieldFocus)
	}

	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModAlt))
	if d.MassRenameMode != ui.MassRenameModeUIRegex {
		t.Fatalf("mode = %v, want regex", d.MassRenameMode)
	}
	if d.FocusedField != massRenameFindFieldFocus {
		t.Fatalf("after Alt+R: focus = %d, want %d", d.FocusedField, massRenameFindFieldFocus)
	}
	if d.Fields[0].Label != "Pattern" {
		t.Fatalf("label = %q, want Pattern", d.Fields[0].Label)
	}

	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModAlt))
	if d.MassRenameMode != ui.MassRenameModeUISimple {
		t.Fatalf("mode = %v, want simple", d.MassRenameMode)
	}
	if d.FocusedField != massRenameFindFieldFocus {
		t.Fatalf("after Alt+S: focus = %d, want %d", d.FocusedField, massRenameFindFieldFocus)
	}
	if d.Fields[0].Label != "Find" {
		t.Fatalf("label = %q, want Find", d.Fields[0].Label)
	}
}

func TestMassRenameModeShortcutKeepsReplaceFocus(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "x.txt")
	writeFile(t, aPath)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{aPath: true}

	app.dispatch(keymap.ActionFileRename)
	d := &app.model.FileDialog
	const replaceFocus = 3
	d.FocusedField = replaceFocus
	for _, r := range "y" {
		app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	if d.FocusedField != replaceFocus {
		t.Fatalf("setup: focus = %d, want %d (Replace)", d.FocusedField, replaceFocus)
	}

	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModAlt))
	if d.MassRenameMode != ui.MassRenameModeUIRegex {
		t.Fatalf("mode = %v, want regex", d.MassRenameMode)
	}
	if d.FocusedField != replaceFocus {
		t.Fatalf("after Alt+R: focus = %d, want %d (Replace)", d.FocusedField, replaceFocus)
	}

	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModAlt))
	if d.MassRenameMode != ui.MassRenameModeUISimple {
		t.Fatalf("mode = %v, want simple", d.MassRenameMode)
	}
	if d.FocusedField != replaceFocus {
		t.Fatalf("after Alt+S: focus = %d, want %d (Replace)", d.FocusedField, replaceFocus)
	}
}

func TestMassRenameRegexpCompileHintForBackslashPattern(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "x.txt")
	writeFile(t, aPath)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{aPath: true}

	app.dispatch(keymap.ActionFileRename)
	d := &app.model.FileDialog
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModAlt))
	d.FocusedField = 2
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, '\\', tcell.ModNone))

	if !d.Fields[0].InputInvalid {
		t.Fatal("expected invalid pattern field")
	}
	hint := strings.TrimSpace(d.MassRenamePatternCompileHint)
	if hint == "" {
		t.Fatal("expected regexp compile hint under pattern field")
	}
	if !strings.Contains(hint, "backslash") {
		t.Fatalf("hint = %q, want backslash detail", hint)
	}
}

func TestMassRenameEnterCancelClosesWithInvalidRegex(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "x.txt")
	writeFile(t, aPath)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{aPath: true}

	app.dispatch(keymap.ActionFileRename)
	d := &app.model.FileDialog
	if !d.Open || d.DialogType != ui.FileDialogMassRename {
		t.Fatalf("expected mass rename dialog")
	}
	// Regex mode + invalid pattern
	d.FocusedField = 1
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	d.FocusedField = 2
	for _, r := range "a++" {
		app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	if !d.Fields[0].InputInvalid {
		t.Fatal("expected invalid pattern")
	}
	d.FocusedField = ui.FileDialogCancelFocusIndex(*d)
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if app.model.FileDialog.Open {
		t.Fatal("Enter on Cancel should close dialog even when regexp is invalid")
	}
}

func TestMassRenameConflictBlocksOKWithCriticalToast(t *testing.T) {
	dir := t.TempDir()
	names := []string{"Season1", "Season2", "Season3", "Season4"}
	paths := make([]string, len(names))
	for i, name := range names {
		p := filepath.Join(dir, name)
		paths[i] = p
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = make(map[string]bool, len(paths))
	for _, path := range paths {
		p.SelectedPaths[path] = true
	}

	app.dispatch(keymap.ActionFileRename)
	d := &app.model.FileDialog
	if !d.Open || d.DialogType != ui.FileDialogMassRename {
		t.Fatalf("expected mass rename dialog")
	}
	for _, r := range "1" {
		app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	d.FocusedField = 3
	for _, r := range "2" {
		app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	if len(d.MassRenamePreviewBefore) != len(paths) {
		t.Fatalf("preview rows = %d, want %d (no banner row)", len(d.MassRenamePreviewBefore), len(paths))
	}
	for _, lb := range d.MassRenamePreviewBefore {
		if strings.HasPrefix(lb, "!") {
			t.Fatalf("unexpected banner row %q", lb)
		}
	}
	conflictIdx := -1
	for i, lb := range d.MassRenamePreviewBefore {
		if lb == "Season1" {
			conflictIdx = i
			break
		}
	}
	if conflictIdx < 0 {
		t.Fatal("Season1 preview row missing")
	}
	if len(d.MassRenamePreviewAfterError) != len(paths) || !d.MassRenamePreviewAfterError[conflictIdx] {
		t.Fatalf("after error flags = %v, want index %d set", d.MassRenamePreviewAfterError, conflictIdx)
	}
	if ui.FileDialogMassRenameOKEnabled(*d) {
		t.Fatal("OK action should be blocked when preview has conflicts")
	}
	okIdx := ui.FileDialogOKFocusIndex(*d)
	d.FocusedField = 4
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if d.FocusedField != okIdx {
		t.Fatalf("Down from checkbox: focus = %d, want OK %d", d.FocusedField, okIdx)
	}
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if !app.model.FileDialog.Open {
		t.Fatal("Enter on OK with conflicts should not close dialog")
	}
	if app.model.MessageUrgency != ui.MessageUrgencyCritical {
		t.Fatalf("MessageUrgency = %v, want MessageUrgencyCritical", app.model.MessageUrgency)
	}
	if strings.TrimSpace(app.model.Message) == "" {
		t.Fatal("expected critical toast explaining the conflict")
	}
	if !strings.Contains(app.model.Message, "Season2") {
		t.Fatalf("message = %q, want conflict detail mentioning Season2", app.model.Message)
	}
}

func TestFileDialogMkdirCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileMkdir)
	if !app.model.FileDialog.Open {
		t.Fatal("dialog should be open")
	}

	for _, r := range "newdir" {
		app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}

	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if app.model.FileDialog.Open {
		t.Fatal("dialog should be closed after Enter")
	}
	info, err := os.Stat(filepath.Join(dir, "newdir"))
	if err != nil {
		t.Fatalf("created directory not found: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("created path is not a directory")
	}
}

func TestFileDialogInsertRune(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileRename)
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone))

	field := &app.model.FileDialog.Fields[0]
	if field.Value != "n" {
		t.Fatalf("field value = %q, want n", field.Value)
	}
	if field.Cursor != 1 {
		t.Fatalf("cursor = %d, want 1", field.Cursor)
	}
}

func TestFileDialogBackspace(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileMkdir)
	for _, r := range "hello" {
		app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone))

	field := &app.model.FileDialog.Fields[0]
	if field.Value != "hell" {
		t.Fatalf("field value = %q, want hell", field.Value)
	}
}

func TestFileDialogLeftRightMoveCursor(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileMkdir)
	for _, r := range "abc" {
		app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}

	// cursor should be at 3 (end)
	field := &app.model.FileDialog.Fields[0]
	expectedCursor := 3
	if field.Cursor != expectedCursor {
		t.Fatalf("cursor = %d, want %d", field.Cursor, expectedCursor)
	}

	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	if field.Cursor != 2 {
		t.Fatalf("cursor after left = %d, want 2", field.Cursor)
	}

	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if field.Cursor != 3 {
		t.Fatalf("cursor after right = %d, want 3", field.Cursor)
	}
}

func TestFileDialogCtrlWKillWord(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileMkdir)
	for _, r := range "/foo/bar" {
		app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	field := &app.model.FileDialog.Fields[0]
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyCtrlW, 0, tcell.ModNone))
	if field.Value != "/foo/" || field.Cursor != 5 {
		t.Fatalf("after Ctrl+W: value=%q cursor=%d", field.Value, field.Cursor)
	}
}

func TestFileDialogHomeEnd(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileMkdir)
	for _, r := range "abcdef" {
		app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}

	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	// cursor should be at 3 now (moved left 3 from 6)

	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	field := &app.model.FileDialog.Fields[0]
	if field.Cursor != 6 {
		t.Fatalf("cursor after End = %d, want 6", field.Cursor)
	}

	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone))
	if field.Cursor != 0 {
		t.Fatalf("cursor after Home = %d, want 0", field.Cursor)
	}
}

func TestRenameDialogFooterListsRestoreDefaultShortcut(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "existing.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileRename)
	keys := app.activeFooterKeys()
	if len(keys) != 5 {
		t.Fatalf("footer len = %d, want Esc + Sanitize + Slugify + Default + F10", len(keys))
	}
	if keys[1].Hint != "Sanitize" || keys[1].Key != tcell.KeyF2 {
		t.Fatalf("footer sanitize = %+v, want F2 Sanitize", keys[1])
	}
	if keys[2].Hint != "Slugify" || keys[2].Key != tcell.KeyF3 {
		t.Fatalf("footer slugify = %+v, want F3 Slugify", keys[2])
	}
	if keys[3].Hint != "Default" || keys[3].KeyLabel != "C-r" {
		t.Fatalf("footer restore = %+v, want C-r Default", keys[3])
	}
	if keys[4].Key != tcell.KeyF10 {
		t.Fatalf("last footer = %+v, want F10 Quit", keys[4])
	}

	// On OK button the restore hint is hidden (no prefill field focused).
	app.model.FileDialog.FocusedField = 1
	keys = app.activeFooterKeys()
	if len(keys) != 4 {
		t.Fatalf("on OK footer len = %d, want Esc + Sanitize + Slugify + F10", len(keys))
	}
}

func TestRenameDialogSanitizeF2ApplyTransformsName(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "x.y_z"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileRename)
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyF2, 0, tcell.ModNone))
	if app.model.FileDialog.RenamePhase != ui.RenamePhaseSanitize {
		t.Fatalf("phase = %v, want Sanitize", app.model.FileDialog.RenamePhase)
	}
	if got, want := app.model.FileDialog.FocusedField, ui.FileDialogOKFocusIndex(app.model.FileDialog); got != want {
		t.Fatalf("sanitize open focus = %d, want OK (%d)", got, want)
	}
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if app.model.FileDialog.RenamePhase != ui.RenamePhaseMain {
		t.Fatalf("want back on main rename, got phase %v", app.model.FileDialog.RenamePhase)
	}
	if got := app.model.FileDialog.Fields[0].Value; got != "x y z" {
		t.Fatalf("name = %q, want %q", got, "x y z")
	}
}

func TestRenameDialogSlugifyF3ApplyTransformsName(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "my file"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileRename)
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyF3, 0, tcell.ModNone))
	if app.model.FileDialog.RenamePhase != ui.RenamePhaseSlugify {
		t.Fatalf("phase = %v, want Slugify", app.model.FileDialog.RenamePhase)
	}
	if got, want := app.model.FileDialog.FocusedField, ui.FileDialogOKFocusIndex(app.model.FileDialog); got != want {
		t.Fatalf("slugify open focus = %d, want OK (%d)", got, want)
	}
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if app.model.FileDialog.RenamePhase != ui.RenamePhaseMain {
		t.Fatalf("want back on main rename, got phase %v", app.model.FileDialog.RenamePhase)
	}
	if got := app.model.FileDialog.Fields[0].Value; got != "my.file" {
		t.Fatalf("name = %q, want %q", got, "my.file")
	}
}

func TestRenameDialogSanitizeEscReturnsWithoutApply(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.b"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileRename)
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyF2, 0, tcell.ModNone))
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if app.model.FileDialog.RenamePhase != ui.RenamePhaseMain {
		t.Fatalf("phase = %v, want Main", app.model.FileDialog.RenamePhase)
	}
	if got := app.model.FileDialog.Fields[0].Value; got != "a.b" {
		t.Fatalf("name = %q, want unchanged a.b", got)
	}
}

func TestRenameDialogPrefillClearsOnType(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "existing.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileRename)
	field := &app.model.FileDialog.Fields[0]
	if field.Value != "existing.txt" {
		t.Fatalf("prefill value = %q, want existing.txt", field.Value)
	}

	// First printable should clear prefill.
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone))
	if field.Value != "n" {
		t.Fatalf("after first printable: value = %q, want n", field.Value)
	}
	if field.PrefillPending {
		t.Fatal("PrefillPending should be false after typing")
	}
}

func TestMkdirDialogPrefillsFromCursorEntry(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "cursor-name.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileMkdir)
	f := &app.model.FileDialog.Fields[0]
	if f.Value != "cursor-name.txt" || f.Prefill != "cursor-name.txt" {
		t.Fatalf("field = %+v, want Value and Prefill cursor-name.txt", f)
	}
	if !f.PrefillPending {
		t.Fatal("PrefillPending should be true with a non-empty suggestion")
	}
}

func TestRenameDialogPrefillBackspaceCommitsBeforeDelete(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "existing.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileRename)
	field := &app.model.FileDialog.Fields[0]
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone))
	if field.PrefillPending {
		t.Fatal("PrefillPending should be false after backspace")
	}
	if field.Value != "existing.tx" {
		t.Fatalf("value = %q, want existing.tx", field.Value)
	}
}

func TestRenameDialogCtrlRRestoresClearedPrefill(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "existing.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileRename)
	field := &app.model.FileDialog.Fields[0]

	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyCtrlL, 0, tcell.ModNone))
	if field.Value != "" || field.PrefillPending {
		t.Fatalf("after Ctrl+L: value=%q pending=%v, want empty/false", field.Value, field.PrefillPending)
	}

	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyCtrlR, 0, tcell.ModNone))
	if field.Value != "existing.txt" {
		t.Fatalf("after Ctrl+R: value = %q, want existing.txt", field.Value)
	}
	if field.Cursor != len([]rune("existing.txt")) {
		t.Fatalf("after Ctrl+R: cursor = %d, want %d", field.Cursor, len([]rune("existing.txt")))
	}
	if !field.PrefillPending {
		t.Fatal("after Ctrl+R: PrefillPending should be true")
	}
}

func TestRenameDialogCtrlDRestoresEditedPrefill(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "existing.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileRename)
	field := &app.model.FileDialog.Fields[0]

	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone))
	if field.Value == "existing.txt" {
		t.Fatalf("expected prefill to be replaced after typing 'x'; value=%q", field.Value)
	}

	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyCtrlD, 0, tcell.ModNone))
	if field.Value != "existing.txt" {
		t.Fatalf("after Ctrl+D: value = %q, want existing.txt", field.Value)
	}
	if !field.PrefillPending {
		t.Fatal("after Ctrl+D: PrefillPending should be true")
	}
}

func TestFileDialogExecutesDelete(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	writeFile(t, filePath)

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileDelete)
	if !app.model.FileDialog.Open {
		t.Fatal("delete dialog should be open")
	}

	// Default focus is No; move to Yes then Enter confirms delete.
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if app.model.FileDialog.Open {
		t.Fatal("dialog should be closed after confirm")
	}
	flushBackgroundJobs(t, app)
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatal("file should be deleted")
	}
	if app.model.Message == "" {
		t.Fatal("expected success message after delete")
	}
}

func TestKeybindingDispatcherOpensFileDialogs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	// Test F7 for mkdir.
	app.handleKey(tcell.NewEventKey(tcell.KeyF7, 0, tcell.ModNone))
	if !app.model.FileDialog.Open || app.model.FileDialog.DialogType != ui.FileDialogMkdir {
		t.Fatal("F7 should open mkdir dialog")
	}
	app.closeFileDialog()

	// Test F8 for delete (default global binding).
	app.handleKey(tcell.NewEventKey(tcell.KeyF8, 0, tcell.ModNone))
	if !app.model.FileDialog.Open || app.model.FileDialog.DialogType != ui.FileDialogDelete {
		t.Fatal("F8 should open delete dialog")
	}
	app.closeFileDialog()
}

func TestEmptyPanelShowsErrorForFileOperations(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	// Empty dir - just . and possibly no files.
	// Make it truly empty by using panel with no entries (the only entry might be parent reference).
	// Actually panel doesn't show ".." in v1, so empty means empty.

	app.dispatch(keymap.ActionFileRename)
	if app.model.FileDialog.Open {
		t.Fatal("dialog should not open on empty panel")
	}
	if app.model.Message == "" {
		t.Fatal("expected error message for operation on empty panel")
	}
}

func TestFileDialogEnterOnDeleteWithNoItems(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileDelete)
	// Esc should work.
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if app.model.FileDialog.Open {
		t.Fatal("Esc should close delete dialog")
	}
}

func TestDeleteDialogEnterDefaultCancelsWithoutDelete(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "keep.txt")
	writeFile(t, filePath)

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileDelete)
	if app.model.FileDialog.FocusedField != 1 {
		t.Fatalf("FocusedField = %d, want 1 (No)", app.model.FileDialog.FocusedField)
	}
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if app.model.FileDialog.Open {
		t.Fatal("Enter on No should close dialog")
	}
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("file should still exist: %v", err)
	}
}

func TestDeleteDialogWarningPluralDirectories(t *testing.T) {
	dir := t.TempDir()
	d1 := filepath.Join(dir, "a")
	d2 := filepath.Join(dir, "b")
	if err := os.Mkdir(d1, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(d2, 0o755); err != nil {
		t.Fatal(err)
	}

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{d1: true, d2: true}
	if err := p.Refresh(app.activeViewportRows()); err != nil {
		t.Fatal(err)
	}

	app.dispatch(keymap.ActionFileDelete)
	if got, want := app.model.FileDialog.DeleteSummary, "Delete 2 selections?"; got != want {
		t.Fatalf("DeleteSummary = %q, want %q", got, want)
	}
	if got, want := app.model.FileDialog.DeleteWarning, "Warning: 2 directories will be removed recursively!"; got != want {
		t.Fatalf("DeleteWarning = %q, want %q", got, want)
	}
	if len(app.model.FileDialog.DeleteEntries) != 2 || app.model.FileDialog.DeleteEntries[0].Name != "a" || app.model.FileDialog.DeleteEntries[1].Name != "b" {
		t.Fatalf("DeleteEntries = %v, want [a b]", app.model.FileDialog.DeleteEntries)
	}
}

func TestDeleteDialogShowsContextPathWhenSelectionOffPanel(t *testing.T) {
	dir := t.TempDir()
	series := filepath.Join(dir, "Some Series")
	season := filepath.Join(series, "Season 01")
	other := filepath.Join(dir, "Other")
	if err := os.MkdirAll(season, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(other, 0o755); err != nil {
		t.Fatal(err)
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	if err := p.Load(other); err != nil {
		t.Fatal(err)
	}
	p.SelectedPaths = map[string]bool{season: true}
	p.SelectionsStripOrder = []string{season}

	app.dispatch(keymap.ActionFileDelete)
	if !app.model.FileDialog.Open {
		t.Fatal("delete dialog should be open")
	}
	want := filepath.Join("Some Series", "Season 01")
	if len(app.model.FileDialog.DeleteEntries) != 1 || app.model.FileDialog.DeleteEntries[0].Name != want {
		t.Fatalf("DeleteEntries = %#v, want Name %q", app.model.FileDialog.DeleteEntries, want)
	}
}

func TestDeleteDialogListScrollsWithPageDown(t *testing.T) {
	dir := t.TempDir()
	for i := range 25 {
		writeFile(t, filepath.Join(dir, fmt.Sprintf("selection-%02d.txt", i)))
	}

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = make(map[string]bool)
	for i := range 25 {
		p.SelectedPaths[filepath.Join(dir, fmt.Sprintf("selection-%02d.txt", i))] = true
	}
	if err := p.Refresh(app.activeViewportRows()); err != nil {
		t.Fatal(err)
	}

	app.dispatch(keymap.ActionFileDelete)
	if app.model.FileDialog.DeleteListScroll != 0 {
		t.Fatalf("DeleteListScroll = %d, want 0 on open", app.model.FileDialog.DeleteListScroll)
	}
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone))
	if app.model.FileDialog.DeleteListScroll == 0 {
		t.Fatal("PgDn should advance DeleteListScroll when list is clipped")
	}
}

func TestAddBookmarkDialogOpenPrefillsBasename(t *testing.T) {
	dir := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionBookmarkAdd)

	if !app.model.FileDialog.Open {
		t.Fatal("FileDialog should be open after ActionBookmarkAdd")
	}
	if app.model.FileDialog.DialogType != ui.FileDialogAddBookmark {
		t.Fatalf("dialog type = %d, want FileDialogAddBookmark", app.model.FileDialog.DialogType)
	}
	if got, want := len(app.model.FileDialog.Fields), 1; got != want {
		t.Fatalf("Fields length = %d, want %d", got, want)
	}
	wantName := filepath.Base(dir)
	if app.model.FileDialog.Fields[0].Value != wantName {
		t.Fatalf("Fields[0].Value = %q, want %q", app.model.FileDialog.Fields[0].Value, wantName)
	}
	if !app.model.FileDialog.Fields[0].PrefillPending {
		t.Fatal("Fields[0].PrefillPending should be true on open")
	}
	if app.model.FileDialog.Message != dir {
		t.Fatalf("Message = %q, want %q", app.model.FileDialog.Message, dir)
	}
}

func TestAddBookmarkExecuteAppendsToMarksFile(t *testing.T) {
	dir := t.TempDir()
	marksPath := filepath.Join(t.TempDir(), ".fzf-marks")

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.config.Bookmarks.File = marksPath

	app.dispatch(keymap.ActionBookmarkAdd)
	if !app.model.FileDialog.Open {
		t.Fatal("dialog should be open")
	}

	// Replace prefilled name by typing a fresh value (first printable clears prefill).
	for _, r := range "myproject" {
		app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if app.model.FileDialog.Open {
		t.Fatal("dialog should be closed after Enter")
	}
	data, err := os.ReadFile(marksPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", marksPath, err)
	}
	wantLine := "myproject : " + dir + "\n"
	if string(data) != wantLine {
		t.Fatalf("marks file contents = %q, want %q", string(data), wantLine)
	}
	if !strings.Contains(app.model.Message, "Bookmark added") {
		t.Fatalf("transient message = %q, want it to mention bookmark added", app.model.Message)
	}
	var logText strings.Builder
	for _, e := range app.model.MessageLog {
		logText.WriteString(e.Text)
	}
	if !strings.Contains(logText.String(), marksPath) {
		t.Fatalf("message log should include marks file path %q; log=%#v", marksPath, app.model.MessageLog)
	}
	if app.model.MessageUrgency != ui.MessageUrgencyInfo {
		t.Fatalf("message urgency = %v, want info", app.model.MessageUrgency)
	}
}

func TestAddBookmarkConfirmFromOKButtonWritesFile(t *testing.T) {
	dir := t.TempDir()
	marksPath := filepath.Join(t.TempDir(), ".fzf-marks")

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.config.Bookmarks.File = marksPath

	app.dispatch(keymap.ActionBookmarkAdd)
	for _, r := range "okbtn" {
		app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	// Move focus from name field to OK, then confirm (Enter must still append).
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if app.model.FileDialog.FocusedField != 1 {
		t.Fatalf("FocusedField = %d, want 1 (OK)", app.model.FileDialog.FocusedField)
	}
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if app.model.FileDialog.Open {
		t.Fatal("dialog should be closed after Enter on OK")
	}
	data, err := os.ReadFile(marksPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", marksPath, err)
	}
	wantLine := "okbtn : " + dir + "\n"
	if string(data) != wantLine {
		t.Fatalf("marks file contents = %q, want %q", string(data), wantLine)
	}
}

func TestAddBookmarkEmptyNameClosesWithError(t *testing.T) {
	dir := t.TempDir()
	marksPath := filepath.Join(t.TempDir(), ".fzf-marks")

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, dir)
	app.config.Bookmarks.File = marksPath

	app.dispatch(keymap.ActionBookmarkAdd)
	// Wipe the prefilled value so name is empty.
	app.model.FileDialog.Fields[0].Value = ""
	app.model.FileDialog.Fields[0].Cursor = 0
	app.model.FileDialog.Fields[0].PrefillPending = false

	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if app.model.FileDialog.Open {
		t.Fatal("dialog should be closed after rejected confirm")
	}
	if app.model.MessageUrgency != ui.MessageUrgencyError {
		t.Fatalf("message urgency = %v, want error", app.model.MessageUrgency)
	}
	if _, err := os.Stat(marksPath); !os.IsNotExist(err) {
		t.Fatalf("marks file should not exist; stat err = %v", err)
	}
}

func TestAddBookmarkDefaultName(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{path: "/", want: "root"},
		{path: "/home/user/projects", want: "projects"},
		{path: ".", want: "root"},
		{path: "", want: "root"},
	}
	for _, tt := range tests {
		if got := defaultBookmarkName(tt.path); got != tt.want {
			t.Fatalf("defaultBookmarkName(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestQuickViewDisablesSyncWithWarn(t *testing.T) {
	root := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	app.model.ActivePanel = ui.LeftPanel
	app.model.SyncFollowEnabled = true
	app.model.SyncFollowPanel = ui.LeftPanel

	app.dispatch(keymap.ActionFileQuickView)

	if !app.model.QuickViewEnabled {
		t.Fatal("quick view should be enabled")
	}
	if app.model.SyncFollowEnabled {
		t.Fatal("sync should be disabled when quick view is enabled")
	}
	if app.model.MessageUrgency != ui.MessageUrgencyWarn {
		t.Fatalf("message urgency = %v, want warn", app.model.MessageUrgency)
	}
	if !strings.Contains(app.model.Message, "sync disabled") {
		t.Fatalf("message = %q, want sync disabled notice", app.model.Message)
	}
}

func TestSyncDisablesQuickViewWithWarn(t *testing.T) {
	root := t.TempDir()
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	app.model.ActivePanel = ui.LeftPanel
	app.model.QuickViewEnabled = true

	app.dispatch(keymap.ActionPanelToggleSync)

	if !app.model.SyncFollowEnabled {
		t.Fatal("sync should be enabled")
	}
	if app.model.QuickViewEnabled {
		t.Fatal("quick view should be disabled when sync is enabled")
	}
	if app.filePreviewOpen() {
		t.Fatal("file preview should be closed when sync displaces quick view")
	}
	if app.model.MessageUrgency != ui.MessageUrgencyWarn {
		t.Fatalf("message urgency = %v, want warn", app.model.MessageUrgency)
	}
	if !strings.Contains(app.model.Message, "quick view disabled") {
		t.Fatalf("message = %q, want quick view disabled notice", app.model.Message)
	}
}

func TestToggleSyncEnablesAndImmediatelyMirrorsHighlightedFolder(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	if err := os.Mkdir(alpha, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	app.model.ActivePanel = ui.LeftPanel
	selectPanelEntryByName(t, app.panelByID(ui.LeftPanel), "alpha")

	app.dispatch(keymap.ActionPanelToggleSync)

	if !app.model.SyncFollowEnabled || app.model.SyncFollowPanel != ui.LeftPanel {
		t.Fatalf("Sync state after enable = (enabled=%v panel=%d), want (true, LeftPanel)", app.model.SyncFollowEnabled, app.model.SyncFollowPanel)
	}
	if got, want := filepath.Clean(app.panelByID(ui.RightPanel).Path.String()), filepath.Clean(alpha); got != want {
		t.Fatalf("right panel path after enable = %q, want %q", got, want)
	}
	if !strings.Contains(app.model.Message, "Sync") {
		t.Fatalf("transient message = %q, want Sync notice", app.model.Message)
	}
}

// Regression: Parent must center the exited directory in the file list on first paint (same as rename recall).
func TestParentNavigationCentersInAppViewport(t *testing.T) {
	root := t.TempDir()
	bar := filepath.Join(root, "bar")
	asdf := filepath.Join(bar, "asdf")
	if err := os.MkdirAll(asdf, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("%02d_dir", i)
		if err := os.Mkdir(filepath.Join(bar, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, bar)
	app.model.ActivePanel = ui.LeftPanel
	selectPanelEntryByName(t, app.panelByID(ui.LeftPanel), "asdf")
	if _, err := app.activePanel().Enter(app.activeViewportRows()); err != nil {
		t.Fatal(err)
	}
	app.dispatch(keymap.ActionNavParent)
	app.render()
	p := app.activePanel()
	vr := app.activeViewportRows()
	if vr < 1 {
		t.Fatalf("viewportRows = %d", vr)
	}
	entry, ok := p.CurrentEntry()
	if !ok || entry.Name != "asdf" {
		t.Fatalf("highlight = %q ok=%v, want asdf", entry.Name, ok)
	}
	row := p.Cursor - p.ScrollOffset
	mid := vr / 2
	if row != mid && row != vr-1 {
		t.Fatalf("after Parent: viewport row = %d, want %d (centered) or %d (tail); cursor=%d scroll=%d vr=%d",
			row, mid, vr-1, p.Cursor, p.ScrollOffset, vr)
	}
	app.reconcileAfterEvent()
	row = p.Cursor - p.ScrollOffset
	if row != mid && row != vr-1 {
		t.Fatalf("after second reconcile: viewport row = %d, want %d or %d; cursor=%d scroll=%d",
			row, mid, vr-1, p.Cursor, p.ScrollOffset)
	}
}

// Regression: Parent must re-resolve viewport rows after chdir when the selections strip layout changes.
func TestParentCentersWhenSelectionsStripShrinksAfterChdir(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("%02d_dir", i)
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	other := filepath.Join(root, "walnut.txt")
	writeFile(t, other)

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	left := app.panelByID(ui.LeftPanel)
	if left.SelectedPaths == nil {
		left.SelectedPaths = make(map[string]bool)
	}
	for i := 0; i < 4; i++ {
		p := filepath.Join(root, fmt.Sprintf("peer%d.txt", i))
		writeFile(t, p)
		left.SelectedPaths[p] = true
	}
	left.SelectedPaths[other] = true

	if ui.SelectionsStripLayoutItemCount(left, ui.LeftPanel, ui.LeftPanel, false) != 0 {
		t.Fatal("strip should be hidden while cross-dir selections are in the current directory")
	}
	selectPanelEntryByName(t, left, "sub")
	if _, err := left.Enter(app.activeViewportRows()); err != nil {
		t.Fatal(err)
	}
	if ui.SelectionsStripLayoutItemCount(left, ui.LeftPanel, ui.LeftPanel, false) == 0 {
		t.Fatal("strip should be visible after entering sub with cross-dir selection")
	}

	if left.FileListViewportRows == nil {
		t.Fatal("FileListViewportRows callback not wired")
	}
	staleVR := app.panelViewportRows(ui.LeftPanel) // still in sub: includes selections strip
	origViewport := left.FileListViewportRows
	var scrollPath string
	var scrollVR int
	left.FileListViewportRows = func() int {
		scrollPath = left.PathString()
		scrollVR = origViewport()
		return scrollVR
	}
	if err := left.Parent(staleVR); err != nil {
		t.Fatal(err)
	}
	vr := app.activeViewportRows()
	if staleVR >= vr {
		t.Fatalf("staleVR = %d, want smaller than post-parent %d (strip must shrink file list)", staleVR, vr)
	}
	if ui.SelectionsStripLayoutItemCount(left, ui.LeftPanel, ui.LeftPanel, false) != 0 {
		t.Fatal("strip should be hidden in parent after chdir")
	}
	if vr != ui.FileListViewportRows(
		app.layoutForTerminalSize(80, 24).Left,
		left,
		ui.LeftPanel,
		ui.LeftPanel,
		false,
		app.model.SelectionsPanelMaxRows,
	) {
		t.Fatalf("viewportRows = %d, want post-parent file list rows %d", vr,
			ui.FileListViewportRows(app.layoutForTerminalSize(80, 24).Left, left, ui.LeftPanel, ui.LeftPanel, false, app.model.SelectionsPanelMaxRows))
	}
	entry, ok := left.CurrentEntry()
	if !ok || entry.Name != "sub" {
		t.Fatalf("highlight = %q ok=%v, want sub", entry.Name, ok)
	}
	if scrollPath != left.PathString() {
		t.Fatalf("scrollPath = %q, want parent %q", scrollPath, left.PathString())
	}
	if scrollVR != vr {
		t.Fatalf("scrollVR = %d, want live post-parent %d", scrollVR, vr)
	}
	// Regression: first list navigation must not re-scroll using the pre-Parent strip viewport.
	beforeScroll := left.ScrollOffset
	left.EnsureCursorInViewport(staleVR)
	if left.ScrollOffset != beforeScroll {
		t.Fatalf("stale viewport %d changed ScrollOffset %d -> %d; cursor=%d vr=%d",
			staleVR, beforeScroll, left.ScrollOffset, left.Cursor, vr)
	}
}

// Regression: disk-total resort after Parent must not undo centered scroll from ApplyListing.
func TestParentStaysCenteredAfterDiskUsageResort(t *testing.T) {
	root := t.TempDir()
	bar := filepath.Join(root, "bar")
	asdf := filepath.Join(bar, "asdf")
	if err := os.MkdirAll(asdf, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("%02d_dir", i)
		if err := os.Mkdir(filepath.Join(bar, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(bar, "00_dir", "leaf.dat"))

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, bar)
	left := app.panelByID(ui.LeftPanel)
	left.Sort.DiskUsageIdleSizeSort = true
	left.DiskSorter = app.diskUsage.Size
	app.setDiskUsageScanScope(bar, []string{bar})
	app.startDiskUsageScanForPanel(ui.LeftPanel)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		app.pollDiskUsageUpdates()
		if !app.diskUsageScanBusy() && left.ListingFullyDiskCached() {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	if !left.ListingFullyDiskCached() {
		t.Fatal("listing not fully disk-cached")
	}

	app.model.ActivePanel = ui.LeftPanel
	selectPanelEntryByName(t, left, "asdf")
	if _, err := left.Enter(app.activeViewportRows()); err != nil {
		t.Fatal(err)
	}
	app.dispatch(keymap.ActionNavParent)
	app.resortPanelsDiskUsageSorted()

	vr := app.activeViewportRows()
	p := app.activePanel()
	row := p.Cursor - p.ScrollOffset
	mid := vr / 2
	if row != mid && row != vr-1 {
		t.Fatalf("after Parent+disk resort: viewport row = %d, want %d or %d; cursor=%d scroll=%d",
			row, mid, vr-1, p.Cursor, p.ScrollOffset)
	}
}

// Regression: key handling calls render() before the Run loop's trailing reconcileAfterEvent(),
// so latched sync must run inside render(); otherwise the follower updates one tick late and
// the UI tracks the previous directory highlight.
func TestSyncFollowAppliesBeforeRenderAfterNav(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"alpha", "beta", "gamma"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	app.model.ActivePanel = ui.LeftPanel
	selectPanelEntryByName(t, app.panelByID(ui.LeftPanel), "alpha")
	app.dispatch(keymap.ActionPanelToggleSync)

	app.dispatch(keymap.ActionNavDown)
	app.render()
	if got, want := filepath.Clean(app.panelByID(ui.RightPanel).Path.String()), filepath.Clean(filepath.Join(root, "beta")); got != want {
		t.Fatalf("after down+render follower path = %q, want %q", got, want)
	}

	app.dispatch(keymap.ActionNavDown)
	app.render()
	if got, want := filepath.Clean(app.panelByID(ui.RightPanel).Path.String()), filepath.Clean(filepath.Join(root, "gamma")); got != want {
		t.Fatalf("after second down+render follower path = %q, want %q", got, want)
	}
}

func TestPanelSyncFollowNavDebounceDefersFollowerUntilCleared(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"alpha", "beta"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	app.model.ActivePanel = ui.LeftPanel
	selectPanelEntryByName(t, app.panelByID(ui.LeftPanel), "alpha")
	app.dispatch(keymap.ActionPanelToggleSync)
	if got, want := filepath.Clean(app.panelByID(ui.RightPanel).Path.String()), filepath.Clean(filepath.Join(root, "alpha")); got != want {
		t.Fatalf("right after sync enable = %q, want %q", got, want)
	}
	app.config.UI.PanelSyncFollowNavDebounceMS = 500
	app.dispatch(keymap.ActionNavDown)
	app.reconcileAfterEvent()
	if got, want := filepath.Clean(app.panelByID(ui.RightPanel).Path.String()), filepath.Clean(filepath.Join(root, "alpha")); got != want {
		t.Fatalf("follower path after debounced nav+reconcile = %q, want %q (still coalescing)", got, want)
	}
	app.clearPanelSyncFollowNavCoalesce()
	app.reconcileAfterEvent()
	if got, want := filepath.Clean(app.panelByID(ui.RightPanel).Path.String()), filepath.Clean(filepath.Join(root, "beta")); got != want {
		t.Fatalf("follower path after clear+reconcile = %q, want %q", got, want)
	}
}

// Regression: with selections-strip focus, sync must follow the strip row (what the user
// is steering), not the file-list cursor — otherwise the other panel shows a stale directory.
func TestSyncFollowUsesSelectionsStripWhenStripFocused(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	beta := filepath.Join(root, "beta")
	if err := os.Mkdir(alpha, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(beta, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	app.model.ActivePanel = ui.LeftPanel
	app.model.ActiveSubFocus = ui.SubFocusFileList
	left := app.panelByID(ui.LeftPanel)
	selectPanelEntryByName(t, left, "beta")
	if selected, _ := left.ToggleSelection(); !selected {
		t.Fatal("toggle selection on beta")
	}
	selectPanelEntryByName(t, left, "alpha")
	if err := left.NavigateTo(alpha, "", 20); err != nil {
		t.Fatalf("NavigateTo alpha: %v", err)
	}
	if left.SelectionsStripCount() == 0 {
		t.Fatal("expected selections strip to list beta while cwd is alpha")
	}
	app.model.ActiveSubFocus = ui.SubFocusSelectionsStrip
	left.SelectionsStripCursor = 0

	app.dispatch(keymap.ActionPanelToggleSync)

	want := filepath.Clean(beta)
	if got := filepath.Clean(app.panelByID(ui.RightPanel).Path.String()); got != want {
		t.Fatalf("follower path = %q want %q (strip row should drive sync, not file-list cursor)", got, want)
	}
}

func TestToggleSyncDisablesWhenAlreadyDriving(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "alpha"), 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	app.model.ActivePanel = ui.LeftPanel
	selectPanelEntryByName(t, app.panelByID(ui.LeftPanel), "alpha")

	app.dispatch(keymap.ActionPanelToggleSync)
	if !app.model.SyncFollowEnabled || app.model.SyncFollowPanel != ui.LeftPanel {
		t.Fatalf("Sync state after enable = (enabled=%v panel=%d), want (true, LeftPanel)", app.model.SyncFollowEnabled, app.model.SyncFollowPanel)
	}

	app.dispatch(keymap.ActionPanelToggleSync)
	if app.model.SyncFollowEnabled {
		t.Fatalf("SyncFollowEnabled after disable = true, want false")
	}
	if !strings.Contains(app.model.Message, "Sync disabled") {
		t.Fatalf("transient message = %q, want Sync disabled", app.model.Message)
	}
}

func TestToggleSyncFromOtherPanelClearsPreviousDriverFirst(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	gamma := filepath.Join(alpha, "gamma")
	if err := os.MkdirAll(gamma, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	app.model.ActivePanel = ui.LeftPanel
	selectPanelEntryByName(t, app.panelByID(ui.LeftPanel), "alpha")
	app.dispatch(keymap.ActionPanelToggleSync)
	if !app.model.SyncFollowEnabled || app.model.SyncFollowPanel != ui.LeftPanel {
		t.Fatalf("Sync state after left enable = (enabled=%v panel=%d), want (true, LeftPanel)", app.model.SyncFollowEnabled, app.model.SyncFollowPanel)
	}
	// Right panel should now be inside /alpha (synced from left).
	if got, want := filepath.Clean(app.panelByID(ui.RightPanel).Path.String()), filepath.Clean(alpha); got != want {
		t.Fatalf("right panel after left enable = %q, want %q", got, want)
	}

	// Switch focus to right and toggle sync there: should clear left's sync, then enable right.
	app.model.ActivePanel = ui.RightPanel
	selectPanelEntryByName(t, app.panelByID(ui.RightPanel), "gamma")
	app.dispatch(keymap.ActionPanelToggleSync)

	if !app.model.SyncFollowEnabled || app.model.SyncFollowPanel != ui.RightPanel {
		t.Fatalf("Sync state after right toggle = (enabled=%v panel=%d), want (true, RightPanel)", app.model.SyncFollowEnabled, app.model.SyncFollowPanel)
	}
	if got, want := filepath.Clean(app.panelByID(ui.LeftPanel).Path.String()), filepath.Clean(gamma); got != want {
		t.Fatalf("left panel path after right takes over = %q, want %q", got, want)
	}
}

func TestSyncFollowsCursorMovementOverDirectory(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	beta := filepath.Join(root, "beta")
	for _, p := range []string{alpha, beta} {
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	app.model.ActivePanel = ui.LeftPanel
	selectPanelEntryByName(t, app.panelByID(ui.LeftPanel), "alpha")
	app.dispatch(keymap.ActionPanelToggleSync)
	if got, want := filepath.Clean(app.panelByID(ui.RightPanel).Path.String()), filepath.Clean(alpha); got != want {
		t.Fatalf("right panel after enable = %q, want %q", got, want)
	}

	// Move cursor onto beta and verify the right panel mirrors it.
	selectPanelEntryByName(t, app.panelByID(ui.LeftPanel), "beta")
	app.reconcileAfterEvent()

	if got, want := filepath.Clean(app.panelByID(ui.RightPanel).Path.String()), filepath.Clean(beta); got != want {
		t.Fatalf("right panel after move = %q, want %q", got, want)
	}
}

func TestSyncSkipsCursorMovementOverFile(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	if err := os.Mkdir(alpha, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "notes.txt"))
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	app.model.ActivePanel = ui.LeftPanel
	selectPanelEntryByName(t, app.panelByID(ui.LeftPanel), "alpha")
	app.dispatch(keymap.ActionPanelToggleSync)
	if got, want := filepath.Clean(app.panelByID(ui.RightPanel).Path.String()), filepath.Clean(alpha); got != want {
		t.Fatalf("right panel after enable = %q, want %q", got, want)
	}

	selectPanelEntryByName(t, app.panelByID(ui.LeftPanel), "notes.txt")
	app.reconcileAfterEvent()

	if got, want := filepath.Clean(app.panelByID(ui.RightPanel).Path.String()), filepath.Clean(alpha); got != want {
		t.Fatalf("right panel after non-dir hover = %q, want unchanged %q", got, want)
	}
}

func TestQuickViewFollowsDirectoryHighlight(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	if err := os.Mkdir(alpha, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	app.config.UI.QuickViewPreviewDebounceMS = 0

	app.model.ActivePanel = ui.LeftPanel
	selectPanelEntryByName(t, app.panelByID(ui.LeftPanel), "alpha")
	app.model.QuickViewEnabled = true
	app.reconcileAfterEvent()

	if got, want := filepath.Clean(app.panelByID(ui.RightPanel).Path.String()), filepath.Clean(alpha); got != want {
		t.Fatalf("inactive panel path = %q, want %q", got, want)
	}
	if app.filePreviewOpen() {
		t.Fatal("file preview should be closed while quick view shows directory listing")
	}
}

func TestQuickViewFollowsCursorBetweenSubdirectories(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	beta := filepath.Join(root, "beta")
	for _, p := range []string{alpha, beta} {
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	app.config.UI.QuickViewPreviewDebounceMS = 0

	app.model.ActivePanel = ui.LeftPanel
	selectPanelEntryByName(t, app.panelByID(ui.LeftPanel), "alpha")
	app.model.QuickViewEnabled = true
	app.reconcileAfterEvent()
	if got, want := filepath.Clean(app.panelByID(ui.RightPanel).Path.String()), filepath.Clean(alpha); got != want {
		t.Fatalf("inactive after alpha = %q, want %q", got, want)
	}

	selectPanelEntryByName(t, app.panelByID(ui.LeftPanel), "beta")
	app.reconcileAfterEvent()
	if got, want := filepath.Clean(app.panelByID(ui.RightPanel).Path.String()), filepath.Clean(beta); got != want {
		t.Fatalf("inactive after beta = %q, want %q", got, want)
	}
}

func TestQuickViewShowsPreviewOnFileHighlight(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "notes.txt"))
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	app.config.UI.QuickViewPreviewDebounceMS = 0

	app.model.ActivePanel = ui.LeftPanel
	selectPanelEntryByName(t, app.panelByID(ui.LeftPanel), "notes.txt")
	app.model.QuickViewEnabled = true
	app.reconcileAfterEvent()

	if !app.filePreviewOpen() {
		t.Fatal("file preview should open for a highlighted file with quick view on")
	}
}

func TestQuickViewPreviewNavDebounceDefersPreviewUntilFlush(t *testing.T) {
	root := t.TempDir()
	notes := filepath.Join(root, "notes.txt")
	readme := filepath.Join(root, "readme.txt")
	writeFile(t, notes)
	writeFile(t, readme)
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	app.config.UI.QuickViewPreviewDebounceMS = 500

	app.model.ActivePanel = ui.LeftPanel
	selectPanelEntryByName(t, app.panelByID(ui.LeftPanel), "notes.txt")
	app.model.QuickViewEnabled = true
	app.applyQuickViewPreviewImmediately()

	app.commandsMu.RLock()
	firstPath := app.model.FilePreview.Path
	app.commandsMu.RUnlock()
	if firstPath != notes {
		t.Fatalf("preview path = %q, want %q", firstPath, notes)
	}

	app.dispatch(keymap.ActionNavDown)
	app.reconcileAfterEvent()

	app.commandsMu.RLock()
	stillPath := app.model.FilePreview.Path
	app.commandsMu.RUnlock()
	if stillPath != notes {
		t.Fatalf("preview path after debounced nav+reconcile = %q, want %q (still coalescing)", stillPath, notes)
	}

	app.clearQuickViewNavCoalesce()
	app.reconcileAfterEvent()
	if !app.applyQuickViewPreviewFlush(quickViewFlushPayload{gen: app.quickViewDebounceGen.Load()}) {
		t.Fatal("applyQuickViewPreviewFlush should apply deferred preview")
	}

	app.commandsMu.RLock()
	gotPath := app.model.FilePreview.Path
	app.commandsMu.RUnlock()
	if gotPath != readme {
		t.Fatalf("preview path after flush = %q, want %q", gotPath, readme)
	}
}

func TestFilePreviewRunGenStaleSkipsRunningPatch(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "notes.txt")
	writeFile(t, path)
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	app.patchFilePreview(func(st *ui.FilePreviewState) {
		st.Open = true
		st.Phase = ui.FilePreviewPhasePending
		st.Path = path
	})
	staleGen := app.filePreviewRunGen.Add(1)
	app.filePreviewRunGen.Add(1)

	app.runFilePreview(context.Background(), path, []string{"/bin/true"}, root, false, staleGen)

	app.commandsMu.RLock()
	ph := app.model.FilePreview.Phase
	app.commandsMu.RUnlock()
	if ph != ui.FilePreviewPhasePending {
		t.Fatalf("Phase = %v, want Pending when run gen is stale at start", ph)
	}
}

func TestQuickViewOffRetainsInactivePath(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	if err := os.Mkdir(alpha, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)
	app.config.UI.QuickViewPreviewDebounceMS = 0

	app.model.ActivePanel = ui.LeftPanel
	selectPanelEntryByName(t, app.panelByID(ui.LeftPanel), "alpha")
	app.model.QuickViewEnabled = true
	app.reconcileAfterEvent()
	inactiveAfterDir := filepath.Clean(app.panelByID(ui.RightPanel).Path.String())

	app.dispatch(keymap.ActionFileQuickView)
	if app.model.QuickViewEnabled {
		t.Fatal("quick view should be disabled after toggle")
	}
	if app.filePreviewOpen() {
		t.Fatal("file preview should be closed after quick view off")
	}
	if got, want := filepath.Clean(app.panelByID(ui.RightPanel).Path.String()), inactiveAfterDir; got != want {
		t.Fatalf("inactive path after quick view off = %q, want retained %q", got, want)
	}
}

func TestSyncDoesNotFollowFromNonDriverActivePanel(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	beta := filepath.Join(root, "beta")
	for _, p := range []string{alpha, beta} {
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	app.model.ActivePanel = ui.LeftPanel
	selectPanelEntryByName(t, app.panelByID(ui.LeftPanel), "alpha")
	app.dispatch(keymap.ActionPanelToggleSync)
	rightAfterEnable := filepath.Clean(app.panelByID(ui.RightPanel).Path.String())

	// Switch focus to the non-driver (right) panel and move its cursor.
	app.dispatch(keymap.ActionPanelSwitch)
	if app.model.ActivePanel != ui.RightPanel {
		t.Fatalf("ActivePanel after switch = %d, want RightPanel", app.model.ActivePanel)
	}
	if !app.model.SyncFollowEnabled || app.model.SyncFollowPanel != ui.LeftPanel {
		t.Fatalf("Tab should not change Sync state; got (enabled=%v panel=%d), want (true, LeftPanel)", app.model.SyncFollowEnabled, app.model.SyncFollowPanel)
	}
	selectPanelEntryByName(t, app.panelByID(ui.LeftPanel), "beta")
	app.reconcileAfterEvent()

	if got := filepath.Clean(app.panelByID(ui.RightPanel).Path.String()); got != rightAfterEnable {
		t.Fatalf("right panel changed while non-driver was active: got %q, want unchanged %q", got, rightAfterEnable)
	}
}

// Regression: bookmark / history-picker navigation jumps the active panel via
// navigatePanelToDirectory (which loads, then would historically need its own sync
// trigger). With the post-event reconciler, the next reconcileAfterEvent re-mirrors
// the follower automatically — proving the chokepoint catches paths that bypass dispatch.
func TestSyncFollowsBookmarkLikeNavigationFromActivePanel(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	beta := filepath.Join(root, "beta")
	betaChild := filepath.Join(beta, "child")
	if err := os.MkdirAll(betaChild, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(alpha, 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	app.model.ActivePanel = ui.LeftPanel
	selectPanelEntryByName(t, app.panelByID(ui.LeftPanel), "alpha")
	app.dispatch(keymap.ActionPanelToggleSync)
	if got, want := filepath.Clean(app.panelByID(ui.RightPanel).Path.String()), filepath.Clean(alpha); got != want {
		t.Fatalf("right panel after enable = %q, want %q", got, want)
	}

	// Simulate a bookmark/history jump: navigate the active panel to /beta.
	if err := app.navigatePanelToDirectory(ui.LeftPanel, beta, ""); err != nil {
		t.Fatalf("navigatePanelToDirectory: %v", err)
	}
	// In the Run loop this is what fires after the bookmark dialog closes.
	app.reconcileAfterEvent()

	// Cursor in /beta lands on "child" (only entry); sync should mirror it.
	if got, want := filepath.Clean(app.panelByID(ui.RightPanel).Path.String()), filepath.Clean(betaChild); got != want {
		t.Fatalf("right panel after bookmark-like jump = %q, want %q (sync should re-mirror)", got, want)
	}
}

// Regression guard: when the inactive (follower) panel changes directory by other means
// (e.g. an out-of-band Load), sync must NOT trigger from it because only the driver-while-active
// fires sync hops.
func TestSyncDoesNotFollowWhenInactivePanelChangesDirectory(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	beta := filepath.Join(root, "beta")
	for _, p := range []string{alpha, beta} {
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	app.model.ActivePanel = ui.LeftPanel
	selectPanelEntryByName(t, app.panelByID(ui.LeftPanel), "alpha")
	app.dispatch(keymap.ActionPanelToggleSync)
	leftBefore := filepath.Clean(app.panelByID(ui.LeftPanel).Path.String())

	// Mutate the follower (right) panel directly. The driver (left) is still active and
	// has not moved its cursor, so the left panel should stay put even after reconcile.
	if err := app.panelByID(ui.RightPanel).Load(beta); err != nil {
		t.Fatalf("Load: %v", err)
	}
	app.reconcileAfterEvent()

	if got := filepath.Clean(app.panelByID(ui.LeftPanel).Path.String()); got != leftBefore {
		t.Fatalf("left panel changed because follower moved: got %q, want %q", got, leftBefore)
	}
}

// Regression-by-design: the Insert key (panel.select-toggle) calls
// ToggleSelectionAndAdvance, which moves the cursor down by one. With the previous
// per-call-site wiring this branch had no syncFollowFromActive() call, so sync
// would have silently gone stale. The post-event reconciler catches it for free.
func TestSyncFollowsAfterSelectToggleAdvance(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"alpha", "beta"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	app.model.ActivePanel = ui.LeftPanel
	selectPanelEntryByName(t, app.panelByID(ui.LeftPanel), "alpha")
	app.dispatch(keymap.ActionPanelToggleSync)
	if got, want := filepath.Clean(app.panelByID(ui.RightPanel).Path.String()), filepath.Clean(filepath.Join(root, "alpha")); got != want {
		t.Fatalf("right panel after enable = %q, want %q", got, want)
	}

	// Insert: toggle selection on alpha and advance cursor to beta.
	app.dispatch(keymap.ActionPanelSelectToggle)
	app.reconcileAfterEvent()

	if got, want := filepath.Clean(app.panelByID(ui.RightPanel).Path.String()), filepath.Clean(filepath.Join(root, "beta")); got != want {
		t.Fatalf("right panel after Insert advance = %q, want %q (reconciler should mirror new highlight)", got, want)
	}
}

// reconcileAfterEvent walks both panels. Uncached listings must not enable IdleDiskTotalsSort;
// per-panel idle timers are only armed once ListingFullyDiskCached holds ( subtree events ).
func TestDiskUsageIdleArmingSurvivesPanelSwitchViaReconciler(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"alpha", "beta"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	right := app.panelByID(ui.RightPanel)
	right.Sort.DiskUsageIdleSizeSort = true
	right.DiskUsageIdleSortActivated = true
	right.IdleDiskTotalsSort = false

	if app.diskIdleSort[ui.RightPanel].timer != nil {
		t.Fatal("right idle timer should be nil before reconcile")
	}

	app.dispatch(keymap.ActionPanelSwitch)
	if app.model.ActivePanel != ui.RightPanel {
		t.Fatalf("ActivePanel after switch = %d, want RightPanel", app.model.ActivePanel)
	}
	app.reconcileAfterEvent()

	if app.diskIdleSort[ui.RightPanel].timer != nil {
		t.Fatal("uncached listing must not arm idle-sort timer from reconcile alone")
	}
	if right.IdleDiskTotalsSort {
		t.Fatalf("IdleDiskTotalsSort = true; want false until listing is fully disk-cached")
	}
}

func TestApplyIdleDiskSortRequiresFullListingCache(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"alpha", "beta"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	left := app.panelByID(ui.LeftPanel)
	alphaPath := filepath.Join(root, "alpha")

	left.Sort.DiskUsageIdleSizeSort = true
	left.DiskUsageIdleSortActivated = true
	left.IdleDiskTotalsSort = false
	left.DiskSorter = func(abs string) (int64, bool) {
		if filepath.Clean(abs) == filepath.Clean(alphaPath) {
			return 42, true
		}
		return 0, false
	}

	app.applyIdleDiskSort(ui.LeftPanel, app.diskIdleSort[ui.LeftPanel].epoch)

	if left.IdleDiskTotalsSort {
		t.Fatal("IdleDiskTotalsSort should stay false when listing is not fully disk-cached")
	}
}

func TestHandlePanelDirChangedRightDoesNotInvalidateLeftIdleTimer(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	if err := os.Mkdir(alpha, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(alpha, "f.txt"))

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	leftRoot := filepath.Clean(app.panelByID(ui.LeftPanel).Path.String())
	left := app.panelByID(ui.LeftPanel)
	left.Sort.DiskUsageIdleSizeSort = true
	left.DiskUsageIdleSortActivated = true
	left.IdleDiskTotalsSort = false
	left.DiskSorter = func(abs string) (int64, bool) { return 1, true }
	app.setDiskUsageScanScope(leftRoot, []string{filepath.Clean(alpha)})

	app.diskIdleNavPath[ui.LeftPanel] = leftRoot
	app.armIdleDiskSortTimer(ui.LeftPanel)
	if app.diskIdleSort[ui.LeftPanel].timer == nil {
		t.Fatal("expected idle timer armed")
	}

	right := app.panelByID(ui.RightPanel)
	right.Sort.DiskUsageIdleSizeSort = true
	right.DiskUsageIdleSortActivated = true
	right.IdleDiskTotalsSort = false
	vr := app.panelViewportRows(ui.RightPanel)
	if err := right.NavigateTo(filepath.Clean(alpha), "", vr); err != nil {
		t.Fatalf("NavigateTo: %v", err)
	}

	app.handlePanelDirChanged(ui.RightPanel)

	if app.diskIdleSort[ui.LeftPanel].timer == nil {
		t.Fatal("only the navigated panel should reset idle-sort debounce")
	}
	app.invalidateIdleDiskSortPanel(ui.LeftPanel)
}

func TestHandlePanelDirChangedLeftClearsIdleTimerOnChdir(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	if err := os.Mkdir(alpha, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(alpha, "f.txt"))

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	leftRoot := filepath.Clean(app.panelByID(ui.LeftPanel).Path.String())

	left := app.panelByID(ui.LeftPanel)
	left.Sort.DiskUsageIdleSizeSort = true
	left.DiskUsageIdleSortActivated = true
	left.IdleDiskTotalsSort = false
	left.DiskSorter = func(abs string) (int64, bool) { return 1, true }
	app.setDiskUsageScanScope(leftRoot, []string{filepath.Clean(alpha)})

	app.diskIdleNavPath[ui.LeftPanel] = leftRoot
	app.armIdleDiskSortTimer(ui.LeftPanel)

	vr := app.panelViewportRows(ui.LeftPanel)
	if err := left.NavigateTo(filepath.Clean(alpha), "", vr); err != nil {
		t.Fatalf("NavigateTo: %v", err)
	}
	app.handlePanelDirChanged(ui.LeftPanel)

	if app.diskIdleSort[ui.LeftPanel].timer != nil {
		t.Fatal("idle timer should clear when panel cwd changes")
	}
}

func TestDiskIdleSortActivatesAfterScanWhenListingCached(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a", "b"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	left := app.panelByID(ui.LeftPanel)
	left.Sort.DiskUsageIdleSizeSort = true
	left.DiskUsageIdleSortActivated = true
	left.IdleDiskTotalsSort = false

	app.startDiskUsageScanForPanel(ui.LeftPanel)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		app.pollDiskUsageUpdates()
		if !app.diskUsageScanBusy() && left.ListingFullyDiskCached() {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	if !left.ListingFullyDiskCached() {
		t.Fatalf("listing not fully cached after scan busy=%v", app.diskUsageScanBusy())
	}

	ep := app.diskIdleSort[ui.LeftPanel].epoch
	app.applyIdleDiskSort(ui.LeftPanel, ep)

	if !left.IdleDiskTotalsSort {
		t.Fatalf("IdleDiskTotalsSort still false epoch=%d busy=%v", ep, app.diskUsageScanBusy())
	}
}

func TestHandlePanelDirChangedAppliesDiskSortWhenUsageSortEnabledWithoutActivatedLatch(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	left := app.panelByID(ui.LeftPanel)
	left.Sort.DiskUsageIdleSizeSort = true
	left.DiskUsageIdleSortActivated = false // must not deadlock idle disk ordering
	left.IdleDiskTotalsSort = false
	left.DiskSorter = func(abs string) (int64, bool) { return 1, true }
	app.setDiskUsageScanScope(left.PathString(), []string{filepath.Join(left.PathString(), "x")})

	app.handlePanelDirChanged(ui.LeftPanel)

	if !left.IdleDiskTotalsSort {
		t.Fatal("expected IdleDiskTotalsSort when DiskUsageIdleSizeSort is on and listing is fully cached")
	}
}

func TestNavigateOutsideDiskUsageScanScopeClearsIdleSort(t *testing.T) {
	root := t.TempDir()
	scanned := filepath.Join(root, "scanned")
	other := filepath.Join(root, "other")
	for _, p := range []string{scanned, other} {
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(scanned, "in-scan.dat"))
	writeFile(t, filepath.Join(other, "out-scan.dat"))

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	left := app.panelByID(ui.LeftPanel)
	left.Sort.DiskUsageIdleSizeSort = true
	app.setDiskUsageScanScope(root, []string{scanned})
	app.model.DiskUsageShown = true
	app.model.DiskUsagePanelID = ui.LeftPanel

	left.DiskSorter = app.diskUsage.Size
	left.IdleDiskTotalsSort = true
	left.ApplySort()

	vr := app.panelViewportRows(ui.LeftPanel)
	if err := left.NavigateTo(other, "", vr); err != nil {
		t.Fatalf("NavigateTo other: %v", err)
	}
	app.handlePanelDirChanged(ui.LeftPanel)

	if left.IdleDiskTotalsSort {
		t.Fatal("IdleDiskTotalsSort should be false outside scan scope")
	}
	if app.listingInDiskUsageScanScope(other) {
		t.Fatal("other directory should be outside scan scope")
	}

	if err := left.NavigateTo(scanned, "", vr); err != nil {
		t.Fatalf("NavigateTo scanned: %v", err)
	}
	app.startDiskUsageScanForPanel(ui.LeftPanel)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		app.pollDiskUsageUpdates()
		if !app.diskUsageScanBusy() && left.ListingFullyDiskCached() {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	app.handlePanelDirChanged(ui.LeftPanel)
	if !left.IdleDiskTotalsSort {
		t.Fatal("IdleDiskTotalsSort should apply inside scan scope when fully cached")
	}
	if !app.listingInDiskUsageScanScope(scanned) {
		t.Fatal("scanned subtree should be in scope")
	}
}

func TestDiskUsageScanScopeAppliesOnEitherPanel(t *testing.T) {
	root := t.TempDir()
	scanned := filepath.Join(root, "scanned")
	if err := os.Mkdir(scanned, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(scanned, "a.dat"))
	writeFile(t, filepath.Join(scanned, "b.dat"))

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	left := app.panelByID(ui.LeftPanel)
	left.Sort.DiskUsageIdleSizeSort = true
	app.startDiskUsageScanForPanel(ui.LeftPanel)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		app.pollDiskUsageUpdates()
		if !app.diskUsageScanBusy() && left.ListingFullyDiskCached() {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}

	right := app.panelByID(ui.RightPanel)
	right.Sort.DiskUsageIdleSizeSort = true
	vrRight := app.panelViewportRows(ui.RightPanel)
	if err := right.NavigateTo(scanned, "", vrRight); err != nil {
		t.Fatalf("NavigateTo scanned on right: %v", err)
	}
	app.handlePanelDirChanged(ui.RightPanel)

	if !app.listingInDiskUsageScanScope(scanned) {
		t.Fatal("scanned path should be in global scan scope")
	}
	if !right.IdleDiskTotalsSort {
		t.Fatal("right panel should idle-sort by disk totals inside scan scope")
	}
	if !app.model.DiskUsageShown ||
		!panel.ListingPathInDiskUsageScanScope(right.PathString(), app.model.DiskUsageScanOrigin, app.model.DiskUsageScanRoots) {
		t.Fatal("right panel listing should be eligible for disk usage display inside scan scope")
	}
}

func TestApplyIdleDiskSortIgnoresStaleEpoch(t *testing.T) {
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, t.TempDir())

	left := app.panelByID(ui.LeftPanel)
	left.Sort.DiskUsageIdleSizeSort = true
	left.DiskUsageIdleSortActivated = true
	left.IdleDiskTotalsSort = false
	left.DiskSorter = func(abs string) (int64, bool) { return 1, true }

	stale := app.diskIdleSort[ui.LeftPanel].epoch
	app.invalidateIdleDiskSortPanel(ui.LeftPanel)
	app.applyIdleDiskSort(ui.LeftPanel, stale)

	if left.IdleDiskTotalsSort {
		t.Fatal("stale epoch must not apply idle disk sort")
	}
}

func TestClearAllDiskUsageData(t *testing.T) {
	root := t.TempDir()
	scanned := filepath.Join(root, "scanned")
	if err := os.Mkdir(scanned, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(scanned, "a.dat"))
	writeFile(t, filepath.Join(scanned, "b.dat"))

	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	left := app.panelByID(ui.LeftPanel)
	left.Sort.DiskUsageIdleSizeSort = true
	app.startDiskUsageScanForPanel(ui.LeftPanel)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		app.pollDiskUsageUpdates()
		if !app.diskUsageScanBusy() && left.ListingFullyDiskCached() {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	if !app.model.DiskUsageShown {
		t.Fatal("expected disk usage to be shown after scan")
	}
	if _, ok := app.diskUsage.Size(scanned); !ok {
		t.Fatal("expected cached size for scanned directory")
	}

	left.IdleDiskTotalsSort = true
	app.clearAllDiskUsageData()

	if app.model.DiskUsageShown {
		t.Fatal("DiskUsageShown should be false after clear")
	}
	if app.model.DiskUsageScanOrigin != "" || len(app.model.DiskUsageScanRoots) > 0 {
		t.Fatal("scan scope should be cleared")
	}
	if left.IdleDiskTotalsSort {
		t.Fatal("IdleDiskTotalsSort should be false after clear")
	}
	if _, ok := app.diskUsage.Size(scanned); ok {
		t.Fatal("engine cache should be empty after clear")
	}
	if app.model.Message != "Disk usage data cleared" {
		t.Fatalf("message = %q, want Disk usage data cleared", app.model.Message)
	}
}

func TestSyncFollowSkipsHistoryRecordingOnFollower(t *testing.T) {
	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	beta := filepath.Join(root, "beta")
	for _, p := range []string{alpha, beta} {
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	screen := newScreen(t, 80, 24)
	app := newApp(t, screen, root)

	app.model.ActivePanel = ui.LeftPanel
	rightHistoryAtStart := append([]string(nil), app.panelByID(ui.RightPanel).History...)

	selectPanelEntryByName(t, app.panelByID(ui.LeftPanel), "alpha")
	app.dispatch(keymap.ActionPanelToggleSync)
	selectPanelEntryByName(t, app.panelByID(ui.LeftPanel), "beta")
	app.reconcileAfterEvent()

	right := app.panelByID(ui.RightPanel)
	if got, want := filepath.Clean(right.PathString()), filepath.Clean(beta); got != want {
		t.Fatalf("right panel path = %q, want %q", got, want)
	}
	// Sync hops use Load (not NavigateTo), so the follower's directory history must remain untouched
	// beyond whatever it already had at startup.
	if len(right.History) != len(rightHistoryAtStart) {
		t.Fatalf("right history length = %d, want %d (sync should not record history)", len(right.History), len(rightHistoryAtStart))
	}
}

func TestMkdirDialogWithoutSelectionHidesActionRadios(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.txt"))

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)

	app.dispatch(keymap.ActionFileMkdir)
	if !app.model.FileDialog.Open || app.model.FileDialog.DialogType != ui.FileDialogMkdir {
		t.Fatal("F7 should open mkdir dialog")
	}
	if app.model.FileDialog.MkdirShowActions {
		t.Fatal("MkdirShowActions should be false without selections")
	}
	if got, want := ui.FileDialogFocusForm(app.model.FileDialog).TotalFocus(), 3; got != want {
		t.Fatalf("focus count = %d, want %d (field + OK + Cancel)", got, want)
	}
}

func TestMkdirDialogWithSelectionShowsActionRadiosAndNav(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	writeFile(t, src)

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{src: true}

	app.dispatch(keymap.ActionFileMkdir)
	if !app.model.FileDialog.Open {
		t.Fatal("dialog should open")
	}
	if !app.model.FileDialog.MkdirShowActions {
		t.Fatal("MkdirShowActions should be true with selections")
	}
	if app.model.FileDialog.MkdirAction != ui.MkdirActionCreate {
		t.Fatalf("MkdirAction = %v, want MkdirActionCreate (default)", app.model.FileDialog.MkdirAction)
	}
	if got, want := ui.FileDialogFocusForm(app.model.FileDialog).TotalFocus(), 6; got != want {
		t.Fatalf("focus count = %d, want %d (field + 3 radios + OK + Cancel)", got, want)
	}

	// Down navigates: field(0) -> radio0(1) -> radio1(2) -> radio2(3) -> OK(4)
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if app.model.FileDialog.FocusedField != 1 {
		t.Fatalf("Down from field: focus = %d, want 1 (first radio)", app.model.FileDialog.FocusedField)
	}
	// Space on first radio selects MkdirActionCreate (already default).
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	if app.model.FileDialog.MkdirAction != ui.MkdirActionCreate {
		t.Fatalf("MkdirAction = %v, want MkdirActionCreate after Space on first radio", app.model.FileDialog.MkdirAction)
	}
	// Move to second radio and select copy.
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, ' ', tcell.ModNone))
	if app.model.FileDialog.MkdirAction != ui.MkdirActionCreateCopySelect {
		t.Fatalf("MkdirAction = %v, want MkdirActionCreateCopySelect after Space on second radio", app.model.FileDialog.MkdirAction)
	}
	// Down past last radio reaches OK button.
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if app.model.FileDialog.FocusedField != 4 {
		t.Fatalf("Down to OK: focus = %d, want 4", app.model.FileDialog.FocusedField)
	}
	// Right moves OK -> Cancel.
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if app.model.FileDialog.FocusedField != 5 {
		t.Fatalf("Right OK->Cancel: focus = %d, want 5", app.model.FileDialog.FocusedField)
	}
}

func TestMkdirDialogAltShortcutsSelectActionFromInputField(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	writeFile(t, src)

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{src: true}

	app.dispatch(keymap.ActionFileMkdir)
	if app.model.FileDialog.FocusedField != 0 {
		t.Fatalf("initial focus = %d, want 0 (directory name field)", app.model.FileDialog.FocusedField)
	}

	alt := tcell.ModAlt
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'c', alt))
	if app.model.FileDialog.MkdirAction != ui.MkdirActionCreateCopySelect {
		t.Fatalf("Alt+c: MkdirAction = %v, want copy", app.model.FileDialog.MkdirAction)
	}
	if app.model.FileDialog.FocusedField != 2 {
		t.Fatalf("Alt+c: focus = %d, want 2 (copy radio)", app.model.FileDialog.FocusedField)
	}

	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'm', alt))
	if app.model.FileDialog.MkdirAction != ui.MkdirActionCreateMoveSelect {
		t.Fatalf("Alt+m: MkdirAction = %v, want move", app.model.FileDialog.MkdirAction)
	}
	if app.model.FileDialog.FocusedField != 3 {
		t.Fatalf("Alt+m: focus = %d, want 3 (move radio)", app.model.FileDialog.FocusedField)
	}

	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'r', alt))
	if app.model.FileDialog.MkdirAction != ui.MkdirActionCreate {
		t.Fatalf("Alt+r: MkdirAction = %v, want create", app.model.FileDialog.MkdirAction)
	}
	if app.model.FileDialog.FocusedField != 1 {
		t.Fatalf("Alt+r: focus = %d, want 1 (create radio)", app.model.FileDialog.FocusedField)
	}

	// Alt+C must not close the dialog in mkdir-with-actions mode.
	if !app.model.FileDialog.Open {
		t.Fatal("dialog closed after Alt+c; want copy selection")
	}
}

func TestMkdirDialogRadioRowsRejectTextInput(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	writeFile(t, src)

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)
	p := app.activePanel()
	p.SelectedPaths = map[string]bool{src: true}

	app.dispatch(keymap.ActionFileMkdir)
	// Type a fresh value then move down to a radio row.
	for _, r := range "newdir" {
		app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if !app.fileDialogOnRadio() {
		t.Fatalf("expected focus on a radio row, focus = %d", app.model.FileDialog.FocusedField)
	}
	// Typing must not alter the text field while on a radio row.
	beforeValue := app.model.FileDialog.Fields[0].Value
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, 'X', tcell.ModNone))
	if app.model.FileDialog.Fields[0].Value != beforeValue {
		t.Fatalf("text field changed while on radio: %q -> %q", beforeValue, app.model.FileDialog.Fields[0].Value)
	}
	// Backspace also must not alter the text field on a radio row.
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone))
	if app.model.FileDialog.Fields[0].Value != beforeValue {
		t.Fatalf("text field changed via Backspace on radio: %q -> %q", beforeValue, app.model.FileDialog.Fields[0].Value)
	}
}

func TestMkdirOpenInInactiveOpensOtherPanelAfterCreate(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "keep-cursor.txt"))
	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)
	defer app.stopWorker()

	left := app.panelByID(ui.LeftPanel)
	for i := 0; i < left.VisibleEntryCount(); i++ {
		entry, _, ok := left.VisibleEntry(i)
		if ok && entry.Name == "keep-cursor.txt" {
			left.Cursor = i
			break
		}
	}

	app.model.ActivePanel = ui.LeftPanel
	app.dispatch(keymap.ActionFileMkdirOpenInOther)
	if !app.model.FileDialog.Open {
		t.Fatal("expected mkdir dialog open")
	}
	if !app.model.FileDialog.MkdirOpenInInactive {
		t.Fatal("MkdirOpenInInactive = false, want true for file.mkdir-open-in-other")
	}
	for _, r := range "otherdir" {
		app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	wantOther := filepath.Join(dir, "otherdir")
	if got := filepath.Clean(app.panelByID(ui.RightPanel).Path.String()); got != filepath.Clean(wantOther) {
		t.Fatalf("inactive panel path = %q want %q", got, wantOther)
	}
	if got := filepath.Clean(app.panelByID(ui.LeftPanel).Path.String()); got != filepath.Clean(dir) {
		t.Fatalf("active panel path = %q want %q", got, dir)
	}
	entry, ok := left.CurrentEntry()
	if !ok || entry.Name != "keep-cursor.txt" {
		t.Fatalf("active panel cursor = %q, want keep-cursor.txt", entry.Name)
	}
}

func TestMkdirActionCreateOnlyDoesNotQueueJob(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	writeFile(t, src)

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)
	defer app.stopWorker()

	p := app.activePanel()
	p.SelectedPaths = map[string]bool{src: true}

	app.dispatch(keymap.ActionFileMkdir)
	// Type fresh name (first printable clears prefill).
	for _, r := range "newdir" {
		app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	// MkdirActionCreate is the default; confirm without changing the radio.
	app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if app.model.FileDialog.Open {
		t.Fatal("dialog should close after Enter")
	}
	if _, err := os.Stat(filepath.Join(dir, "newdir")); err != nil {
		t.Fatalf("expected new directory: %v", err)
	}
	if got := len(app.jobState.AllJobs()); got != 0 {
		t.Fatalf("expected 0 jobs after plain Create, got %d", got)
	}
	if !p.SelectedPaths[src] {
		t.Fatal("plain Create must preserve selection")
	}
}

func TestMkdirActionCreateAndCopyQueuesCopyJob(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	writeFile(t, src)

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)
	defer app.stopWorker()

	p := app.activePanel()
	p.SelectedPaths = map[string]bool{src: true}

	app.dispatch(keymap.ActionFileMkdir)
	for _, r := range "newdir" {
		app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	// Set MkdirActionCreateCopySelect via the model (focus-independent path).
	app.model.FileDialog.MkdirAction = ui.MkdirActionCreateCopySelect
	app.executeFileDialog()

	if app.model.FileDialog.Open {
		t.Fatal("dialog should close after execute")
	}
	created := filepath.Join(dir, "newdir")
	if info, err := os.Stat(created); err != nil || !info.IsDir() {
		t.Fatalf("expected new directory at %q (err=%v)", created, err)
	}
	jobsList := app.jobState.AllJobs()
	if len(jobsList) != 1 {
		t.Fatalf("expected 1 job after Create+Copy, got %d", len(jobsList))
	}
	j := jobsList[0]
	if j.Type != jobs.TypeCopy {
		t.Fatalf("job type = %v, want TypeCopy", j.Type)
	}
	if filepath.Clean(j.Destination.String()) != filepath.Clean(created) {
		t.Fatalf("job destination = %q, want %q", j.Destination, created)
	}
	if len(j.Sources) != 1 || filepath.Clean(j.Sources[0].String()) != filepath.Clean(src) {
		t.Fatalf("job sources = %v, want [%q]", j.Sources, src)
	}
	if len(p.SelectedPaths) != 0 {
		t.Fatalf("selection should be cleared after queueing copy job, got %d", len(p.SelectedPaths))
	}
	waitUntilAppJobsFinished(t, app, 5*time.Second)
}

func TestMkdirActionCreateAndMoveQueuesMoveJob(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	writeFile(t, src)

	screen := newScreen(t, 80, 20)
	app := newApp(t, screen, dir)
	defer app.stopWorker()

	p := app.activePanel()
	p.SelectedPaths = map[string]bool{src: true}

	app.dispatch(keymap.ActionFileMkdir)
	for _, r := range "newdir" {
		app.handleFileDialogKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.model.FileDialog.MkdirAction = ui.MkdirActionCreateMoveSelect
	app.executeFileDialog()

	if app.model.FileDialog.Open {
		t.Fatal("dialog should close after execute")
	}
	created := filepath.Join(dir, "newdir")
	if info, err := os.Stat(created); err != nil || !info.IsDir() {
		t.Fatalf("expected new directory at %q (err=%v)", created, err)
	}
	jobsList := app.jobState.AllJobs()
	if len(jobsList) != 1 {
		t.Fatalf("expected 1 job after Create+Move, got %d", len(jobsList))
	}
	j := jobsList[0]
	if j.Type != jobs.TypeMove {
		t.Fatalf("job type = %v, want TypeMove", j.Type)
	}
	if filepath.Clean(j.Destination.String()) != filepath.Clean(created) {
		t.Fatalf("job destination = %q, want %q", j.Destination, created)
	}
	if len(p.SelectedPaths) != 0 {
		t.Fatalf("selection should be cleared after queueing move job, got %d", len(p.SelectedPaths))
	}
	waitUntilAppJobsFinished(t, app, 5*time.Second)
}
