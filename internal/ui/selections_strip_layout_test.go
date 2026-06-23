package ui

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestSelectionsStripLayoutItemCount(t *testing.T) {
	st := &panel.State{Path: pathloc.MustParse("/cwd/here")}
	st.SelectedPaths = map[string]bool{"/other/a": true}
	st.SelectionsStripOrder = []string{"/other/a"}
	if got := SelectionsStripLayoutItemCount(st, PrimaryPanel, SecondaryPanel, false); got != 0 {
		t.Fatalf("inactive left with right active: got %d want 0", got)
	}
	if got := SelectionsStripLayoutItemCount(st, SecondaryPanel, SecondaryPanel, false); got != 1 {
		t.Fatalf("active right: got %d want 1", got)
	}
	if got := SelectionsStripLayoutItemCount(st, PrimaryPanel, PrimaryPanel, false); got != 1 {
		t.Fatalf("active left: got %d want 1", got)
	}
	// Theme picker: left column keeps strip for preview even when right is active.
	if got := SelectionsStripLayoutItemCount(st, PrimaryPanel, SecondaryPanel, true); got != 1 {
		t.Fatalf("theme preview left inactive-but-previewed: got %d want 1", got)
	}
	if got := SelectionsStripLayoutItemCount(st, SecondaryPanel, SecondaryPanel, true); got != 1 {
		t.Fatalf("theme preview right active: got %d want 1", got)
	}
	empty := &panel.State{}
	if got := SelectionsStripLayoutItemCount(empty, PrimaryPanel, PrimaryPanel, false); got != 0 {
		t.Fatalf("no strip items: got %d want 0", got)
	}
}
