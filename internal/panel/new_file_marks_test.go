package panel

import (
	"os"
	"testing"

	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panellist"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func TestNewFileMarkTierAndDropOnLeave(t *testing.T) {
	t.Parallel()
	destDir := t.TempDir()
	dest := pathloc.MustParse(destDir)
	s := State{Path: dest}
	s.AddNewFileMarks(dest, []string{"alpha.txt"})
	ent := localfs.Entry{Name: "alpha.txt", Path: destDir + "/alpha.txt", Type: localfs.EntryFile}
	if got := s.NewFileMarkTier(ent); got != panellist.NewFileMarkLatest {
		t.Fatalf("tier = %v, want latest", got)
	}
	other := pathloc.MustParse(t.TempDir())
	if err := s.load(other, "", 10, noIndexCursorFallback, remoteLoadOpts{}); err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := s.NewFileMarkTier(ent); got != panellist.NewFileMarkNone {
		t.Fatal("marks should not apply after leaving dest directory")
	}
	if err := s.load(dest, "", 10, noIndexCursorFallback, remoteLoadOpts{}); err != nil {
		t.Fatalf("reload dest: %v", err)
	}
	if got := s.NewFileMarkTier(ent); got != panellist.NewFileMarkNone {
		t.Fatal("marks should be cleared when leaving dest and not restored on return")
	}
}

func TestNewFileMarkTierPromotesPreviousBatch(t *testing.T) {
	t.Parallel()
	dest := pathloc.MustParse(t.TempDir())
	s := State{Path: dest}
	s.AddNewFileMarks(dest, []string{"first.txt", "second.txt"})
	s.AddNewFileMarks(dest, []string{"third.txt"})

	first := localfs.Entry{Name: "first.txt", Type: localfs.EntryFile}
	second := localfs.Entry{Name: "second.txt", Type: localfs.EntryFile}
	third := localfs.Entry{Name: "third.txt", Type: localfs.EntryFile}

	if got := s.NewFileMarkTier(first); got != panellist.NewFileMarkPrevious {
		t.Fatalf("first tier = %v, want previous", got)
	}
	if got := s.NewFileMarkTier(second); got != panellist.NewFileMarkPrevious {
		t.Fatalf("second tier = %v, want previous", got)
	}
	if got := s.NewFileMarkTier(third); got != panellist.NewFileMarkLatest {
		t.Fatalf("third tier = %v, want latest", got)
	}
}

func TestApplyPeriodicRefreshMarksExternallyCreatedFileAsNew(t *testing.T) {
	t.Parallel()
	loc := pathloc.MustParse(t.TempDir())
	s := State{
		Path:    loc,
		Entries: []localfs.Entry{{Name: "alpha.txt", Type: localfs.EntryFile}},
		Sort:    defaultSortState(),
	}
	fresh := []fsbackend.Entry{
		{Name: "alpha.txt", Type: fsbackend.EntryFile},
		{Name: "gamma.txt", Type: fsbackend.EntryFile},
	}
	if _, err := s.ApplyPeriodicRefresh(loc, fresh, 5); err != nil {
		t.Fatalf("ApplyPeriodicRefresh: %v", err)
	}
	gamma := localfs.Entry{Name: "gamma.txt", Type: localfs.EntryFile}
	if got := s.NewFileMarkTier(gamma); got != panellist.NewFileMarkLatest {
		t.Fatalf("gamma tier = %v, want latest", got)
	}
	alpha := localfs.Entry{Name: "alpha.txt", Type: localfs.EntryFile}
	if got := s.NewFileMarkTier(alpha); got != panellist.NewFileMarkNone {
		t.Fatalf("alpha tier = %v, want none (pre-existing file)", got)
	}
}

func TestLoadIntoDirectoryDoesNotMarkExistingEntriesAsNew(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/preexisting.txt", nil, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	loc := pathloc.MustParse(dir)
	s := State{Path: pathloc.MustParse(t.TempDir()), Sort: defaultSortState()}
	if err := s.load(loc, "", 10, noIndexCursorFallback, remoteLoadOpts{}); err != nil {
		t.Fatalf("load: %v", err)
	}
	ent := localfs.Entry{Name: "preexisting.txt", Type: localfs.EntryFile}
	if got := s.NewFileMarkTier(ent); got != panellist.NewFileMarkNone {
		t.Fatalf("tier = %v, want none for file present on first navigation", got)
	}
}

func TestNewFileMarkTierRecopyPromotesToLatest(t *testing.T) {
	t.Parallel()
	dest := pathloc.MustParse(t.TempDir())
	s := State{Path: dest}
	s.AddNewFileMarks(dest, []string{"alpha.txt"})
	s.AddNewFileMarks(dest, []string{"beta.txt"})
	s.AddNewFileMarks(dest, []string{"alpha.txt"})

	alpha := localfs.Entry{Name: "alpha.txt", Type: localfs.EntryFile}
	beta := localfs.Entry{Name: "beta.txt", Type: localfs.EntryFile}

	if got := s.NewFileMarkTier(alpha); got != panellist.NewFileMarkLatest {
		t.Fatalf("alpha tier = %v, want latest after recopy", got)
	}
	if got := s.NewFileMarkTier(beta); got != panellist.NewFileMarkPrevious {
		t.Fatalf("beta tier = %v, want previous", got)
	}
}
