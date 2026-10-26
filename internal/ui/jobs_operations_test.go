package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func opLabels(ops []JobOperation) []string {
	out := make([]string, len(ops))
	for i, o := range ops {
		out[i] = o.Label
		if o.Failed {
			out[i] += "!"
		}
	}
	return out
}

func TestAppendJobOperationDedupesAndReplaces(t *testing.T) {
	var ops []JobOperation
	ops = AppendJobOperation(ops, JobOperation{Label: "lantern.txt"})
	ops = AppendJobOperation(ops, JobOperation{Label: "lantern.txt"}) // repeated progress sample
	ops = AppendJobOperation(ops, JobOperation{Label: "orchard.bin"})
	ops = AppendJobOperation(ops, JobOperation{Label: "lantern.txt", Failed: true, Error: "permission denied"})
	if got, want := strings.Join(opLabels(ops), ","), "orchard.bin,lantern.txt!"; got != want {
		t.Fatalf("after failure = %s, want %s", got, want)
	}
	ops = AppendJobOperation(ops, JobOperation{Label: "lantern.txt"}) // retry succeeded
	if got, want := strings.Join(opLabels(ops), ","), "orchard.bin,lantern.txt"; got != want {
		t.Fatalf("after retry = %s, want %s", got, want)
	}
}

func TestAppendJobOperationCapKeepsFailures(t *testing.T) {
	var ops []JobOperation
	ops = AppendJobOperation(ops, JobOperation{Label: "meadow.sock", Failed: true, Error: "unsupported"})
	for i := 0; i < jobOperationsMaxRows+10; i++ {
		ops = AppendJobOperation(ops, JobOperation{Label: fmt.Sprintf("pebble-%03d.txt", i)})
	}
	if len(ops) != jobOperationsMaxRows {
		t.Fatalf("len = %d, want %d", len(ops), jobOperationsMaxRows)
	}
	if !ops[0].Failed || ops[0].Label != "meadow.sock" {
		t.Fatalf("oldest row = %+v, want the kept failure", ops[0])
	}
	if last := ops[len(ops)-1].Label; last != fmt.Sprintf("pebble-%03d.txt", jobOperationsMaxRows+9) {
		t.Fatalf("newest row = %q", last)
	}
}

func TestVisibleJobOperationsNewestFirstAndFiltered(t *testing.T) {
	ops := []JobOperation{
		{Label: "harbor/anchor.txt"},
		{Label: "harbor/beacon.log", Failed: true, Error: "input/output error"},
		{Label: "valley/anchor.md"},
	}
	labels := func(rows []JobOperationRow) string {
		var out []string
		for _, r := range rows {
			out = append(out, r.Op.Label)
		}
		return strings.Join(out, ",")
	}
	var st JobsViewState
	if got, want := labels(VisibleJobOperations(ops, st, true)), "valley/anchor.md,harbor/beacon.log,harbor/anchor.txt"; got != want {
		t.Fatalf("all = %s, want %s", got, want)
	}
	st.OpsResultHidden = dialog.JobResultSuccess
	if got, want := labels(VisibleJobOperations(ops, st, true)), "harbor/beacon.log"; got != want {
		t.Fatalf("failed only = %s, want %s", got, want)
	}
	st.OpsResultHidden = dialog.JobResultNone
	st.OpsFilter.Query = "anchr"
	rows := VisibleJobOperations(ops, st, true)
	if got, want := labels(rows), "valley/anchor.md,harbor/anchor.txt"; got != want {
		t.Fatalf("fuzzy = %s, want %s", got, want)
	}
	if len(rows[0].Ranges) == 0 {
		t.Fatal("fuzzy match ranges missing")
	}
}

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

func TestDrawJobsOperationsPanelBordersAndOrder(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(70, 10)
	rect := Rect{X: 0, Y: 0, Width: 70, Height: 8}
	ops := []JobOperation{
		{Label: "quarry/first.txt"},
		{Label: "quarry/second.txt", Failed: true, Error: "permission denied"},
		{Label: "quarry/third.txt", Failed: true, Error: "unsupported file type"},
		{Label: "quarry/fourth.txt"},
	}
	st := JobsViewState{OpsResultHidden: dialog.JobResultSuccess}
	st.OpsFilter.Query = "quarry"
	drawJobsOperationsPanel(screen, rect, st, ops, true, theme.Default(), false, true, false)

	top := screenRowText(screen, rect, rect.Y)
	if !strings.Contains(top, "Operations") || !strings.Contains(top, "2 failures") {
		t.Fatalf("top border = %q, want title and failure count", top)
	}
	if first := screenRowText(screen, rect, rect.Y+1); !strings.Contains(first, "quarry/third.txt: unsupported file type") {
		t.Fatalf("first row = %q, want the newest failure", first)
	}
	if second := screenRowText(screen, rect, rect.Y+2); !strings.Contains(second, "quarry/second.txt") {
		t.Fatalf("second row = %q", second)
	}
	if third := screenRowText(screen, rect, rect.Y+3); strings.Contains(third, "quarry") {
		t.Fatalf("third row = %q, want successes hidden", third)
	}
	if bottom := screenRowText(screen, rect, rect.Y+rect.Height-1); !strings.Contains(bottom, "Failed · > quarry") {
		t.Fatalf("bottom border = %q, want the active filters", bottom)
	}
}

func TestFitJobOperationKeepsLabelAndError(t *testing.T) {
	op := JobOperation{Label: "harbor/locked.dat", Failed: true, Error: "open /srv/harbor/locked.dat: permission denied"}
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
		t.Fatalf("narrow text %q: want the basename kept, then the error start", text)
	}
}
