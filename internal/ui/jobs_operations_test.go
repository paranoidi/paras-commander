package ui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/oplog"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func screenRowText(screen tcell.SimulationScreen, rect Rect, y int) string {
	var b strings.Builder
	for x := rect.X; x < rect.X+rect.Width; x++ {
		ch, _, _ := screen.Get(x, y)
		if ch == "" {
			ch = " "
		}
		b.WriteString(ch)
	}
	return b.String()
}

func newOpsScreen(t *testing.T) tcell.SimulationScreen {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)
	screen.SetSize(70, 10)
	return screen
}

func TestDrawJobsOperationsPanelRowsAndFilterLabel(t *testing.T) {
	screen := newOpsScreen(t)
	rect := Rect{X: 0, Y: 0, Width: 70, Height: 8}
	win := JobsOpsWindow{JobID: "copy-1", Total: 3, Rows: []JobOperationRow{
		{Op: JobOperation{Result: oplog.Failed, Label: "quarry/third.txt", Reason: "permission denied"}, Ranges: []search.Range{{Start: 0, End: 6}}},
		{Op: JobOperation{Result: oplog.Skipped, Label: "quarry/second.txt", Reason: "exists"}},
		{Op: JobOperation{Result: oplog.Done, Label: "quarry/first.txt"}},
	}}
	st := JobsViewState{OpsResultHidden: dialog.JobResultSuccess}
	st.OpsFilter.Query = "quarry"
	drawJobsOperationsPanel(screen, rect, st, win, "", theme.Default(), false, true, false)

	if top := screenRowText(screen, rect, rect.Y); !strings.HasPrefix(top, "┌─> quarry ") || strings.Contains(top, "Operations") {
		t.Fatalf("top border = %q, want the query at the top left in place of the title", top)
	}
	for i, want := range []string{"quarry/third.txt: permission denied", "quarry/second.txt: exists", "quarry/first.txt"} {
		if row := screenRowText(screen, rect, rect.Y+1+i); !strings.Contains(row, want) {
			t.Fatalf("row %d = %q, want %q", i, row, want)
		}
	}
	if bottom := screenRowText(screen, rect, rect.Y+rect.Height-1); !strings.Contains(bottom, "─ Failed + Skipped ─") {
		t.Fatalf("bottom border = %q, want the active filters", bottom)
	}
}

func TestDrawJobsOperationsPanelPendingAndScanning(t *testing.T) {
	screen := newOpsScreen(t)
	rect := Rect{X: 0, Y: 0, Width: 70, Height: 8}
	win := JobsOpsWindow{JobID: "copy-1", Total: 2, Scanning: true, Rows: []JobOperationRow{{Pending: true}, {Pending: true}}}
	var st JobsViewState
	st.OpsFilter.Query = "lantern"
	drawJobsOperationsPanel(screen, rect, st, win, "", theme.Default(), false, true, false)
	if row := screenRowText(screen, rect, rect.Y+1); !strings.Contains(row, "…") {
		t.Fatalf("pending row = %q, want a placeholder", row)
	}
	if bottom := screenRowText(screen, rect, rect.Y+rect.Height-1); !strings.Contains(bottom, "─ filtering… ─") {
		t.Fatalf("bottom border = %q, want the scanning note", bottom)
	}
	if top := screenRowText(screen, rect, rect.Y); !strings.HasPrefix(top, "┌─> lantern ") {
		t.Fatalf("top border = %q, want the query", top)
	}
}

func TestDrawJobsOperationsPanelQueryNoMatchAndCaret(t *testing.T) {
	screen := newOpsScreen(t)
	rect := Rect{X: 0, Y: 0, Width: 70, Height: 8}
	var st JobsViewState
	st.OpsFilter.Query, st.OpsFilter.Cursor, st.OpsFilter.Editing = "fern", 4, true
	styles := theme.Default()
	drawJobsOperationsPanel(screen, rect, st, JobsOpsWindow{JobID: "copy-1"}, "", styles, false, true, false)
	if _, style, _ := screen.Get(rect.X+2, rect.Y); style != styles.FuzzyInputNomatch {
		t.Fatal("a query matching nothing must use the no-match style")
	}
	if _, style, _ := screen.Get(rect.X+2+len("> fern"), rect.Y); style != styles.FuzzyInputNomatch.Reverse(true) {
		t.Fatal("editing query in the focused pane must show a caret after the text")
	}
}

func TestFitJobOperationKeepsLabelAndReason(t *testing.T) {
	op := JobOperation{Result: oplog.Failed, Label: "harbor/locked.dat", Reason: "open /srv/harbor/locked.dat: permission denied"}
	text, labelEnd, whole := fitJobOperation(op, 200)
	if text != op.Text() || labelEnd != len(op.Label) || !whole {
		t.Fatalf("wide: %q %d %v", text, labelEnd, whole)
	}
	text, labelEnd, _ = fitJobOperation(op, 40)
	if n := len([]rune(text)); n > 40 {
		t.Fatalf("narrow text %q is %d runes, want <= 40", text, n)
	}
	label := string([]rune(text)[:labelEnd])
	if !strings.HasSuffix(label, "locked.dat") || !strings.HasPrefix(string([]rune(text)[labelEnd:]), ": open") {
		t.Fatalf("narrow text %q: want the basename kept, then the reason start", text)
	}
}

func TestJobFilesLineColorsFailedAndSkipped(t *testing.T) {
	j := JobEntry{ID: "copy-1", OpsStats: oplog.Stats{Done: 1234, Failed: 2, Skipped: 5}}
	var text strings.Builder
	for _, p := range jobFilesLineParts(j) {
		text.WriteString(p.Text)
	}
	if got, want := text.String(), "Files:       1234 done, 2 failed, 5 skipped"; got != want {
		t.Fatalf("line = %q, want %q", got, want)
	}
	styles := theme.Default()
	body := tcell.StyleDefault
	spans := jobFilesLineSpans(j, styles, body)
	if len(spans) != 2 {
		t.Fatalf("spans = %+v, want failed and skipped", spans)
	}
	line := []rune(text.String())
	if s := string(line[spans[0].Start:spans[0].End]); s != "2 failed" || spans[0].Style != styles.JobsOperationOn(theme.JobsOperationFailed, body) {
		t.Fatalf("failed span = %q", s)
	}
	if s := string(line[spans[1].Start:spans[1].End]); s != "5 skipped" || spans[1].Style != styles.JobsOperationOn(theme.JobsOperationSkipped, body) {
		t.Fatalf("skipped span = %q", s)
	}
	if parts := jobFilesLineParts(JobEntry{ID: "copy-2"}); parts != nil {
		t.Fatalf("empty log parts = %+v, want no line", parts)
	}
	dropped := jobFilesLineParts(JobEntry{OpsStats: oplog.Stats{Done: 10, Dropped: 4}})
	if last := dropped[len(dropped)-1].Text; last != " (log incomplete, 4 dropped)" {
		t.Fatalf("dropped note = %q", last)
	}
}

func TestJobFilesLineShowsCanceledItems(t *testing.T) {
	styles := theme.Default()
	body := tcell.StyleDefault
	for _, tc := range []struct {
		name string
		j    JobEntry
		want string
	}{
		{"copy counted", JobEntry{Type: "copy", Status: "canceled", TotalFiles: 40, TotalDirs: 6, ProcessedItems: 13, TotalsComplete: true, OpsStats: oplog.Stats{Done: 12, Failed: 1}}, "Files:       12 done, 1 failed, 21 canceled"},
		{"copy still scanning", JobEntry{Type: "copy", Status: "canceled", TotalFiles: 40, OpsStats: oplog.Stats{Done: 12}}, "Files:       12 done, canceled: incomplete scan"},
		{"delete before any row", JobEntry{Type: "delete", Status: "canceled", TotalFiles: 3}, "Files:       0 done, 3 canceled"},
		{"move", JobEntry{Type: "move", Status: "canceled", TotalFiles: 40, TotalsComplete: true, OpsStats: oplog.Stats{Done: 2}}, "Files:       2 done, canceled: incomplete scan"},
		{"flatten", JobEntry{Type: "flatten", Status: "canceled", TotalFiles: 40, TotalsComplete: true}, "Files:       0 done, canceled: unsupported"},
		{"completed", JobEntry{Type: "copy", Status: "completed", TotalFiles: 40, TotalsComplete: true, OpsStats: oplog.Stats{Done: 12}}, "Files:       12 done"},
	} {
		var text strings.Builder
		for _, p := range jobFilesLineParts(tc.j) {
			text.WriteString(p.Text)
		}
		if got := text.String(); got != tc.want {
			t.Fatalf("%s: line = %q, want %q", tc.name, got, tc.want)
		}
		if tc.j.Status != "canceled" {
			continue
		}
		spans := jobFilesLineSpans(tc.j, styles, body)
		last := spans[len(spans)-1]
		line := []rune(text.String())
		if s := string(line[last.Start:last.End]); !strings.Contains(s, "canceled") || last.Style != styles.JobsOperationOn(theme.JobsOperationSkipped, body) {
			t.Fatalf("%s: canceled span = %q, want it in the skipped (yellow) style", tc.name, s)
		}
	}
}

func TestDrawJobsOperationsPanelSourceDeletedIcon(t *testing.T) {
	screen := newOpsScreen(t)
	rect := Rect{X: 0, Y: 0, Width: 70, Height: 8}
	win := JobsOpsWindow{JobID: "move-1", Total: 2, Rows: []JobOperationRow{
		{Op: JobOperation{Result: oplog.Skipped, Label: "harbor/lantern.txt", Reason: ops.ReasonSourceDeleted, SourceDeleted: true}},
		{Op: JobOperation{Result: oplog.Skipped, Label: "harbor/meadow.txt", Reason: ops.ReasonSourceDeleted}}, // the reason text alone is not the outcome
	}}
	th := theme.Default()
	drawJobsOperationsPanel(screen, rect, JobsViewState{}, win, "", th, false, true, false)

	for i, want := range []string{th.IconJobsOperation(theme.JobsOperationSourceDeleted), th.IconJobsOperation(theme.JobsOperationSkipped)} {
		if row := screenRowText(screen, rect, rect.Y+1+i); !strings.Contains(row, want) {
			t.Fatalf("row %d = %q, want icon %q", i, row, want)
		}
	}
	if th.IconJobsOperation(theme.JobsOperationSourceDeleted) != "" {
		t.Fatalf("source deleted icon = %q", th.IconJobsOperation(theme.JobsOperationSourceDeleted))
	}
}
