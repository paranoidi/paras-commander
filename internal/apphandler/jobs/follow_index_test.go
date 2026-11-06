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

// activePanelHost serves a fixed active panel and records the transient messages shown.
type activePanelHost struct {
	jobsHostStub
	active *panel.State
	msgs   *[]string
}

func (h activePanelHost) ActivePanel() *panel.State { return h.active }

func (h activePanelHost) SetTransientMessage(msg string, _ ui.MessageUrgency) {
	*h.msgs = append(*h.msgs, msg)
}

// followHandler returns a handler whose active panel shows path (browsing with fileindex when
// withIndex), and the state its jobs land in.
func followHandler(t *testing.T, path string, withIndex bool) (*Handler, *jobs.State) {
	t.Helper()
	h, state, _ := followHandlerMsgs(t, path, withIndex)
	return h, state
}

// followHandlerMsgs is followHandler plus the messages the handler shows.
func followHandlerMsgs(t *testing.T, path string, withIndex bool) (*Handler, *jobs.State, *[]string) {
	t.Helper()
	state := jobs.NewState()
	msgs := &[]string{}
	p := &panel.State{Path: pathloc.MustParse(path), WithIndex: withIndex}
	h := New(Deps{Host: activePanelHost{active: p, msgs: msgs}, Model: &ui.Model{}, State: state, Config: config.Default()})
	return h, state, msgs
}

func onlyJob(t *testing.T, s *jobs.State) *jobs.Job {
	t.Helper()
	all := s.AllJobs()
	if len(all) != 1 {
		t.Fatalf("jobs = %d, want 1", len(all))
	}
	return all[0]
}

func TestTransferJobKeepsIndexPathsAndFollows(t *testing.T) {
	cases := []struct {
		name      string
		panel     string
		withIndex bool
		typ       jobs.Type
		src, dest string
		follow    bool
	}{
		{"copy out of the fileindex", "fileindex:///orchard", false, jobs.TypeCopy, "fileindex:///orchard/harbor.txt", "/meadow", true},
		{"move inside the fileindex", "fileindex:///orchard", false, jobs.TypeMove, "fileindex:///orchard/harbor.txt", "fileindex:///meadow", true},
		{"browse with fileindex", "/orchard", true, jobs.TypeMove, "/orchard/harbor.txt", "/meadow", true},
		{"normal local panel", "/orchard", false, jobs.TypeMove, "/orchard/harbor.txt", "/meadow", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h, state := followHandler(t, c.panel, c.withIndex)
			h.AddTransferJob(TransferJobRequest{Type: c.typ, Sources: []string{c.src}, Dest: c.dest})
			j := onlyJob(t, state)
			if j.FollowIndex != c.follow {
				t.Errorf("FollowIndex = %v, want %v", j.FollowIndex, c.follow)
			}
			if j.Sources[0].String() != c.src || j.Destination.String() != c.dest {
				t.Errorf("job paths %s -> %s, want them untranslated (%s -> %s)", j.Sources[0], j.Destination, c.src, c.dest)
			}
		})
	}
}

func TestTransferJobRefusalsNeverQueue(t *testing.T) {
	cases := []struct {
		name      string
		panel     string
		typ       jobs.Type
		src, dest string
		want      string
	}{
		{"move out of the fileindex", "fileindex:///orchard", jobs.TypeMove, "fileindex:///orchard/harbor.txt", "/meadow", ops.IndexMoveOutRefusal},
		{"copy into the fileindex", "/orchard", jobs.TypeCopy, "/orchard/harbor.txt", "fileindex:///meadow", ops.IndexReadOnlyRefusal},
		{"move into the fileindex", "/orchard", jobs.TypeMove, "/orchard/harbor.txt", "fileindex:///meadow", ops.IndexReadOnlyRefusal},
		{"copy inside the fileindex", "fileindex:///orchard", jobs.TypeCopy, "fileindex:///orchard/harbor.txt", "fileindex:///meadow", ops.IndexReadOnlyRefusal},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h, state, msgs := followHandlerMsgs(t, c.panel, false)
			if h.AddTransferJob(TransferJobRequest{Type: c.typ, Sources: []string{c.src}, Dest: c.dest}) {
				t.Fatal("AddTransferJob reported a queued job")
			}
			if n := len(state.AllJobs()); n != 0 {
				t.Fatalf("jobs = %d, want none", n)
			}
			if len(*msgs) != 1 || (*msgs)[0] != c.want {
				t.Fatalf("messages = %q, want %q", *msgs, c.want)
			}
		})
	}
}

func TestDeleteJobFollowsAndKeepsIndexPaths(t *testing.T) {
	// A fileindex:// delete is the index backend's: sources stay as given and nothing is deleted on disk.
	h, state := followHandler(t, "fileindex:///orchard", false)
	h.EnqueueDeleteJob([]string{"fileindex:///orchard/harbor.txt"}, false, false)
	j := onlyJob(t, state)
	if !j.FollowIndex || j.DeleteOnDisk || j.Sources[0].Scheme() != pathloc.SchemeFileIndex {
		t.Fatalf("delete job = follow %v, on disk %v, source %s; want follow, index only, source kept", j.FollowIndex, j.DeleteOnDisk, j.Sources[0])
	}

	h, state = followHandler(t, "/orchard", false)
	h.EnqueueDeleteJob([]string{"/orchard/harbor.txt"}, false, false)
	if onlyJob(t, state).FollowIndex {
		t.Fatal("a delete from a normal local panel must not follow the index")
	}
}

func TestDedupDeleteJobDeletesOnDisk(t *testing.T) {
	h, state := followHandler(t, "/orchard", false)
	h.EnqueueDedupDeleteJob([]string{"fileindex:///orchard/harbor.txt"}, true)
	j := onlyJob(t, state)
	if !j.DeleteOnDisk || !j.FollowIndex || !j.DeleteRemoveEmptyDirs || j.PromptDanglingDirs || j.Sources[0].Scheme() != pathloc.SchemeFileIndex {
		t.Fatalf("dedup delete job = %+v; want on disk, following, cleaning empty dirs, no prompt, source kept", j)
	}
	if clone := j.RetryClone(); !clone.DeleteOnDisk {
		t.Fatal("a retried dedup delete must still act on disk")
	}
}

func TestFlattenJobRefusesIndexPaths(t *testing.T) {
	for name, req := range map[string]FlattenJobRequest{
		"root": {Roots: []string{"fileindex:///orchard/lantern"}, Dest: "/meadow"},
		"dest": {Roots: []string{"/orchard/lantern"}, Dest: "fileindex:///meadow"},
	} {
		h, state, msgs := followHandlerMsgs(t, "fileindex:///orchard", false)
		h.AddFlattenJob(req)
		if n := len(state.AllJobs()); n != 0 || len(*msgs) != 1 || (*msgs)[0] != ops.FlattenIndexRefusal {
			t.Fatalf("%s: jobs %d, messages %q; want a refusal and no job", name, n, *msgs)
		}
	}

	h, state := followHandler(t, "/orchard", false)
	h.AddFlattenJob(FlattenJobRequest{Roots: []string{"/orchard/lantern"}, Dest: "/orchard"})
	if onlyJob(t, state).FollowIndex {
		t.Fatal("a flatten from a normal local panel must not follow the index")
	}
}

func TestExtractJobTranslatesSourcesAndDoesNotFollowIndex(t *testing.T) {
	h, state := followHandler(t, "fileindex:///orchard", false)
	h.EnqueueExtractJob([]string{"fileindex:///orchard/harbor.zip"}, "/meadow")
	j := onlyJob(t, state)
	if j.FollowIndex {
		t.Fatal("an extract only creates files: it must not follow the index")
	}
	if j.Sources[0].String() != "/orchard/harbor.zip" || j.Destination.String() != "/meadow" {
		t.Fatalf("extract job = %s -> %s; want the archive read from disk", j.Sources[0], j.Destination)
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

// directMoveHost is a fileindex:// active panel with a source and a local inactive panel.
type directMoveHost struct {
	activePanelHost
	inactive *panel.State
}

func (h directMoveHost) ActivePanelSources() []string {
	return []string{"fileindex:///orchard/harbor.txt"}
}
func (h directMoveHost) InactivePanel() *panel.State { return h.inactive }

func TestDirectMoveOutOfIndexKeepsRefusalMessage(t *testing.T) {
	h, state, msgs := followHandlerMsgs(t, "fileindex:///orchard", false)
	h.host = directMoveHost{
		activePanelHost: h.host.(activePanelHost),
		inactive:        &panel.State{Path: pathloc.MustParse("/meadow")},
	}
	h.EnqueueMoveJob()
	if got := state.AllJobs(); len(got) != 0 {
		t.Fatalf("queued %d jobs, want none", len(got))
	}
	if len(*msgs) == 0 || (*msgs)[len(*msgs)-1] != ops.IndexMoveOutRefusal {
		t.Fatalf("messages = %q, want the refusal last", *msgs)
	}
}
