// Package app wires together UI, state, and services for the TUI application.
package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	comparectrl "github.com/paranoidi/paras-commander/internal/apphandler/compare"
	dedupctrl "github.com/paranoidi/paras-commander/internal/apphandler/dedup"
	findctrl "github.com/paranoidi/paras-commander/internal/apphandler/find"
	jobsctrl "github.com/paranoidi/paras-commander/internal/apphandler/jobs"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/diskusage"
	_ "github.com/paranoidi/paras-commander/internal/fsbackend/file"
	_ "github.com/paranoidi/paras-commander/internal/fsbackend/sftp"
	"github.com/paranoidi/paras-commander/internal/gitignore"
	"github.com/paranoidi/paras-commander/internal/gitstatus"
	"github.com/paranoidi/paras-commander/internal/jobbridge"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/pools"
	"github.com/paranoidi/paras-commander/internal/preview/chromastyles"
	"github.com/paranoidi/paras-commander/internal/sched"
	"github.com/paranoidi/paras-commander/internal/sshconfig"
	"github.com/paranoidi/paras-commander/internal/subshell"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
	"github.com/paranoidi/paras-commander/internal/usermenu"
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

// metaRenderFlushPayload is posted by the debounced meta render timer to coalesce frequent
// metaWakePayload renders into a single repaint (avoids one render per entry with large dirs).
type metaRenderFlushPayload struct{}

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
	screen              tcell.Screen
	config              config.Config
	styles              theme.Theme
	themes              map[string]theme.Theme
	paths               config.Paths
	keys                *keymap.Bundle // global keymap plus per-view/per-dialog overlays
	devMode             bool
	subshell            *subshell.Subshell  // persistent MC-style shell; nil until first Ctrl+O (lazy start)
	terminalFeed        *subshell.PanelFeed // emulator feed shadowing the subshell for its whole life (started by ensureSubshell)
	terminalWakePending atomic.Bool         // coalesces terminalWakePayload posts from the PTY reader
	model               ui.Model
	// themeAtDialogOpen is the active theme when the theme dialog was opened; Esc restores it after preview.
	themeAtDialogOpen theme.Theme
	// previewStyleAtPickerOpen is preview.style when the F3 Chroma style picker opens.
	previewStyleAtPickerOpen string
	// previewStylePickerDebounceGen invalidates in-flight F3 style-picker preview debounce callbacks.
	previewStylePickerDebounceGen atomic.Uint64
	previewStylePickerDebounce    sched.ManagedTimer
	// jobState manages background job queue and worker lifecycle.
	jobState        *jobs.State
	jobsCtrl        *jobsctrl.Handler
	findCtrl        *findctrl.Handler
	compareCtrl     *comparectrl.Handler
	dedupCtrl       *dedupctrl.Handler
	jobStopCh       chan struct{}
	jobStopOnce     bool
	diskUsage       *diskusage.Engine
	diskUsageIgnore diskusage.ShouldIgnoreFolder
	gitignoreCache  *gitignore.Cache
	gitStatusCache  *gitstatus.Cache
	// diskUsageScanToastArmed is set when a user-initiated disk usage scan starts and cleared
	// after the "scan finished" toast fires. Selection-size background scans do not set it, so
	// their EventJobFinished completions never trigger the toast even when DiskUsageShown is true.
	diskUsageScanToastArmed bool
	diskIdleSort            [2]diskIdleSortPanel // indexed by ui.PrimaryPanel / ui.SecondaryPanel (0/1)
	// diskIdleNavPath records last reconciled panel cwd so idle-sort debounce survives benign reconcile but resets on chdir.
	diskIdleNavPath [2]string
	// selectionSizeScanFP is the last enqueued directory set fingerprint per panel for selection-size scans.
	selectionSizeScanFP [2]string
	// selectionSizeScanGen / selectionSizeScanPath skip reconcile work when selection-derived input is unchanged.
	selectionSizeScanGen  [2]uint64
	selectionSizeScanPath [2]string
	// deleteDialogScanFP is the last enqueued directory set fingerprint for the delete confirmation dialog.
	deleteDialogScanFP string
	// deleteDialogSelGen / deleteDialogPanelPath / deleteDialogPrunedPaths skip ResolveSource while the delete dialog is open.
	deleteDialogSelGen      uint64
	deleteDialogPanelPath   string
	deleteDialogPrunedPaths []string
	// findDialogSelectionScanFP is the last enqueued directory set fingerprint for find-dialog selection-size scans.
	findDialogSelectionScanFP string
	// findDialogSelectionScanGen skips reconcile work when marked-selection derived input is unchanged.
	findDialogSelectionScanGen uint64
	// metaActiveEntries holds ordered entry names for active meta columns per panel.
	metaActiveEntries [2][]string
	// metaNavPath holds the last panel path for which meta was run (used to detect navigation).
	metaNavPath [2]string
	// metaCancel holds the cancel function for the in-flight meta run per panel (nil if none).
	metaCancel [2]context.CancelFunc
	// metaRunGen is a monotonically increasing generation counter per panel for meta runs.
	// Workers carry the generation; stale (cancelled) results are discarded by the event handler.
	metaRunGen [2]uint64
	// metaLoadGen is a monotonically increasing generation counter per panel for async meta file loads.
	// Stale loads (navigated away before load finished) are discarded by the event handler.
	metaLoadGen [2]uint64
	// metaRenderTimer debounces meta result renders; posted events call scheduleMetaRenderDebounced
	// instead of a.render() directly so burst results (large dirs) coalesce into few repaints.
	metaRenderTimer *time.Timer
	// metaCache stores computed meta results by [cmdName][absPath] for entries with cache = true.
	// Nil until first caching write. Protected by metaCacheMu.
	metaCache   map[string]map[string]string
	metaCacheMu sync.RWMutex
	// messageExpiryGen increments whenever the transient message or its schedule changes;
	// scheduled expirations carry the generation and are ignored if stale.
	messageExpiryGen     atomic.Uint64
	spinnerRedrawTimer   *time.Timer
	diskUsageRedrawTimer *time.Timer
	// deferDiskUsagePoll skips one pollDiskUsageUpdates drain after partial file-list nav while a scan is busy.
	deferDiskUsagePoll   atomic.Bool
	pathPickerValidate   sched.Debouncer
	transferDestValidate sched.Debouncer
	// syncFollowNavGen invalidates in-flight debounce callbacks for latched panel sync (file-list cursor).
	syncFollowNavGen atomic.Uint64
	// syncFollowNavSkipReconcile, when true, suppresses syncFollowFromActive in reconcileAfterEvent
	// until the debounce flush runs or coalesce is cleared.
	syncFollowNavSkipReconcile atomic.Bool
	syncFollowNav              sched.ManagedTimer
	// quickViewDebounceGen invalidates in-flight quick view preview debounce callbacks.
	quickViewDebounceGen     atomic.Uint64
	quickViewDebounce        sched.ManagedTimer
	quickViewLastFingerprint string
	// quickViewNavSkipReconcile suppresses reconcileQuickViewPreview while file-list nav coalesce
	// is holding a pending preview flush (mirrors syncFollowNavSkipReconcile).
	quickViewNavSkipReconcile atomic.Bool
	// carouselPreviewDebounceGen invalidates in-flight carousel side-preview debounce callbacks.
	carouselPreviewDebounceGen atomic.Uint64
	carouselPreviewDebounce    sched.ManagedTimer
	// debounceCalibrateRelease infers key release between calibration trials.
	debounceCalibrateRelease sched.ManagedTimer
	// navParentBackspaceGuarded, when true, suppresses nav.parent triggered by backspace.
	// Armed when backspace erases the last filter character; cleared when the key is released
	// (debounce timer fires). Prevents accidental directory navigation after erasing filter text.
	navParentBackspaceGuarded  atomic.Bool
	navParentBackspaceDebounce sched.ManagedTimer
	// carouselPreviewNavSkipSnapshot, when true, reuses cached carousel parent/child snapshots during render.
	carouselPreviewNavSkipSnapshot atomic.Bool
	// cursorNameHintNavSkip, when true, holds the previous bottom-border full-name overlay during file-list nav debounce.
	cursorNameHintNavSkip atomic.Bool
	cursorNameHintNav     sched.ManagedTimer
	// filePreviewRunGen invalidates in-flight preview subprocess completions (skip stale postCommandWake).
	filePreviewRunGen atomic.Uint64
	// filePreviewHold keeps the last completed inactive-column preview for stale-while-revalidate draws.
	filePreviewHold ui.FilePreviewState
	// fullscreenFilePreviewHold keeps the last completed F3 preview body while the next file loads.
	fullscreenFilePreviewHold ui.FilePreviewState
	// carouselFilePreviewHold keeps the last completed carousel child preview body while loading.
	carouselFilePreviewHold ui.FilePreviewState
	// carouselFilePreviewRunGen invalidates in-flight carousel child-column preview subprocess completions.
	carouselFilePreviewRunGen atomic.Uint64
	// carouselFilePreviewLastFingerprint tracks the last carousel file preview highlight for debouncing.
	carouselFilePreviewLastFingerprint string
	// previewLastWidth records the TextWidth each preview target's content was last requested at
	// (indexed by previewTarget), so a terminal resize can detect a width change and re-run the
	// preview (markdown word-wrap/table layout is baked into emitted cells at request time).
	previewLastWidth [3]int

	// zoomActivePanelOverride is nil → layout uses cfg.UI.ZoomActivePanel; when non-nil it forces
	// zoom on/off for this session only (Alt+z / panel.toggle-zoom-active-panel). Cleared on
	// Configuration OK so saved TOML is the sole persisted source of truth. Layout still suppresses
	// zoom while quick view / file preview is active and when terminal width ≥ cfg.UI.ZoomActivePanelDisabledAboveWidth (when > 0).
	zoomActivePanelOverride *bool
	// paneSplitOrientationOverride is nil → layout uses cfg.UI.PaneSplitOrientation; when non-nil it
	// forces stacked or side-by-side for this session only (panel.toggle-split-orientation). Cleared on Configuration OK.
	paneSplitOrientationOverride *ui.SplitOrientation

	commandsMu              sync.RWMutex
	commandsBatchesInflight atomic.Int32
	commandsCtx             context.Context
	commandsCancel          context.CancelFunc

	// commandProcsMu guards commandProcs, which maps a Commands-view row index to the
	// handle needed to terminate/kill its running subprocess (commands.terminate/commands.kill).
	commandProcsMu sync.Mutex
	commandProcs   map[int]*commandProcHandle

	workPools *workpool.Registry

	volumeRefreshInFlight [2]atomic.Bool
	panelRefreshInFlight  [2]atomic.Bool
	// duplicateFocus defers SelectVisibleEntryCentered until a queued duplicate job creates the entry.
	duplicateFocus duplicateFocusPending

	sftpMu                 sync.Mutex
	sftpHostKeyWait        *sftpHostKeyWait
	sftpPasswordWait       *sftpPasswordWait
	sftpConnectTargetPanel int
	sftpConnectHosts       []sshconfig.HostEntry

	remotePanelLoadGen  [2]atomic.Uint64
	gitStatusLoadGen    [2]atomic.Uint64
	quickViewGitLoadGen atomic.Uint64

	// lastScreenContentHash is the FNV hash of the logical buffer after the last successful Show
	// when ScreenRenderHashCache is enabled (see emitScreenAfterFullRender).
	lastScreenContentHash uint64
	// pendingCursor is the hardware-cursor state set by syncTerminalPanelCursor for the
	// current frame; lastFlushedCursor is what the last Show flushed. Cursor-only moves
	// (arrow keys in the terminal panel) change no cells, so the hash cache must also
	// compare these to know a flush is needed.
	pendingCursor     hwCursorState
	lastFlushedCursor hwCursorState
	// chooserFile is non-empty in Helix/editor file-picker mode (--chooser-file).
	chooserFile string

	// quickActionOnActivate/quickActionOnKey/quickActionFooterExtra are the callbacks
	// and footer hints for whichever quick-action list is currently open (model.QuickAction).
	// Only one quick-action dialog can be open at a time.
	quickActionOnActivate  func(int)
	quickActionOnKey       func(*tcell.EventKey) bool
	quickActionFooterExtra []menu.FunctionKey

	// userMenuVisible/userMenuPath are the F2 user-menu quick-action consumer's state:
	// the currently visible entries and the resolved menu.toml path they came from.
	userMenuVisible []usermenu.MenuEntry
	userMenuPath    string
}

// LaunchConfig controls process-level startup (CLI flags).
type LaunchConfig struct {
	DevMode           bool
	ChooserFile       string
	ChooserSelect     string
	ChooserNoCarousel bool
	// StartPaths are optional existing local paths (at most two). See applyStartPaths.
	StartPaths []string
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
	// ChooserFile enables file-picker mode; Enter on a file writes the path here and quits.
	ChooserFile string
	// ChooserSelect is an optional file or directory to open and highlight at chooser startup.
	ChooserSelect string
	// ChooserNoCarousel disables the default carousel view on the left panel at startup.
	ChooserNoCarousel bool
	// StartPaths are optional existing local paths (at most two). See applyStartPaths.
	StartPaths []string
}

// Run initializes and starts the terminal application.
func Run(cfg LaunchConfig) error {
	paths, err := config.DefaultPaths()
	if err != nil {
		return err
	}
	paths = paths.WithResolvedLocations()

	startup, useBuiltIn, err := resolveStartupConfig(paths)
	if err != nil {
		return err
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
		CWD:               os.Getwd,
		Config:            startup.Config,
		Theme:             startup.Theme,
		ThemeChoices:      startup.ThemeChoices,
		KeymapBundle:      startup.Keymap,
		Paths:             paths,
		DevMode:           cfg.DevMode,
		ChooserFile:       cfg.ChooserFile,
		ChooserSelect:     cfg.ChooserSelect,
		ChooserNoCarousel: cfg.ChooserNoCarousel,
		StartPaths:        cfg.StartPaths,
	})
	if err != nil {
		return err
	}
	if useBuiltIn {
		app.setTransientMessage("Started with built-in defaults (configuration failed to load).", ui.MessageUrgencyWarn)
	}
	if warns := chromastyles.LoadWarnings(); len(warns) > 0 {
		app.setTransientMessage(fmt.Sprintf("Preview styles: %v", errors.Join(warns...)), ui.MessageUrgencyWarn)
	}
	// Registered after screen.Fini's defer so the shell child dies before tcell releases the TTY.
	defer app.closeSubshell()
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
	keys, err := resolveKeymapBundle(opts)
	if err != nil {
		return nil, err
	}
	styles, availableThemes, themeChoices, err := resolveThemes(opts)
	if err != nil {
		return nil, err
	}
	path, err := cwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}
	sortMode, _ := panel.ParseSortMode(cfg.Panels.DefaultSort)
	listingFormat, _ := panel.ParseListFormat(cfg.Panels.DefaultListingFormat)
	listOptions := localfs.ListOptions{
		ShowHidden: cfg.Panels.ShowHidden,
	}
	var giCache *gitignore.Cache
	if cfg.Panels.RespectGitignore {
		giCache = gitignore.NewCache()
	}
	duEngine := diskusage.NewWithWalkConcurrency(cfg.DiskUsage.WalkConcurrency)

	panelOpts := browserPanelOptions{
		list:       listOptions,
		gitignore:  giCache,
		cfg:        cfg,
		sortMode:   sortMode,
		listFormat: listingFormat,
		scrollMode: scrollModeFromConfig(cfg.UI.Scroll.Mode),
		diskEngine: duEngine,
	}
	left, err := newBrowserPanel(path, panelOpts)
	if err != nil {
		return nil, err
	}
	right, err := newBrowserPanel(path, panelOpts)
	if err != nil {
		return nil, err
	}
	jobState := jobs.NewState()
	jobState.SetTransferFunc(jobbridge.TransferFunc(cfg.Operations, cfg.Jobs))
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
		screen:         screen,
		config:         cfg,
		styles:         styles,
		themes:         availableThemes,
		paths:          resolvedPaths,
		keys:           keys,
		devMode:        opts.DevMode,
		commandsCtx:    cmdCtx,
		commandsCancel: cmdCancel,
		workPools:      workpool.NewRegistry(poolDefs),
		model: ui.Model{
			Primary:                    left,
			Secondary:                  right,
			ActivePanel:                ui.PrimaryPanel,
			SelectionsPanelMaxRows:     cfg.UI.SelectionsPanelMaxRows,
			HideMenuBar:                !cfg.UI.ShowMenuBar,
			ShowFileIcons:              cfg.UI.ShowFileIcons,
			CarouselLayout:             carouselLayoutFromConfig(cfg.Carousel),
			ShrunkenShowsNameOnly:      cfg.UI.ShrunkenShowsNameOnly,
			JobsThroughputChartEnabled: cfg.Jobs.ThroughputChartEnabled,
			UserHomeDir:                homeDir,
			DiskUsage:                  duEngine,
			DiskUsageShown:             false,
			ViewMode:                   ui.ViewBrowser,
			JobActivity:                make(map[string][]string),
			MenuDefinitions:            menu.BrowserDefinitions(keys.Global, opts.DevMode),
			ThemeDialog: dialog.ThemeDialogState{
				CurrentName: styles.Name,
				Choices:     uiThemeChoices(themeChoices),
			},
			Menu: menu.State{
				ActiveMenu: menu.DefaultIndex(),
			},
			FooterKeys: menu.FunctionKeys,
			// TerminalPanel starts hidden; input.go (later phase) owns toggle/resize/focus.
			TerminalPanel: ui.TerminalPanelState{
				Rows: cfg.Shell.TerminalPanelHeight,
			},
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
	jobState.SetScanConfig(jobs.ScanConfig{
		YieldInterval:       time.Duration(cfg.Jobs.ScanYieldIntervalMS) * time.Millisecond,
		YieldEveryN:         cfg.Jobs.ScanYieldEveryN,
		NiceIncrement:       cfg.Jobs.ScanNiceIncrement,
		ProgressMinInterval: time.Duration(cfg.Jobs.ScanProgressMinIntervalMS) * time.Millisecond,
	})
	jobState.SetScanFunc(jobbridge.ScanFunc())
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
	app.model.Primary.SuppressHeavyPathProbes = suppressHeavyPathProbes
	app.model.Secondary.SuppressHeavyPathProbes = suppressHeavyPathProbes
	app.wireFileListViewportRows()
	app.jobsCtrl = jobsctrl.New(jobsctrl.Deps{
		Host:     jobsHost{appShellHost: appShellHost{app: app}},
		Screen:   screen,
		Model:    &app.model,
		State:    jobState,
		Config:   cfg,
		Keys:     keys.Global,
		KeysJobs: keys.Jobs,
	})
	jobState.SetEmitHook(app.jobsCtrl.OnJobEmitted)
	app.findCtrl = findctrl.New(findctrl.Deps{
		Host:           findHost{appShellHost: appShellHost{app: app}},
		Screen:         screen,
		Model:          &app.model,
		Config:         cfg,
		Keys:           keys.Global,
		KeysFindDialog: keys.FindDialog,
	})
	app.compareCtrl = comparectrl.New(comparectrl.Deps{
		Host:        compareHost{appShellHost: appShellHost{app: app}},
		Screen:      screen,
		Model:       &app.model,
		Config:      cfg,
		Keys:        keys.Global,
		KeysCompare: keys.Compare,
		Gitignore:   giCache,
		DiskIgnore:  duIgnorer,
		Jobs:        app.jobsCtrl,
	})
	app.dedupCtrl = dedupctrl.New(dedupctrl.Deps{
		Host:       dedupHost{appShellHost: appShellHost{app: app}},
		Screen:     screen,
		Model:      &app.model,
		Config:     cfg,
		Gitignore:  giCache,
		DiskIgnore: duIgnorer,
	})
	if err := app.configureSFTP(); err != nil {
		app.stopWorker()
		return nil, fmt.Errorf("configure sftp: %w", err)
	}
	app.wireRemotePanelLoaders()
	app.wireTreeChildLoaders()
	app.wireGitStatusLoaders()
	app.model.Primary.RescheduleGitStatusIfNeeded()
	app.model.Secondary.RescheduleGitStatusIfNeeded()
	app.jobsCtrl.SyncJobPathMarks()
	if secs := cfg.Jobs.FreeSpacePollIntervalSecs; secs > 0 {
		go app.runVolumeSpaceTicker(time.Duration(secs)*time.Second, app.jobStopCh)
	}
	if ms := cfg.Panels.RefreshIntervalMS; ms > 0 {
		go app.runPanelRefreshTicker(time.Duration(ms)*time.Millisecond, app.jobStopCh)
	}
	if cfg.Jobs.ThroughputChartEnabled {
		go app.runThroughputChartTicker(
			time.Duration(cfg.Jobs.ThroughputChartColumnMS)*time.Millisecond,
			app.jobStopCh,
		)
	}
	if opts.ChooserFile != "" {
		app.chooserFile = opts.ChooserFile
	}
	if opts.ChooserSelect != "" {
		if err := app.applyChooserSelect(opts.ChooserSelect); err != nil {
			app.stopWorker()
			return nil, fmt.Errorf("select: %w", err)
		}
	}
	if len(opts.StartPaths) > 0 {
		if err := app.applyStartPaths(opts.StartPaths); err != nil {
			app.stopWorker()
			return nil, err
		}
	}
	if opts.ChooserFile != "" {
		if !opts.ChooserNoCarousel {
			app.model.Primary.CarouselMode = true
			app.model.ActivePanel = ui.PrimaryPanel
		}
		app.model.QuickViewEnabled = true
		app.model.QuickViewPanel = app.model.ActivePanel
		app.applyQuickViewPreviewImmediately()
	}
	return app, nil
}

// loadKeymapBundle resolves the raw keymap.Bundle: an explicit bundle or global
// override for tests takes precedence, otherwise it loads from opts.Paths.
func loadKeymapBundle(opts Options) (*keymap.Bundle, error) {
	switch {
	case opts.KeymapBundle != nil:
		return opts.KeymapBundle, nil
	case opts.Keymap != nil:
		return &keymap.Bundle{Global: opts.Keymap}, nil
	default:
		bundle, err := keymap.LoadFromPaths(opts.Paths)
		if err != nil {
			return nil, fmt.Errorf("load keybindings: %w", err)
		}
		return bundle, nil
	}
}

// resolveKeymapBundle resolves the global keymap and every per-view overlay
// keymap, falling back to package defaults for any overlay the bundle didn't supply.
func resolveKeymapBundle(opts Options) (*keymap.Bundle, error) {
	bundle, err := loadKeymapBundle(opts)
	if err != nil {
		return nil, err
	}
	if bundle.Global == nil {
		return nil, fmt.Errorf("load keybindings: global keymap is nil")
	}
	// Work on a copy so callers passing an explicit Options.KeymapBundle don't
	// have their overlays mutated by the defaulting below.
	rk := *bundle
	for _, step := range []struct {
		km    **keymap.Map
		build func() map[string][]string
		label string
	}{
		{&rk.DialogInput, func() map[string][]string { return map[string][]string{} }, "empty dialog input"},
		{&rk.RenameDialog, keymap.DefaultRenameDialogOverlayKeys, "rename dialog overlay"},
		{&rk.MkdirDialog, keymap.DefaultMkdirDialogOverlayKeys, "mkdir dialog overlay"},
		{&rk.BookmarkDialog, keymap.DefaultBookmarkDialogOverlayKeys, "bookmark dialog overlay"},
		{&rk.FindDialog, keymap.DefaultFindDialogOverlayKeys, "find dialog overlay"},
		{&rk.HistoryDialog, keymap.DefaultHistoryDialogOverlayKeys, "history dialog overlay"},
		{&rk.FilePreview, keymap.DefaultFilePreviewOverlayKeys, "file preview overlay"},
		{&rk.FlattenDialog, keymap.DefaultFlattenDialogOverlayKeys, "flatten dialog overlay"},
		{&rk.TransferDialog, keymap.DefaultTransferDialogOverlayKeys, "transfer dialog overlay"},
		{&rk.Compare, keymap.DefaultCompareOverlayKeys, "compare overlay"},
		{&rk.Dedup, keymap.DefaultDedupOverlayKeys, "dedup overlay"},
		{&rk.Terminal, keymap.DefaultTerminalOverlayKeys, "terminal overlay"},
	} {
		if *step.km != nil {
			continue
		}
		m, err := keymap.Build(step.build())
		if err != nil {
			return nil, fmt.Errorf("build %s map: %w", step.label, err)
		}
		*step.km = m
	}
	return &rk, nil
}

// resolveThemes resolves the active theme (falling back to theme.Default when
// unset), the full set of selectable theme choices (loaded from disk when the
// caller didn't supply one), and registers the active theme as a choice if it
// isn't already one (e.g. a config-only custom theme).
func resolveThemes(opts Options) (styles theme.Theme, availableThemes map[string]theme.Theme, themeChoices []theme.NamedTheme, err error) {
	styles = opts.Theme
	if styles.Name == "" {
		styles = theme.Default()
	}
	themeChoices = opts.ThemeChoices
	if len(themeChoices) == 0 {
		themeChoices, err = theme.ThemeChoices(opts.Paths.WithResolvedLocations().ThemesDir)
		if err != nil {
			return theme.Theme{}, nil, nil, fmt.Errorf("load theme choices: %w", err)
		}
	}
	availableThemes = map[string]theme.Theme{}
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
	return styles, availableThemes, themeChoices, nil
}

// browserPanelOptions bundles the config-derived settings applied identically
// to both the left and right panel at startup (see newBrowserPanel).
type browserPanelOptions struct {
	list       localfs.ListOptions
	gitignore  *gitignore.Cache
	cfg        config.Config
	sortMode   panel.SortMode
	listFormat panel.ListFormat
	scrollMode panel.ScrollMode
	diskEngine *diskusage.Engine
}

// newBrowserPanel constructs a panel.State at path with sort/list/scroll/filter
// options applied consistently; used for both the left and right startup panel.
func newBrowserPanel(path string, opts browserPanelOptions) (panel.State, error) {
	p, err := panel.NewWithOptions(path, opts.list, opts.gitignore)
	if err != nil {
		return panel.State{}, err
	}
	p.Sort.Mode = opts.sortMode
	p.Sort.Reverse = opts.cfg.Panels.SortReverse
	p.Sort.DirectoriesFirst = opts.cfg.Panels.DirectoriesFirst
	p.Sort.DiskUsageIdleSizeSort = opts.cfg.DiskUsage.IdleSizeSort
	p.DiskUsageIdleSortActivated = opts.cfg.DiskUsage.IdleSizeSort
	p.ListFormat = opts.listFormat
	p.DiskSorter = opts.diskEngine.Size
	p.ApplySort()
	p.Filter.CaseInsensitive = opts.cfg.Filter.CaseInsensitive
	p.Filter.CycleMatches = opts.cfg.Filter.CycleMatches
	p.ScrollMode = opts.scrollMode
	p.ScrollEdgeMargin = opts.cfg.UI.Scroll.EdgeMargin
	return p, nil
}

// eventOutcome carries the loop-local render/poll flags produced by handleInterruptPayload
// back to Run's post-switch tail.
type eventOutcome struct {
	didRender              bool
	pollJobsAfter          bool
	applyJobRefreshesAfter bool
	pollDiskUsageAfter     bool
}

// handleInterruptPayload handles the EventInterrupt payload type-switch for Run.
func (a *App) handleInterruptPayload(data any) eventOutcome {
	out := eventOutcome{pollDiskUsageAfter: true}
	switch d := data.(type) {
	case jobsctrl.WakePayload:
		out.pollJobsAfter = true
		out.applyJobRefreshesAfter = true
		// Progress wakes are frequent; reconciling both panels here caused
		// extra sync/stat work and starved spinner ticks on slow mounts.
		out.pollDiskUsageAfter = false
	case terminalWakePayload:
		out.pollDiskUsageAfter = false
		a.handleTerminalWake()
		out.didRender = true
	case statusMessageExpiryPayload:
		a.applyStatusMessageExpiry(d)
		a.render()
		out.didRender = true
	case spinnerTickPayload:
		out.pollDiskUsageAfter = false
		if a.menuBarSpinnerBusy() {
			a.model.SpinPhase++
			w, h := a.screen.Size()
			if ui.DrawMenuBarSpinnerOnly(a.screen, a.layoutForTerminalSize(w, h), a.model, a.styles) {
				a.emitScreenAfterPartialPaint()
			}
			out.didRender = true
		}
	case diskIdleSortPayload:
		a.applyIdleDiskSort(d.PanelID, d.Epoch)
		a.render()
		out.didRender = true
	case diskUsageRedrawPayload:
		out.pollDiskUsageAfter = false
		a.resortPanelsDiskUsageSorted()
		a.refreshDeleteDialogSummary()
		if a.model.FindDialog.Open {
			a.model.FindDialog.InvalidateMarkedSelectionSizeLabel()
			a.renderFindDialogUpdate()
		} else if a.deleteDialogOpen() {
			a.renderDeleteDialogUpdate()
		} else if a.paintDiskUsageBrowserUpdate() {
			a.armSpinnerRedrawTimer()
		} else {
			a.render()
		}
		out.didRender = true
	case volumeSpaceRefreshPayload:
		out.pollDiskUsageAfter = false
		if a.applyVolumeSpaceRefresh(d) && a.model.ViewMode == ui.ViewJobs {
			a.render()
			out.didRender = true
		}
	case commandWakePayload:
		a.applyCommandWake(d)
		a.render()
		out.didRender = true
	case metaWakePayload:
		if d.gen == a.metaRunGen[d.panelID] {
			a.applyMetaWakeResult(d)
		}
		a.scheduleMetaRenderDebounced()
	case metaRenderFlushPayload:
		a.render()
		out.didRender = true
	case metaLoadPayload:
		a.applyMetaLoad(d)
		a.render()
		out.didRender = true
	case metaExecFailedPayload:
		if d.gen == a.metaRunGen[d.panelID] {
			a.applyMetaExecFailed(d)
		}
		a.render()
		out.didRender = true
	case pathPickerValidatePayload:
		a.render()
		out.didRender = true
	case transferDestValidatePayload:
		a.render()
		out.didRender = true
	case jobsctrl.JobBlockerNextPayload:
		if a.jobsCtrl.ApplyBlockerNextPayload(d) {
			a.render()
			out.didRender = true
		}
	case syncFollowNavFlushPayload:
		if a.applyPanelSyncFollowNavFlush(d) {
			a.render()
			out.didRender = true
		}
	case quickViewFlushPayload:
		if a.applyQuickViewPreviewFlush(d) {
			a.render()
			out.didRender = true
		}
	case carouselPreviewFlushPayload:
		if a.applyCarouselPreviewFlush(d) {
			a.render()
			out.didRender = true
		}
	case cursorNameHintFlushPayload:
		a.render()
		out.didRender = true
	case previewStylePickerFlushPayload:
		if a.applyPreviewStylePickerFlush(d) {
			a.render()
			out.didRender = true
		}
	case debounceCalibrateReleasePayload:
		if a.applyDebounceCalibrateReleasePayload() {
			a.render()
			out.didRender = true
		}
	case findctrl.WakePayload:
		if a.findCtrl.PollUpdates(d) {
			a.renderFindDialogUpdate()
			out.didRender = true
		}
	case findctrl.RankWakePayload:
		if a.findCtrl.ApplyPendingRank() {
			a.renderFindDialogUpdate()
			out.didRender = true
		}
	case findctrl.ThrottleRankWakePayload:
		if a.findCtrl.HandleThrottleRankWake() {
			a.renderFindDialogUpdate()
			out.didRender = true
		}
	case findctrl.DebounceRankWakePayload:
		if a.findCtrl.HandleDebounceRankWake() {
			a.renderFindDialogUpdate()
			out.didRender = true
		}
	case findctrl.FindNavIdlePayload:
		if a.findCtrl.HandleFindNavIdle(d.Epoch) {
			a.renderFindDialogUpdate()
			out.didRender = true
		}
	case comparectrl.WakePayload:
		if a.pollCompareUpdates(d) {
			a.render()
			out.didRender = true
		}
	case dedupctrl.WakePayload:
		if a.pollDedupUpdates(d) {
			a.render()
			out.didRender = true
		}
	case throughputChartTickPayload:
		out.pollDiskUsageAfter = false
		if a.applyThroughputChartTick() {
			a.render()
			out.didRender = true
		}
	case sftpConnectPayload:
		a.applySFTPConnect(d)
		out.didRender = true
	case sftpHostKeyOpenPayload:
		a.openHostKeyDialog(d.prompt)
		a.render()
		out.didRender = true
	case sftpPasswordOpenPayload:
		a.openSFTPPasswordDialog(d.prompt)
		a.render()
		out.didRender = true
	case remotePanelLoadPayload:
		if a.applyRemotePanelLoad(d) {
			a.render()
			out.didRender = true
		}
	case treeChildLoadPayload:
		if a.applyTreeChildLoad(d) {
			a.render()
			out.didRender = true
		}
	case gitStatusPayload:
		if a.applyGitStatusLoad(d) {
			a.render()
			out.didRender = true
		}
	case quickViewGitStatusPayload:
		if a.applyQuickViewGitStatusLoad(d) {
			a.render()
			out.didRender = true
		}
	case panelRefreshTickPayload:
		a.handlePanelRefreshTick()
	case panelRefreshApplyPayload:
		if a.applyPanelListingRefresh(d) {
			a.render()
			out.didRender = true
		}
	}
	return out
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
			a.clearCursorNameHintNavCoalesce()
			a.screen.Sync()
			a.ensurePanelsVisible()
			a.resizeTerminalFeedToLayout()
			a.refreshPreviewsAfterResize()
			a.render()
			didRender = true
		case *tcell.EventKey:
			if a.model.ViewMode != ui.ViewJobs {
				a.jobsCtrl.DrainDiscardProgressEvents()
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
			outcome := a.handleInterruptPayload(event.Data())
			didRender = outcome.didRender
			pollJobsAfter = outcome.pollJobsAfter
			applyJobRefreshesAfter = outcome.applyJobRefreshesAfter
			pollDiskUsageAfter = outcome.pollDiskUsageAfter
		}

		if pollJobsAfter {
			jobsDirty = a.jobsCtrl.PollEvents()
			shouldRenderJobs = jobsDirty && a.jobsCtrl.AffectVisible()
		}
		if applyJobRefreshesAfter {
			if a.jobsCtrl.ApplyRefreshes() && !didRender {
				a.render()
				didRender = true
			}
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
		if !a.deferDiskUsagePoll.Swap(false) {
			a.pollDiskUsageUpdates()
		}
	}
}

// handleDialogKey routes keys for modal overlays (transfer, conflict, quit confirm).
func (a *App) handleDialogKey(event *tcell.EventKey) bool {
	switch {
	case a.model.ConflictDialog.Open:
		a.jobsCtrl.HandleBlockerDialogKey(event)
		return false
	case a.model.TransferDialog.Open:
		a.handleTransferDialogKey(event)
		return false
	case a.model.FlattenDialog.Open:
		a.handleFlattenDialogKey(event)
		return false
	case a.model.QuitConfirm.Open:
		return a.handleQuitConfirmKey(event)
	case a.model.DedupEmptyDirsConfirm.Open:
		return a.handleDedupEmptyDirsConfirmKey(event)
	case a.model.StashRestoreDialog.Open:
		a.handleStashRestoreDialogKey(event)
		return false
	}
	return false
}

func carouselLayoutFromConfig(c config.CarouselConfig) panelcarousel.Layout {
	layout, err := panelcarousel.ParseLayout(c.Split, c.ShowSize)
	if err != nil {
		// ponytail: config.Validate() already normalizes CarouselConfig, so this fallback
		// should be unreachable; parse the same config-level default rather than duplicating
		// it as a second hardcoded value.
		layout, _ = panelcarousel.ParseLayout(config.DefaultCarouselSplit(), config.DefaultCarouselShowSize())
	}
	return layout
}
