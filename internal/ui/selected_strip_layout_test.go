package ui

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestSelectedStripLayoutItemCount(t *testing.T) {
	st := &panel.State{Path: pathloc.MustParse("/cwd/here")}
	st.SelectedPaths = map[string]bool{"/other/a": true}
	st.SelectedStripOrder = []string{"/other/a"}
	if got := SelectedStripLayoutItemCount(st, PrimaryPanel, SecondaryPanel); got != 0 {
		t.Fatalf("inactive left with right active: got %d want 0", got)
	}
	if got := SelectedStripLayoutItemCount(st, SecondaryPanel, SecondaryPanel); got != 1 {
		t.Fatalf("active right: got %d want 1", got)
	}
	if got := SelectedStripLayoutItemCount(st, PrimaryPanel, PrimaryPanel); got != 1 {
		t.Fatalf("active left: got %d want 1", got)
	}
	empty := &panel.State{}
	if got := SelectedStripLayoutItemCount(empty, PrimaryPanel, PrimaryPanel); got != 0 {
		t.Fatalf("no strip items: got %d want 0", got)
	}
}
