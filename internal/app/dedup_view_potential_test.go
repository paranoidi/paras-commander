package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

type nopTaskReporter struct{}

func (nopTaskReporter) Progress(jobs.TaskProgress) {}
func (nopTaskReporter) Warn(string)                {}
func (nopTaskReporter) FinishSource(int)           {}

// The action hashes the potential files in a job and the view refreshes itself: the real
// duplicates become a confirmed group, the file that differs leaves, and a mark survives.
func TestDedupHashPotentialRefreshesView(t *testing.T) {
	a, screen, dir, _ := fileIndexApp(t)
	root := filepath.Join(dir, "reed")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, tail := range map[string]byte{"anchor.bin": 1, "lantern.bin": 1, "willow.bin": 2} {
		b := make([]byte, 9000)
		b[8999] = tail
		if err := os.WriteFile(filepath.Join(root, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.fileIndex.ScanFastTask([]string{root}).Run(context.Background(), nopTaskReporter{}); err != nil {
		t.Fatal(err)
	}
	loc, err := pathloc.FileIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	a.activePanel().Path = loc
	a.dedupCtrl.Open()
	drainInterruptEventsUntil(t, a, screen, 5*time.Second, func() bool {
		return a.model.DedupSnapshot.Phase == comparepkg.DedupDone && len(a.model.DedupSnapshot.Groups) == 1
	})
	if g := a.model.DedupSnapshot.Groups[0]; !g.Potential || len(g.Files) != 3 {
		t.Fatalf("group before hashing = %+v", g)
	}
	anchor, err := pathloc.FileIndex(filepath.Join(root, "anchor.bin"))
	if err != nil {
		t.Fatal(err)
	}
	marked := anchor.String()
	a.model.DedupView.Marked[marked] = true
	a.model.DedupView.GroupFilter = func(comparepkg.DedupGroup) bool { return true }
	a.model.DedupView.NameFilterLabel = "reed"
	if !a.tryDispatchDedup(keymap.ActionDedupHashPotential) {
		t.Fatal("action not handled")
	}
	drainInterruptEventsUntil(t, a, screen, 10*time.Second, func() bool {
		s := a.model.DedupSnapshot
		return s.Phase == comparepkg.DedupDone && len(s.Groups) == 1 && !s.Groups[0].Potential
	})
	if g := a.model.DedupSnapshot.Groups[0]; len(g.Files) != 2 {
		t.Fatalf("group after hashing = %+v, want the two identical files", g)
	}
	if !a.model.DedupView.Marked[marked] {
		t.Fatalf("mark lost: %v", a.model.DedupView.Marked)
	}
	if a.model.DedupView.GroupFilter == nil || a.model.DedupView.NameFilterLabel != "reed" {
		t.Fatal("group filter lost")
	}
}

// The delete dialog warns only when a marked file is in a potential duplicate group.
func TestDedupDeleteDialogWarnsAboutPotentialDuplicates(t *testing.T) {
	file := func(p string) comparepkg.DedupFile {
		return comparepkg.DedupFile{Rel: p[1:], Abs: pathloc.MustParse(p)}
	}
	a, _, _, _ := fileIndexApp(t)
	a.model.DedupSnapshot = comparepkg.DedupSnapshot{Groups: []comparepkg.DedupGroup{
		{Size: 9000, Files: []comparepkg.DedupFile{file("/lantern.bin"), file("/harbor.bin")}},
		{Size: 9000, Potential: true, Files: []comparepkg.DedupFile{file("/meadow.bin"), file("/willow.bin")}},
	}}
	a.model.DedupView.Marked = map[string]bool{"/lantern.bin": true}
	a.openDedupDeleteDialog()
	if a.model.FileDialog.Note != "" {
		t.Fatalf("note = %q with only a confirmed duplicate marked", a.model.FileDialog.Note)
	}
	a.model.DedupView.Marked["/willow.bin"] = true
	a.openDedupDeleteDialog()
	if a.model.FileDialog.Note != dedupPotentialDeleteNote {
		t.Fatalf("note = %q, want the potential duplicate warning", a.model.FileDialog.Note)
	}
}
