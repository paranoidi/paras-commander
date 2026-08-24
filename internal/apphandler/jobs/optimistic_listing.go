package jobs

import (
	"path/filepath"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// applyOptimisticListingForJob updates both panels' in-memory listings immediately when a
// mutating job is enqueued, so move/delete sources disappear (and copy/move destinations appear)
// without waiting for the async directory reload that lands later on terminal job events.
func (h *Handler) applyOptimisticListingForJob(job *jobs.Job) {
	if job == nil {
		return
	}
	var remove []string
	var insert []localfs.Entry
	switch job.Type {
	case jobs.TypeMove, jobs.TypeFlatten:
		remove = pathStrings(job.Sources)
		insert = h.destEntriesForJob(job)
	case jobs.TypeDelete:
		remove = pathStrings(job.Sources)
	case jobs.TypeCopy, jobs.TypeExtract:
		insert = h.destEntriesForJob(job)
	default:
		return
	}
	h.applyListingPatchBoth(remove, insert)
}

func (h *Handler) applyListingPatchBoth(remove []string, insert []localfs.Entry) {
	for _, pan := range []*panel.State{h.host.PrimaryPanel(), h.host.SecondaryPanel()} {
		if pan == nil {
			continue
		}
		vr := panelViewport(pan)
		if len(remove) > 0 {
			pan.RemoveEntriesByPath(remove, vr)
		}
		for _, e := range insert {
			pan.InsertEntry(e, vr)
		}
	}
}

func panelViewport(pan *panel.State) int {
	if pan != nil && pan.FileListViewportRows != nil {
		return pan.FileListViewportRows()
	}
	return 0
}

func pathStrings(locs []pathloc.Path) []string {
	out := make([]string, 0, len(locs))
	for _, loc := range locs {
		if loc.IsZero() {
			continue
		}
		out = append(out, loc.String())
	}
	return out
}

func (h *Handler) destEntriesForJob(job *jobs.Job) []localfs.Entry {
	if job.Destination.IsZero() || len(job.Sources) == 0 {
		return nil
	}
	nameRoot := ops.TransferNameRoot(job.Sources)
	if job.FlatDestNames() {
		nameRoot = pathloc.Path{}
	}
	out := make([]localfs.Entry, 0, len(job.Sources))
	for _, src := range job.Sources {
		entry := h.lookupSourceEntry(src)
		var destLoc pathloc.Path
		var err error
		if job.DestIsDir {
			name := ops.TransferDestName(src, nameRoot)
			destLoc, err = job.Destination.Join(name)
			if err != nil {
				continue
			}
		} else {
			destLoc = job.Destination
		}
		entry.Path = destLoc.String()
		entry.Name = destLoc.Base()
		if entry.Name == "" {
			entry.Name = filepath.Base(entry.Path)
		}
		out = append(out, entry)
	}
	return out
}

func (h *Handler) lookupSourceEntry(src pathloc.Path) localfs.Entry {
	srcPath := filepath.Clean(src.String())
	for _, pan := range []*panel.State{h.host.PrimaryPanel(), h.host.SecondaryPanel()} {
		if pan == nil {
			continue
		}
		if e, ok := pan.EntriesByPath()[srcPath]; ok {
			return e
		}
		for _, e := range pan.Entries {
			if filepath.Clean(e.Path) == srcPath {
				return e
			}
		}
	}
	return localfs.Entry{
		Name: src.Base(),
		Path: srcPath,
		Type: localfs.EntryFile,
	}
}
