// Package app wires together UI, state, and services for the TUI application.
package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	commandsctrl "github.com/paranoidi/paras-commander/internal/apphandler/commands"
	comparectrl "github.com/paranoidi/paras-commander/internal/apphandler/compare"
	dedupctrl "github.com/paranoidi/paras-commander/internal/apphandler/dedup"
	dialogctrl "github.com/paranoidi/paras-commander/internal/apphandler/dialog"
	findctrl "github.com/paranoidi/paras-commander/internal/apphandler/find"
	jobsctrl "github.com/paranoidi/paras-commander/internal/apphandler/jobs"
	metactrl "github.com/paranoidi/paras-commander/internal/apphandler/meta"
	previewctrl "github.com/paranoidi/paras-commander/internal/apphandler/preview"
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
	"github.com/paranoidi/paras-commander/internal/preview"
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

// diskUsageState groups the disk-usage engine and its scan/idle-sort bookkeeping.
type diskUsageState struct {
	engine *diskusage.Engine
	ignore diskusage.ShouldIgnoreFolder
	// scanToastArmed is set when a user-initiated disk usage scan starts and cleared
	// after the "scan finished" toast fires. Selection-size background scans do not set it, so
	// their EventJobFinished completions never trigger the toast even when DiskUsageShown is true.
	scanToastArmed bool
	idleSort       [2]diskIdleSortPanel // indexed by ui.PrimaryPanel / ui.SecondaryPanel (0/1)
	// idleNavPath records last reconciled panel cwd so idle-sort debounce survives benign reconcile but resets on chdir.
	idleNavPath [2]string
	redrawTimer *time.Timer
	// deferPoll skips one pollDiskUsageUpdates drain after partial file-list nav while a scan is busy.
	deferPoll atomic.Bool
}

// sftpState groups SFTP connection-prompt state (host-key/password waiters) and the
// SFTP connect dialog's target panel and host list.
type sftpState struct {
	mu                 sync.Mutex
	hostKeyWait        *sftpHostKeyWait
	passwordWait       *sftpPasswordWait
	connectTargetPanel int
	connectHosts       []sshconfig.HostEntry
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
	// jobState manages background job queue and worker lifecycle.
	jobState       *jobs.State
	jobsCtrl       *jobsctrl.Handler
	findCtrl       *findctrl.Handler
	metaCtrl       *metactrl.Handler
	commandsCtrl   *commandsctrl.Handler
	compareCtrl    *comparectrl.Handler
	dedupCtrl      *dedupctrl.Handler
	previewCtrl    *previewctrl.Handler
	dialogCtrl     *dialogctrl.Handler
	jobStopCh      chan struct{}
	jobStopOnce    bool
	disk           diskUsageState
	gitignoreCache *gitignore.Cache
	gitStatusCache *gitstatus.Cache
	// selectionSizeScanFP is the last enqueued directory set fingerprint per panel for selection-size scans.
	selectionSizeScanFP [2]string
	// selectionSizeScanGen / selectionSizeScanPath skip reconcile work when selection-derived input is unchanged.
	selectionSizeScanGen  [2]uint64
	selectionSizeScanPath [2]string
	// findDialogSelectionScanFP is the last enqueued directory set fingerprint for find-dialog selection-size scans.
	findDialogSelectionScanFP string
	// findDialogSelectionScanGen skips reconcile work when marked-selection derived input is unchanged.
	findDialogSelectionScanGen uint64
	// messageExpiryGen increments whenever the transient message or its schedule changes;
	// scheduled expirations carry the generation and are ignored if stale.
	messageExpiryGen   atomic.Uint64
	spinnerRedrawTimer *time.Timer
	// syncFollowNavGen invalidates in-flight debounce callbacks for latched panel sync (file-list cursor).
	syncFollowNavGen atomic.Uint64
	// syncFollowNavSkipReconcile, when true, suppresses syncFollowFromActive in reconcileAfterEvent
	// until the debounce flush runs or coalesce is cleared.
	syncFollowNavSkipReconcile atomic.Bool
	syncFollowNav              sched.ManagedTimer
	// debounceCalibrateRelease infers key release between calibration trials.
	debounceCalibrateRelease sched.ManagedTimer
	// navParentBackspaceGuarded, when true, suppresses nav.parent triggered by backspace.
	// Armed when backspace erases the last filter character; cleared when the key is released
	// (debounce timer fires). Prevents accidental directory navigation after erasing filter text.
	navParentBackspaceGuarded  atomic.Bool
	navParentBackspaceDebounce sched.ManagedTimer
	// cursorNameHintNavSkip, when true, holds the previous bottom-border full-name overlay during file-list nav debounce.
	cursorNameHintNavSkip atomic.Bool
	cursorNameHintNav     sched.ManagedTimer

	// zoomActivePanelOverride is nil → layout uses cfg.UI.ZoomActivePanel; when non-nil it forces
	// zoom on/off for this session only (Alt+z / panel.toggle-zoom-active-panel). Cleared on
	// Configuration OK so saved TOML is the sole persisted source of truth. Layout still suppresses
	// zoom while quick view / file preview is active and when terminal width ≥ cfg.UI.ZoomActivePanelDisabledAboveWidth (when > 0).
	zoomActivePanelOverride *bool
	// paneSplitOrientationOverride is nil → layout uses cfg.UI.PaneSplitOrientation; when non-nil it
	// forces stacked or side-by-side for this session only (panel.toggle-split-orientation). Cleared on Configuration OK.
	paneSplitOrientationOverride *ui.SplitOrientation

	// commandsMu is the shared async-model-mutation lock for model fields written from
	// background goroutines (CommandsList via commandsCtrl, FilePreview/QuickView/etc. from the
	// preview subsystem). render() copies the whole a.model under this same lock, so every
	// goroutine that mutates a model field must use this mutex rather than a private one — a
	// split lock would let that whole-struct copy race with a concurrent mutation.
	commandsMu sync.RWMutex
	// commandsCtx/commandsCancel is the app-lifetime cancellation context for background
	// command and preview subprocesses; commandsCancel runs once at quit (see quit.go).
	commandsCtx    context.Context
	commandsCancel context.CancelFunc

	workPools *workpool.Registry

	volumeRefreshInFlight [2]atomic.Bool
	panelRefreshInFlight  [2]atomic.Bool

	statusCmdText    string
	statusCmdRunning atomic.Bool
	statusCmdStopCh  chan struct{}

	sftp           sftpState
	image          imageOverlay
	placeholderImg placeholderImage

	// Indexed by panel ID (ui.PrimaryPanel, ui.SecondaryPanel, ui.QuickViewOverlayPanel).
	panelAsyncLoadGen [3]atomic.Uint64
	gitStatusLoadGen  [3]atomic.Uint64

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
	// launchedFileViewer is true when the app was started with a single CLI file
	// argument (pc <file>), opening straight into the fullscreen preview with no
	// twin-panel browser to fall back to.
	launchedFileViewer bool

	// leaderMenuOnActivate runs the selected menu entry after the leader menu closes.
	// Returns true when the app should exit (e.g. quit).
	leaderMenuOnActivate func(int) bool

	// leaderMenuActions holds action IDs parallel to Items when the built-in Esc menu is open.
	leaderMenuActions []string

	// userMenuVisible/userMenuPath hold the resolved menu.toml entries while the user leader menu is open.
	userMenuVisible []usermenu.MenuEntry
	userMenuPath    string

	// userMenuStack holds ancestor levels while a submenu is open, for Esc back-navigation.
	userMenuStack [][]usermenu.MenuEntry
}

// LaunchConfig controls process-level startup (CLI flags).
type LaunchConfig struct {
	DevMode           bool
	ChooserFile       string
	ChooserSelect     string
	ChooserNoCarousel bool
	// StartPaths are optional existing local paths (at most two). See applyStartPaths.
	StartPaths []string
	// QuickPreview enables Quick View at startup (pc -qp, requires StartPaths).
	QuickPreview bool
	// Carousel starts the primary panel directly in carousel view (pc --carousel).
	Carousel bool
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
	// QuickPreview enables Quick View at startup (pc -qp, requires StartPaths).
	QuickPreview bool
	// Carousel starts the primary panel directly in carousel view (pc --carousel).
	Carousel bool
}

// Run initializes and starts the terminal application.
func Run(cfg LaunchConfig) (err error) {
	// Runs last (registered first: LIFO defer order runs screen.Fini/closeSubshell below
	// before this), so the terminal is already restored by the time a crash is reported.
	defer func() {
		if r := recover(); r != nil {
			err = reportCrash(r, debug.Stack())
		}
	}()

	paths, err := config.DefaultPaths()
	if err != nil {
		return err
	}
	paths = paths.WithResolvedLocations()

	startup, useBuiltIn, err := resolveStartupConfig(paths)
	if err != nil {
		return err
	}

	// Fired before the screen even exists: NewWithOptions/App.Run must never block on a `tmux
	// display-message` subprocess for the first image preview, so the caches these calls warm
	// (ResolveImageProtocol / TmuxSupportsKittyUnicodePlaceholders / TmuxSupportsNativeSixel)
	// get a head start here rather than on the render path. Deliberately not in NewWithOptions:
	// that constructor is also used directly by internal/app's unit tests (without ever calling
	// Run), and warming there would leak a real `tmux` subprocess result into the process-wide
	// sync.OnceValue cache tests rely on being unpopulated.
	preview.WarmTmuxCaches(os.Getenv)

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
		QuickPreview:      cfg.QuickPreview,
		Carousel:          cfg.Carousel,
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
	duEngine := diskusage.NewWithFSWalk(cfg.FSWalk)

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
			Primary:                      left,
			Secondary:                    right,
			ActivePanel:                  ui.PrimaryPanel,
			SelectionsPanelMaxRows:       cfg.UI.SelectionsPanelMaxRows,
			SelectionsPanelActivePercent: cfg.UI.SelectionsPanelActivePercent,
			HideMenuBar:                  !cfg.UI.ShowMenuBar,
			ShowFileIcons:                cfg.UI.ShowFileIcons,
			CarouselLayout:               carouselLayoutFromConfig(cfg.Carousel),
			ShrunkenShowsNameOnly:        cfg.UI.ShrunkenShowsNameOnly,
			JobsThroughputChartEnabled:   cfg.Jobs.ThroughputChartEnabled,
			UserHomeDir:                  homeDir,
			DiskUsage:                    duEngine,
			DiskUsageShown:               false,
			ViewMode:                     ui.ViewBrowser,
			JobActivity:                  make(map[string][]string),
			MenuDefinitions:              menu.BrowserDefinitions(keys.Global, opts.DevMode),
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

		disk: diskUsageState{
			engine: duEngine,
			ignore: duIgnorer,
		},
		gitignoreCache: giCache,
		gitStatusCache: gitstatus.NewCache(),
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
	app.metaCtrl = metactrl.New(metactrl.Deps{
		Host:      metaHost{appShellHost: appShellHost{app: app}},
		Screen:    screen,
		Model:     &app.model,
		Config:    cfg,
		ConfigDir: resolvedPaths.ConfigDir,
	})
	app.commandsCtrl = commandsctrl.New(commandsctrl.Deps{
		Host:         commandsHost{appShellHost: appShellHost{app: app}},
		Screen:       screen,
		Model:        &app.model,
		Keys:         keys.Global,
		KeysCommands: keys.Commands,
		Mu:           &app.commandsMu,
		Ctx:          app.commandsCtx,
		WorkPools:    app.workPools,
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
	app.previewCtrl = previewctrl.New(previewctrl.Deps{
		Host:            previewHost{appShellHost: appShellHost{app: app}},
		Screen:          screen,
		Model:           &app.model,
		KeysDialogInput: keys.DialogInput,
		Mu:              &app.commandsMu,
		Ctx:             app.commandsCtx,
	})
	app.dialogCtrl = dialogctrl.New(dialogctrl.Deps{
		Host:                 dialogHost{appShellHost: appShellHost{app: app}},
		Screen:               screen,
		Model:                &app.model,
		KeysRenameDialog:     keys.RenameDialog,
		KeysMkdirDialog:      keys.MkdirDialog,
		KeysDialogInput:      keys.DialogInput,
		KeysGlobal:           keys.Global,
		KeysTransferDialog:   keys.TransferDialog,
		KeysFlattenDialog:    keys.FlattenDialog,
		KeysBookmarkDialog:   keys.BookmarkDialog,
		KeysMassRenameDialog: keys.MassRenameDialog,
		KeysRunForEachDialog: keys.RunForEachDialog,
		ConfigDir:            resolvedPaths.ConfigDir,
		Jobs:                 app.jobsCtrl,
		Commands:             app.commandsCtrl,
		Preview:              app.previewCtrl,
		Dedup:                app.dedupCtrl,
		DiskUsage:            app.disk.engine,
		DiskUsageIgnore:      duIgnorer,
	})
	if err := app.configureSFTP(); err != nil {
		app.stopWorker()
		return nil, fmt.Errorf("configure sftp: %w", err)
	}
	app.wireAsyncPanelLoaders()
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
	app.restartStatusCommandTicker(cfg.StatusCommand)
	go app.runThroughputChartTicker(
		time.Duration(cfg.Jobs.ThroughputChartColumnMS)*time.Millisecond,
		app.jobStopCh,
	)
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
		if opts.QuickPreview && !app.model.QuickViewEnabled {
			app.model.QuickViewEnabled = true
			app.model.QuickViewPanel = app.model.ActivePanel
			app.previewCtrl.ApplyQuickViewPreviewImmediately()
		}
	}
	if opts.ChooserFile != "" {
		if !opts.ChooserNoCarousel {
			app.model.Primary.CarouselMode = true
			app.model.ActivePanel = ui.PrimaryPanel
		}
		app.model.QuickViewEnabled = true
		app.model.QuickViewPanel = app.model.ActivePanel
		app.previewCtrl.ApplyQuickViewPreviewImmediately()
	}
	if opts.Carousel {
		app.model.Primary.CarouselMode = true
		app.model.Primary.SetListLayout(panel.ListLayoutFlat, app.panelViewportRows(ui.PrimaryPanel))
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
	p.StripFilter.CaseInsensitive = opts.cfg.Filter.CaseInsensitive
	p.StripFilter.CycleMatches = opts.cfg.Filter.CycleMatches
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
		a.dialogCtrl.RefreshDeleteDialogSummary()
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
	case statusCommandResultPayload:
		out.pollDiskUsageAfter = false
		if a.applyStatusCommandResult(d) {
			a.render()
			out.didRender = true
		}
	case commandsctrl.WakePayload:
		a.commandsCtrl.ApplyWake(d)
		a.render()
		out.didRender = true
	case previewctrl.RenderWakePayload:
		a.render()
		out.didRender = true
	case metactrl.WakePayload:
		a.metaCtrl.HandleWake(d)
	case metactrl.RenderFlushPayload:
		a.render()
		out.didRender = true
	case metactrl.LoadPayload:
		a.metaCtrl.HandleLoad(d)
		a.render()
		out.didRender = true
	case metactrl.ExecFailedPayload:
		a.metaCtrl.HandleExecFailed(d)
		a.render()
		out.didRender = true
	case dialogctrl.PathPickerValidatePayload:
		a.render()
		out.didRender = true
	case dialogctrl.TransferDestValidatePayload:
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
	case previewctrl.QuickViewFlushPayload:
		if a.previewCtrl.ApplyQuickViewPreviewFlush(d) {
			a.render()
			out.didRender = true
		}
	case previewctrl.CarouselPreviewFlushPayload:
		if a.previewCtrl.ApplyCarouselPreviewFlush(d) {
			a.render()
			out.didRender = true
		}
	case cursorNameHintFlushPayload:
		a.render()
		out.didRender = true
	case previewctrl.StylePickerFlushPayload:
		if a.previewCtrl.ApplyPreviewStylePickerFlush(d) {
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
	case panelAsyncLoadPayload:
		if a.applyPanelAsyncLoad(d) {
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
			a.previewCtrl.ClearNavCoalesces()
			a.clearCursorNameHintNavCoalesce()
			a.resetImageOverlayForResize()
			a.screen.Sync()
			a.ensurePanelsVisible()
			a.resizeTerminalFeedToLayout()
			a.previewCtrl.RefreshPreviewsAfterResize()
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
		if !a.disk.deferPoll.Swap(false) {
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
		a.dialogCtrl.HandleTransferDialogKey(event)
		return false
	case a.model.FlattenDialog.Open:
		a.dialogCtrl.HandleFlattenDialogKey(event)
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
