package find

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func TestFindDepthInputKeys(t *testing.T) {
	t.Parallel()
	h := newTestFindHandler(&fakeFindHost{}, &ui.Model{})
	st := &h.model.FindDialog
	for _, tc := range []struct {
		ev   *tcell.EventKey
		want string
	}{
		{tcell.NewEventKey(tcell.KeyRune, '3', tcell.ModNone), "3"},
		{tcell.NewEventKey(tcell.KeyRune, '7', tcell.ModNone), "7"},
		{tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone), "7"},
		{tcell.NewEventKey(tcell.KeyRune, '0', tcell.ModNone), ""},
		{tcell.NewEventKey(tcell.KeyRune, '5', tcell.ModNone), "5"},
		{tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone), ""},
	} {
		h.handleFindDepthKey(tc.ev)
		if st.DepthInput != tc.want {
			t.Fatalf("after %v: DepthInput = %q, want %q", tc.ev.Name(), st.DepthInput, tc.want)
		}
	}
}

func TestFindDepthKeyCommitsAndFocusesQuery(t *testing.T) {
	t.Parallel()
	h := newTestFindHandler(&fakeFindHost{}, &ui.Model{})
	st := &h.model.FindDialog
	st.Focus = dialog.FindDepth
	h.handleFindDepthKey(tcell.NewEventKey(tcell.KeyRune, '3', tcell.ModNone))
	if st.MaxDepth != 3 || st.Focus != dialog.FindQuery {
		t.Fatalf("MaxDepth = %d, Focus = %q; want 3, query", st.MaxDepth, st.Focus)
	}
	st.Focus = dialog.FindDepth
	h.handleFindDepthKey(tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone))
	if st.MaxDepth != 0 || st.Focus != dialog.FindQuery {
		t.Fatalf("after Backspace: MaxDepth = %d, Focus = %q; want 0, query", st.MaxDepth, st.Focus)
	}
}

func TestCommitFindDepthLowerPrunesRaiseRestarts(t *testing.T) {
	t.Parallel()
	h := newTestFindHandler(&fakeFindHost{}, &ui.Model{})
	st := &h.model.FindDialog
	st.IndexDone = true

	st.DepthInput = "4"
	h.commitFindDepth() // unlimited -> 4: tighter
	if st.MaxDepth != 4 || h.scanGen != 0 || st.IndexDone {
		t.Fatalf("tighten from unlimited: MaxDepth=%d scanGen=%d IndexDone=%v", st.MaxDepth, h.scanGen, st.IndexDone)
	}
	st.DepthInput = "2"
	h.commitFindDepth()
	if st.MaxDepth != 2 || h.scanGen != 0 {
		t.Fatalf("tighten: MaxDepth=%d scanGen=%d", st.MaxDepth, h.scanGen)
	}
	st.DepthInput = "2"
	h.commitFindDepth() // unchanged
	if h.scanGen != 0 {
		t.Fatalf("unchanged restarted the indexer (scanGen=%d)", h.scanGen)
	}
	st.DepthInput = "6"
	h.commitFindDepth()
	if st.MaxDepth != 6 || h.scanGen != 1 {
		t.Fatalf("raise: MaxDepth=%d scanGen=%d, want restart", st.MaxDepth, h.scanGen)
	}
	st.DepthInput = ""
	h.commitFindDepth()
	if st.MaxDepth != 0 || h.scanGen != 2 {
		t.Fatalf("unlimited: MaxDepth=%d scanGen=%d, want restart", st.MaxDepth, h.scanGen)
	}
}
