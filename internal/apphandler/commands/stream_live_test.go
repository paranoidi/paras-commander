package commands

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/ui"
)

type layoutHost struct{ Host }

func (layoutHost) LayoutForTerminalSize(w, h int) ui.Layout {
	return ui.Layout{Width: w, Height: h, Secondary: ui.Rect{X: 40, Y: 1, Width: 40, Height: 24}}
}

func manyLines(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("line %d", i)
	}
	return out
}

func key(k tcell.Key) *tcell.EventKey { return tcell.NewEventKey(k, 0, 0) }

func TestOutputDialogHomeEndAndFollow(t *testing.T) {
	h, _ := newOutputDialogHandler(t)
	h.host = layoutHost{}
	h.OpenRunningOutputDialog("row1", "T", false, func() {}, "", "")
	st := &h.model.CommandOutputDialog
	if !st.Follow {
		t.Fatal("new dialog should follow")
	}
	// Live update while Running: lines arrive, scroll keys work in the Running state.
	h.ApplyWake(WakePayload{OutputDialogLive: &OutputDialogLive{RunID: "other", Stdout: "nope"}})
	if len(st.Lines) != 0 {
		t.Fatalf("non-matching live applied: %v", st.Lines)
	}
	h.ApplyWake(WakePayload{OutputDialogLive: &OutputDialogLive{RunID: "row1", Stdout: strings.Join(manyLines(50), "\n")}})
	if len(st.Lines) != 50 || !st.Running {
		t.Fatalf("live not applied: %d lines running=%v", len(st.Lines), st.Running)
	}
	h.HandleOutputDialogKey(key(tcell.KeyHome))
	if st.Scroll != 0 || st.Follow || !st.Running {
		t.Fatalf("Home: scroll=%d follow=%v running=%v", st.Scroll, st.Follow, st.Running)
	}
	h.HandleOutputDialogKey(key(tcell.KeyEnd))
	visH := h.outputDialogVisH(*st)
	if st.Scroll != 50-visH || !st.Follow {
		t.Fatalf("End: scroll=%d follow=%v want %d", st.Scroll, st.Follow, 50-visH)
	}
	h.HandleOutputDialogKey(key(tcell.KeyUp))
	if st.Follow || st.Scroll != 50-visH-1 {
		t.Fatalf("Up from tail: scroll=%d follow=%v", st.Scroll, st.Follow)
	}
	h.HandleOutputDialogKey(key(tcell.KeyPgDn))
	if !st.Follow {
		t.Fatal("PgDn to bottom should re-follow")
	}
}

func TestOutputDialogFinalResultKeepsScroll(t *testing.T) {
	h, _ := newOutputDialogHandler(t)
	h.host = &bannerHost{}
	h.OpenRunningOutputDialog("row1", "T", false, func() {}, "", "")
	h.model.CommandOutputDialog.Scroll, h.model.CommandOutputDialog.Follow = 3, false
	res := OutputDialogResult("row1", "T", cmdrun.RunResult{Stdout: []byte("a\nb")}, "", "", "")
	h.ApplyWake(WakePayload{OpenOutputDialog: &res})
	if st := h.model.CommandOutputDialog; st.Running || st.Scroll != 3 || st.Follow {
		t.Fatalf("after final: %+v", st)
	}
}

func TestScrollPaneHomeEndFollow(t *testing.T) {
	h := &Handler{}
	scroll, detached := 0, false
	h.scrollPane(&scroll, &detached, 20, tcell.KeyUp)
	if scroll != 19 || !detached {
		t.Fatalf("Up while following: scroll=%d detached=%v", scroll, detached)
	}
	h.scrollPane(&scroll, &detached, 20, tcell.KeyHome)
	if scroll != 0 || !detached {
		t.Fatalf("Home: %d %v", scroll, detached)
	}
	h.scrollPane(&scroll, &detached, 20, tcell.KeyEnd)
	if scroll != 20 || detached {
		t.Fatalf("End: %d %v", scroll, detached)
	}
	// Output grows while following: still anchored to the new bottom.
	h.scrollPane(&scroll, &detached, 30, tcell.KeyUp)
	if scroll != 29 {
		t.Fatalf("Up after growth: %d", scroll)
	}
}

func TestStreamLivePatchesEntryAndStopWaits(t *testing.T) {
	h, _ := newOutputDialogHandler(t)
	h.mu = &sync.RWMutex{}
	idx := h.AppendEntry(ui.CommandRunEntry{ID: "row1"})
	live := &cmdrun.Live{}
	live.Stdout.SetMaxIfZero(1024)
	stop := h.StreamLive(context.Background(), idx, "row1", live)
	_, _ = live.Stdout.Write([]byte("tick\n"))
	deadline := time.Now().Add(3 * time.Second)
	for {
		h.mu.RLock()
		got := h.model.CommandsList[idx].Stdout
		h.mu.RUnlock()
		if got == "tick\n" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("entry never received live stdout")
		}
		time.Sleep(10 * time.Millisecond)
	}
	stop() // returns only after the goroutine exited
}
