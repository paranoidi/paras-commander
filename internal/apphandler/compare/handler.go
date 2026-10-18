package compare

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/apphandler/hashwalk"
	"github.com/paranoidi/paras-commander/internal/apphandler/host"
	jobsctrl "github.com/paranoidi/paras-commander/internal/apphandler/jobs"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/gitignore"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

// Deps wires the compare handler at app construction.
type Deps struct {
	Host        Host
	Screen      tcell.Screen
	Model       *ui.Model
	Config      config.Config
	Keys        *keymap.Map
	KeysCompare *keymap.Map
	Gitignore   *gitignore.Cache
	DiskIgnore  diskusage.ShouldIgnoreFolder
	Jobs        *jobsctrl.Handler
}

// Host is the app shell surface compare needs.
type Host interface {
	host.PanelNavigationHost
	TogglePanelSelection(panelID int, path string) (conflicts bool)
	// SetPanelSelection adds (selected) or removes path in the panel's selection; true when it changed.
	SetPanelSelection(panelID int, path string, selected bool) (changed bool)
	// ClearPanelSelection removes every tagged path from the panel's selection.
	ClearPanelSelection(panelID int)
	SetTransientMessage(text string, urgency ui.MessageUrgency)
	ClearTransientMessage()
	CompareMenuDefinitions() []menu.Definition
	BrowserMenuDefinitions() []menu.Definition
}

// Handler owns the compare full-screen view.
type Handler struct {
	host        Host
	screen      tcell.Screen
	model       *ui.Model
	config      config.Config
	keys        *keymap.Map
	keysCompare *keymap.Map
	gitignore   *gitignore.Cache
	diskIgnore  diskusage.ShouldIgnoreFolder
	jobsCtrl    *jobsctrl.Handler

	session *comparepkg.Session
	wake    host.WakeCoalescer

	// Open arguments replayed by Refresh; onClose is the return hook fired once
	// by Close (dedup detour), never by Refresh.
	openedPrimary    pathloc.Path
	openedSecondary  pathloc.Path
	openedShowHidden bool
	openedGate       diskusage.ListingVolumeGate
	onClose          func()
}

// WakePayload wakes PollEvent when compare session updates.
type WakePayload struct{}

// New constructs a compare handler.
func New(d Deps) *Handler {
	return &Handler{
		host:        d.Host,
		screen:      d.Screen,
		model:       d.Model,
		config:      d.Config,
		keys:        d.Keys,
		keysCompare: d.KeysCompare,
		gitignore:   d.Gitignore,
		diskIgnore:  d.DiskIgnore,
		jobsCtrl:    d.Jobs,
	}
}

func (h *Handler) postWake() {
	h.wake.Post(h.screen, WakePayload{})
}

// Open starts comparing primary and secondary panel paths.
func (h *Handler) Open() {
	if ui.IsAuxiliaryView(h.model.ViewMode) && h.model.ViewMode != ui.ViewCompare {
		return
	}
	// The left compare column is the panel drawn on the left in the main view.
	left, right := &h.model.Primary, &h.model.Secondary
	if h.model.SwapPanes {
		left, right = right, left
	}
	volGate := diskusage.ListingVolumeGate{}
	if h.config.Compare.StayOnVolumeDefault {
		volGate = diskusage.ListingVolumeGate{
			Enabled: left.ListingDeviceValid,
			RefDev:  left.ListingDevice,
			Valid:   left.ListingDeviceValid,
		}
	}
	h.open(left.Path, right.Path, left.ShowHidden, volGate, h.model.SwapPanes, nil)
}

// OpenPaths opens compare on two arbitrary local roots (the dedup entry point).
// No aux-view guard (the caller owns the context) and no panel-anchored volume
// gate (the roots may live on any device). onClose fires exactly once when the
// view is closed — but not on Refresh. Returns false when validation fails,
// leaving the current view untouched.
func (h *Handler) OpenPaths(primary, secondary pathloc.Path, showHidden bool, onClose func()) bool {
	return h.open(primary, secondary, showHidden, diskusage.ListingVolumeGate{}, false, onClose)
}

func (h *Handler) open(primary, secondary pathloc.Path, showHidden bool, volGate diskusage.ListingVolumeGate, swap bool, onClose func()) bool {
	// Validate before any teardown so a failed open is a true no-op for the
	// caller's view (dedup stays up when the pair overlaps).
	if primary.IsZero() || secondary.IsZero() {
		h.host.SetTransientMessage("Compare: both sides need a path", ui.MessageUrgencyError)
		return false
	}
	if pathloc.TreesOverlap(primary, secondary) {
		h.host.SetTransientMessage("Compare: paths must point to separate directory trees", ui.MessageUrgencyError)
		return false
	}
	h.teardown()
	h.openedPrimary, h.openedSecondary = primary, secondary
	h.openedShowHidden = showHidden
	h.openedGate = volGate
	h.onClose = onClose

	h.model.ViewMode = ui.ViewCompare
	h.model.CompareView = ui.CompareViewState{
		Filter:      comparepkg.FilterAll,
		FocusColumn: ui.CompareColumnPrimary,
		IgnoreEmpty: true,
		SwapPanels:  swap,
	}
	h.model.MenuDefinitions = h.host.CompareMenuDefinitions()
	h.model.Menu.ActiveMenu = menu.DefaultIndexCompare()

	hs := hashwalk.FromCompareConfig(h.config.Compare, h.diskIgnore, volGate)

	h.session = comparepkg.Start(context.Background(), primary, secondary, comparepkg.Options{
		Walk: comparepkg.WalkOptions{
			ShowHidden:    showHidden,
			Gitignore:     h.gitignore,
			ShouldSkipDir: hs.ShouldSkip,
		},
		HashWorkers:  hs.HashWorkers,
		ReadBuffer:   hs.ReadBuffer,
		MaxHashBytes: hs.MaxHashBytes,
		OnUpdate:     func(_ comparepkg.Snapshot) { h.postWake() },
	})
	h.model.CompareSnapshot = h.session.Snapshot()
	return true
}

// Close cancels compare, returns to the browser, and fires the return hook once.
func (h *Handler) Close() {
	h.teardown()
	if cb := h.onClose; cb != nil {
		h.onClose = nil
		cb() // runs with ViewMode already reset, so the caller may re-enter its view
	}
}

// DiscardReturn drops the return hook so the next Close falls back to the browser.
func (h *Handler) DiscardReturn() { h.onClose = nil }

// teardown cancels the session and clears compare view state. It never touches
// onClose, so Refresh (which reopens via open → teardown) keeps the return hook.
func (h *Handler) teardown() {
	if h.session != nil {
		h.session.Close()
		h.session = nil
	}
	if h.model.ViewMode == ui.ViewCompare {
		h.model.ViewMode = ui.ViewBrowser
		h.model.MenuDefinitions = h.host.BrowserMenuDefinitions()
		h.model.Menu.ActiveMenu = menu.DefaultIndex()
	}
	h.model.CompareView = ui.CompareViewState{}
	h.model.CompareSnapshot = comparepkg.Snapshot{}
	h.model.CompareMergeDialog = dialog.CompareMergeDialogState{}
	h.model.CompareFilterDialog = dialog.CompareFilterDialogState{}
}

// PollUpdates applies the latest session snapshot. Returns true when UI should repaint.
func (h *Handler) PollUpdates(_ WakePayload) bool {
	_ = h.wake.Take()
	if h.session != nil {
		h.model.CompareSnapshot = h.session.Snapshot()
		return true
	}
	return false
}

// Refresh re-runs compare with the roots it was opened on, keeping the return hook.
func (h *Handler) Refresh() {
	if h.model.ViewMode != ui.ViewCompare {
		return
	}
	// open resets the view state; keep cursor, scroll, focus and filters so a refresh after a
	// delete does not jump to the top (EnsureSelectionVisible clamps once rows arrive).
	view := h.model.CompareView
	h.open(h.openedPrimary, h.openedSecondary, h.openedShowHidden, h.openedGate, h.model.CompareView.SwapPanels, h.onClose)
	h.model.CompareView = view
	h.model.CompareView.RangeAnchor = ""
}

// ForgetPaths drops deleted absolute paths (or directories) from the results without a
// re-walk or re-hash, in both the running session and the visible snapshot.
func (h *Handler) ForgetPaths(paths []string) {
	if h.session == nil || h.model.ViewMode != ui.ViewCompare {
		return
	}
	snap := h.model.CompareSnapshot
	rels := func(root pathloc.Path) []string {
		var out []string
		for _, p := range paths {
			if rel, err := filepath.Rel(root.String(), p); err == nil && rel != ".." && !strings.HasPrefix(rel, "../") {
				out = append(out, filepath.ToSlash(rel))
			}
		}
		return out
	}
	pRels, sRels := rels(snap.PrimaryRoot), rels(snap.SecondaryRoot)
	h.session.Forget(pRels, sRels)
	h.model.CompareSnapshot.Rows = comparepkg.PruneRows(snap.Rows, pRels, sRels)
	h.ensureSelectionVisible(0)
}

// CycleFilter advances the category filter.
func (h *Handler) CycleFilter() {
	h.model.CompareView.Filter = comparepkg.CycleFilter(h.model.CompareView.Filter)
	h.ensureSelectionVisible(0)
}

// SetFilter sets the category filter directly.
func (h *Handler) SetFilter(f comparepkg.Filter) {
	h.model.CompareView.Filter = f
	h.ensureSelectionVisible(0)
}

// ToggleIgnoreEmpty flips whether zero-byte compare rows are hidden.
func (h *Handler) ToggleIgnoreEmpty() {
	if h.model.ViewMode != ui.ViewCompare {
		return
	}
	st := &h.model.CompareView
	st.IgnoreEmpty = !st.IgnoreEmpty
	h.model.MenuDefinitions = h.host.CompareMenuDefinitions()
	h.ensureSelectionVisible(0)
}

// FilteredRows returns snapshot rows matching the active filter and empty-file toggle.
func (h *Handler) FilteredRows() []comparepkg.Row {
	st := h.model.CompareView
	return comparepkg.FilteredRows(h.model.CompareSnapshot, st.Filter, st.IgnoreEmpty, st.NameFilter)
}

// EnsureSelectionVisible keeps the cursor row visible for visibleRows height.
func (h *Handler) EnsureSelectionVisible(visibleRows int) {
	h.ensureSelectionVisible(visibleRows)
}

func (h *Handler) ensureSelectionVisible(visibleRows int) {
	h.model.CompareView.EnsureSelectionVisible(len(h.FilteredRows()), visibleRows)
}

// MoveColumnFocus shifts primary/secondary column focus (sticky across row navigation).
func (h *Handler) MoveColumnFocus(delta int) {
	st := &h.model.CompareView
	prev := st.FocusColumn
	if delta < 0 {
		st.FocusColumn = ui.CompareColumnPrimary
	} else if delta > 0 {
		st.FocusColumn = ui.CompareColumnSecondary
	}
	if st.FocusColumn != prev {
		st.RangeAnchor = ""
	}
}

// ToggleColumnFocus flips focus between the primary and secondary column (panel.switch, Tab).
func (h *Handler) ToggleColumnFocus() {
	st := &h.model.CompareView
	if st.FocusColumn == ui.CompareColumnSecondary {
		st.FocusColumn = ui.CompareColumnPrimary
	} else {
		st.FocusColumn = ui.CompareColumnSecondary
	}
	st.RangeAnchor = ""
}

// selectedColumnTarget resolves the row/column under the current selection and focused
// column to an absolute path and its owning panel. ok is false when nothing is selected or
// the focused side has no counterpart entry; err is set when the resolved path itself is
// invalid (rare — a caller may surface it, or ignore it and treat it like !ok).
func (h *Handler) selectedColumnTarget() (abs pathloc.Path, panelID int, ok bool, err error) {
	st := &h.model.CompareView
	rows := h.FilteredRows()
	if st.Selected < 0 || st.Selected >= len(rows) {
		return pathloc.Path{}, 0, false, nil
	}
	return h.focusedSide(rows[st.Selected])
}

// focusedSide resolves row's entry on the focused column's side to an absolute path and
// its owning panel. ok is false when that side has no counterpart entry.
func (h *Handler) focusedSide(row comparepkg.Row) (abs pathloc.Path, panelID int, ok bool, err error) {
	return h.sideOf(row, h.model.CompareView.FocusColumn == ui.CompareColumnSecondary)
}

// sideOf resolves row's entry on the secondary (or primary) side like focusedSide.
func (h *Handler) sideOf(row comparepkg.Row, secondary bool) (abs pathloc.Path, panelID int, ok bool, err error) {
	snap := h.model.CompareSnapshot
	rel := row.PrimaryRel
	root := snap.PrimaryRoot
	panelID = h.model.CompareView.PanelID(secondary)
	if secondary {
		rel = row.SecondaryRel
		root = snap.SecondaryRoot
	}
	if rel == "" {
		return pathloc.Path{}, 0, false, nil
	}
	abs, err = comparepkg.JoinRel(root, rel)
	if err != nil {
		return pathloc.Path{}, 0, false, err
	}
	return abs, panelID, true, nil
}

// ToggleColumnSelection toggles the path under the focused column into the matching panel.
func (h *Handler) ToggleColumnSelection() (conflicts bool) {
	abs, panelID, ok, err := h.selectedColumnTarget()
	if err != nil {
		h.host.SetTransientMessage(err.Error(), ui.MessageUrgencyError)
		return false
	}
	if !ok {
		return false
	}
	return h.host.TogglePanelSelection(panelID, abs.String())
}

// SelectColumnRange is panel.select-range: the first press anchors the cursor row's focused-side
// path (RangeStarted; the caller moves the cursor down); the next press selects every visible
// row's focused-side path between anchor and cursor and clears the anchor (RangeDone). If the
// anchor row is no longer visible the press re-anchors. Rows without a focused-side entry are skipped.
func (h *Handler) SelectColumnRange() (res primitive.RangeResult, added int) {
	st := &h.model.CompareView
	abs, _, ok, err := h.selectedColumnTarget()
	if err != nil {
		h.host.SetTransientMessage(err.Error(), ui.MessageUrgencyError)
		return primitive.RangeNone, 0
	}
	if !ok {
		return primitive.RangeNone, 0
	}
	rows := h.FilteredRows()
	ai := primitive.RangeAnchorIndex(len(rows), st.RangeAnchor, func(i int) string {
		a, _, ok, _ := h.focusedSide(rows[i])
		if !ok {
			return ""
		}
		return a.String()
	})
	if ai < 0 {
		st.RangeAnchor = abs.String()
		return primitive.RangeStarted, 0
	}
	for i := min(ai, st.Selected); i <= max(ai, st.Selected); i++ {
		a, id, ok, _ := h.focusedSide(rows[i])
		if ok && h.host.SetPanelSelection(id, a.String(), true) {
			added++
		}
	}
	st.RangeAnchor = ""
	return primitive.RangeDone, added
}

// SelectedColumnTarget returns the absolute path under the focused column and the panel that
// owns it (pin, copy, delete). Compare rows are always files. False when nothing is selected
// or the focused side has no counterpart entry.
func (h *Handler) SelectedColumnTarget() (path string, panelID int, ok bool) {
	abs, panelID, ok, err := h.selectedColumnTarget()
	if !ok || err != nil {
		return "", 0, false
	}
	return abs.String(), panelID, true
}

// NavigateFromSelection opens the highlighted path in the browser.
func (h *Handler) NavigateFromSelection(viewportRows int) {
	_ = viewportRows
	rows := h.FilteredRows()
	st := &h.model.CompareView
	if st.Selected < 0 || st.Selected >= len(rows) {
		return
	}
	row := rows[st.Selected]
	snap := h.model.CompareSnapshot
	if row.PrimaryRel != "" {
		_ = h.navigateSide(st.PanelID(false), snap.PrimaryRoot, row.PrimaryRel)
		return
	}
	if row.SecondaryRel != "" {
		_ = h.navigateSide(st.PanelID(true), snap.SecondaryRoot, row.SecondaryRel)
	}
}

func (h *Handler) navigateSide(panelID int, root pathloc.Path, rel string) error {
	abs, err := comparepkg.JoinRel(root, rel)
	if err != nil {
		return err
	}
	selectName := filepath.Base(filepath.FromSlash(rel))
	parent := abs.Parent()
	return h.host.NavigatePanelToPath(panelID, parent.String(), selectName)
}
