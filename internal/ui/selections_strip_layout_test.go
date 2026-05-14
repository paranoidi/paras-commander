package ui

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/panel"
)

func TestSelectionsStripLayoutItemCount(t *testing.T) {
	st := &panel.State{Path: "/cwd/here"}
	st.SelectedPaths = map[string]bool{"/other/a": true}
	st.SelectionsStripOrder = []string{"/other/a"}
	if got := SelectionsStripLayoutItemCount(st, LeftPanel, RightPanel, false); got != 0 {
		t.Fatalf("inactive left with right active: got %d want 0", got)
	}
	if got := SelectionsStripLayoutItemCount(st, RightPanel, RightPanel, false); got != 1 {
		t.Fatalf("active right: got %d want 1", got)
	}
	if got := SelectionsStripLayoutItemCount(st, LeftPanel, LeftPanel, false); got != 1 {
		t.Fatalf("active left: got %d want 1", got)
	}
	// Theme picker: left column keeps strip for preview even when right is active.
	if got := SelectionsStripLayoutItemCount(st, LeftPanel, RightPanel, true); got != 1 {
		t.Fatalf("theme preview left inactive-but-previewed: got %d want 1", got)
	}
	if got := SelectionsStripLayoutItemCount(st, RightPanel, RightPanel, true); got != 1 {
		t.Fatalf("theme preview right active: got %d want 1", got)
	}
	empty := &panel.State{}
	if got := SelectionsStripLayoutItemCount(empty, LeftPanel, LeftPanel, false); got != 0 {
		t.Fatalf("no strip items: got %d want 0", got)
	}
}
