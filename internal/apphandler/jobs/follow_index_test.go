package jobs

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// activePanelHost serves a fixed active panel.
type activePanelHost struct {
	jobsHostStub
	active *panel.State
}

func (h activePanelHost) ActivePanel() *panel.State { return h.active }

// followHandler returns a handler whose active panel shows path (browsing with fileindex when
// withIndex), and the state its jobs land in.
func followHandler(t *testing.T, path string, withIndex bool) (*Handler, *jobs.State) {
	t.Helper()
	state := jobs.NewState()
	p := &panel.State{Path: pathloc.MustParse(path), WithIndex: withIndex}
	h := New(Deps{Host: activePanelHost{active: p}, Model: &ui.Model{}, State: state, Config: config.Default()})
	return h, state
}

func onlyJob(t *testing.T, s *jobs.State) *jobs.Job {
	t.Helper()
	all := s.AllJobs()
	if len(all) != 1 {
		t.Fatalf("jobs = %d, want 1", len(all))
	}
	return all[0]
}

func TestTransferJobTranslatesIndexPathsAndFollows(t *testing.T) {
	cases := []struct {
		name      string
		panel     string
		withIndex bool
		src, dest string
		follow    bool
	}{
		{"fileindex panel", "fileindex:///orchard", false, "fileindex:///orchard/harbor.txt", "/meadow", true},
		{"browse with fileindex", "/orchard", true, "/orchard/harbor.txt", "/meadow", true},
		{"index destination from a local panel", "/orchard", false, "/orchard/harbor.txt", "fileindex:///meadow", true},
		{"normal local panel", "/orchard", false, "/orchard/harbor.txt", "/meadow", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h, state := followHandler(t, c.panel, c.withIndex)
			h.AddTransferJob(TransferJobRequest{Type: jobs.TypeMove, Sources: []string{c.src}, Dest: c.dest})
			j := onlyJob(t, state)
			if j.FollowIndex != c.follow {
				t.Errorf("FollowIndex = %v, want %v", j.FollowIndex, c.follow)
			}
			for _, p := range append([]pathloc.Path{j.Destination}, j.Sources...) {
				if p.Scheme() != pathloc.SchemeFile {
					t.Errorf("job path %s is not a disk path", p)
				}
			}
		})
	}
}

func TestDeleteJobFollowsButKeepsIndexPaths(t *testing.T) {
	// A delete decides per path on the worker whether the file is on disk, so its sources stay as given.
	h, state := followHandler(t, "fileindex:///orchard", false)
	h.EnqueueDeleteJob([]string{"fileindex:///orchard/harbor.txt"}, false, false)
	j := onlyJob(t, state)
	if !j.FollowIndex || j.Sources[0].Scheme() != pathloc.SchemeFileIndex {
		t.Fatalf("delete job = follow %v, source %s; want follow with the fileindex:// source kept", j.FollowIndex, j.Sources[0])
	}

	h, state = followHandler(t, "/orchard", false)
	h.EnqueueDeleteJob([]string{"/orchard/harbor.txt"}, false, false)
	if onlyJob(t, state).FollowIndex {
		t.Fatal("a delete from a normal local panel must not follow the index")
	}
}

func TestFlattenJobTranslatesIndexPathsAndFollows(t *testing.T) {
	h, state := followHandler(t, "fileindex:///orchard", false)
	h.AddFlattenJob(FlattenJobRequest{Roots: []string{"fileindex:///orchard/lantern"}, Dest: "fileindex:///orchard"})
	j := onlyJob(t, state)
	if !j.FollowIndex || j.Sources[0].String() != "/orchard/lantern" || j.Destination.String() != "/orchard" {
		t.Fatalf("flatten job = follow %v, %s -> %s; want a following job on disk paths", j.FollowIndex, j.Sources[0], j.Destination)
	}

	h, state = followHandler(t, "/orchard", false)
	h.AddFlattenJob(FlattenJobRequest{Roots: []string{"/orchard/lantern"}, Dest: "/orchard"})
	if onlyJob(t, state).FollowIndex {
		t.Fatal("a flatten from a normal local panel must not follow the index")
	}
}

func TestExtractJobTranslatesIndexPaths(t *testing.T) {
	h, state := followHandler(t, "fileindex:///orchard", false)
	h.EnqueueExtractJob([]string{"fileindex:///orchard/harbor.zip"}, "fileindex:///meadow")
	j := onlyJob(t, state)
	if j.FollowIndex || j.Sources[0].String() != "/orchard/harbor.zip" || j.Destination.String() != "/meadow" {
		t.Fatalf("extract job = follow %v, %s -> %s; want disk paths and no follow (it only creates)", j.FollowIndex, j.Sources[0], j.Destination)
	}
}

func TestUndoJobKeepsFollowIndex(t *testing.T) {
	state := jobs.NewState()
	done := &jobs.Job{
		ID: "done", Type: jobs.TypeRename, Status: jobs.StatusCompleted, FollowIndex: true,
		Sources: pathloc.PathsForTest("/orchard/harbor.txt"),
	}
	done.Undo = []ops.UndoStep{{Kind: ops.UndoMoveBack, Path: pathloc.MustParse("/orchard/pebble.txt"), From: pathloc.MustParse("/orchard/harbor.txt")}}
	state.AddFinishedJob(done)
	undo, ok := state.EnqueueUndo("done")
	if !ok || !undo.FollowIndex {
		t.Fatalf("undo = %v, ok %v; want a job that follows the index like its target", undo, ok)
	}
}
