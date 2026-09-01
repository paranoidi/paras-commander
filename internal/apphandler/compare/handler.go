package compare

import (
	"context"
	"path/filepath"

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

	session       *comparepkg.Session
	wake          host.WakeCoalescer
	sessionCtx    context.Context
	sessionCancel context.CancelFunc

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
	volGate := diskusage.ListingVolumeGate{}
	if h.config.Compare.StayOnVolumeDefault {
		volGate = diskusage.ListingVolumeGate{
			Enabled: h.model.Primary.ListingDeviceValid,
			RefDev:  h.model.Primary.ListingDevice,
			Valid:   h.model.Primary.ListingDeviceValid,
		}
	}
	h.open(h.model.Primary.Path, h.model.Secondary.Path, h.model.Primary.ShowHidden, volGate, nil)
}

// OpenPaths opens compare on two arbitrary local roots (the dedup entry point).
// No aux-view guard (the caller owns the context) and no panel-anchored volume
// gate (the roots may live on any device). onClose fires exactly once when the
// view is closed — but not on Refresh. Returns false when validation fails,
// leaving the current view untouched.
func (h *Handler) OpenPaths(primary, secondary pathloc.Path, showHidden bool, onClose func()) bool {
	return h.open(primary, secondary, showHidden, diskusage.ListingVolumeGate{}, onClose)
}

func (h *Handler) open(primary, secondary pathloc.Path, showHidden bool, volGate diskusage.ListingVolumeGate, onClose func()) bool {
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
	h.model.MenuDefinitions = h.host.CompareMenuDefinitions()
	h.model.Menu.ActiveMenu = 0
	h.model.CompareView = ui.CompareViewState{
		Filter:      comparepkg.FilterAll,
		FocusColumn: ui.CompareColumnPrimary,
		IgnoreEmpty: true,
	}

	hs := hashwalk.FromCompareConfig(h.config.Compare, h.diskIgnore, volGate)

	ctx, cancel := context.WithCancel(context.Background())
	h.sessionCtx = ctx
	h.sessionCancel = cancel

	h.session = comparepkg.Start(ctx, primary, secondary, comparepkg.Options{
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
	if h.sessionCancel != nil {
		h.sessionCancel()
	}
	if h.session != nil {
		h.session.Close()
		h.session = nil
	}
	h.sessionCancel = nil
	if h.model.ViewMode == ui.ViewCompare {
		h.model.ViewMode = ui.ViewBrowser
		h.model.MenuDefinitions = h.host.BrowserMenuDefinitions()
		h.model.Menu.ActiveMenu = menu.DefaultIndex()
	}
	h.model.CompareView = ui.CompareViewState{}
	h.model.CompareSnapshot = comparepkg.Snapshot{}
	h.model.CompareMergeDialog = dialog.CompareMergeDialogState{}
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
	h.open(h.openedPrimary, h.openedSecondary, h.openedShowHidden, h.openedGate, h.onClose)
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
	h.ensureSelectionVisible(0)
}

// FilteredRows returns snapshot rows matching the active filter and empty-file toggle.
func (h *Handler) FilteredRows() []comparepkg.Row {
	st := h.model.CompareView
	return comparepkg.FilteredRows(h.model.CompareSnapshot, st.Filter, st.IgnoreEmpty)
}

// EnsureSelectionVisible keeps the cursor row visible for visibleRows height.
func (h *Handler) EnsureSelectionVisible(visibleRows int) {
	h.ensureSelectionVisible(visibleRows)
}

func (h *Handler) ensureSelectionVisible(visibleRows int) {
	rows := h.FilteredRows()
	n := len(rows)
	st := &h.model.CompareView
	if n == 0 {
		st.Selected = 0
		st.ListScroll = 0
		return
	}
	if st.Selected < 0 {
		st.Selected = 0
	}
	if st.Selected >= n {
		st.Selected = n - 1
	}
	if visibleRows <= 0 {
		return
	}
	if st.ListScroll > st.Selected {
		st.ListScroll = st.Selected
	}
	if st.Selected >= st.ListScroll+visibleRows {
		st.ListScroll = st.Selected - visibleRows + 1
	}
	maxScroll := max(0, n-visibleRows)
	if st.ListScroll > maxScroll {
		st.ListScroll = maxScroll
	}
	if st.ListScroll < 0 {
		st.ListScroll = 0
	}
}

// MoveColumnFocus shifts primary/secondary column focus (sticky across row navigation).
func (h *Handler) MoveColumnFocus(delta int) {
	st := &h.model.CompareView
	if delta < 0 {
		st.FocusColumn = ui.CompareColumnPrimary
		return
	}
	if delta > 0 {
		st.FocusColumn = ui.CompareColumnSecondary
	}
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
	row := rows[st.Selected]
	snap := h.model.CompareSnapshot
	var rel string
	root := snap.PrimaryRoot
	panelID = ui.PrimaryPanel
	switch st.FocusColumn {
	case ui.CompareColumnSecondary:
		rel = row.SecondaryRel
		root = snap.SecondaryRoot
		panelID = ui.SecondaryPanel
	default:
		rel = row.PrimaryRel
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

// SelectedColumnPinTarget returns the absolute path under the focused column, for pinning.
// Compare rows are always files. False when nothing is selected or the focused side has no
// counterpart entry.
func (h *Handler) SelectedColumnPinTarget() (path string, ok bool) {
	abs, _, ok, err := h.selectedColumnTarget()
	if !ok || err != nil {
		return "", false
	}
	return abs.String(), true
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
		_ = h.navigateSide(ui.PrimaryPanel, snap.PrimaryRoot, row.PrimaryRel)
		return
	}
	if row.SecondaryRel != "" {
		_ = h.navigateSide(ui.SecondaryPanel, snap.SecondaryRoot, row.SecondaryRel)
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
