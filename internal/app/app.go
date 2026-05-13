// Package app wires together UI, state, and services for the TUI application.
package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/localfs"
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
	keysDialogInput    *keymap.Map // chords active only while a dialog input field is focused
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
	kmDialogInput := bundle.DialogInput
	if kmDialogInput == nil {
		m, err := keymap.Build(map[string][]string{})
		if err != nil {
			return nil, fmt.Errorf("build empty dialog input map: %w", err)
		}
		kmDialogInput = m
	}
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
	listingFormat, _ := panel.ParseListFormat(cfg.DefaultListingFormat)
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
	left.ListFormat = listingFormat
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
	right.ListFormat = listingFormat
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
		keysDialogInput:    kmDialogInput,
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

// handleDialogKey routes keys for modal overlays (transfer, conflict, quit confirm).
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
