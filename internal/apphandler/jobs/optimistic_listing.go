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
		if len(insert) > 0 {
			pan.InsertEntries(insert, vr)
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
	lookup := h.newSourceEntryLookup()
	out := make([]localfs.Entry, 0, len(job.Sources))
	for _, src := range job.Sources {
		entry := lookup.at(src)
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

// sourceEntryLookup resolves job sources against both panels' listings without any
// per-source scanning. ListingEntryAt is an O(1) hit against the panel's incrementally
// maintained path index; the cleaned-path map is built at most once (and only when some
// source's path doesn't match a raw listing key), so resolving N sources stays O(N +
// panel entries) instead of O(N × panel entries) — the latter froze the UI when a
// large selection was queued from the panel that isn't checked first.
type sourceEntryLookup struct {
	panels  []*panel.State
	cleaned map[string]localfs.Entry
}

func (h *Handler) newSourceEntryLookup() *sourceEntryLookup {
	return &sourceEntryLookup{panels: []*panel.State{h.host.PrimaryPanel(), h.host.SecondaryPanel()}}
}

func (l *sourceEntryLookup) at(src pathloc.Path) localfs.Entry {
	srcPath := filepath.Clean(src.String())
	for _, pan := range l.panels {
		if pan == nil {
			continue
		}
		if e, ok := pan.ListingEntryAt(srcPath); ok {
			return e
		}
	}
	if l.cleaned == nil {
		l.cleaned = map[string]localfs.Entry{}
		for _, pan := range l.panels {
			if pan == nil {
				continue
			}
			for _, e := range pan.Entries {
				l.cleaned[filepath.Clean(e.Path)] = e
			}
		}
	}
	if e, ok := l.cleaned[srcPath]; ok {
		return e
	}
	return localfs.Entry{
		Name: src.Base(),
		Path: srcPath,
		Type: localfs.EntryFile,
	}
}
