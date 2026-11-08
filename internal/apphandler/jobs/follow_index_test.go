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

func TestTransferJobKeepsIndexPaths(t *testing.T) {
	cases := []struct {
		name      string
		panel     string
		withIndex bool
		typ       jobs.Type
		src, dest string
	}{
		{"copy out of the fileindex", "fileindex:///orchard", false, jobs.TypeCopy, "fileindex:///orchard/harbor.txt", "/meadow"},
		{"move inside the fileindex", "fileindex:///orchard", false, jobs.TypeMove, "fileindex:///orchard/harbor.txt", "fileindex:///meadow"},
		{"browse with fileindex", "/orchard", true, jobs.TypeMove, "/orchard/harbor.txt", "/meadow"},
		{"normal local panel", "/orchard", false, jobs.TypeMove, "/orchard/harbor.txt", "/meadow"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h, state := followHandler(t, c.panel, c.withIndex)
			h.AddTransferJob(TransferJobRequest{Type: c.typ, Sources: []string{c.src}, Dest: c.dest})
			j := onlyJob(t, state)
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

func TestDeleteJobKeepsIndexPaths(t *testing.T) {
	// A fileindex:// delete is the index backend's: sources stay as given and nothing is deleted on disk.
	h, state := followHandler(t, "fileindex:///orchard", false)
	h.EnqueueDeleteJob([]string{"fileindex:///orchard/harbor.txt"}, false, false)
	j := onlyJob(t, state)
	if j.DeleteOnDisk || j.Sources[0].Scheme() != pathloc.SchemeFileIndex {
		t.Fatalf("delete job = on disk %v, source %s; want index only, source kept", j.DeleteOnDisk, j.Sources[0])
	}
}

func TestDedupDeleteJobDeletesOnDisk(t *testing.T) {
	h, state := followHandler(t, "/orchard", false)
	h.EnqueueDedupDeleteJob([]string{"fileindex:///orchard/harbor.txt"}, true)
	j := onlyJob(t, state)
	if !j.DeleteOnDisk || !j.DeleteRemoveEmptyDirs || j.PromptDanglingDirs || j.Sources[0].Scheme() != pathloc.SchemeFileIndex {
		t.Fatalf("dedup delete job = %+v; want on disk, cleaning empty dirs, no prompt, source kept", j)
	}
	if clone := j.RetryClone(); !clone.DeleteOnDisk {
		t.Fatal("a retried dedup delete must still act on disk")
	}
}
