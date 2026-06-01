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
	findctrl "github.com/paranoidi/paras-commander/internal/apphandler/find"
	jobsctrl "github.com/paranoidi/paras-commander/internal/apphandler/jobs"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/diskusage"
	_ "github.com/paranoidi/paras-commander/internal/fsbackend/file"
	_ "github.com/paranoidi/paras-commander/internal/fsbackend/sftp"
	"github.com/paranoidi/paras-commander/internal/gitignore"
	"github.com/paranoidi/paras-commander/internal/gitstatus"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/pools"
	"github.com/paranoidi/paras-commander/internal/sched"
	"github.com/paranoidi/paras-commander/internal/sshconfig"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
	"github.com/paranoidi/paras-commander/internal/workpool"
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

// syncFollowNavFlushPayload applies latched panel sync after file-list cursor debounce elapses.
type syncFollowNavFlushPayload struct {
	gen uint64
}

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
	keysMessages       *keymap.Map // chords active only in Messages view (overlay)
	keysDialogInput    *keymap.Map // chords active only while a dialog input field is focused
	keysRenameDialog   *keymap.Map // sanitize/slugify while main rename dialog is focused
	keysBookmarkDialog *keymap.Map // delete fzf-marks entry while bookmarks picker is open
	keysFindDialog     *keymap.Map // select all while find dialog is open
	devMode            bool
	model              ui.Model
	// themeAtDialogOpen is the active theme when the theme dialog was opened; Esc restores it after preview.
	themeAtDialogOpen theme.Theme
	// jobState manages background job queue and worker lifecycle.
	jobState        *jobs.State
	jobsCtrl        *jobsctrl.Handler
	findCtrl        *findctrl.Handler
	jobStopCh       chan struct{}
	jobStopOnce     bool
	diskUsage       *diskusage.Engine
	diskUsageIgnore diskusage.ShouldIgnoreFolder
	gitignoreCache  *gitignore.Cache
	gitStatusCache  *gitstatus.Cache
	diskIdleSort    [2]diskIdleSortPanel // indexed by ui.LeftPanel / ui.RightPanel (0/1)
	// diskIdleNavPath records last reconciled panel cwd so idle-sort debounce survives benign reconcile but resets on chdir.
	diskIdleNavPath [2]string
	// selectionSizeScanFP is the last enqueued directory set fingerprint per panel for selection-size scans.
	selectionSizeScanFP [2]string
	// metaActiveCmd holds the name of the active meta command per panel (empty = none).
	metaActiveCmd [2]string
	// metaNavPath holds the last panel path for which meta was run (used to detect navigation).
	metaNavPath [2]string
	// metaCache stores computed meta results by [cmdName][absPath] for entries with cache = true.
	// Nil until first caching write. Protected by metaCacheMu.
	metaCache   map[string]map[string]string
	metaCacheMu sync.RWMutex
	// messageExpiryGen increments whenever the transient message or its schedule changes;
	// scheduled expirations carry the generation and are ignored if stale.
	messageExpiryGen     atomic.Uint64
	spinnerRedrawTimer   *time.Timer
	diskUsageRedrawTimer *time.Timer
	pathPickerValidate   sched.Debouncer
	transferDestValidate sched.Debouncer
	// syncFollowNavGen invalidates in-flight debounce callbacks for latched panel sync (file-list cursor).
	syncFollowNavGen atomic.Uint64
	// syncFollowNavSkipReconcile, when true, suppresses syncFollowFromActive in reconcileAfterEvent
	// until the debounce flush runs or coalesce is cleared.
	syncFollowNavSkipReconcile atomic.Bool
	syncFollowNavMu            sync.Mutex
	syncFollowNavTimer         *time.Timer
	// quickViewDebounceGen invalidates in-flight quick view preview debounce callbacks.
	quickViewDebounceGen     atomic.Uint64
	quickViewDebounceMu      sync.Mutex
	quickViewDebounceTimer   *time.Timer
	quickViewLastFingerprint string
	// quickViewNavSkipReconcile suppresses reconcileQuickViewPreview while file-list nav coalesce
	// is holding a pending preview flush (mirrors syncFollowNavSkipReconcile).
	quickViewNavSkipReconcile atomic.Bool
	// carouselPreviewDebounceGen invalidates in-flight carousel side-preview debounce callbacks.
	carouselPreviewDebounceGen   atomic.Uint64
	carouselPreviewDebounceMu    sync.Mutex
	carouselPreviewDebounceTimer *time.Timer
	// carouselPreviewNavSkipSnapshot, when true, reuses cached carousel parent/child snapshots during render.
	carouselPreviewNavSkipSnapshot atomic.Bool
	// filePreviewRunGen invalidates in-flight preview subprocess completions (skip stale postCommandWake).
	filePreviewRunGen atomic.Uint64

	// zoomActivePanelOverride is nil → layout uses cfg.UI.ZoomActivePanel; when non-nil it forces
	// zoom on/off for this session only (Alt+z / panel.toggle-zoom-active-panel). Cleared on
	// Configuration OK so saved TOML is the sole persisted source of truth. Layout still suppresses
	// zoom while quick view / file preview is active and when terminal width ≥ cfg.UI.ZoomActivePanelDisabledAboveWidth (when > 0).
	zoomActivePanelOverride *bool

	commandsMu              sync.RWMutex
	commandsBatchesInflight atomic.Int32
	commandsCtx             context.Context
	commandsCancel          context.CancelFunc

	workPools *workpool.Registry

	volumeRefreshInFlight [2]atomic.Bool

	sftpMu                 sync.Mutex
	sftpHostKeyWait        *sftpHostKeyWait
	sftpPasswordWait       *sftpPasswordWait
	sftpConnectTargetPanel int
	sftpConnectHosts       []sshconfig.HostEntry

	remotePanelLoadGen [2]atomic.Uint64
	gitStatusLoadGen   [2]atomic.Uint64

	// lastScreenContentHash is the FNV hash of the logical buffer after the last successful Show
	// when ScreenRenderHashCache is enabled (see emitScreenAfterFullRender).
	lastScreenContentHash uint64
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
	// DevMode appends the Dev pulldown menu with test helpers (pc -dev).
	DevMode bool
}

// Run initializes and starts the terminal application.
// When devMode is true, the Dev pulldown menu is available (see pc -dev).
func Run(devMode bool) error {
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
		DevMode:      devMode,
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
	kmMessages := bundle.Messages
	kmDialogInput := bundle.DialogInput
	if kmDialogInput == nil {
		m, err := keymap.Build(map[string][]string{})
		if err != nil {
			return nil, fmt.Errorf("build empty dialog input map: %w", err)
		}
		kmDialogInput = m
	}
	kmRenameDialog := bundle.RenameDialog
	if kmRenameDialog == nil {
		m, err := keymap.Build(keymap.DefaultRenameDialogOverlayKeys())
		if err != nil {
			return nil, fmt.Errorf("build rename dialog overlay map: %w", err)
		}
		kmRenameDialog = m
	}
	kmBookmarkDialog := bundle.BookmarkDialog
	if kmBookmarkDialog == nil {
		m, err := keymap.Build(keymap.DefaultBookmarkDialogOverlayKeys())
		if err != nil {
			return nil, fmt.Errorf("build bookmark dialog overlay map: %w", err)
		}
		kmBookmarkDialog = m
	}
	kmFindDialog := bundle.FindDialog
	if kmFindDialog == nil {
		m, err := keymap.Build(keymap.DefaultFindDialogOverlayKeys())
		if err != nil {
			return nil, fmt.Errorf("build find dialog overlay map: %w", err)
		}
		kmFindDialog = m
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
	var giCache *gitignore.Cache
	if cfg.RespectGitignore {
		giCache = gitignore.NewCache()
	}
	duEngine := diskusage.NewWithWalkConcurrency(cfg.DiskUsageWalkConcurrency)

	left, err := panel.NewWithOptions(path, listOptions, giCache)
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
	right, err := panel.NewWithOptions(path, listOptions, giCache)
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
	left.CenterScrolling = cfg.UI.CenterScrolling
	right.CenterScrolling = cfg.UI.CenterScrolling
	jobState := jobs.NewState()
	jobState.SetTransferFunc(jobTransferFunc(cfg.Operations, cfg.Jobs))
	jobState.SetThroughputChart(
		time.Duration(cfg.Jobs.ThroughputChartColumnMS)*time.Millisecond,
		time.Duration(cfg.Jobs.ThroughputChartWindowSec)*time.Second,
		cfg.Jobs.ThroughputChartEnabled,
	)
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
	resolvedPaths := opts.Paths.WithResolvedLocations()
	poolDefs, err := pools.LoadGlobal(cfg, homeDir, resolvedPaths.ConfigDir)
	if err != nil {
		return nil, err
	}
	cmdCtx, cmdCancel := context.WithCancel(context.Background())
	app := &App{
		screen:             screen,
		config:             cfg,
		styles:             styles,
		themes:             availableThemes,
		paths:              resolvedPaths,
		keys:               km,
		keysJobs:           kmJobs,
		keysCommands:       kmCommands,
		keysMessages:       kmMessages,
		keysDialogInput:    kmDialogInput,
		keysRenameDialog:   kmRenameDialog,
		keysBookmarkDialog: kmBookmarkDialog,
		keysFindDialog:     kmFindDialog,
		devMode:            opts.DevMode,
		commandsCtx:        cmdCtx,
		commandsCancel:     cmdCancel,
		workPools:          workpool.NewRegistry(poolDefs),
		model: ui.Model{
			Left:                       left,
			Right:                      right,
			ActivePanel:                ui.LeftPanel,
			SelectionsPanelMaxRows:     cfg.UI.SelectionsPanelMaxRows,
			HideMenuBar:                !cfg.UI.ShowMenuBar,
			ShowFileIcons:              cfg.UI.ShowFileIcons,
			ShrunkenShowsNameOnly:      cfg.UI.ShrunkenShowsNameOnly,
			JobsThroughputChartEnabled: cfg.Jobs.ThroughputChartEnabled,
			UserHomeDir:                homeDir,
			DiskUsage:                  duEngine,
			DiskUsageShown:             false,
			ViewMode:                   ui.ViewBrowser,
			JobActivity:                make(map[string][]string),
			MenuDefinitions:            menu.BrowserDefinitions(km, opts.DevMode),
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
		gitignoreCache:  giCache,
		gitStatusCache:  gitstatus.NewCache(),
	}
	// OnDirectoryChange hooks are intentionally left unset: derived UI invariants
	// (panel sync, disk-usage idle-sort arming) are reconciled centrally in
	// App.reconcileAfterEvent(), which runs at the end of every Run-loop iteration.
	jobState.SetEmitHook(app.onJobEmitted)
	jobState.SetScanConfig(jobs.ScanConfig{
		YieldInterval:       time.Duration(cfg.Jobs.ScanYieldIntervalMS) * time.Millisecond,
		YieldEveryN:         cfg.Jobs.ScanYieldEveryN,
		NiceIncrement:       cfg.Jobs.ScanNiceIncrement,
		ProgressMinInterval: time.Duration(cfg.Jobs.ScanProgressMinIntervalMS) * time.Millisecond,
	})
	jobState.SetScanFunc(jobScanFunc())
	jobState.StartWorker(app.jobStopCh)
	suppressHeavyPathProbes := func(loc pathloc.Path) bool {
		if loc.IsRemote() {
			return true
		}
		host, err := loc.FilePath()
		if err != nil {
			return false
		}
		return app.pathVolumeContendsWithActiveJob(host)
	}
	app.model.Left.SuppressHeavyPathProbes = suppressHeavyPathProbes
	app.model.Right.SuppressHeavyPathProbes = suppressHeavyPathProbes
	app.wireFileListViewportRows()
	app.wireDiskUsageScanScopeChecks()
	app.jobsCtrl = jobsctrl.New(jobsctrl.Deps{
		Host:     jobsHost{appShellHost: appShellHost{app: app}},
		Screen:   screen,
		Model:    &app.model,
		State:    jobState,
		Config:   cfg,
		Keys:     km,
		KeysJobs: kmJobs,
	})
	app.findCtrl = findctrl.New(findctrl.Deps{
		Host:           findHost{appShellHost: appShellHost{app: app}},
		Screen:         screen,
		Model:          &app.model,
		Config:         cfg,
		Keys:           km,
		KeysFindDialog: kmFindDialog,
	})
	if err := app.configureSFTP(); err != nil {
		app.stopWorker()
		return nil, fmt.Errorf("configure sftp: %w", err)
	}
	app.wireRemotePanelLoaders()
	app.wireGitStatusLoaders()
	app.syncJobPathMarks()
	if secs := cfg.Jobs.FreeSpacePollIntervalSecs; secs > 0 {
		go app.runVolumeSpaceTicker(time.Duration(secs)*time.Second, app.jobStopCh)
	}
	if cfg.Jobs.ThroughputChartEnabled {
		go app.runThroughputChartTicker(
			time.Duration(cfg.Jobs.ThroughputChartColumnMS)*time.Millisecond,
			app.jobStopCh,
		)
	}
	return app, nil
}

// Run starts the event loop.
func (a *App) Run() error {
	a.screen.HideCursor()
	a.ensurePanelsVisible()
	a.render()
	for {
		event := a.screen.PollEvent()
		var jobsDirty bool
		var shouldRenderJobs bool
		didRender := false
		pollJobsAfter := false
		applyJobRefreshesAfter := false
		pollDiskUsageAfter := true

		switch event := event.(type) {
		case *tcell.EventResize:
			pollJobsAfter = true
			a.clearPanelSyncFollowNavCoalesce()
			a.clearQuickViewNavCoalesce()
			a.clearCarouselPreviewNavCoalesce()
			a.screen.Sync()
			a.ensurePanelsVisible()
			a.render()
			didRender = true
		case *tcell.EventKey:
			if a.model.ViewMode != ui.ViewJobs {
				a.drainDiscardProgressEvents()
			} else {
				pollJobsAfter = true
			}
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
			case jobsWakePayload:
				pollJobsAfter = true
				applyJobRefreshesAfter = true
				// Progress wakes are frequent; reconciling both panels here caused
				// extra sync/stat work and starved spinner ticks on slow mounts.
				pollDiskUsageAfter = false
			case statusMessageExpiryPayload:
				a.applyStatusMessageExpiry(d)
				a.render()
				didRender = true
			case spinnerTickPayload:
				pollDiskUsageAfter = false
				if a.menuBarSpinnerBusy() {
					a.model.SpinPhase++
					w, h := a.screen.Size()
					if ui.DrawMenuBarSpinnerOnly(a.screen, a.layoutForTerminalSize(w, h), a.model, a.styles) {
						a.emitScreenAfterPartialPaint()
					}
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
			case volumeSpaceRefreshPayload:
				pollDiskUsageAfter = false
				if a.applyVolumeSpaceRefresh(d) && a.model.ViewMode == ui.ViewJobs {
					a.render()
					didRender = true
				}
			case commandWakePayload:
				a.applyCommandWake(d)
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
			case syncFollowNavFlushPayload:
				if a.applyPanelSyncFollowNavFlush(d) {
					a.render()
					didRender = true
				}
			case quickViewFlushPayload:
				if a.applyQuickViewPreviewFlush(d) {
					a.render()
					didRender = true
				}
			case carouselPreviewFlushPayload:
				if a.applyCarouselPreviewFlush(d) {
					a.render()
					didRender = true
				}
			case findctrl.WakePayload:
				if a.pollFindUpdates(d) {
					a.renderFindDialogUpdate()
					didRender = true
				}
			case findctrl.RankWakePayload:
				if a.applyFindRank() {
					a.renderFindDialogUpdate()
					didRender = true
				}
			case findctrl.ThrottleRankWakePayload:
				if a.handleFindThrottleRankWake() {
					a.renderFindDialogUpdate()
					didRender = true
				}
			case findctrl.DebounceRankWakePayload:
				if a.handleFindDebounceRankWake() {
					a.renderFindDialogUpdate()
					didRender = true
				}
			case findctrl.FindNavIdlePayload:
				if a.handleFindNavIdle(d.Epoch) {
					a.renderFindDialogUpdate()
					didRender = true
				}
			case throughputChartTickPayload:
				pollDiskUsageAfter = false
				if a.applyThroughputChartTick() {
					a.render()
					didRender = true
				}
			case sftpConnectPayload:
				a.applySFTPConnect(d)
				didRender = true
			case sftpHostKeyOpenPayload:
				a.openHostKeyDialog(d.prompt)
				a.render()
				didRender = true
			case sftpPasswordOpenPayload:
				a.openSFTPPasswordDialog(d.prompt)
				a.render()
				didRender = true
			case remotePanelLoadPayload:
				if a.applyRemotePanelLoad(d) {
					a.render()
					didRender = true
				}
			case gitStatusPayload:
				if a.applyGitStatusLoad(d) {
					a.render()
					didRender = true
				}
			}
		}

		if pollJobsAfter {
			jobsDirty = a.pollJobEvents()
			shouldRenderJobs = jobsDirty && a.jobsCtrl.AffectVisible()
		}
		if applyJobRefreshesAfter {
			a.applyJobRefreshes()
		}
		if shouldRenderJobs && !didRender {
			if !a.jobsCtrl.LastBatchMenuBarStripOnly() || !a.paintMenuBarJobsStripOnly() {
				a.render()
			}
			didRender = true
		}
		if a.menuBarSpinnerBusy() {
			a.armSpinnerRedrawTimer()
		}
		if pollDiskUsageAfter {
			a.reconcileAfterEvent()
		}
		// Always drain disk-usage events regardless of pollDiskUsageAfter.
		// pollDiskUsageAfter only suppresses the expensive reconcileAfterEvent()
		// on high-frequency ticks (spinner, jobs progress). Without this, a
		// spinnerTickPayload arriving after scan completion leaves EventJobFinished
		// unread in the engine channel, so maybeScheduleIdleDiskSortBothPanels()
		// is never called and the idle-sort timer is never armed.
		a.pollDiskUsageUpdates()
	}
}

// handleDialogKey routes keys for modal overlays (transfer, conflict, quit confirm).
func (a *App) handleDialogKey(event *tcell.EventKey) bool {
	switch {
	case a.model.TransferDialog.Open:
		a.handleTransferDialogKey(event)
		return false
	case a.model.FlattenDialog.Open:
		a.handleFlattenDialogKey(event)
		return false
	case a.model.ConflictDialog.Open:
		a.handleConflictDialogKey(event)
		return false
	case a.model.QuitConfirm.Open:
		return a.handleQuitConfirmKey(event)
	case a.model.StashRestoreDialog.Open:
		a.handleStashRestoreDialogKey(event)
		return false
	}
	return false
}
