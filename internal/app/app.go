// Package app wires together UI, state, and services for the TUI application.
package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

// statusMessageExpiryPayload is posted through tcell when a transient status banner TTL elapses.
type statusMessageExpiryPayload struct {
	gen uint64
}

// spinnerTickPayload is posted periodically to animate the menu-bar activity spinner.
type spinnerTickPayload struct{}

// pathPickerValidatePayload wakes PollEvent after debounced path-picker filter validation.
type pathPickerValidatePayload struct{}

// transferDestValidatePayload wakes PollEvent after debounced copy/move destination path validation.
type transferDestValidatePayload struct{}

// diskIdleSortPayload applies deferred disk-total sort for one panel after idle delay.
type diskIdleSortPayload struct {
	PanelID int
	Epoch   uint64
}

// diskUsageRedrawPayload flushes debounced disk-usage cache/paint updates while a scan is busy.
type diskUsageRedrawPayload struct{}

type diskIdleSortPanel struct {
	timer *time.Timer
	epoch uint64
}

// jobsWakePayload wakes PollEvent so job channel updates can drain and repaint.
type jobsWakePayload struct{}

// App owns lifecycle, state, and input dispatch.
type App struct {
	screen             tcell.Screen
	config             config.Config
	styles             theme.Theme
	themes             map[string]theme.Theme
	paths              config.Paths
	keys               *keymap.Map
	keysJobs           *keymap.Map // chords active only in jobs view (overlay)
	keysCommands       *keymap.Map // chords active only in Commands view (overlay)
	keysPathPickerHost *keymap.Map // copy/move dest + symlink/hardlink path-picker host overlay
	model              ui.Model
	// themeAtDialogOpen is the active theme when the theme dialog was opened; Esc restores it after preview.
	themeAtDialogOpen theme.Theme
	// jobState manages background job queue and worker lifecycle.
	jobState        *jobs.State
	jobStopCh       chan struct{}
	jobStopOnce     bool
	diskUsage       *diskusage.Engine
	diskUsageIgnore diskusage.ShouldIgnoreFolder
	diskIdleSort    [2]diskIdleSortPanel // indexed by ui.LeftPanel / ui.RightPanel (0/1)
	// diskIdleNavPath records last reconciled panel cwd so idle-sort debounce survives benign reconcile but resets on chdir.
	diskIdleNavPath [2]string
	// metaActiveCmd holds the name of the active meta command per panel (empty = none).
	metaActiveCmd [2]string
	// metaNavPath holds the last panel path for which meta was run (used to detect navigation).
	metaNavPath [2]string
	// messageExpiryGen increments whenever the transient message or its schedule changes;
	// scheduled expirations carry the generation and are ignored if stale.
	messageExpiryGen          atomic.Uint64
	spinnerRedrawTimer        *time.Timer
	diskUsageRedrawTimer      *time.Timer
	jobsWakeMu                sync.Mutex
	jobsWakeTimer             *time.Timer
	pathPickerValidateTimer   *time.Timer
	transferDestValidateTimer *time.Timer
	// pathPickerValidateGen / transferDestValidateGen invalidate debounced path checks when input
	// changes or the host dialog closes before the timer fires (avoids stale AfterFunc callbacks).
	pathPickerValidateGen   atomic.Uint64
	transferDestValidateGen atomic.Uint64

	commandsMu              sync.RWMutex
	commandsBatchesInflight atomic.Int32
	commandsCtx             context.Context
	commandsCancel          context.CancelFunc
}

// Options controls app construction while keeping startup behavior testable.
type Options struct {
	CWD          func() (string, error)
	Config       config.Config
	Theme        theme.Theme
	ThemeChoices []theme.NamedTheme
	Paths        config.Paths
	Keymap       *keymap.Map    // optional global-only override for tests (ignored when Bundle set)
	KeymapBundle *keymap.Bundle // optional full bundle override for tests
}

// Run initializes and starts the terminal application.
func Run() error {
	paths, err := config.DefaultPaths()
	if err != nil {
		return err
	}
	cfg, err := config.LoadFromPaths(paths)
	if err != nil {
		return err
	}
	styles, themeErr := theme.Resolve(cfg.Theme, paths.ThemesDir)
	themeChoices, choicesErr := theme.ThemeChoices(paths.ThemesDir)
	if choicesErr != nil && themeErr == nil {
		themeErr = choicesErr
	}
	screen, err := tcell.NewScreen()
	if err != nil {
		return fmt.Errorf("create screen: %w", err)
	}
	if err := screen.Init(); err != nil {
		return fmt.Errorf("initialize screen: %w", err)
	}
	defer screen.Fini()
	app, err := NewWithOptions(screen, Options{
		CWD:          os.Getwd,
		Config:       cfg,
		Theme:        styles,
		ThemeChoices: themeChoices,
		Paths:        paths,
	})
	if err != nil {
		return err
	}
	if themeErr != nil {
		app.setTransientMessage(themeErr.Error(), ui.MessageUrgencyError)
	}
	return app.Run()
}

// New creates an App. The cwd function keeps startup state easy to test.
func New(screen tcell.Screen, cwd func() (string, error)) (*App, error) {
	return NewWithOptions(screen, Options{CWD: cwd, Config: config.Default()})
}

// NewWithOptions creates an App with explicit startup options.
func NewWithOptions(screen tcell.Screen, opts Options) (*App, error) {
	cwd := opts.CWD
	if cwd == nil {
		cwd = os.Getwd
	}
	cfg := opts.Config
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate app config: %w", err)
	}
	var bundle *keymap.Bundle
	switch {
	case opts.KeymapBundle != nil:
		bundle = opts.KeymapBundle
	case opts.Keymap != nil:
		bundle = &keymap.Bundle{Global: opts.Keymap}
	default:
		var err error
		bundle, err = keymap.LoadFromPaths(opts.Paths)
		if err != nil {
			return nil, fmt.Errorf("load keybindings: %w", err)
		}
	}
	if bundle.Global == nil {
		return nil, fmt.Errorf("load keybindings: global keymap is nil")
	}
	km := bundle.Global
	kmJobs := bundle.Jobs
	kmCommands := bundle.Commands
	kmPathPickerHost := bundle.PathPickerHost
	styles := opts.Theme
	if styles.Name == "" {
		styles = theme.Default()
	}
	themeChoices := opts.ThemeChoices
	if len(themeChoices) == 0 {
		var err error
		themeChoices, err = theme.ThemeChoices(opts.Paths.WithResolvedLocations().ThemesDir)
		if err != nil {
			return nil, fmt.Errorf("load theme choices: %w", err)
		}
	}
	availableThemes := map[string]theme.Theme{}
	for _, choice := range themeChoices {
		availableThemes[choice.Name] = choice.Theme
	}
	if _, ok := availableThemes[styles.Name]; styles.Name != "" && !ok {
		availableThemes[styles.Name] = styles
		themeChoices = append(themeChoices, theme.NamedTheme{
			Name:  styles.Name,
			Label: styles.Name,
			Theme: styles,
		})
	}
	path, err := cwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}
	sortMode, _ := panel.ParseSortMode(cfg.DefaultSort)
	listOptions := localfs.ListOptions{
		ShowHidden: cfg.ShowHidden,
	}
	duEngine := diskusage.NewWithWalkConcurrency(cfg.DiskUsageWalkConcurrency)

	left, err := panel.NewWithOptions(path, listOptions)
	if err != nil {
		return nil, err
	}
	left.Sort.Mode = sortMode
	left.Sort.Reverse = cfg.SortReverse
	left.Sort.DirectoriesFirst = cfg.DirectoriesFirst
	left.Sort.DiskUsageIdleSizeSort = cfg.DiskUsageIdleSizeSort
	left.DiskUsageIdleSortActivated = cfg.DiskUsageIdleSizeSort
	left.DiskSorter = duEngine.Size
	left.ApplySort()
	left.Filter.CaseInsensitive = cfg.CaseInsensitiveFilter
	left.Filter.CycleMatches = cfg.Filter.CycleMatches
	right, err := panel.NewWithOptions(path, listOptions)
	if err != nil {
		return nil, err
	}
	right.Sort.Mode = sortMode
	right.Sort.Reverse = cfg.SortReverse
	right.Sort.DirectoriesFirst = cfg.DirectoriesFirst
	right.Sort.DiskUsageIdleSizeSort = cfg.DiskUsageIdleSizeSort
	right.DiskUsageIdleSortActivated = cfg.DiskUsageIdleSizeSort
	right.DiskSorter = duEngine.Size
	right.ApplySort()
	right.Filter.CaseInsensitive = cfg.CaseInsensitiveFilter
	right.Filter.CycleMatches = cfg.Filter.CycleMatches
	jobState := jobs.NewState()
	jobState.SetTransferFunc(jobTransferFunc(cfg.Operations, cfg.Jobs))
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		homeDir = filepath.Clean(homeDir)
	}
	var duIgnorer diskusage.ShouldIgnoreFolder
	if homeDir != "" {
		if ig, err := diskusage.LoadGoduIgnoreBasenames(filepath.Join(homeDir, ".goduignore")); err == nil && ig != nil {
			duIgnorer = ig
		}
	}
	if duIgnorer == nil {
		duIgnorer = func(string) bool { return false }
	}
	cmdCtx, cmdCancel := context.WithCancel(context.Background())
	app := &App{
		screen:             screen,
		config:             cfg,
		styles:             styles,
		themes:             availableThemes,
		paths:              opts.Paths.WithResolvedLocations(),
		keys:               km,
		keysJobs:           kmJobs,
		keysCommands:       kmCommands,
		keysPathPickerHost: kmPathPickerHost,
		commandsCtx:        cmdCtx,
		commandsCancel:     cmdCancel,
		model: ui.Model{
			Left:                   left,
			Right:                  right,
			ActivePanel:            ui.LeftPanel,
			SelectionsPanelMaxRows: cfg.UI.SelectionsPanelMaxRows,
			HideMenuBar:            !cfg.UI.ShowMenuBar,
			ShowFileIcons:          cfg.UI.ShowFileIcons,
			UserHomeDir:            homeDir,
			DiskUsage:              duEngine,
			DiskUsageShown:         false,
			ViewMode:               ui.ViewBrowser,
			JobActivity:            make(map[string][]string),
			MenuDefinitions:        menu.BrowserDefinitions(km),
			ThemeDialog: ui.ThemeDialogState{
				CurrentName: styles.Name,
				Choices:     uiThemeChoices(themeChoices),
			},
			Menu: menu.State{
				ActiveMenu: menu.DefaultIndex(),
			},
			FooterKeys: menu.FunctionKeys,
		},
		jobState:  jobState,
		jobStopCh: make(chan struct{}),

		diskUsage:       duEngine,
		diskUsageIgnore: duIgnorer,
	}
	// OnDirectoryChange hooks are intentionally left unset: derived UI invariants
	// (panel sync, disk-usage idle-sort arming) are reconciled centrally in
	// App.reconcileAfterEvent(), which runs at the end of every Run-loop iteration.
	jobState.SetEmitHook(app.onJobEmitted)
	jobState.StartWorker(app.jobStopCh)
	return app, nil
}

// Run starts the event loop.
func (a *App) Run() error {
	a.screen.HideCursor()
	a.ensurePanelsVisible()
	a.render()
	for {
		event := a.screen.PollEvent()
		jobsDirty := a.pollJobEvents()
		didRender := false

		switch event := event.(type) {
		case *tcell.EventResize:
			a.screen.Sync()
			a.ensurePanelsVisible()
			a.render()
			didRender = true
		case *tcell.EventKey:
			quit, keyRendered := a.handleKey(event)
			if quit {
				a.stopWorker()
				return nil
			}
			if keyRendered {
				didRender = true
			}
		case *tcell.EventInterrupt:
			switch d := event.Data().(type) {
			case statusMessageExpiryPayload:
				a.applyStatusMessageExpiry(d)
				a.render()
				didRender = true
			case spinnerTickPayload:
				if a.menuBarSpinnerBusy() {
					a.model.SpinPhase++
					a.render()
					didRender = true
				}
			case diskIdleSortPayload:
				a.applyIdleDiskSort(d.PanelID, d.Epoch)
				a.render()
				didRender = true
			case diskUsageRedrawPayload:
				a.resortPanelsDiskUsageSorted()
				a.render()
				didRender = true
			case jobsWakePayload:
				if jobsDirty {
					a.render()
					didRender = true
				}
			case commandWakePayload:
				a.render()
				didRender = true
			case metaWakePayload:
				a.render()
				didRender = true
			case pathPickerValidatePayload:
				a.render()
				didRender = true
			case transferDestValidatePayload:
				a.render()
				didRender = true
			}
		}

		if jobsDirty && !didRender {
			a.render()
		}
		a.reconcileAfterEvent()
		a.pollDiskUsageUpdates()
	}
}

// ============================================================================
// Dialog key routing + handlers (copy, move, jobs, conflict, quit, rename)
// ============================================================================
func (a *App) handleDialogKey(event *tcell.EventKey) bool {
	switch {
	case a.model.TransferDialog.Open:
		a.handleTransferDialogKey(event)
		return false
	case a.model.ConflictDialog.Open:
		a.handleConflictDialogKey(event)
		return false
	case a.model.QuitConfirm.Open:
		return a.handleQuitConfirmKey(event)
	}
	return false
}

// ---------------------------------------------------------------------------
// Quit safety
// ---------------------------------------------------------------------------
func (a *App) handleQuit() bool {
	if a.hasActiveJobs() || a.hasRunningCommands() {
		a.openQuitConfirm()
		return false
	}
	return true
}
func (a *App) hasActiveJobs() bool {
	for _, j := range a.jobState.AllJobs() {
		if j.Status == jobs.StatusQueued || j.Status == jobs.StatusPaused || j.Status == jobs.StatusRunning || j.Status == jobs.StatusWaitingDecision {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Copy / Move destination dialog (shared)
// ---------------------------------------------------------------------------
func (a *App) openCopyDialog() {
	a.openTransferDialog(ui.TransferKindCopy)
}

func (a *App) openMoveDialog() {
	a.openTransferDialog(ui.TransferKindMove)
}

func absPathClean(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return filepath.Clean(p)
	}
	return filepath.Clean(abs)
}

func transferPrefilledDestination(path string) ui.FileDialogField {
	rn := len([]rune(path))
	return ui.FileDialogField{
		Value:          path,
		Prefill:        path,
		Cursor:         rn,
		PrefillPending: path != "",
	}
}

func (a *App) openTransferDialog(kind ui.TransferKind) {
	passive := a.inactivePanel()
	st := ui.TransferDialogState{
		Open:         true,
		Kind:         kind,
		Destination:  transferPrefilledDestination(passive.Path),
		DestSubFocus: ui.TransferDestSubFocusText,
		FocusField:   0, // destination path row
	}
	if kind == ui.TransferKindCopy {
		st.PreservePermissions = a.config.Operations.PreservePermissions
		st.PreserveTimestamps = a.config.Operations.PreserveTimestamps
	}
	a.model.TransferDialog = st
	a.clearTransientMessage()
	a.armTransferDestinationValidateTimer()
}

func transferSelfCopyNewNamePrefilled(base string) ui.FileDialogField {
	rn := len([]rune(base))
	return ui.FileDialogField{
		Value:          base,
		Prefill:        base,
		Cursor:         rn,
		PrefillPending: base != "",
	}
}

// openTransferDialogSelfCopyRename opens the transfer modal directly on the "new name" step (e.g. F5/F6 onto self).
func (a *App) openTransferDialogSelfCopyRename(kind ui.TransferKind, absDestDir, sourcePath string) {
	base := filepath.Base(sourcePath)
	st := ui.TransferDialogState{
		Open:                 true,
		Kind:                 kind,
		Phase:                ui.TransferPhaseSelfCopyRename,
		Destination:          ui.FileDialogField{},
		DestSubFocus:         ui.TransferDestSubFocusText,
		SelfCopyDestDir:      absDestDir,
		SelfCopyOrigBasename: base,
		SelfCopyNewName:      transferSelfCopyNewNamePrefilled(base),
		FocusField:           0,
	}
	if kind == ui.TransferKindCopy {
		st.PreservePermissions = a.config.Operations.PreservePermissions
		st.PreserveTimestamps = a.config.Operations.PreserveTimestamps
	}
	a.model.TransferDialog = st
	a.clearTransientMessage()
}

func (a *App) closeTransferDialog() {
	a.stopTransferDestinationValidateTimer()
	a.transferDestValidateGen.Add(1)
	a.model.TransferDialog = ui.TransferDialogState{}
}

func (a *App) handleTransferDialogKey(event *tcell.EventKey) {
	d := &a.model.TransferDialog
	// Alt+O = OK, Alt+C = Cancel, Alt+P = Add paused (mnemonics; must run before field edit).
	if event.Key() == tcell.KeyRune && event.Modifiers() == tcell.ModAlt {
		switch event.Rune() {
		case 'o', 'O':
			a.confirmTransfer()
			return
		case 'c', 'C':
			a.closeTransferDialog()
			return
		case 'p', 'P':
			a.confirmTransferPaused()
			return
		}
	}
	if event.Key() == tcell.KeyEsc {
		a.closeTransferDialog()
		return
	}
	if a.tryPathPickerHostShortcut(event) {
		return
	}
	if d.FocusField == 0 && d.Phase == ui.TransferPhaseSelfCopyRename {
		if editTransferSelfCopyNewNameKey(event, &d.SelfCopyNewName) {
			return
		}
	}
	if d.FocusField == 0 && d.Phase == ui.TransferPhaseDestination {
		if d.DestSubFocus == ui.TransferDestSubFocusPicker {
			switch event.Key() {
			case tcell.KeyLeft:
				d.DestSubFocus = ui.TransferDestSubFocusText
				runes := []rune(d.Destination.Value)
				d.Destination.Cursor = len(runes)
				return
			case tcell.KeyEnter:
				a.openPathPickerForTransfer()
				return
			case tcell.KeyTab, tcell.KeyBacktab, tcell.KeyDown, tcell.KeyUp:
				d.DestSubFocus = ui.TransferDestSubFocusText
			default:
				return
			}
		} else {
			switch event.Key() {
			case tcell.KeyRight:
				dest := &d.Destination
				runes := []rune(dest.Value)
				c := dest.Cursor
				if c < 0 {
					c = 0
				}
				if c > len(runes) {
					c = len(runes)
				}
				// First Right on a pending placeholder commits it; second Right at EOT moves to the glyph.
				if dest.Prefill != "" && dest.PrefillPending && dest.Value == dest.Prefill && c >= len(runes) {
					dest.CommitPrefill()
					return
				}
				if c >= len(runes) {
					d.DestSubFocus = ui.TransferDestSubFocusPicker
					return
				}
				dest.MoveCursor(1)
				return
			case tcell.KeyLeft:
				d.Destination.MoveCursor(-1)
				return
			}
		}
	}
	if focus, ok := ui.TransferDialogMoveFocus(*d, d.FocusField, event.Key()); ok {
		prev := d.FocusField
		d.FocusField = focus
		if prev == 0 && focus != 0 {
			d.DestSubFocus = ui.TransferDestSubFocusText
		}
		return
	}
	if event.Key() == tcell.KeyEnter {
		tf := ui.NewTransferDialogLinearForm(ui.TransferDialogEffectiveNumContent(*d))
		if d.Phase == ui.TransferPhaseDestination && d.FocusField == 0 && d.DestSubFocus == ui.TransferDestSubFocusText {
			a.confirmTransfer()
			return
		}
		if d.Phase == ui.TransferPhaseSelfCopyRename && d.FocusField == 0 {
			a.confirmTransfer()
			return
		}
		switch d.FocusField {
		case tf.CancelIndex():
			a.closeTransferDialog()
			return
		case tf.OKIndex():
			a.confirmTransfer()
			return
		case tf.AddPausedIndex():
			a.confirmTransferPaused()
			return
		}
	}
	if d.FocusField == 0 && d.Phase != ui.TransferPhaseSelfCopyRename {
		if editTransferSelfCopyNewNameKey(event, &d.Destination) {
			a.armTransferDestinationValidateTimer()
			return
		}
	}
	if d.Phase == ui.TransferPhaseDestination && d.Kind == ui.TransferKindCopy && event.Key() == tcell.KeyRune && event.Rune() == ' ' {
		switch d.FocusField {
		case 1:
			d.PreservePermissions = !d.PreservePermissions
		case 2:
			d.PreserveTimestamps = !d.PreserveTimestamps
		}
	}
}

func editTransferSelfCopyNewNameKey(event *tcell.EventKey, f *ui.FileDialogField) bool {
	if f == nil {
		return false
	}
	switch event.Key() {
	case tcell.KeyLeft:
		f.MoveCursor(-1)
		return true
	case tcell.KeyRight:
		f.MoveCursor(1)
		return true
	case tcell.KeyHome:
		f.MoveCursorStart()
		return true
	case tcell.KeyEnd:
		f.MoveCursorEnd()
		return true
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		f.Backspace()
		return true
	case tcell.KeyDelete:
		f.Delete()
		return true
	case tcell.KeyCtrlL:
		f.Clear()
		return true
	case tcell.KeyRune:
		if isPlainPrintableRune(event) {
			f.InsertRune(event.Rune())
			return true
		}
		return false
	default:
		return false
	}
}

func transferBasenameIssue(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "Name required"
	}
	if trimmed == "." || trimmed == ".." {
		return "Invalid name"
	}
	if filepath.Dir(trimmed) != "." {
		return "Use a single file or folder name"
	}
	return ""
}

func (a *App) confirmTransfer() {
	a.confirmTransferEnqueue(false)
}

func (a *App) confirmTransferPaused() {
	a.confirmTransferEnqueue(true)
}

func (a *App) confirmTransferEnqueue(startPaused bool) {
	d := &a.model.TransferDialog
	sources := a.activePanelSources()
	if len(sources) == 0 {
		if d.Kind == ui.TransferKindCopy {
			a.setTransientMessage("No files to copy", ui.MessageUrgencyWarn)
		} else {
			a.setTransientMessage("No files to move", ui.MessageUrgencyWarn)
		}
		return
	}

	if d.Phase == ui.TransferPhaseSelfCopyRename {
		a.confirmTransferSelfCopyRename(sources, startPaused)
		return
	}

	dest := strings.TrimSpace(d.Destination.Value)
	if dest == "" {
		a.setTransientMessage("Destination required", ui.MessageUrgencyWarn)
		return
	}
	absDest := absPathClean(dest)

	nSelf := 0
	for _, src := range sources {
		if ops.ResolvedSameAsSource(src, absDest) {
			nSelf++
		}
	}
	if nSelf > 0 {
		if len(sources) > 1 {
			a.setTransientMessage("Cannot transfer multiple items when some would overwrite themselves", ui.MessageUrgencyWarn)
			return
		}
		d.Phase = ui.TransferPhaseSelfCopyRename
		d.SelfCopyDestDir = absDest
		base := filepath.Base(sources[0])
		d.SelfCopyOrigBasename = base
		d.SelfCopyNewName = transferSelfCopyNewNamePrefilled(base)
		d.FocusField = 0
		a.stopTransferDestinationValidateTimer()
		a.transferDestValidateGen.Add(1)
		d.DestPathInvalid = false
		d.DestPathCheckPending = false
		return
	}

	var jobType jobs.Type
	switch d.Kind {
	case ui.TransferKindCopy:
		jobType = jobs.TypeCopy
	case ui.TransferKindMove:
		jobType = jobs.TypeMove
	default:
		a.closeTransferDialog()
		return
	}
	if jobType == jobs.TypeCopy && a.rejectCopyIfInsufficientDisk(sources, dest) {
		return
	}
	sourcesCopy := append([]string(nil), sources...)
	a.activePanel().ClearSelection()
	a.addTransferJob(jobType, sourcesCopy, dest, startPaused)
	a.closeTransferDialog()
	a.setTransferQueuedMessage(jobType, startPaused)
}

func (a *App) confirmTransferSelfCopyRename(sources []string, startPaused bool) {
	d := &a.model.TransferDialog
	if len(sources) != 1 {
		a.closeTransferDialog()
		return
	}
	trimmed := strings.TrimSpace(d.SelfCopyNewName.Value)
	if msg := transferBasenameIssue(trimmed); msg != "" {
		a.setTransientMessage(msg, ui.MessageUrgencyWarn)
		return
	}
	if trimmed == d.SelfCopyOrigBasename {
		a.setTransientMessage("New name must differ from the original", ui.MessageUrgencyWarn)
		return
	}

	var jobType jobs.Type
	switch d.Kind {
	case ui.TransferKindCopy:
		jobType = jobs.TypeCopy
	case ui.TransferKindMove:
		jobType = jobs.TypeMove
	default:
		a.closeTransferDialog()
		return
	}
	finalDest := filepath.Join(d.SelfCopyDestDir, trimmed)
	if jobType == jobs.TypeCopy && a.rejectCopyIfInsufficientDisk(sources, finalDest) {
		return
	}
	sourcesCopy := append([]string(nil), sources...)
	a.activePanel().ClearSelection()
	a.addTransferJob(jobType, sourcesCopy, finalDest, startPaused)
	a.closeTransferDialog()
	a.setTransferQueuedMessage(jobType, startPaused)
}

func (a *App) setTransferQueuedMessage(jobType jobs.Type, paused bool) {
	var msg string
	if jobType == jobs.TypeCopy {
		if paused {
			msg = "Copy queued (paused)"
		} else {
			msg = "Copy queued"
		}
	} else if paused {
		msg = "Move queued (paused)"
	} else {
		msg = "Move queued"
	}
	a.setTransientMessage(msg, ui.MessageUrgencyInfo)
}

// confirmCopy confirms the unified transfer dialog when opened as copy (tests).
func (a *App) confirmCopy() {
	a.confirmTransfer()
}

// ---------------------------------------------------------------------------
// Conflict dialog
// ---------------------------------------------------------------------------
func (a *App) handleConflictDialogKey(event *tcell.EventKey) {
	switch event.Key() {
	case tcell.KeyEsc:
		a.model.ConflictDialog = ui.ConflictDialogState{}
	case tcell.KeyTab:
		a.model.ConflictDialog.Focus = (a.model.ConflictDialog.Focus + 1) % 5
	case tcell.KeyBacktab:
		a.model.ConflictDialog.Focus = (a.model.ConflictDialog.Focus + 4) % 5
	case tcell.KeyLeft:
		if a.model.ConflictDialog.Focus > 0 {
			a.model.ConflictDialog.Focus--
		}
	case tcell.KeyRight:
		if a.model.ConflictDialog.Focus < 4 {
			a.model.ConflictDialog.Focus++
		}
	case tcell.KeyEnter:
		// Conflict decisions: 0=overwrite, 1=skip, 2=overwrite-all, 3=skip-all, 4=cancel
		// Our jobs engine handles these internally via the transfer func.
		// For v1, just close and re-emit via a pending decision channel (future work).
		a.model.ConflictDialog = ui.ConflictDialogState{}
	}
}

// ---------------------------------------------------------------------------
// Quit confirm dialog
// ---------------------------------------------------------------------------
func (a *App) openQuitConfirm() {
	st := ui.QuitConfirmState{Open: true, Focus: 0}
	jobs := a.hasActiveJobs()
	cmds := a.hasRunningCommands()
	switch {
	case jobs && cmds:
		st.WarnLine1 = "Active jobs or commands are running."
		st.WarnLine2 = "Quitting will interrupt background work."
	case cmds && !jobs:
		st.WarnLine1 = "Commands are still running."
		st.WarnLine2 = "Quitting will cancel running subprocesses."
	}
	a.model.QuitConfirm = st
}
func (a *App) handleQuitConfirmKey(event *tcell.EventKey) bool {
	switch event.Key() {
	case tcell.KeyEsc:
		a.model.QuitConfirm = ui.QuitConfirmState{}
		return false
	case tcell.KeyLeft:
		a.model.QuitConfirm.Focus = ui.DialogPairLeftRight(a.model.QuitConfirm.Focus, false)
		return false
	case tcell.KeyRight:
		a.model.QuitConfirm.Focus = ui.DialogPairLeftRight(a.model.QuitConfirm.Focus, true)
		return false
	case tcell.KeyEnter:
		quitAnyway := a.model.QuitConfirm.Focus == 1
		a.model.QuitConfirm = ui.QuitConfirmState{}
		return quitAnyway
	}
	return false
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
// activePanelSources returns file paths from the active panel: selected entries
// if any, otherwise the cursor entry. Returns nil if no valid sources exist.
func (a *App) activePanelSources() []string {
	source, err := ops.ResolveSource(a.activePanel())
	if err != nil {
		return nil
	}
	return ops.SourcePaths(source)
}

func (a *App) handleQuickFilterFunctionKey(event *tcell.EventKey) bool {
	viewportRows := a.activeViewportRows()
	a.activePanel().CancelFilter(viewportRows)
	if event.Key() == tcell.KeyF9 {
		a.openMenu()
		return false
	}
	label, _ := menu.FunctionKeyLabelByKey(event.Key())
	if def, item, ok := menu.FindItemByFKeyLabel(menu.ActiveDefinitions(a.model.MenuDefinitions), label); ok {
		return a.activateMenuSelection(def, item)
	}
	return false
}
func (a *App) handleMenuKey(event *tcell.EventKey) bool {
	switch event.Key() {
	case tcell.KeyEsc, tcell.KeyCtrlLeftSq:
		if a.model.Menu.PulldownOpen {
			a.model.Menu.PulldownOpen = false
		} else {
			a.closeMenu()
		}
	case tcell.KeyF9:
		a.closeMenu()
	case tcell.KeyLeft:
		a.moveMenu(-1)
	case tcell.KeyRight:
		a.moveMenu(1)
	case tcell.KeyUp:
		if a.model.Menu.PulldownOpen {
			// Up from first selectable item closes pulldown, stays on same menu.
			menus := menu.ActiveDefinitions(a.model.MenuDefinitions)
			if a.model.Menu.ActiveMenu >= 0 && a.model.Menu.ActiveMenu < len(menus) {
				firstIdx := firstSelectableMenuItem(menus[a.model.Menu.ActiveMenu])
				if a.model.Menu.SelectedItem == firstIdx {
					a.model.Menu.PulldownOpen = false
					break
				}
			}
			a.moveMenuItem(-1)
		}
	case tcell.KeyDown:
		if a.model.Menu.PulldownOpen {
			a.moveMenuItem(1)
		} else {
			a.model.Menu.PulldownOpen = true
			a.model.Menu.SelectedItem = firstSelectableMenuItem(menu.ActiveDefinitions(a.model.MenuDefinitions)[a.model.Menu.ActiveMenu])
		}
	case tcell.KeyEnter:
		if a.model.Menu.PulldownOpen {
			return a.activateMenuItem()
		}
		a.model.Menu.PulldownOpen = true
		a.model.Menu.SelectedItem = firstSelectableMenuItem(menu.ActiveDefinitions(a.model.MenuDefinitions)[a.model.Menu.ActiveMenu])
	case tcell.KeyRune:
		if event.Modifiers() == tcell.ModNone && event.Rune() == '\x1b' {
			if a.model.Menu.PulldownOpen {
				a.model.Menu.PulldownOpen = false
			} else {
				a.closeMenu()
			}
			break
		}
		if event.Modifiers() == tcell.ModAlt {
			if a.selectTopMenuShortcut(event.Rune()) {
				a.model.Menu.PulldownOpen = true
				break
			}
		}
		if a.model.Menu.PulldownOpen {
			// Pulldown open: plain letters activate pulldown item shortcuts only.
			if a.selectMenuShortcut(event.Rune()) {
				return a.activateMenuItem()
			}
		} else {
			// Menu bar active (no pulldown): plain letters open the matching
			// top menu's pulldown. Pulldown item shortcuts are not active here.
			if a.openMenuByShortcut(event.Rune()) {
				a.model.Menu.PulldownOpen = true
			}
		}
	}
	return false
}
func (a *App) openMenu() {
	menus := menu.ActiveDefinitions(a.model.MenuDefinitions)
	if a.model.Menu.ActiveMenu < 0 || a.model.Menu.ActiveMenu >= len(menus) {
		a.model.Menu.ActiveMenu = menu.DefaultIndex()
	}
	a.model.Menu.Open = true
	a.model.Menu.PulldownOpen = false
	a.model.Menu.SelectedItem = 0
}

func (a *App) openMenuByShortcut(shortcut rune) bool {
	menus := menu.ActiveDefinitions(a.model.MenuDefinitions)
	for index, def := range menus {
		if def.Shortcut != 0 && unicode.ToLower(def.Shortcut) == unicode.ToLower(shortcut) {
			a.model.Menu.ActiveMenu = index
			a.model.Menu.Open = true
			a.model.Menu.PulldownOpen = true
			a.model.Menu.SelectedItem = firstSelectableMenuItem(menus[index])
			return true
		}
	}
	return false
}
func (a *App) closeMenu() {
	a.model.Menu.Open = false
	a.model.Menu.PulldownOpen = false
	a.model.Menu.SelectedItem = 0
}
func (a *App) moveMenu(delta int) {
	menus := menu.ActiveDefinitions(a.model.MenuDefinitions)
	if len(menus) == 0 {
		return
	}
	a.model.Menu.ActiveMenu = wrap(a.model.Menu.ActiveMenu+delta, len(menus))
	if a.model.Menu.PulldownOpen {
		a.model.Menu.SelectedItem = firstSelectableMenuItem(menus[a.model.Menu.ActiveMenu])
	} else {
		a.model.Menu.SelectedItem = 0
	}
}
func (a *App) moveMenuItem(delta int) {
	menus := menu.ActiveDefinitions(a.model.MenuDefinitions)
	if a.model.Menu.ActiveMenu < 0 || a.model.Menu.ActiveMenu >= len(menus) {
		return
	}
	count := len(menus[a.model.Menu.ActiveMenu].Items)
	if count == 0 {
		a.model.Menu.SelectedItem = 0
		return
	}
	next := a.model.Menu.SelectedItem
	for range count {
		next = wrap(next+delta, count)
		if !menus[a.model.Menu.ActiveMenu].Items[next].Separator {
			a.model.Menu.SelectedItem = next
			return
		}
	}
	a.model.Menu.SelectedItem = 0
}
func (a *App) activateMenuItem() bool {
	menus := menu.ActiveDefinitions(a.model.MenuDefinitions)
	if a.model.Menu.ActiveMenu < 0 || a.model.Menu.ActiveMenu >= len(menus) {
		a.closeMenu()
		return false
	}
	mbar := menus[a.model.Menu.ActiveMenu]
	items := mbar.Items
	if a.model.Menu.SelectedItem < 0 || a.model.Menu.SelectedItem >= len(items) || items[a.model.Menu.SelectedItem].Separator {
		a.closeMenu()
		return false
	}
	quit := a.activateMenuSelection(mbar, items[a.model.Menu.SelectedItem])
	a.closeMenu()
	return quit
}
func (a *App) activateMenuSelection(def menu.Definition, item menu.Item) bool {
	if def.PanelScope != menu.PanelScopeNone {
		a.activateScopedPanelMenu(def.PanelScope, item)
		return false
	}
	switch def.ID {
	case menu.TopCommand:
		a.dispatch(item.Action)
	case menu.TopFile:
		switch item.Action {
		case keymap.ActionAppQuit:
			return a.handleQuit()
		case keymap.ActionPanelSelectGroup:
			a.openGroupSelect("select")
		case keymap.ActionPanelUnselectGroup:
			a.openGroupSelect("unselect")
		case keymap.ActionPanelInvertSelection:
			a.activePanel().InvertSelection()
			a.setTransientMessage("Selection inverted", ui.MessageUrgencyInfo)
		case keymap.ActionCopy:
			a.enqueueCopyJob()
			return false
		case keymap.ActionMove:
			p := a.activePanel()
			if len(p.SelectedPaths) == 0 {
				if entry, ok := p.CurrentEntry(); ok {
					dest := a.inactivePanel().Path
					if entry.Path == filepath.Join(dest, entry.Name) {
						a.dispatch(keymap.ActionFileRename)
						return false
					}
				}
			}
			a.enqueueMoveJob()
			return false
		default:
			a.dispatchFileMenuItem(item)
		}
	case menu.TopOptions:
		switch item.Action {
		case keymap.ActionUIOpenTheme:
			a.openThemeDialog()
		case keymap.ActionUIOpenConfig:
			a.openConfigDialog()
		default:
			a.setUnsupportedMessage(item.Label)
		}
	case menu.TopJobs:
		a.dispatch(item.Action)
	case menu.TopCommands:
		a.dispatch(item.Action)
	default:
		a.setUnsupportedMessage(item.Label)
	}
	return false
}

func (a *App) activateScopedPanelMenu(panelScope int, item menu.Item) {
	target := a.panelByID(panelScope)
	label := panelLabel(panelScope)
	switch item.Action {
	case keymap.ActionPanelSortDialog:
		a.openSortDialogForPanel(panelScope)
	case keymap.ActionPanelToggleHidden:
		if err := target.ToggleHidden(a.panelViewportRows(panelScope)); err != nil {
			a.setErrorMessage(label+" toggle hidden failed", err)
			return
		}
		visibility := "hidden"
		if target.ShowHidden {
			visibility = "shown"
		}
		a.setTransientMessage(fmt.Sprintf("%s hidden files %s", label, visibility), ui.MessageUrgencyInfo)
	case keymap.ActionPanelRefresh:
		if err := target.Refresh(a.panelViewportRows(panelScope)); err != nil {
			a.setErrorMessage(label+" refresh failed", err)
			return
		}
		a.setTransientMessage(label+" refreshed", ui.MessageUrgencyInfo)
	case keymap.ActionPanelDiskUsageScan:
		a.startDiskUsageScanForPanel(panelScope)
	case keymap.ActionPanelHistoryDialog:
		a.openHistoryDialog(panelScope)
	case keymap.ActionPanelExternalBrowser:
		a.openPanelPathInExternalBrowser(panelScope)
	case keymap.ActionPanelMeta:
		a.openMetaDialog(panelScope)
	default:
		a.setUnsupportedMessage(item.Label)
	}
}
func (a *App) openMessageDialog(title, message string) {
	a.model.MessageDialog.Title = title
	a.model.MessageDialog.Message = message
	a.model.MessageDialog.TwoButtons = false
	a.model.MessageDialog.ButtonFocus = 0
	a.model.MessageDialog.Open = true
}

func (a *App) openMessageDialogTwoButton(title, message string) {
	a.model.MessageDialog.Title = title
	a.model.MessageDialog.Message = message
	a.model.MessageDialog.TwoButtons = true
	a.model.MessageDialog.ButtonFocus = 0
	a.model.MessageDialog.Open = true
}

func (a *App) closeMessageDialog() {
	a.model.MessageDialog.Open = false
	a.model.MessageDialog.Title = ""
	a.model.MessageDialog.Message = ""
	a.model.MessageDialog.TwoButtons = false
	a.model.MessageDialog.ButtonFocus = 0
}

func (a *App) handleMessageDialogKey(event *tcell.EventKey) {
	d := &a.model.MessageDialog
	if event.Key() == tcell.KeyRune && event.Modifiers() == tcell.ModAlt {
		switch event.Rune() {
		case 'o', 'O':
			a.closeMessageDialog()
		case 'c', 'C':
			if d.TwoButtons {
				a.closeMessageDialog()
			}
		}
		return
	}
	if d.TwoButtons {
		switch event.Key() {
		case tcell.KeyLeft:
			if d.ButtonFocus > 0 {
				d.ButtonFocus--
			}
			return
		case tcell.KeyRight:
			if d.ButtonFocus < 1 {
				d.ButtonFocus++
			}
			return
		case tcell.KeyEsc:
			a.closeMessageDialog()
			return
		case tcell.KeyEnter:
			a.closeMessageDialog()
			return
		default:
			return
		}
	}
	switch event.Key() {
	case tcell.KeyEsc, tcell.KeyEnter:
		a.closeMessageDialog()
	}
}

func (a *App) handleThemeDialogKey(event *tcell.EventKey) {
	// Alt+O = OK, Alt+C = Cancel
	if event.Key() == tcell.KeyRune && event.Modifiers() == tcell.ModAlt {
		switch event.Rune() {
		case 'o', 'O':
			a.activateThemeDialogSelection()
			return
		case 'c', 'C':
			a.styles = a.themeAtDialogOpen
			a.closeThemeDialog()
			return
		}
	}

	switch event.Key() {
	case tcell.KeyEsc, tcell.KeyF9:
		a.styles = a.themeAtDialogOpen
		a.closeThemeDialog()
	case tcell.KeyF5:
		a.previewThemeAtSelection()
	case tcell.KeyEnter:
		switch a.model.ThemeDialog.Focus {
		case 2: // Cancel
			a.styles = a.themeAtDialogOpen
			a.closeThemeDialog()
		default: // list or OK
			a.activateThemeDialogSelection()
		}
	case tcell.KeyTab:
		a.model.ThemeDialog.Focus = (a.model.ThemeDialog.Focus + 1) % 3
	case tcell.KeyBacktab:
		a.model.ThemeDialog.Focus = (a.model.ThemeDialog.Focus + 2) % 3
	case tcell.KeyUp:
		switch a.model.ThemeDialog.Focus {
		case 0:
			a.moveThemeDialog(-1)
		default:
			a.model.ThemeDialog.Focus = 0 // Up from buttons goes to list
		}
	case tcell.KeyDown:
		switch a.model.ThemeDialog.Focus {
		case 0:
			a.moveThemeDialog(1)
		case 1:
			a.model.ThemeDialog.Focus = 2 // OK -> Cancel
		}
		// Down from Cancel stays on Cancel (no wrap)
	case tcell.KeyHome:
		if a.model.ThemeDialog.Focus == 0 && len(a.model.ThemeDialog.Choices) > 0 {
			a.model.ThemeDialog.Selected = 0
			a.previewThemeAtSelection()
		}
	case tcell.KeyEnd:
		if a.model.ThemeDialog.Focus == 0 && len(a.model.ThemeDialog.Choices) > 0 {
			a.model.ThemeDialog.Selected = len(a.model.ThemeDialog.Choices) - 1
			a.previewThemeAtSelection()
		}
	case tcell.KeyPgUp:
		if a.model.ThemeDialog.Focus == 0 && len(a.model.ThemeDialog.Choices) > 0 {
			a.moveThemeDialog(-a.themeDialogListViewportRows())
		}
	case tcell.KeyPgDn:
		if a.model.ThemeDialog.Focus == 0 && len(a.model.ThemeDialog.Choices) > 0 {
			a.moveThemeDialog(a.themeDialogListViewportRows())
		}
	case tcell.KeyLeft:
		switch a.model.ThemeDialog.Focus {
		case 1:
			a.model.ThemeDialog.Focus = 0 // Left from OK goes to list
		case 2:
			a.model.ThemeDialog.Focus = 1 // Left from Cancel goes to OK
		}
	case tcell.KeyRight:
		if a.model.ThemeDialog.Focus == 1 {
			a.model.ThemeDialog.Focus = 2 // Right from OK goes to Cancel
		}
		// Right from Cancel: stay
	case tcell.KeyRune:
		if event.Modifiers() != tcell.ModNone {
			break
		}
		switch event.Rune() {
		case 'o', 'O':
			a.activateThemeDialogSelection()
		case 'c', 'C':
			a.styles = a.themeAtDialogOpen
			a.closeThemeDialog()
		case ' ':
			switch a.model.ThemeDialog.Focus {
			case 0:
				a.activateThemeDialogSelection()
			case 1:
				a.activateThemeDialogSelection()
			case 2:
				a.styles = a.themeAtDialogOpen
				a.closeThemeDialog()
			}
		}
	}
}
func (a *App) openThemeDialog() {
	if len(a.model.ThemeDialog.Choices) == 0 {
		a.setTransientMessage("No themes available", ui.MessageUrgencyWarn)
		return
	}
	a.closeMessageDialog()
	a.themeAtDialogOpen = a.styles
	a.model.ThemeDialog.Selected = a.currentThemeChoiceIndex()
	a.model.ThemeDialog.Focus = 0
	a.model.ThemeDialog.Open = true
	a.clearTransientMessage()
	a.previewThemeAtSelection()
}
func (a *App) closeThemeDialog() {
	a.model.ThemeDialog.Open = false
}
func (a *App) moveThemeDialog(delta int) {
	count := len(a.model.ThemeDialog.Choices)
	if count == 0 {
		a.model.ThemeDialog.Selected = 0
		return
	}
	a.model.ThemeDialog.Selected = wrap(a.model.ThemeDialog.Selected+delta, count)
	a.previewThemeAtSelection()
}

func (a *App) themeDialogListViewportRows() int {
	w, h := a.screen.Size()
	layout := a.layoutForTerminalSize(w, h)
	if layout.TooSmall {
		return 1
	}
	return ui.ThemeDialogListViewportRows(layout, len(a.model.ThemeDialog.Choices))
}
func (a *App) previewThemeAtSelection() {
	if !a.model.ThemeDialog.Open {
		return
	}
	choices := a.model.ThemeDialog.Choices
	sel := a.model.ThemeDialog.Selected
	if sel < 0 || sel >= len(choices) {
		return
	}
	name := choices[sel].Name
	next, err := theme.Resolve(name, a.paths.ThemesDir)
	if err != nil {
		a.setTransientMessage(firstMessageLine(err.Error()), ui.MessageUrgencyCritical)
		if cached, ok := a.themes[name]; ok {
			a.styles = cached
		}
		return
	}
	a.closeMessageDialog()
	a.clearTransientMessage()
	a.styles = next
	a.themes[name] = next
}
func (a *App) activateThemeDialogSelection() {
	choices := a.model.ThemeDialog.Choices
	selected := a.model.ThemeDialog.Selected
	if selected < 0 || selected >= len(choices) {
		a.closeThemeDialog()
		return
	}
	if !a.applyTheme(choices[selected].Name) {
		return
	}
	a.closeThemeDialog()
}
func (a *App) currentThemeChoiceIndex() int {
	currentName := a.model.ThemeDialog.CurrentName
	if currentName == "" {
		currentName = a.styles.Name
	}
	for index, choice := range a.model.ThemeDialog.Choices {
		if choice.Name == currentName {
			return index
		}
	}
	return 0
}
func (a *App) selectTopMenuShortcut(shortcut rune) bool {
	menus := menu.ActiveDefinitions(a.model.MenuDefinitions)
	for index, def := range menus {
		if def.Shortcut != 0 && unicode.ToLower(def.Shortcut) == unicode.ToLower(shortcut) && index != a.model.Menu.ActiveMenu {
			a.model.Menu.ActiveMenu = index
			a.model.Menu.SelectedItem = firstSelectableMenuItem(menus[index])
			return true
		}
	}
	return false
}

func (a *App) selectMenuShortcut(shortcut rune) bool {
	menus := menu.ActiveDefinitions(a.model.MenuDefinitions)
	if a.model.Menu.ActiveMenu < 0 || a.model.Menu.ActiveMenu >= len(menus) {
		return false
	}
	for index, item := range menus[a.model.Menu.ActiveMenu].Items {
		if item.Separator || item.Shortcut == 0 {
			continue
		}
		if unicode.ToLower(item.Shortcut) == unicode.ToLower(shortcut) {
			a.model.Menu.SelectedItem = index
			return true
		}
	}
	return false
}
func (a *App) applyTheme(name string) bool {
	nextTheme, err := theme.Resolve(name, a.paths.ThemesDir)
	if err != nil {
		a.openMessageDialog("Theme failed", err.Error())
		return false
	}
	a.styles = nextTheme
	a.themes[name] = nextTheme
	a.config.Theme = name
	a.model.ThemeDialog.CurrentName = name
	msg := fmt.Sprintf("Theme changed to %s", name)
	urgency := ui.MessageUrgencyInfo
	if err := a.persistPartial(map[string]interface{}{"theme": name}); err != nil {
		msg = fmt.Sprintf("%s (config save failed: %v)", msg, err)
		urgency = ui.MessageUrgencyWarn
	}
	a.setTransientMessage(msg, urgency)
	return true
}
func (a *App) persistPartial(patch map[string]interface{}) error {
	if !a.paths.CanPersist() {
		return nil
	}
	return config.WriteMergedPartial(a.paths, patch)
}
func uiThemeChoices(choices []theme.NamedTheme) []ui.ThemeChoice {
	result := make([]ui.ThemeChoice, 0, len(choices))
	for _, choice := range choices {
		result = append(result, ui.ThemeChoice{Name: choice.Name, Label: choice.Label})
	}
	return result
}
func (a *App) clearTransientMessage() {
	a.messageExpiryGen.Add(1)
	a.model.Message = ""
	a.model.MessageUrgency = ui.MessageUrgencyInfo
}

func (a *App) statusMessageTTL() time.Duration {
	sec := a.config.UI.StatusMessageTTLSeconds
	if sec <= 0 {
		return 0
	}
	return time.Duration(sec * float64(time.Second))
}

func (a *App) setTransientMessage(msg string, urgency ui.MessageUrgency) {
	if strings.TrimSpace(msg) == "" {
		a.clearTransientMessage()
		return
	}
	gen := a.messageExpiryGen.Add(1)
	a.model.Message = msg
	a.model.MessageUrgency = urgency
	ttl := a.statusMessageTTL()
	if ttl <= 0 {
		return
	}
	go func(g uint64, d time.Duration) {
		time.Sleep(d)
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(statusMessageExpiryPayload{gen: g}))
	}(gen, ttl)
}

func (a *App) applyStatusMessageExpiry(p statusMessageExpiryPayload) {
	if p.gen != a.messageExpiryGen.Load() {
		return
	}
	if strings.TrimSpace(a.model.Message) == "" {
		return
	}
	a.model.Message = ""
	a.model.MessageUrgency = ui.MessageUrgencyInfo
}

func (a *App) setUnsupportedMessage(label string) {
	a.setTransientMessage(label+" is not implemented yet", ui.MessageUrgencyWarn)
}
func (a *App) setErrorMessage(prefix string, err error) {
	if err == nil {
		return
	}
	if short := transientErrorText(err); short != err.Error() {
		a.setTransientMessage(short, ui.MessageUrgencyError)
		return
	}
	a.setTransientMessage(fmt.Sprintf("%s: %v", prefix, err), ui.MessageUrgencyError)
}

// transientErrorText maps common filesystem errors to compact status-banner text.
func transientErrorText(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, fs.ErrPermission) {
		return "permission denied"
	}
	return err.Error()
}

const jobFailureBannerMaxRunes = 72

// jobFailureBannerDetail returns short text for the status banner on job failure.
// Full detail remains on the job record (jobs panel).
func jobFailureBannerDetail(err error, fallback string) string {
	if err != nil {
		if short := transientErrorText(err); short != err.Error() {
			return short
		}
		return truncateStatusBannerRunes(firstMessageLine(err.Error()), jobFailureBannerMaxRunes)
	}
	line := firstMessageLine(fallback)
	if strings.Contains(strings.ToLower(line), "permission denied") {
		return "permission denied"
	}
	return truncateStatusBannerRunes(line, jobFailureBannerMaxRunes)
}

func truncateStatusBannerRunes(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	if maxRunes <= 0 || s == "" {
		return s
	}
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return strings.TrimSpace(string(r[:maxRunes])) + "…"
}

// firstMessageLine returns the first non-empty line of s (after trim), for errors that join
// multiple messages with newlines.
func firstMessageLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	for _, part := range strings.Split(s, "\n") {
		line := strings.TrimSpace(part)
		if line != "" {
			return line
		}
	}
	return s
}

func firstSelectableMenuItem(menuDefinition menu.Definition) int {
	for index, item := range menuDefinition.Items {
		if !item.Separator {
			return index
		}
	}
	return 0
}
func wrap(value, count int) int {
	if count <= 0 {
		return 0
	}
	value %= count
	if value < 0 {
		value += count
	}
	return value
}
func (a *App) switchPanel() {
	if a.model.ActivePanel == ui.LeftPanel {
		a.model.ActivePanel = ui.RightPanel
	} else {
		a.model.ActivePanel = ui.LeftPanel
	}
	a.model.ActiveSubFocus = ui.SubFocusFileList
}
func (a *App) reloadActive(successMessage string) {
	if a.model.ActivePanel == ui.LeftPanel {
		if err := a.model.Left.Refresh(a.activeViewportRows()); err != nil {
			a.setErrorMessage("Refresh failed", err)
			return
		}
		a.setTransientMessage(successMessage, ui.MessageUrgencyInfo)
		return
	}
	if err := a.model.Right.Refresh(a.activeViewportRows()); err != nil {
		a.setErrorMessage("Refresh failed", err)
		return
	}
	a.setTransientMessage(successMessage, ui.MessageUrgencyInfo)
}
func (a *App) activePanel() *panel.State {
	if a.model.ActivePanel == ui.LeftPanel {
		return &a.model.Left
	}
	return &a.model.Right
}
func (a *App) panelByID(panelID int) *panel.State {
	if panelID == ui.LeftPanel {
		return &a.model.Left
	}
	return &a.model.Right
}
func (a *App) activeViewportRows() int {
	return a.panelViewportRows(a.model.ActivePanel)
}
func (a *App) panelViewportRows(panelID int) int {
	width, height := a.screen.Size()
	layout := a.layoutForTerminalSize(width, height)
	if layout.TooSmall {
		return 0
	}
	col := layout.Left
	stripN := a.model.Left.SelectionsStripCount()
	if panelID == ui.RightPanel {
		col = layout.Right
		stripN = a.model.Right.SelectionsStripCount()
	}
	fileCol, _ := ui.SplitPanelColumn(col, stripN, a.model.SelectionsPanelMaxRows, 3)
	return ui.PanelListRows(fileCol)
}

func (a *App) selectionsStripViewportRows(panelID int) int {
	width, height := a.screen.Size()
	layout := a.layoutForTerminalSize(width, height)
	if layout.TooSmall {
		return 0
	}
	col := layout.Left
	stripN := a.model.Left.SelectionsStripCount()
	if panelID == ui.RightPanel {
		col = layout.Right
		stripN = a.model.Right.SelectionsStripCount()
	}
	_, stripCol := ui.SplitPanelColumn(col, stripN, a.model.SelectionsPanelMaxRows, 3)
	return ui.SelectionsStripListRows(stripCol)
}

func (a *App) toggleSelectionsStripFocus() {
	if a.model.ActiveSubFocus == ui.SubFocusSelectionsStrip {
		a.model.ActiveSubFocus = ui.SubFocusFileList
		return
	}
	if a.activePanel().SelectionsStripCount() > 0 {
		a.model.ActiveSubFocus = ui.SubFocusSelectionsStrip
		a.activePanel().EnsureSelectionsStripCursorVisible(a.selectionsStripViewportRows(a.model.ActivePanel))
	}
}

// navigateFromSelectionsStrip opens the directory for the highlighted strip path in the active panel
// (the directory itself if the path is a directory, otherwise its parent) and focuses the file list.
func (a *App) navigateFromSelectionsStrip() {
	p := a.activePanel()
	selPath, ok := p.SelectedPathAtStripIndex(p.SelectionsStripCursor)
	if !ok {
		return
	}
	abs := filepath.Clean(selPath)
	info, err := os.Stat(abs)
	if err != nil {
		a.setErrorMessage("Cannot open path", err)
		return
	}
	var dirToLoad string
	var selectName string
	if info.IsDir() {
		dirToLoad = abs
	} else {
		dirToLoad = filepath.Clean(filepath.Dir(abs))
		selectName = filepath.Base(abs)
	}
	vr := a.activeViewportRows()
	if err := p.NavigateTo(dirToLoad, selectName, vr); err != nil {
		a.setErrorMessage("Open failed", err)
		return
	}
	p.EnsureCursorVisible(vr)
	a.model.ActiveSubFocus = ui.SubFocusFileList
}

// navigatePanelToDirectory loads dirPath into panelID's listing and navigation history.
func (a *App) navigatePanelToDirectory(panelID int, dirPath, selectedName string) error {
	p := a.panelByID(panelID)
	vr := a.panelViewportRows(panelID)
	return p.NavigateTo(filepath.Clean(dirPath), selectedName, vr)
}

// toggleSyncFollow flips latched panel sync on the active panel with mutual exclusion:
// if the active panel already drives sync it is disabled; otherwise the other panel's sync
// (if any) is cleared first and the active panel becomes the new driver. Enabling fires
// one immediate sync hop so the follower lands on the highlighted folder right away.
func (a *App) toggleSyncFollow() {
	if a.model.ViewMode != ui.ViewBrowser {
		return
	}
	active := a.model.ActivePanel
	if a.model.SyncFollowEnabled && a.model.SyncFollowPanel == active {
		a.model.SyncFollowEnabled = false
		a.setTransientMessage("Sync disabled", ui.MessageUrgencyInfo)
		return
	}
	a.model.SyncFollowEnabled = true
	a.model.SyncFollowPanel = active
	arrow := "→"
	driver := "Left"
	follower := "Right"
	if active == ui.RightPanel {
		arrow = "←"
		driver = "Right"
		follower = "Left"
	}
	a.setTransientMessage(fmt.Sprintf("Sync: %s %s %s", driver, arrow, follower), ui.MessageUrgencyInfo)
	a.syncFollowFromActive()
}

// reconcileAfterEvent runs after each input event in Run(). It re-establishes derived
// UI invariants. Each invariant must be idempotent (a no-op when state is already
// consistent). New invariants belong here, not sprinkled at call sites: any code path
// that mutates panel state automatically triggers them via the Run-loop chokepoint.
func (a *App) reconcileAfterEvent() {
	a.syncFollowFromActive()
	a.handlePanelDirChanged(ui.LeftPanel)
	a.handlePanelDirChanged(ui.RightPanel)
	a.handleMetaPanelDirChanged(ui.LeftPanel)
	a.handleMetaPanelDirChanged(ui.RightPanel)
}

// syncFollowFromActive mirrors the active panel's highlighted directory into the inactive panel
// when the active panel drives latched sync. Uses panel.State.Load (no NavigateTo) so the
// follower's directory history is left intact. Non-directory cursor entries are silent no-ops.
// Idempotent: when the follower already sits at the target path it returns without work.
func (a *App) syncFollowFromActive() {
	if a.model.ViewMode != ui.ViewBrowser {
		return
	}
	if !a.model.SyncFollowEnabled || a.model.SyncFollowPanel != a.model.ActivePanel {
		return
	}
	entry, ok := a.activePanel().CurrentEntry()
	if !ok || entry.Type != localfs.EntryDirectory {
		return
	}
	followerID := a.inactivePanelID()
	follower := a.panelByID(followerID)
	if filepath.Clean(follower.Path) == filepath.Clean(entry.Path) {
		return
	}
	if err := follower.Load(entry.Path); err != nil {
		return
	}
	follower.EnsureCursorVisible(a.panelViewportRows(followerID))
}

// tryDispatchSelectionsStrip handles actions while the selections strip has keyboard focus.
func (a *App) tryDispatchSelectionsStrip(actionID string) bool {
	if a.model.ViewMode != ui.ViewBrowser || a.model.ActiveSubFocus != ui.SubFocusSelectionsStrip {
		return false
	}
	vr := a.selectionsStripViewportRows(a.model.ActivePanel)
	p := a.activePanel()
	switch actionID {
	case keymap.ActionNavUp:
		p.MoveSelectionsStrip(-1, vr)
	case keymap.ActionNavDown:
		p.MoveSelectionsStrip(1, vr)
	case keymap.ActionNavPageUp:
		step := vr
		if step < 1 {
			step = 1
		}
		p.MoveSelectionsStrip(-step, vr)
	case keymap.ActionNavPageDown:
		step := vr
		if step < 1 {
			step = 1
		}
		p.MoveSelectionsStrip(step, vr)
	case keymap.ActionNavTop:
		p.SelectionsStripTop(vr)
	case keymap.ActionNavBottom:
		p.SelectionsStripBottom(vr)
	case keymap.ActionPanelSelectToggle:
		p.ToggleOrRemoveStripSelection()
		if p.SelectionsStripCount() == 0 {
			a.model.ActiveSubFocus = ui.SubFocusFileList
		}
	case keymap.ActionPanelFocusSelections:
		a.toggleSelectionsStripFocus()
	case keymap.ActionPanelSwitch:
		a.switchPanel()
	case keymap.ActionAppOpenMenu:
		a.openMenu()
	case keymap.ActionNavOpen:
		a.navigateFromSelectionsStrip()
	default:
		return false
	}
	return true
}

func (a *App) ensurePanelsVisible() {
	width, height := a.screen.Size()
	layout := a.layoutForTerminalSize(width, height)
	if layout.TooSmall {
		a.model.Left.EnsureCursorVisible(0)
		a.model.Right.EnsureCursorVisible(0)
		return
	}
	leftFile, _ := ui.SplitPanelColumn(layout.Left, a.model.Left.SelectionsStripCount(), a.model.SelectionsPanelMaxRows, 3)
	rightFile, _ := ui.SplitPanelColumn(layout.Right, a.model.Right.SelectionsStripCount(), a.model.SelectionsPanelMaxRows, 3)
	a.model.Left.EnsureCursorVisible(ui.PanelListRows(leftFile))
	a.model.Right.EnsureCursorVisible(ui.PanelListRows(rightFile))
}
func panelLabel(panelID int) string {
	if panelID == ui.LeftPanel {
		return "Left panel"
	}
	return "Right panel"
}

// Sort dialog handlers
func (a *App) openSortDialog() {
	a.openSortDialogForPanel(a.model.ActivePanel)
}
func (a *App) openSortDialogForPanel(panelID int) {
	target := a.panelByID(panelID)
	a.model.SortDialog = ui.SortDialogState{
		Open:                  true,
		SortMode:              target.Sort.Mode,
		SortReverse:           target.Sort.Reverse,
		DirectoriesFirst:      target.Sort.DirectoriesFirst,
		DiskUsageIdleSizeSort: target.Sort.DiskUsageIdleSizeSort,
		Focus:                 0,
		PanelID:               panelID,
	}
}
func (a *App) closeSortDialog() {
	a.model.SortDialog.Open = false
}

func (a *App) applySortDialog() {
	target := a.panelByID(a.model.SortDialog.PanelID)
	target.ApplySortFromDialog(panel.SortState{
		Mode:                  a.model.SortDialog.SortMode,
		Reverse:               a.model.SortDialog.SortReverse,
		DirectoriesFirst:      a.model.SortDialog.DirectoriesFirst,
		DiskUsageIdleSizeSort: a.model.SortDialog.DiskUsageIdleSizeSort,
	}, a.panelViewportRows(a.model.SortDialog.PanelID))
	a.setTransientMessage(fmt.Sprintf("Sort: %s", target.Sort.Mode.String()), ui.MessageUrgencyInfo)
	a.closeSortDialog()
}
func (a *App) handleSortDialogKey(event *tcell.EventKey) {
	form := ui.NewDialogLinearForm(7)
	// Alt+O = OK, Alt+C = Cancel
	if event.Key() == tcell.KeyRune && event.Modifiers() == tcell.ModAlt {
		switch event.Rune() {
		case 'o', 'O':
			a.applySortDialog()
			return
		case 'c', 'C':
			a.closeSortDialog()
			return
		}
	}

	switch event.Key() {
	case tcell.KeyEsc, tcell.KeyF9:
		a.closeSortDialog()
	case tcell.KeyEnter:
		switch a.model.SortDialog.Focus {
		case form.CancelIndex():
			a.closeSortDialog()
		default: // OK button or any radio/checkbox -> apply
			a.applySortDialog()
		}
	case tcell.KeyRune:
		if event.Modifiers() != tcell.ModNone {
			break
		}
		switch event.Rune() {
		case 'n', 'N':
			a.model.SortDialog.SortMode = panel.SortName
			a.model.SortDialog.Focus = 0
		case 'e', 'E':
			a.model.SortDialog.SortMode = panel.SortExtension
			a.model.SortDialog.Focus = 1
		case 's', 'S':
			a.model.SortDialog.SortMode = panel.SortSize
			a.model.SortDialog.Focus = 2
		case 'm', 'M':
			a.model.SortDialog.SortMode = panel.SortMtime
			a.model.SortDialog.Focus = 3
		case 'u', 'U':
			a.model.SortDialog.DiskUsageIdleSizeSort = !a.model.SortDialog.DiskUsageIdleSizeSort
			a.model.SortDialog.Focus = 4
		case 'r', 'R':
			a.model.SortDialog.SortReverse = !a.model.SortDialog.SortReverse
			a.model.SortDialog.Focus = 5
		case 'd', 'D':
			a.model.SortDialog.DirectoriesFirst = !a.model.SortDialog.DirectoriesFirst
			a.model.SortDialog.Focus = 6
		case 'o', 'O':
			a.applySortDialog()
		case 'c', 'C':
			a.closeSortDialog()
		case ' ':
			switch a.model.SortDialog.Focus {
			case 0, 1, 2, 3:
				modes := []panel.SortMode{panel.SortName, panel.SortExtension, panel.SortSize, panel.SortMtime}
				a.model.SortDialog.SortMode = modes[a.model.SortDialog.Focus]
			case 4:
				a.model.SortDialog.DiskUsageIdleSizeSort = !a.model.SortDialog.DiskUsageIdleSizeSort
			case 5:
				a.model.SortDialog.SortReverse = !a.model.SortDialog.SortReverse
			case 6:
				a.model.SortDialog.DirectoriesFirst = !a.model.SortDialog.DirectoriesFirst
			case 7:
				a.applySortDialog()
			case 8:
				a.closeSortDialog()
			}
		}
	}
	if focus, ok := form.MoveFocus(a.model.SortDialog.Focus, event.Key()); ok {
		a.model.SortDialog.Focus = focus
	}
}

func (a *App) openConfigDialog() {
	a.clearTransientMessage()
	a.model.ConfigDialog = ui.ConfigDialogState{
		Open:          true,
		ShowFileIcons: a.config.UI.ShowFileIcons,
		Focus:         0,
	}
}
func (a *App) closeConfigDialog() {
	a.model.ConfigDialog.Open = false
}
func (a *App) applyConfigDialog() {
	val := a.model.ConfigDialog.ShowFileIcons
	a.config.UI.ShowFileIcons = val
	a.model.ShowFileIcons = val
	a.closeConfigDialog()
	msg := "Configuration saved"
	patch := map[string]interface{}{
		"ui": map[string]interface{}{
			"show_file_icons": val,
		},
	}
	if err := a.persistPartial(patch); err != nil {
		msg = fmt.Sprintf("Configuration saved (could not write config: %v)", err)
	}
	a.setTransientMessage(msg, ui.MessageUrgencyInfo)
}
func (a *App) handleConfigDialogKey(event *tcell.EventKey) {
	form := ui.NewDialogLinearForm(1)
	if event.Key() == tcell.KeyRune && event.Modifiers() == tcell.ModAlt {
		switch event.Rune() {
		case 'o', 'O':
			a.applyConfigDialog()
			return
		case 'c', 'C':
			a.closeConfigDialog()
			return
		}
	}
	switch event.Key() {
	case tcell.KeyEsc, tcell.KeyF9:
		a.closeConfigDialog()
	case tcell.KeyEnter:
		switch a.model.ConfigDialog.Focus {
		case form.CancelIndex():
			a.closeConfigDialog()
		default:
			a.applyConfigDialog()
		}
	case tcell.KeyRune:
		if event.Modifiers() != tcell.ModNone {
			break
		}
		switch event.Rune() {
		case 'f', 'F':
			a.model.ConfigDialog.ShowFileIcons = !a.model.ConfigDialog.ShowFileIcons
			a.model.ConfigDialog.Focus = 0
		case 'o', 'O':
			a.applyConfigDialog()
		case 'c', 'C':
			a.closeConfigDialog()
		case ' ':
			switch a.model.ConfigDialog.Focus {
			case 0:
				a.model.ConfigDialog.ShowFileIcons = !a.model.ConfigDialog.ShowFileIcons
			case form.OKIndex():
				a.applyConfigDialog()
			case form.CancelIndex():
				a.closeConfigDialog()
			}
		}
	}
	if focus, ok := form.MoveFocus(a.model.ConfigDialog.Focus, event.Key()); ok {
		a.model.ConfigDialog.Focus = focus
	}
}

// Group selection dialog handlers
func (a *App) openGroupSelect(mode string) {
	a.model.GroupSelect = ui.GroupSelectState{
		Open:             true,
		Text:             "",
		Mode:             mode,
		FilesOnly:        false,
		CaseSensitive:    false,
		UseShellPatterns: true,
		Focus:            0,
	}
}
func (a *App) closeGroupSelect() {
	a.model.GroupSelect.Open = false
	a.model.GroupSelect.Text = ""
}
func (a *App) executeGroupSelect() {
	gs := &a.model.GroupSelect
	if gs.Text == "" {
		return
	}
	p := a.activePanel()
	if gs.Mode == "select" {
		p.SelectGroup(gs.Text, gs.FilesOnly, gs.CaseSensitive, gs.UseShellPatterns)
		a.setTransientMessage(fmt.Sprintf("Selected matching %q", gs.Text), ui.MessageUrgencyInfo)
	} else {
		p.UnselectGroup(gs.Text, gs.FilesOnly, gs.CaseSensitive, gs.UseShellPatterns)
		a.setTransientMessage(fmt.Sprintf("Unselected matching %q", gs.Text), ui.MessageUrgencyInfo)
	}
	a.closeGroupSelect()
}
func (a *App) handleGroupSelectKey(event *tcell.EventKey) {
	gs := &a.model.GroupSelect
	form := ui.NewDialogLinearForm(4)
	switch event.Key() {
	case tcell.KeyEsc, tcell.KeyF9:
		a.closeGroupSelect()
	case tcell.KeyEnter:
		switch gs.Focus {
		case 5: // Cancel
			a.closeGroupSelect()
		default: // pattern input, checkboxes, or OK -> execute
			a.executeGroupSelect()
		}
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if gs.Focus == 0 {
			runes := []rune(gs.Text)
			if len(runes) > 0 {
				gs.Text = string(runes[:len(runes)-1])
			}
		}
	case tcell.KeyRune:
		// Mnemonics follow dialog standards: Alt+letter only (plain typing goes into the pattern).
		if event.Modifiers() == tcell.ModAlt {
			switch event.Rune() {
			case 'o', 'O':
				a.executeGroupSelect()
			case 'c', 'C':
				a.closeGroupSelect()
			case 'f', 'F':
				gs.FilesOnly = !gs.FilesOnly
				gs.Focus = 1
			case 's', 'S':
				gs.CaseSensitive = !gs.CaseSensitive
				gs.Focus = 2
			case 'u', 'U':
				gs.UseShellPatterns = !gs.UseShellPatterns
				gs.Focus = 3
			}
			break
		}
		mod := event.Modifiers()
		if mod != tcell.ModNone && mod != tcell.ModShift {
			break
		}
		if event.Rune() == ' ' {
			if gs.Focus == 0 {
				gs.Text += " "
				break
			}
			switch gs.Focus {
			case 1:
				gs.FilesOnly = !gs.FilesOnly
			case 2:
				gs.CaseSensitive = !gs.CaseSensitive
			case 3:
				gs.UseShellPatterns = !gs.UseShellPatterns
			case 4:
				a.executeGroupSelect()
			case 5:
				a.closeGroupSelect()
			}
			break
		}
		if gs.Focus == 0 && unicode.IsPrint(event.Rune()) {
			gs.Text += string(event.Rune())
		}
	}
	if focus, ok := form.MoveFocus(gs.Focus, event.Key()); ok {
		gs.Focus = focus
	}
}
func (a *App) render() {
	a.stopDiskUsageRedrawDebounce()
	a.model.MenuBarPermission = a.menuBarPermissionText()
	a.model.MenuBarJobsAttention = a.menuBarJobsAttentionText()
	a.model.MenuBarActivitySpinner = a.menuBarSpinnerBusy()
	a.model.FooterKeys = a.activeFooterKeys()
	a.model.DiskUsageDescendIntoMountPoints = a.config.DiskUsageDescendIntoMountPoints
	a.model.DiskUsageGoduIgnore = a.diskUsageIgnore
	if a.model.ViewMode == ui.ViewCommands {
		a.commandsMu.RLock()
		a.model.CommandsDisplay = append([]ui.CommandRunEntry(nil), a.model.CommandsList...)
		a.commandsMu.RUnlock()
	} else {
		a.model.CommandsDisplay = nil
	}
	ui.Render(a.screen, a.model, a.styles)
	a.armSpinnerRedrawTimer()
}

func (a *App) menuBarJobsAttentionText() string {
	n := a.jobState.JobsWaitingDecision()
	if n <= 0 {
		return ""
	}
	word := "jobs"
	if n == 1 {
		word = "job"
	}
	return fmt.Sprintf("󰋗 %d %s waiting", n, word)
}

func (a *App) menuBarPermissionText() string {
	if ui.IsAuxiliaryView(a.model.ViewMode) {
		return ""
	}
	entry, ok := a.activePanel().CurrentEntry()
	if !ok {
		return ""
	}
	return localfs.UnixModeString(entry.Mode)
}

func (a *App) layoutForTerminalSize(width, height int) ui.Layout {
	return ui.CalculateLayout(width, height, a.model.MenuBarLayoutReserved())
}

// --- File dialog methods ---
func (a *App) dispatchFileMenuItem(item menu.Item) {
	switch item.Action {
	case keymap.ActionFileDelete,
		keymap.ActionFileMkdir,
		keymap.ActionFileChmod,
		keymap.ActionFileChown,
		keymap.ActionFileSymlink,
		keymap.ActionFileHardlink,
		keymap.ActionBookmarkOpen,
		keymap.ActionFileView,
		keymap.ActionMenuFileViewPath,
		keymap.ActionMenuFileFilteredView,
		keymap.ActionFileEdit,
		keymap.ActionMenuFileRelativeSymlink,
		keymap.ActionMenuFileEditSymlink,
		keymap.ActionMenuFileAdvancedChown,
		keymap.ActionMenuFileChattr:
		a.dispatch(item.Action)
	default:
		a.setUnsupportedMessage(item.Label)
	}
}
func (a *App) handleFileDialogKey(event *tcell.EventKey) bool {
	// Alt+O = OK, Alt+C = Cancel
	if event.Key() == tcell.KeyRune && event.Modifiers() == tcell.ModAlt {
		switch event.Rune() {
		case 'o', 'O':
			a.executeFileDialog()
			return false
		case 'c', 'C':
			a.closeFileDialog()
			return false
		case 'y', 'Y':
			if a.model.FileDialog.DialogType == ui.FileDialogDelete {
				a.executeDelete()
				return false
			}
		case 'n', 'N':
			if a.model.FileDialog.DialogType == ui.FileDialogDelete {
				a.closeFileDialog()
				return false
			}
		}
	}

	if a.tryPathPickerHostShortcut(event) {
		return false
	}

	onRadio := a.fileDialogOnRadio()

	switch event.Key() {
	case tcell.KeyEsc:
		a.closeFileDialog()
		return false
	case tcell.KeyEnter:
		if a.model.FileDialog.DialogType == ui.FileDialogDelete {
			if a.model.FileDialog.FocusedField == 0 {
				a.executeDelete()
			} else {
				a.closeFileDialog()
			}
			return false
		}
		if f := a.focusedField(); f != nil && f.PathPicker && f.PickerFocused {
			a.openPathPickerForFileField(a.model.FileDialog.FocusedField)
			return false
		}
		if onRadio {
			a.selectFocusedMkdirRadio()
		}
		a.executeFileDialog()
		return false
	case tcell.KeyDown:
		a.fileDialogFocusNext()
		return false
	case tcell.KeyUp:
		a.fileDialogFocusPrev()
		return false
	case tcell.KeyLeft:
		// On button: move between buttons; on radio: no-op; on field: move cursor
		if a.fileDialogOnButton() {
			a.fileDialogFocusButton(-1)
		} else if onRadio {
			return false
		} else if f := a.focusedField(); f != nil && f.PathPicker && f.PickerFocused {
			f.PickerFocused = false
			runes := []rune(f.Value)
			f.Cursor = len(runes)
		} else {
			a.fileDialogMoveCursor(-1)
		}
		return false
	case tcell.KeyRight:
		if a.fileDialogOnButton() {
			a.fileDialogFocusButton(1)
		} else if onRadio {
			return false
		} else if f := a.focusedField(); f != nil && f.PathPicker && !f.PickerFocused {
			runes := []rune(f.Value)
			c := f.Cursor
			if c < 0 {
				c = 0
			}
			if c > len(runes) {
				c = len(runes)
			}
			if f.Prefill != "" && f.PrefillPending && f.Value == f.Prefill && c >= len(runes) {
				f.CommitPrefill()
				return false
			}
			if c >= len(runes) {
				f.PickerFocused = true
			} else {
				a.fileDialogMoveCursor(1)
			}
		} else {
			a.fileDialogMoveCursor(1)
		}
		return false
	case tcell.KeyHome:
		if onRadio {
			return false
		}
		a.fileDialogMoveCursorStart()
		return false
	case tcell.KeyEnd:
		if onRadio {
			return false
		}
		a.fileDialogMoveCursorEnd()
		return false
	case tcell.KeyTab:
		a.fileDialogFocusNext()
		return false
	case tcell.KeyBacktab:
		a.fileDialogFocusPrev()
		return false
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if onRadio {
			return false
		}
		a.fileDialogBackspace()
		return false
	case tcell.KeyDelete:
		if onRadio {
			return false
		}
		a.fileDialogDelete()
		return false
	case tcell.KeyCtrlL:
		if onRadio {
			return false
		}
		a.fileDialogClearField()
		return false
	case tcell.KeyRune:
		if onRadio {
			if isPlainPrintableRune(event) && event.Rune() == ' ' {
				a.selectFocusedMkdirRadio()
			}
			return false
		}
		if isPlainPrintableRune(event) {
			a.fileDialogInsertRune(event.Rune())
		}
		return false
	}
	return false
}

// selectFocusedMkdirRadio commits the currently focused mkdir radio row as the
// active MkdirAction. No-op when focus is not on a radio row.
func (a *App) selectFocusedMkdirRadio() {
	idx := a.fileDialogRadioIndex()
	if idx < 0 {
		return
	}
	switch idx {
	case 0:
		a.model.FileDialog.MkdirAction = ui.MkdirActionCreate
	case 1:
		a.model.FileDialog.MkdirAction = ui.MkdirActionCreateCopySelect
	case 2:
		a.model.FileDialog.MkdirAction = ui.MkdirActionCreateMoveSelect
	}
}
func (a *App) closeFileDialog() {
	a.model.FileDialog = ui.FileDialogState{}
}
func (a *App) refreshBothPanels() {
	viewportRows := a.activeViewportRows()
	_ = a.model.Left.Refresh(viewportRows)
	_ = a.model.Right.Refresh(viewportRows)
}
func (a *App) passivePanel() *panel.State {
	if a.model.ActivePanel == ui.LeftPanel {
		return &a.model.Right
	}
	return &a.model.Left
}
func (a *App) passivePanelPath() string {
	return a.passivePanel().Path
}
func (a *App) openRenameDialog(p *panel.State) {
	entry, err := ops.ResolveSourceSingle(p)
	if err != nil {
		a.setErrorMessage("Rename", err)
		return
	}
	name := entry.Name
	nameRunes := len([]rune(name))
	fields := []ui.FileDialogField{
		{Label: "Name", Value: name, Prefill: name, Cursor: nameRunes, PrefillPending: true},
	}
	a.model.FileDialog = ui.FileDialogState{
		Open:       true,
		DialogType: ui.FileDialogRename,
		Fields:     fields,
	}
}
func (a *App) openMkdirDialog() {
	p := a.activePanel()
	name := ""
	if entry, ok := p.CurrentEntry(); ok {
		name = entry.Name
	}
	cursor := 0
	if name != "" {
		cursor = len([]rune(name))
	}
	pending := name != ""
	fields := []ui.FileDialogField{
		{Label: "Directory name", Value: name, Prefill: name, Cursor: cursor, PrefillPending: pending},
	}
	a.model.FileDialog = ui.FileDialogState{
		Open:             true,
		DialogType:       ui.FileDialogMkdir,
		Fields:           fields,
		MkdirShowActions: len(p.SelectedPaths) > 0,
		MkdirAction:      ui.MkdirActionCreate,
	}
}
func (a *App) openDeleteDialog(p *panel.State) {
	source, err := ops.ResolveSource(p)
	if err != nil {
		a.setErrorMessage("Delete", err)
		return
	}
	dirCount := ops.CountDirectories(source.Entries)
	n := len(source.Entries)
	itemNoun := "items"
	if n == 1 {
		itemNoun = "item"
	}
	msg := fmt.Sprintf("Delete %d %s?", n, itemNoun)
	if dirCount > 0 {
		dirNoun := "directories"
		if dirCount == 1 {
			dirNoun = "directory"
		}
		msg += fmt.Sprintf("\nWarning: %d %s will be removed recursively!", dirCount, dirNoun)
	}
	a.model.FileDialog = ui.FileDialogState{
		Open:         true,
		DialogType:   ui.FileDialogDelete,
		Message:      msg,
		FocusedField: 1, // No (safe default); Yes stays index 0.
	}
}
func (a *App) openChmodDialog(p *panel.State) {
	_, err := ops.ResolveSource(p)
	if err != nil {
		a.setErrorMessage("Chmod", err)
		return
	}
	fields := []ui.FileDialogField{
		{Label: "Mode", Value: ""},
	}
	a.model.FileDialog = ui.FileDialogState{
		Open:       true,
		DialogType: ui.FileDialogChmod,
		Fields:     fields,
	}
	a.model.FileDialog.Fields[0].Cursor = 0
}
func (a *App) openChownDialog(p *panel.State) {
	_, err := ops.ResolveSource(p)
	if err != nil {
		a.setErrorMessage("Chown", err)
		return
	}
	fields := []ui.FileDialogField{
		{Label: "User", Value: ""},
		{Label: "Group", Value: ""},
	}
	a.model.FileDialog = ui.FileDialogState{
		Open:       true,
		DialogType: ui.FileDialogChown,
		Fields:     fields,
	}
}
func (a *App) openSymlinkDialog(p *panel.State) {
	entry, err := ops.ResolveSourceSingle(p)
	if err != nil {
		a.setErrorMessage("Symlink", err)
		return
	}
	targetPath := entry.Path
	defaultLink := filepath.Join(a.passivePanelPath(), entry.Name)
	fields := []ui.FileDialogField{
		{Label: "Target", Value: targetPath, Cursor: len([]rune(targetPath)), PathPicker: true},
		{Label: "Link path", Value: defaultLink, Cursor: len([]rune(defaultLink)), PathPicker: true},
	}
	a.model.FileDialog = ui.FileDialogState{
		Open:       true,
		DialogType: ui.FileDialogSymlink,
		Fields:     fields,
	}
}
func (a *App) openHardlinkDialog(p *panel.State) {
	entry, err := ops.ResolveSourceSingle(p)
	if err != nil {
		a.setErrorMessage("Hardlink", err)
		return
	}
	sourcePath := entry.Path
	defaultDest := filepath.Join(a.passivePanelPath(), entry.Name)
	fields := []ui.FileDialogField{
		{Label: "Source", Value: sourcePath, Cursor: len([]rune(sourcePath)), PathPicker: true},
		{Label: "New path", Value: defaultDest, Cursor: len([]rune(defaultDest)), PathPicker: true},
	}
	a.model.FileDialog = ui.FileDialogState{
		Open:       true,
		DialogType: ui.FileDialogHardlink,
		Fields:     fields,
	}
}
func (a *App) focusedField() *ui.FileDialogField {
	if a.model.FileDialog.FocusedField < 0 || a.model.FileDialog.FocusedField >= len(a.model.FileDialog.Fields) {
		return nil
	}
	return &a.model.FileDialog.Fields[a.model.FileDialog.FocusedField]
}
func (a *App) fileDialogInsertRune(r rune) {
	field := a.focusedField()
	field.InsertRune(r)
}
func (a *App) fileDialogBackspace() {
	field := a.focusedField()
	field.Backspace()
}
func (a *App) fileDialogDelete() {
	field := a.focusedField()
	field.Delete()
}
func (a *App) fileDialogClearField() {
	field := a.focusedField()
	field.Clear()
}
func (a *App) fileDialogMoveCursor(delta int) {
	field := a.focusedField()
	field.MoveCursor(delta)
}
func (a *App) fileDialogMoveCursorStart() {
	field := a.focusedField()
	field.MoveCursorStart()
}
func (a *App) fileDialogMoveCursorEnd() {
	field := a.focusedField()
	field.MoveCursorEnd()
}

// fileDialogFocusCount returns total focusable items in file dialog.
func (a *App) fileDialogFocusCount() int {
	if a.model.FileDialog.DialogType == ui.FileDialogDelete {
		return 2 // Yes, No
	}
	return len(a.model.FileDialog.Fields) + a.mkdirExtraFocusRows() + 2 // fields + (optional) mkdir radios + OK + Cancel
}

// mkdirExtraFocusRows returns the number of extra focus rows contributed by the
// mkdir-with-selections radio section, or 0 when not applicable.
func (a *App) mkdirExtraFocusRows() int {
	d := &a.model.FileDialog
	if d.DialogType == ui.FileDialogMkdir && d.MkdirShowActions {
		return 3
	}
	return 0
}

// fileDialogOnRadio returns true when focus sits on the mkdir post-action radio
// section (between text fields and the OK button).
func (a *App) fileDialogOnRadio() bool {
	d := &a.model.FileDialog
	extra := a.mkdirExtraFocusRows()
	if extra == 0 {
		return false
	}
	base := len(d.Fields)
	return d.FocusedField >= base && d.FocusedField < base+extra
}

// fileDialogRadioIndex returns the 0-based radio index when focus is on a
// mkdir radio row, or -1 otherwise.
func (a *App) fileDialogRadioIndex() int {
	if !a.fileDialogOnRadio() {
		return -1
	}
	return a.model.FileDialog.FocusedField - len(a.model.FileDialog.Fields)
}

// fileDialogOnButton returns true if current focus is on a button (not a field/radio).
func (a *App) fileDialogOnButton() bool {
	d := &a.model.FileDialog
	if d.DialogType == ui.FileDialogDelete {
		return true // delete only has buttons
	}
	return d.FocusedField >= len(d.Fields)+a.mkdirExtraFocusRows()
}

// fileDialogFocusNext moves focus to next item. Down on last button = no-op.
func (a *App) fileDialogFocusNext() {
	for i := range a.model.FileDialog.Fields {
		a.model.FileDialog.Fields[i].PickerFocused = false
	}
	count := a.fileDialogFocusCount()
	if count <= 1 {
		return
	}
	next := a.model.FileDialog.FocusedField + 1
	if next >= count {
		return // no wrap from last button
	}
	a.model.FileDialog.FocusedField = next
}

// fileDialogFocusPrev moves focus to previous item. Up on first item = no-op.
func (a *App) fileDialogFocusPrev() {
	for i := range a.model.FileDialog.Fields {
		a.model.FileDialog.Fields[i].PickerFocused = false
	}
	if a.model.FileDialog.FocusedField <= 0 {
		return // no wrap from first field
	}
	a.model.FileDialog.FocusedField--
}

// fileDialogFocusButton moves focus between buttons only (Left/Right).
func (a *App) fileDialogFocusButton(delta int) {
	d := &a.model.FileDialog
	if d.DialogType == ui.FileDialogDelete {
		// Yes(0), No(1): move between them, no wrap
		next := d.FocusedField + delta
		if next < 0 || next >= 2 {
			return
		}
		d.FocusedField = next
		return
	}
	// Fields + (optional radios) + OK + Cancel: move between OK/Cancel only
	okIdx := len(d.Fields) + a.mkdirExtraFocusRows()
	cancelIdx := okIdx + 1
	if d.FocusedField == okIdx && delta == 1 {
		d.FocusedField = cancelIdx
	} else if d.FocusedField == cancelIdx && delta == -1 {
		d.FocusedField = okIdx
	}
	// Otherwise stay
}
func (a *App) executeFileDialog() {
	switch a.model.FileDialog.DialogType {
	case ui.FileDialogRunForEach:
		a.executeRunForEach()
	case ui.FileDialogRename:
		a.executeRename()
	case ui.FileDialogMkdir:
		a.executeMkdir()
	case ui.FileDialogDelete:
		a.executeDelete()
	case ui.FileDialogChmod:
		a.executeChmod()
	case ui.FileDialogChown:
		a.executeChown()
	case ui.FileDialogSymlink:
		a.executeSymlink()
	case ui.FileDialogHardlink:
		a.executeHardlink()
	case ui.FileDialogAddBookmark:
		a.executeAddBookmark()
	default:
		a.closeFileDialog()
	}
}
func (a *App) executeRename() {
	p := a.activePanel()
	field := a.focusedField()
	if field == nil {
		a.closeFileDialog()
		return
	}
	newName := field.Value
	entry, err := ops.ResolveSourceSingle(p)
	if err != nil {
		a.setErrorMessage("Rename source", err)
		a.closeFileDialog()
		return
	}
	plan, err := ops.PlanRename(entry, newName, p.Path)
	if err != nil {
		a.setErrorMessage("Rename", err)
		a.closeFileDialog()
		return
	}
	if err := ops.ExecuteRename(plan); err != nil {
		a.setErrorMessage("Rename failed", err)
		a.closeFileDialog()
		return
	}
	a.closeFileDialog()
	a.refreshBothPanels()
	a.activePanel().SelectVisibleEntry(plan.NewName)
	a.setTransientMessage(fmt.Sprintf("Renamed to %s", plan.NewName), ui.MessageUrgencyInfo)
}
func (a *App) executeMkdir() {
	p := a.activePanel()
	d := &a.model.FileDialog
	if len(d.Fields) == 0 {
		a.closeFileDialog()
		return
	}
	input := d.Fields[0].Value
	action := ui.MkdirActionCreate
	if d.MkdirShowActions {
		action = d.MkdirAction
	}

	plan, err := ops.PlanMkdir(input, p.Path)
	if err != nil {
		a.setErrorMessage("Mkdir", err)
		a.closeFileDialog()
		return
	}

	// For copy/move post-actions, resolve sources up-front so a missing/empty
	// selection fails fast without leaving an empty directory behind.
	var sources []string
	if action == ui.MkdirActionCreateCopySelect || action == ui.MkdirActionCreateMoveSelect {
		src, srcErr := ops.ResolveSource(p)
		if srcErr != nil {
			a.setErrorMessage("Mkdir source", srcErr)
			a.closeFileDialog()
			return
		}
		if src.Kind != ops.SourceSelected {
			a.setErrorMessage("Mkdir", &ops.Error{Op: "mkdir", Text: "no files selected for transfer"})
			a.closeFileDialog()
			return
		}
		sources = ops.SourcePaths(src)
		if action == ui.MkdirActionCreateCopySelect && a.rejectCopyIfInsufficientDisk(sources, plan.Path) {
			return
		}
	}

	if err := ops.ExecuteMkdir(plan); err != nil {
		a.setErrorMessage("Mkdir failed", err)
		a.closeFileDialog()
		return
	}

	a.closeFileDialog()
	a.refreshBothPanels()
	a.activePanel().SelectVisibleEntry(filepath.Base(plan.Path))

	switch action {
	case ui.MkdirActionCreate:
		a.setTransientMessage(fmt.Sprintf("Created directory %s", plan.Name), ui.MessageUrgencyInfo)
	case ui.MkdirActionCreateCopySelect:
		a.activePanel().ClearSelection()
		a.addTransferJob(jobs.TypeCopy, sources, plan.Path, false)
		a.setTransientMessage(fmt.Sprintf("Created %s; copy queued (%d %s)", plan.Name, len(sources), plural(len(sources), "file", "files")), ui.MessageUrgencyInfo)
	case ui.MkdirActionCreateMoveSelect:
		a.activePanel().ClearSelection()
		a.addTransferJob(jobs.TypeMove, sources, plan.Path, false)
		a.setTransientMessage(fmt.Sprintf("Created %s; move queued (%d %s)", plan.Name, len(sources), plural(len(sources), "file", "files")), ui.MessageUrgencyInfo)
	}
}
func (a *App) executeDelete() {
	p := a.activePanel()
	source, err := ops.ResolveSource(p)
	if err != nil {
		a.setErrorMessage("Delete source", err)
		a.closeFileDialog()
		return
	}
	_, err = ops.PlanDelete(source, a.config.ConfirmDelete, a.config.DeleteMode)
	if err != nil {
		a.setErrorMessage("Delete", err)
		a.closeFileDialog()
		return
	}
	p.ClearSelection()
	a.closeFileDialog()
	sources := make([]string, len(source.Entries))
	for i, e := range source.Entries {
		sources[i] = e.Path
	}
	a.enqueueDeleteJob(sources)
	n := len(sources)
	delNoun := "items"
	if n == 1 {
		delNoun = "item"
	}
	a.setTransientMessage(fmt.Sprintf("Delete queued (%d %s)", n, delNoun), ui.MessageUrgencyInfo)
}
func (a *App) executeChmod() {
	p := a.activePanel()
	field := a.focusedField()
	if field == nil {
		a.closeFileDialog()
		return
	}
	source, err := ops.ResolveSource(p)
	if err != nil {
		a.setErrorMessage("Chmod source", err)
		a.closeFileDialog()
		return
	}
	plan, err := ops.PlanChmod(source, field.Value)
	if err != nil {
		a.setErrorMessage("Chmod", err)
		a.closeFileDialog()
		return
	}
	if err := ops.ExecuteChmod(plan); err != nil {
		a.setErrorMessage("Chmod failed", err)
		a.closeFileDialog()
		return
	}
	a.closeFileDialog()
	a.refreshBothPanels()
	a.setTransientMessage(fmt.Sprintf("Changed mode to %s on %d item(s)", plan.ModeStr, len(plan.Entries)), ui.MessageUrgencyInfo)
}
func (a *App) executeChown() {
	p := a.activePanel()
	if len(a.model.FileDialog.Fields) < 2 {
		a.closeFileDialog()
		return
	}
	user := a.model.FileDialog.Fields[0].Value
	group := a.model.FileDialog.Fields[1].Value
	source, err := ops.ResolveSource(p)
	if err != nil {
		a.setErrorMessage("Chown source", err)
		a.closeFileDialog()
		return
	}
	plan, err := ops.PlanChown(source, user, group)
	if err != nil {
		a.setErrorMessage("Chown", err)
		a.closeFileDialog()
		return
	}
	if err := ops.ExecuteChown(plan); err != nil {
		a.setErrorMessage("Chown failed", err)
		a.closeFileDialog()
		return
	}
	a.closeFileDialog()
	a.refreshBothPanels()
	a.setTransientMessage(fmt.Sprintf("Changed owner on %d item(s)", len(plan.Entries)), ui.MessageUrgencyInfo)
}
func (a *App) executeSymlink() {
	p := a.activePanel()
	if len(a.model.FileDialog.Fields) < 2 {
		a.closeFileDialog()
		return
	}
	target := a.model.FileDialog.Fields[0].Value
	linkPath := a.model.FileDialog.Fields[1].Value
	plan, err := ops.PlanSymlink(target, linkPath, p.Path, a.passivePanelPath())
	if err != nil {
		a.setErrorMessage("Symlink", err)
		a.closeFileDialog()
		return
	}
	if err := ops.ExecuteSymlink(plan); err != nil {
		a.setErrorMessage("Symlink failed", err)
		a.closeFileDialog()
		return
	}
	a.closeFileDialog()
	a.refreshBothPanels()
	a.setTransientMessage(fmt.Sprintf("Created symlink: %s -> %s", filepath.Base(plan.LinkPath), plan.TargetSrc), ui.MessageUrgencyInfo)
}
func (a *App) executeHardlink() {
	p := a.activePanel()
	if len(a.model.FileDialog.Fields) < 2 {
		a.closeFileDialog()
		return
	}
	source := a.model.FileDialog.Fields[0].Value
	newPath := a.model.FileDialog.Fields[1].Value
	plan, err := ops.PlanHardlink(source, newPath, p.Path, a.passivePanelPath())
	if err != nil {
		a.setErrorMessage("Hardlink", err)
		a.closeFileDialog()
		return
	}
	if err := ops.ExecuteHardlink(plan); err != nil {
		a.setErrorMessage("Hardlink failed", err)
		a.closeFileDialog()
		return
	}
	a.closeFileDialog()
	a.refreshBothPanels()
	a.setTransientMessage(fmt.Sprintf("Created hardlink: %s", filepath.Base(plan.NewPath)), ui.MessageUrgencyInfo)
}
