package compare

import (
	"context"
	"path/filepath"
	"sync/atomic"

	"github.com/gdamore/tcell/v2"
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
}

// Host is the app shell surface compare needs.
type Host interface {
	NavigatePanelToPath(panelID int, path string, selectName string) error
	TogglePanelSelection(panelID int, path string) (conflicts bool)
	SetTransientMessage(text string, urgency ui.MessageUrgency)
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

	session       *comparepkg.Session
	wakePending   atomic.Bool
	sessionCtx    context.Context
	sessionCancel context.CancelFunc
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

// Open starts comparing primary and secondary panel paths.
func (h *Handler) Open() {
	if ui.IsAuxiliaryView(h.model.ViewMode) && h.model.ViewMode != ui.ViewCompare {
		return
	}
	h.Close()
	primary := h.model.Primary.Path
	secondary := h.model.Secondary.Path
	if primary.IsZero() || secondary.IsZero() {
		h.host.SetTransientMessage("Compare: both panels need a path", ui.MessageUrgencyError)
		return
	}
	if pathloc.TreesOverlap(primary, secondary) {
		h.host.SetTransientMessage("Compare: panels must point to separate directory trees", ui.MessageUrgencyError)
		return
	}

	h.model.ViewMode = ui.ViewCompare
	h.model.MenuDefinitions = h.host.CompareMenuDefinitions()
	h.model.Menu.ActiveMenu = 0
	h.model.CompareView = ui.CompareViewState{
		Filter:      comparepkg.FilterAll,
		FocusColumn: ui.CompareColumnPrimary,
	}

	stayOnVolume := h.config.Compare.StayOnVolumeDefault
	volGate := diskusage.ListingVolumeGate{}
	if stayOnVolume {
		volGate = diskusage.ListingVolumeGate{
			Enabled: h.model.Primary.ListingDeviceValid,
			RefDev:  h.model.Primary.ListingDevice,
			Valid:   h.model.Primary.ListingDeviceValid,
		}
	}
	shouldSkip := diskusage.ComposeListingVolumeIgnore(h.diskIgnore, volGate)

	ctx, cancel := context.WithCancel(context.Background())
	h.sessionCtx = ctx
	h.sessionCancel = cancel

	bufKiB := h.config.Compare.ReadBufferKiB
	if bufKiB <= 0 {
		bufKiB = config.DefaultCompareReadBufferKiB
	}
	workers := h.config.Compare.HashConcurrency
	if workers <= 0 {
		workers = config.DefaultCompareHashConcurrency
	}

	h.session = comparepkg.Start(ctx, primary, secondary, comparepkg.Options{
		Walk: comparepkg.WalkOptions{
			ShowHidden:    h.model.Primary.ShowHidden,
			Gitignore:     h.gitignore,
			ShouldSkipDir: shouldSkip,
		},
		HashWorkers:  workers,
		ReadBuffer:   make([]byte, bufKiB*1024),
		MaxHashBytes: h.config.Compare.MaxHashBytes,
		OnUpdate:     func(_ comparepkg.Snapshot) { h.postWake() },
	})
	h.model.CompareSnapshot = h.session.Snapshot()
}

// Close cancels compare and returns to the browser.
func (h *Handler) Close() {
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
	h.wakePending.Store(false)
	if h.session != nil {
		h.model.CompareSnapshot = h.session.Snapshot()
		return true
	}
	return false
}

// Refresh re-runs compare with current panel paths.
func (h *Handler) Refresh() {
	if h.model.ViewMode != ui.ViewCompare {
		return
	}
	h.Open()
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

// FilteredRows returns snapshot rows matching the active filter.
func (h *Handler) FilteredRows() []comparepkg.Row {
	return comparepkg.FilteredRows(h.model.CompareSnapshot, h.model.CompareView.Filter)
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

// ToggleColumnSelection toggles the path under the focused column into the matching panel.
func (h *Handler) ToggleColumnSelection() (conflicts bool) {
	st := &h.model.CompareView
	rows := h.FilteredRows()
	if st.Selected < 0 || st.Selected >= len(rows) {
		return false
	}
	row := rows[st.Selected]
	snap := h.model.CompareSnapshot
	var rel string
	panelID := ui.PrimaryPanel
	switch st.FocusColumn {
	case ui.CompareColumnSecondary:
		rel = row.SecondaryRel
		panelID = ui.SecondaryPanel
	default:
		rel = row.PrimaryRel
	}
	if rel == "" {
		return false
	}
	root := snap.PrimaryRoot
	if panelID == ui.SecondaryPanel {
		root = snap.SecondaryRoot
	}
	abs, err := comparepkg.JoinRel(root, rel)
	if err != nil {
		h.host.SetTransientMessage(err.Error(), ui.MessageUrgencyError)
		return false
	}
	return h.host.TogglePanelSelection(panelID, abs.String())
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
