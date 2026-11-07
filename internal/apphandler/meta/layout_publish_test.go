package meta

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/metacmds"
	"github.com/paranoidi/paras-commander/internal/ui"
)

const wideValue = "copper-lantern" // 14 cells

func drawnWidth(h *Handler) int {
	layouts, _ := ui.LayoutMetaColumns(h.model.MetaResults[0])
	return layouts[0].Width
}

func rerun(h *Handler) {
	h.runForPanel(0, []metacmds.MetaEntry{{Name: "words", File: "sleep 30"}}, []ui.MetaColumnState{{EntryName: "words"}})
}

func resolve(h *Handler, value string, paths ...string) {
	for _, path := range paths {
		h.HandleWake(WakePayload{PanelID: 0, EntryName: "words", Path: path, Value: value, Gen: h.runGen[0]})
	}
}

func TestLayout_heldUntilFlushWhileResultsStayLive(t *testing.T) {
	h, cedar, _ := pendingRevealHandler(t, 0)
	before := drawnWidth(h)

	resolve(h, wideValue, cedar)
	if !h.renderDebounce[0].Armed() {
		t.Fatal("the run's hold should still be armed")
	}
	if got := h.model.MetaResults[0][0].Results[cedar]; got != wideValue {
		t.Fatalf("Results[cedar] = %q, sort and filters need the live value", got)
	}
	if got := drawnWidth(h); got != before {
		t.Fatalf("width moved to %d inside the hold, want %d", got, before)
	}

	h.HandleRenderFlush(RenderFlushPayload{})
	layouts, _ := ui.LayoutMetaColumns(h.model.MetaResults[0])
	if layouts[0].Width != len(wideValue) || strings.TrimSpace(layouts[0].Formatted[cedar]) != wideValue {
		t.Fatalf("after flush: width %d, cell %q", layouts[0].Width, layouts[0].Formatted[cedar])
	}
}

func TestLayout_runStartsWithPendingCellsPublished(t *testing.T) {
	h, cedar, _ := pendingRevealHandler(t, 0)
	layouts, _ := ui.LayoutMetaColumns(h.model.MetaResults[0])
	if got := layouts[0].Formatted[cedar]; got != h.host.IconMetaRunning() {
		t.Fatalf("cell = %q, want the running marker from the first draw", got)
	}
}

func TestLayout_lastResultFlushesAtOnce(t *testing.T) {
	h, cedar, willow := pendingRevealHandler(t, 0)
	resolve(h, wideValue, cedar, willow)

	screen, ok := h.screen.(tcell.SimulationScreen)
	if !ok {
		t.Fatal("not a simulation screen")
	}
	deadline := time.Now().Add(config.MetaLayoutInterval / 2)
	for time.Now().Before(deadline) {
		for screen.HasPendingEvent() {
			if ie, ok := screen.PollEvent().(*tcell.EventInterrupt); ok {
				if _, ok := ie.Data().(RenderFlushPayload); ok {
					return
				}
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("no flush posted well before the interval")
}

func TestLayout_panelsPaceIndependently(t *testing.T) {
	h, cedar, _ := pendingRevealHandler(t, 0)
	if h.renderDebounce[1].Armed() {
		t.Fatal("a run on one panel armed the other's timer")
	}
	before := drawnWidth(h)
	resolve(h, wideValue, cedar)
	h.HandleRenderFlush(RenderFlushPayload{PanelID: 1})
	if got := drawnWidth(h); got != before {
		t.Fatalf("the other panel's flush published this one: width %d, want %d", got, before)
	}
}

func TestLayoutGuess_refreshKeepsWidthUntilResolved(t *testing.T) {
	h, cedar, willow := pendingRevealHandler(t, 0)
	resolve(h, wideValue, cedar, willow)
	h.HandleRenderFlush(RenderFlushPayload{})
	if got := drawnWidth(h); got != len(wideValue) {
		t.Fatalf("width = %d, want %d", got, len(wideValue))
	}

	rerun(h)
	if got := drawnWidth(h); got != len(wideValue) {
		t.Fatalf("rerun collapsed the width to %d", got)
	}
	resolve(h, "tin", cedar)
	h.HandleRenderFlush(RenderFlushPayload{})
	if got := drawnWidth(h); got != len(wideValue) {
		t.Fatalf("width = %d with a cell pending, want the guess", got)
	}
	resolve(h, "tin", willow)
	h.HandleRenderFlush(RenderFlushPayload{})
	if got := drawnWidth(h); got != panelMinMetaWidth(t) {
		t.Fatalf("width = %d, want the real width once resolved", got)
	}
}

func TestLayoutGuess_keepsTextAlignmentWhilePending(t *testing.T) {
	h, cedar, willow := pendingRevealHandler(t, 0)
	resolve(h, wideValue, cedar, willow)
	h.HandleRenderFlush(RenderFlushPayload{})
	drawnWidth(h)

	rerun(h)
	if layouts, _ := ui.LayoutMetaColumns(h.model.MetaResults[0]); layouts[0].RightAlign {
		t.Fatal("a text column's header jumped right for the pending cells of a rerun")
	}
}

func TestLayoutGuess_survivesListingWithoutValues(t *testing.T) {
	h, cedar, willow := pendingRevealHandler(t, 0)
	resolve(h, wideValue, cedar, willow)
	h.HandleRenderFlush(RenderFlushPayload{})
	drawnWidth(h)

	p := h.host.PanelByID(0)
	entries := p.Entries
	p.Entries = nil
	rerun(h) // an empty directory: nothing pending, so no floor
	if got := drawnWidth(h); got == len(wideValue) {
		t.Fatalf("width = %d in an empty listing, want no guess held", got)
	}

	p.Entries = entries
	rerun(h)
	if got := drawnWidth(h); got != len(wideValue) {
		t.Fatalf("width = %d, want the guess from before the empty listing", got)
	}
}

// panelMinMetaWidth is the width of a column with short values and no title.
func panelMinMetaWidth(t *testing.T) int {
	t.Helper()
	layouts, _ := ui.LayoutMetaColumns([]ui.MetaColumnState{{Results: map[string]string{"/a": "tin"}}})
	return layouts[0].Width
}
