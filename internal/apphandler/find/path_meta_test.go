package find

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func TestLookupFindPathMetaDoesNotBlockOnStat(t *testing.T) {
	blocked := make(chan struct{})
	findPathStat = func(string) (os.FileInfo, error) {
		<-blocked
		return nil, os.ErrNotExist
	}
	defer func() {
		close(blocked)
		findPathStat = os.Stat
	}()

	root := t.TempDir()
	st := &dialog.FindDialogState{
		RootPath: root,
		Entries:  []dialog.FindEntry{{RelLine: "willow.txt"}},
	}
	st.RebuildPathIndex()
	abs := filepath.Clean(filepath.Join(root, "willow.txt"))

	done := make(chan struct{})
	go func() {
		_, _, _ = lookupFindPathMeta(st, abs)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("lookupFindPathMeta blocked on Stat")
	}
}

func TestFindToggleSelectionDoesNotWaitOnStat(t *testing.T) {
	blocked := make(chan struct{})
	findPathStat = func(string) (os.FileInfo, error) {
		<-blocked
		return nil, os.ErrNotExist
	}
	defer func() {
		close(blocked)
		findPathStat = os.Stat
	}()

	root := t.TempDir()
	host := &fakeFindHost{}
	model := &ui.Model{
		FindDialog: dialog.FindDialogState{
			Open:     true,
			RootPath: root,
			Focus:    0,
			Entries:  []dialog.FindEntry{{RelLine: "cedar.txt"}},
			Ranked:   []int{0},
			Selected: 0,
		},
	}
	h := newTestFindHandler(host, model)
	st := &h.model.FindDialog
	st.RebuildPathIndex()
	h.bindFindDialogPathMeta(st)

	done := make(chan struct{})
	go func() {
		h.findDialogToggleSelectionAndAdvance()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("find selection blocked on Stat")
	}
}

func TestFindMarkedFileSizeFillsInBackground(t *testing.T) {
	root := t.TempDir()
	name := "maple.txt"
	abs := filepath.Join(root, name)
	content := []byte("willow-cedar-maple")
	if err := os.WriteFile(abs, content, 0o644); err != nil {
		t.Fatal(err)
	}
	host := &fakeFindHost{}
	model := &ui.Model{
		FindDialog: dialog.FindDialogState{
			Open:     true,
			RootPath: root,
			Entries:  []dialog.FindEntry{{RelLine: name}},
			MarkedPaths: map[string]bool{
				filepath.Clean(abs): true,
			},
		},
	}
	h := newTestFindHandler(host, model)
	st := &h.model.FindDialog
	st.RebuildPathIndex()
	h.bindFindDialogPathMeta(st)
	st.InvalidateMarkedSelectionDerived()
	h.scheduleFindMarkedFileSizes(st)

	deadline := time.Now().Add(2 * time.Second)
	for {
		h.sizeMu.Lock()
		upd := h.pendingFileSizes
		h.sizeMu.Unlock()
		if upd != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("background size lookup did not finish")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !h.ApplyPendingRank() {
		t.Fatal("ApplyPendingRank should apply file sizes")
	}
	if st.Entries[0].Size != int64(len(content)) {
		t.Fatalf("Size = %d, want %d", st.Entries[0].Size, len(content))
	}
	_, size, ok := lookupFindPathMeta(st, abs)
	if !ok || size != int64(len(content)) {
		t.Fatalf("PathMeta size = %d ok=%v, want %d", size, ok, len(content))
	}
}
