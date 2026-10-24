package dialog

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

func TestSortDialogViewShape(t *testing.T) {
	noMeta := SortDialogView(SortDialogState{})
	if got := len(noMeta.Groups[0].Columns); got != 1 {
		t.Fatalf("columns without meta = %d, want 1", got)
	}
	st := SortDialogState{MetaRadios: sortMetaRadios(4)}
	st.MetaRadios = append(st.MetaRadios, SortDialogMetaRadio{Title: "overflow", Name: "overflow"})
	v := SortDialogView(st)
	if got := len(v.Groups[0].Columns); got != 2 {
		t.Fatalf("columns with meta = %d, want 2", got)
	}
	if got := len(v.Groups[0].Columns[1]); got != len(panel.SortDialogRadios()) {
		t.Fatalf("meta items = %d, want capped at %d", got, len(panel.SortDialogRadios()))
	}
	if len(v.Groups) != 2 || len(v.Groups[1].Columns[0]) != 3 || len(v.Buttons) != 2 {
		t.Fatalf("unexpected groups/buttons: %+v", v)
	}
}

func TestSortDialogApply(t *testing.T) {
	st := SortDialogState{MetaRadios: sortMetaRadios(2)}
	st.Apply(dlg.Event{ID: "mode:Size"})
	if st.SortMode != panel.SortSize {
		t.Fatalf("SortMode = %v, want Size", st.SortMode)
	}
	st.Apply(dlg.Event{ID: "meta:bitrate"})
	if st.SortMode != panel.SortMeta || st.MetaColumn != "bitrate" {
		t.Fatalf("meta pick = %v/%q", st.SortMode, st.MetaColumn)
	}
	st.Apply(dlg.Event{ID: "opt:reverse"})
	st.Apply(dlg.Event{ID: "opt:dirs-first"})
	st.Apply(dlg.Event{ID: "opt:disk-usage"})
	st.Apply(dlg.Event{ID: "opt:disk-usage"})
	if !st.SortReverse || !st.DirectoriesFirst || st.DiskUsageIdleSizeSort {
		t.Fatalf("toggles = %+v", st)
	}
	before := st
	st.Apply(dlg.Event{ID: dlg.OK})
	st.Apply(dlg.Event{})
	if st.SortMode != before.SortMode || st.MetaColumn != before.MetaColumn {
		t.Fatal("OK / empty events must not change the state")
	}
}

// TestSortDialogTwoColumnGrid pins the grid navigation through the real spec: Right on a row
// with no meta radio stays put, Down from the bottom of either column reaches the checkboxes.
func TestSortDialogTwoColumnGrid(t *testing.T) {
	spec := SortDialogView(SortDialogState{MetaRadios: sortMetaRadios(2)})
	focus := dlg.ID("mode:Size")
	step := func(k tcell.Key, want dlg.ID) {
		t.Helper()
		if _, ok := dlg.HandleKey(spec, &focus, tcell.NewEventKey(k, 0, tcell.ModNone)); !ok || focus != want {
			t.Fatalf("key %v: focus %q ok=%v, want %q", k, focus, ok, want)
		}
	}
	step(tcell.KeyRight, "mode:Size")
	step(tcell.KeyUp, "mode:Extension")
	step(tcell.KeyRight, "meta:bitrate")
	step(tcell.KeyDown, "opt:disk-usage")
	step(tcell.KeyUp, "mode:Modify time")
}
