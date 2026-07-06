// Package dedup owns the full-screen "find duplicates within a single directory" view.
package dedup

import (
	"context"
	"fmt"
	"path/filepath"
	"sync/atomic"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/gitignore"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

// dedupConfirmThreshold pauses before hashing when more than this many files are
// hash candidates (share a byte size with another file). ponytail: hard-coded; make
// it config if someone wants to tune it.
const dedupConfirmThreshold = 5000

// Deps wires the dedup handler at app construction.
type Deps struct {
	Host       Host
	Screen     tcell.Screen
	Model      *ui.Model
	Config     config.Config
	Gitignore  *gitignore.Cache
	DiskIgnore diskusage.ShouldIgnoreFolder
}

// Host is the app shell surface dedup needs.
type Host interface {
	NavigatePanelToPath(panelID int, path string, selectName string) error
	EnqueueDeleteJob(paths []string)
	SetTransientMessage(text string, urgency ui.MessageUrgency)
	DedupMenuDefinitions() []menu.Definition
	BrowserMenuDefinitions() []menu.Definition
}

// WakePayload wakes PollEvent when the dedup session updates.
type WakePayload struct{}

// Handler owns the dedup full-screen view.
type Handler struct {
	host       Host
	screen     tcell.Screen
	model      *ui.Model
	config     config.Config
	gitignore  *gitignore.Cache
	diskIgnore diskusage.ShouldIgnoreFolder

	session     *comparepkg.DedupSession
	wakePending atomic.Bool
}

// New constructs a dedup handler.
func New(d Deps) *Handler {
	return &Handler{
		host:       d.Host,
		screen:     d.Screen,
		model:      d.Model,
		config:     d.Config,
		gitignore:  d.Gitignore,
		diskIgnore: d.DiskIgnore,
	}
}

func (h *Handler) postWake() {
	if h.screen == nil {
		return
	}
	if h.wakePending.Swap(true) {
		return
	}
	_ = h.screen.PostEvent(tcell.NewEventInterrupt(WakePayload{}))
}

// Open starts scanning the active panel's directory for duplicates.
func (h *Handler) Open() {
	if ui.IsAuxiliaryView(h.model.ViewMode) && h.model.ViewMode != ui.ViewDedup {
		return
	}
	h.Close()

	p := &h.model.Primary
	if h.model.ActivePanel == ui.SecondaryPanel {
		p = &h.model.Secondary
	}
	root := p.Path
	if root.IsZero() {
		h.host.SetTransientMessage("Find duplicates: panel needs a path", ui.MessageUrgencyError)
		return
	}
	if root.IsRemote() {
		h.host.SetTransientMessage("Find duplicates: remote paths not supported", ui.MessageUrgencyError)
		return
	}

	h.model.ViewMode = ui.ViewDedup
	h.model.MenuDefinitions = h.host.DedupMenuDefinitions()
	h.model.Menu.ActiveMenu = menu.DefaultIndexDedup()
	h.model.DedupView = ui.DedupViewState{Marked: map[string]bool{}, IgnoreEmpty: true}

	volGate := diskusage.ListingVolumeGate{}
	if h.config.Compare.StayOnVolumeDefault {
		volGate = diskusage.ListingVolumeGate{
			Enabled: p.ListingDeviceValid,
			RefDev:  p.ListingDevice,
			Valid:   p.ListingDeviceValid,
		}
	}
	shouldSkip := diskusage.ComposeListingVolumeIgnore(h.diskIgnore, volGate)

	bufKiB := h.config.Compare.ReadBufferKiB
	if bufKiB <= 0 {
		bufKiB = config.DefaultCompareReadBufferKiB
	}
	workers := h.config.Compare.HashConcurrency
	if workers <= 0 {
		workers = config.DefaultCompareHashConcurrency
	}

	h.session = comparepkg.StartDedup(context.Background(), root, comparepkg.DedupOptions{
		Walk: comparepkg.WalkOptions{
			ShowHidden:    p.ShowHidden,
			Gitignore:     h.gitignore,
			ShouldSkipDir: shouldSkip,
		},
		HashWorkers:      workers,
		ReadBuffer:       make([]byte, bufKiB*1024),
		MaxHashBytes:     h.config.Compare.MaxHashBytes,
		ConfirmThreshold: dedupConfirmThreshold,
		OnUpdate:         func(_ comparepkg.DedupSnapshot) { h.postWake() },
	})
	h.model.DedupSnapshot = h.session.Snapshot()
	h.syncDedupList()
}

// Close cancels the scan and returns to the browser.
func (h *Handler) Close() {
	if h.session != nil {
		h.session.Close()
		h.session = nil
	}
	if h.model.ViewMode == ui.ViewDedup {
		h.model.ViewMode = ui.ViewBrowser
		h.model.MenuDefinitions = h.host.BrowserMenuDefinitions()
		h.model.Menu.ActiveMenu = menu.DefaultIndex()
	}
	h.model.DedupView = ui.DedupViewState{}
	h.model.DedupSnapshot = comparepkg.DedupSnapshot{}
	h.model.DedupList = nil
}

// PollUpdates applies the latest session snapshot. Returns true when the UI should repaint.
func (h *Handler) PollUpdates(_ WakePayload) bool {
	h.wakePending.Store(false)
	if h.session != nil {
		h.model.DedupSnapshot = h.session.Snapshot()
		h.syncDedupList()
		h.ensureSelectionVisible(0)
		return true
	}
	return false
}

// Confirm resumes hashing after the >threshold confirmation pause.
func (h *Handler) Confirm() {
	if h.session != nil {
		h.session.Confirm()
	}
}

// Refresh re-runs the scan on the active panel.
func (h *Handler) Refresh() {
	if h.model.ViewMode != ui.ViewDedup {
		return
	}
	h.Open()
}

// ListLen returns the number of selectable duplicate files in display order.
func (h *Handler) ListLen() int {
	return len(h.model.DedupList)
}

// EnsureSelectionVisible keeps the cursor row visible for visibleRows height.
func (h *Handler) EnsureSelectionVisible(visibleRows int) {
	h.ensureSelectionVisible(visibleRows)
}

func (h *Handler) syncDedupList() {
	st := &h.model.DedupView
	h.model.DedupList, st.IgnoredEmptyCount = ui.DedupEntriesFromSnapshot(
		h.model.DedupSnapshot, st.SortByWasted, st.IgnoreEmpty)
}

// ToggleSortOrder flips between order-by-path and most-space-wasted.
func (h *Handler) ToggleSortOrder() {
	if h.model.ViewMode != ui.ViewDedup {
		return
	}
	h.model.DedupView.SortByWasted = !h.model.DedupView.SortByWasted
	h.syncDedupList()
	h.ensureSelectionVisible(0)
}

// ToggleIgnoreEmpty flips whether zero-byte duplicate groups are hidden.
func (h *Handler) ToggleIgnoreEmpty() {
	if h.model.ViewMode != ui.ViewDedup {
		return
	}
	st := &h.model.DedupView
	st.IgnoreEmpty = !st.IgnoreEmpty
	st.Selected = 0
	st.ListScroll = 0
	h.syncDedupList()
	h.ensureSelectionVisible(0)
}

func (h *Handler) ensureSelectionVisible(visibleRows int) {
	n := len(h.model.DedupList)
	h.model.DedupView.EnsureSelectionVisible(n, visibleRows)
}

// ToggleMark flips the delete mark on the selected file.
func (h *Handler) ToggleMark() {
	list := h.model.DedupList
	st := &h.model.DedupView
	if st.Selected < 0 || st.Selected >= len(list) {
		return
	}
	if st.Marked == nil {
		st.Marked = map[string]bool{}
	}
	entry := list[st.Selected]
	if st.Marked[entry.AbsKey] {
		delete(st.Marked, entry.AbsKey)
		st.MarkedCount--
		st.MarkedReclaimBytes -= entry.Size
	} else {
		st.Marked[entry.AbsKey] = true
		st.MarkedCount++
		st.MarkedReclaimBytes += entry.Size
	}
}

// ToggleGroupMark marks every copy in the selected row's duplicate group, or unmarks
// them all when the whole group is already marked.
func (h *Handler) ToggleGroupMark() {
	list := h.model.DedupList
	st := &h.model.DedupView
	if st.Selected < 0 || st.Selected >= len(list) {
		return
	}
	if st.Marked == nil {
		st.Marked = map[string]bool{}
	}
	start, end := ui.DedupGroupBounds(list, st.Selected)
	unmark := ui.DedupGroupFullyMarked(list, st.Marked, st.Selected)
	for i := start; i < end; i++ {
		e := list[i]
		switch {
		case unmark && st.Marked[e.AbsKey]:
			delete(st.Marked, e.AbsKey)
			st.MarkedCount--
			st.MarkedReclaimBytes -= e.Size
		case !unmark && !st.Marked[e.AbsKey]:
			st.Marked[e.AbsKey] = true
			st.MarkedCount++
			st.MarkedReclaimBytes += e.Size
		}
	}
}

// MarkRedundantUnderSelected marks (for deletion) redundant duplicate copies under
// the selected row's directory, leaving one surviving copy of each content group so
// only unique files remain there ("keep uniques"). Mark-only — deletion stays with
// DeleteMarked.
func (h *Handler) MarkRedundantUnderSelected() {
	h.markUnderSelected(ui.DedupRedundantUnder, "No redundant copies under this folder")
}

// MarkDuplicatesUnderSelected marks (for deletion) copies under the selected row's
// directory that are also stored outside it, leaving groups that live only here
// untouched ("delete duplicates from here"). Mark-only — deletion stays with
// DeleteMarked.
func (h *Handler) MarkDuplicatesUnderSelected() {
	h.markUnderSelected(ui.DedupDuplicatesUnder, "No duplicates stored elsewhere under this folder")
}

// markUnderSelected applies a mark-selection rule to the current list and merges the
// result into the delete-mark set, keeping the marked count and reclaim bytes in sync.
func (h *Handler) markUnderSelected(pick func([]ui.DedupEntry, int) []string, emptyMsg string) {
	list := h.model.DedupList
	st := &h.model.DedupView
	keys := pick(list, st.Selected)
	if len(keys) == 0 {
		h.host.SetTransientMessage(emptyMsg, ui.MessageUrgencyInfo)
		return
	}
	if st.Marked == nil {
		st.Marked = map[string]bool{}
	}
	set := make(map[string]bool, len(keys))
	for _, k := range keys {
		set[k] = true
	}
	for _, e := range list {
		if set[e.AbsKey] && !st.Marked[e.AbsKey] {
			st.Marked[e.AbsKey] = true
			st.MarkedCount++
			st.MarkedReclaimBytes += e.Size
		}
	}
}

// ClearMarks unmarks every file, reusing the file-list clear-selection binding.
func (h *Handler) ClearMarks() {
	st := &h.model.DedupView
	if st.MarkedCount == 0 {
		return
	}
	st.Marked = map[string]bool{}
	st.MarkedCount = 0
	st.MarkedReclaimBytes = 0
}

// MarkedPaths returns marked file paths that still exist in the current snapshot,
// in display order.
func (h *Handler) MarkedPaths() []string {
	st := h.model.DedupView
	out := make([]string, 0, len(st.Marked))
	for _, entry := range h.model.DedupList {
		if st.Marked[entry.AbsKey] {
			out = append(out, entry.AbsKey)
		}
	}
	return out
}

// DeleteMarked enqueues a delete job for marked files and optimistically prunes them
// from the view.
func (h *Handler) DeleteMarked() {
	st := &h.model.DedupView
	paths := h.MarkedPaths()
	if len(paths) == 0 {
		return
	}
	h.host.EnqueueDeleteJob(paths)
	noun := "files"
	if len(paths) == 1 {
		noun = "file"
	}
	h.host.SetTransientMessage(fmt.Sprintf("Delete queued (%d %s)", len(paths), noun), ui.MessageUrgencyInfo)
	// Optimistically drop the deleted files; groups under two members disappear.
	// ponytail: no re-walk — if a delete fails the row just vanishes; reopen to rescan.
	h.model.DedupSnapshot = h.model.DedupSnapshot.WithoutPaths(st.Marked)
	st.Marked = map[string]bool{}
	st.MarkedCount = 0
	st.MarkedReclaimBytes = 0
	st.Selected = 0
	st.ListScroll = 0
	h.syncDedupList()
}

// NavigateFromSelection opens the selected file's directory in the active panel.
func (h *Handler) NavigateFromSelection() {
	list := h.model.DedupList
	st := h.model.DedupView
	if st.Selected < 0 || st.Selected >= len(list) {
		return
	}
	f := list[st.Selected].File
	selectName := filepath.Base(filepath.FromSlash(f.Rel))
	_ = h.host.NavigatePanelToPath(h.model.ActivePanel, f.Abs.Parent().String(), selectName)
}
