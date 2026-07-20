package config

// Built-in defaults for fields populated by Default() and used elsewhere (e.g. ops, diskusage).
// Single source of truth: do not duplicate these in internal/ops.

const (
	// DefaultPathPickerValidateDelayMS is the debounce before filesystem checks on the path-picker filter.
	DefaultPathPickerValidateDelayMS = 200

	// DefaultKeyRepeatDebounceMS coalesces rapid file-list cursor steps, quick view preview reloads,
	// carousel child preview reloads, and F3 style-picker re-highlighting. Terminals do not report
	// key-up; after this many milliseconds without another qualifying step, deferred work runs once.
	// Zero disables coalescing (immediate per-event behavior).
	DefaultKeyRepeatDebounceMS = 80

	// DebounceCalibrationMarginMS is added to the measured key-repeat interval when calibrating.
	DebounceCalibrationMarginMS = 10
	// DebounceCalibrationMinRepeatSamples is how many repeat intervals one hold must yield.
	DebounceCalibrationMinRepeatSamples = 8
	// DebounceCalibrationReleaseIdleMS infers key release after hold sampling (no key-up events).
	DebounceCalibrationReleaseIdleMS = 200
	// DebounceCalibrationMinRepeatMS / DebounceCalibrationMaxRepeatMS reject outlier repeat intervals.
	DebounceCalibrationMinRepeatMS = 10
	DebounceCalibrationMaxRepeatMS = 500

	// KeyRepeatDebounceMaxMS upper clamp for key_repeat_debounce_ms in Config.Validate.
	KeyRepeatDebounceMaxMS = 10_000

	DefaultDiskUsageWalkConcurrency = 4

	// DefaultRefreshIntervalMS is how often both file panels re-read their directories from disk (0 disables).
	DefaultRefreshIntervalMS = 2500

	// RefreshIntervalMinMS / RefreshIntervalMaxMS clamp active refresh_interval_ms in Config.Validate.
	RefreshIntervalMinMS = 200
	RefreshIntervalMaxMS = 60_000

	DefaultWorkerProgressMinBytes      = 512 * 1024
	DefaultWorkerProgressMinIntervalMS = 200

	// Jobs timing clamps applied in Config.Validate (milliseconds).
	JobsProgressTimingMinMS = 50
	JobsProgressTimingMaxMS = 5000

	// WorkerProgressMinBytes clamp bounds in Config.Validate.
	WorkerProgressMinBytesMin = 64 * 1024
	WorkerProgressMinBytesMax = 64 * 1024 * 1024

	// ScanYieldEveryNMax is the upper clamp for scan_yield_every_n in Config.Validate.
	ScanYieldEveryNMax = 4096

	// DefaultFindQueryDebounceMS waits after the last keystroke in the find dialog query field
	// before re-ranking the indexed entries. Zero ranks on every keystroke (no debounce).
	DefaultFindQueryDebounceMS = 150

	// DefaultFindIndexingRankThrottleMS is the minimum interval between background re-ranking
	// operations while the find indexer walk is still running. The first rank fires immediately;
	// subsequent batches are coalesced until the interval expires, then one more rank fires.
	DefaultFindIndexingRankThrottleMS = 200

	// DefaultFindIndexingCountThrottleMS is the minimum interval between find-dialog re-renders
	// driven purely by a new batch of indexed entries (i.e. the live indexed-count update). On a
	// fast disk batches arrive many times per second; without this the count field repaints on
	// every batch. The final count is still shown: the indexing-finished/error paths render
	// unconditionally.
	DefaultFindIndexingCountThrottleMS = 500

	// DefaultFindMaxResults caps the number of ranked results the find dialog keeps after each
	// rank computation. Only the top-N scored entries are kept; the full index is always retained.
	// Bounding results limits the size of the ranked/matchRanges data regardless of index size.
	DefaultFindMaxResults = 200

	// DefaultDeleteDialogMaxListRows caps visible delete-confirmation name rows per frame
	// (matches dialog.DeleteDialogMaxListRows; full list still scrolls with PgUp/PgDn).
	DefaultDeleteDialogMaxListRows = 18

	// DefaultFindListNavIdleMS is how long the result list must be idle (no Up/Down/PgUp/PgDn)
	// before a background rank update is applied. This keeps the view stable while the user is
	// navigating, matching the behaviour of disk-usage idle sort in the file listing.
	DefaultFindListNavIdleMS = 400

	// DefaultDedupHashConfirmBytes pauses find-duplicates before hashing when the total
	// byte size of hash candidates exceeds this value (0 disables the gate).
	DefaultDedupHashConfirmBytes = 32 * 1024 * 1024 * 1024

	// DefaultDedupFileProgressBytes shows a per-file progress bar in the
	// find-duplicates dialog for files at or above this size (0 disables it).
	DefaultDedupFileProgressBytes = 256 * 1024 * 1024

	// DefaultDedupChunkBytes compares same-size files this many bytes at a time,
	// bailing out of a file as soon as its content diverges (0 disables chunking).
	DefaultDedupChunkBytes = 32 * 1024 * 1024

	// DefaultCompareHashConcurrency limits parallel file hashing during panel compare.
	DefaultCompareHashConcurrency = 4
	// DefaultCompareReadBufferKiB is the per-worker read buffer for compare hashing.
	DefaultCompareReadBufferKiB = 256
	// DefaultCompareStayOnVolumeDefault is the initial stay-on-volume option when opening compare.
	DefaultCompareStayOnVolumeDefault = true

	// DefaultProgressUIWakeDebounceMS is minimum spacing between main-loop wakes after worker EventProgress.
	DefaultProgressUIWakeDebounceMS = 150

	// DefaultBlockerDialogNextDebounceMS waits after answering one quick blocker dialog before
	// opening the next waiting prompt (0 opens immediately).
	DefaultBlockerDialogNextDebounceMS = 200
	// BlockerDialogNextDebounceMinMS / BlockerDialogNextDebounceMaxMS clamp jobs.blocker_dialog_next_debounce_ms.
	BlockerDialogNextDebounceMinMS = 0
	BlockerDialogNextDebounceMaxMS = 5000

	// DefaultThroughputChartWindowSec is the jobs details throughput strip span (clamped 20–120 in Validate).
	DefaultThroughputChartWindowSec = 45
	// DefaultThroughputChartColumnMS is wall time per chart column and chart ticker interval (clamped in Validate).
	DefaultThroughputChartColumnMS = 400
	// DefaultThroughputChartEnabled turns on the jobs details throughput strip + chart rendering.
	DefaultThroughputChartEnabled = true

	// DefaultFreeSpaceOnProgressWake schedules async statfs on both panels after each progress UI wake.
	DefaultFreeSpaceOnProgressWake = true
	// DefaultFreeSpacePollIntervalSecs is the period for background volume free-space refresh while jobs run (0 disables).
	DefaultFreeSpacePollIntervalSecs = 5

	// DefaultScanYieldIntervalMS is cooperative sleep duration during pre-scan while a transfer is active.
	DefaultScanYieldIntervalMS = 50
	// DefaultScanYieldEveryN invokes cooperative yield every N plan walk entries while a transfer is active.
	DefaultScanYieldEveryN = 64
	// DefaultScanNiceIncrement is added to the process nice value for pre-scan on Linux when a transfer is active.
	DefaultScanNiceIncrement = 10
	// DefaultScanProgressMinIntervalMS throttles scan-progress UI events during pre-scan walks.
	DefaultScanProgressMinIntervalMS = 200

	DefaultPreservePermissions        = true
	DefaultPreserveTimestamps         = true
	DefaultCopyBufferKiB              = 256
	DefaultSyncAfterEachFile          = true
	DefaultDiskSpaceCheckMinFileBytes = 50 * 1024 * 1024
	DefaultCowFileCloning             = true
	DefaultCopyFileRange              = true
	DefaultSparseFileCopy             = false
	DefaultPreallocateDestination     = false
	DefaultPreallocateMinFileBytes    = 1024 * 1024
	DefaultSyncAtJobEnd               = false
	DefaultSyncMinFileKiB             = 0

	// FlattenDefaultLocationActive is the active panel path for flatten dialog default destination.
	FlattenDefaultLocationActive = "active"
	// FlattenDefaultLocationInactive is the inactive panel path for flatten dialog default destination.
	FlattenDefaultLocationInactive = "inactive"
	// DefaultFlattenDefaultLocation is the initial flatten destination prefill panel.
	DefaultFlattenDefaultLocation = FlattenDefaultLocationActive

	// DefaultFlattenRecursive is the initial state of the flatten dialog recursive checkbox.
	DefaultFlattenRecursive = false
	// DefaultFlattenRemoveEmptyDirs is the initial state of the flatten dialog remove-empty checkbox.
	DefaultFlattenRemoveEmptyDirs = true
	// DefaultRemoveDanglingDirs enables the post move/delete prompt to remove directories
	// left empty by the completed job.
	DefaultRemoveDanglingDirs = true
	// DefaultRenameFocusAfter is the initial state of the rename dialog focus-after-rename checkbox.
	DefaultRenameFocusAfter = false

	// DefaultListingFormat is the persisted default_listing_format value (Name and size only).
	DefaultListingFormat = ListingFormatBrief

	// Panel zoom: widen the active browser column when [ui.zoom].active_panel is true.
	DefaultZoomActivePanel          = true
	DefaultPanelZoomActivePercent   = 70
	DefaultPanelZoomInactivePercent = 30
	// DefaultZoomActivePanelDisabledAboveWidth: when > 0 and terminal width (cells) is >= this value,
	// panel zoom is suppressed (even split). Use 0 to never disable zoom based on width.
	DefaultZoomActivePanelDisabledAboveWidth = 140
	// DefaultZoomActivePanelDisabledAboveHeight: when > 0 and terminal height (cells) is >= this value,
	// panel zoom is suppressed in stacked layout. Use 0 to never disable zoom based on height.
	DefaultZoomActivePanelDisabledAboveHeight = 45

	// Pane split orientation TOML values for [ui.zoom].orientation.
	PaneSplitSideBySide = "side_by_side"
	PaneSplitStacked    = "stacked"
	// DefaultPaneSplitOrientation is the default twin-pane layout (primary left, secondary right).
	DefaultPaneSplitOrientation = PaneSplitSideBySide

	// DefaultShrunkenShowsNameOnly: when true, file panels whose list row text width is below
	// ShrunkenListingRowTextWidthThreshold render only the name column (size / meta / mtime / perm hidden).
	DefaultShrunkenShowsNameOnly = true

	// DefaultPanelScrollbar is the file-list scrollbar indicator style (none, thumb, or bar).
	DefaultPanelScrollbar = "thumb"
	// DefaultPanelScrollbarInactive shows scrollbars on the inactive panel when true.
	DefaultPanelScrollbarInactive = false

	// DefaultScrollMode is the file-list scroll policy (minimal, center, or edge).
	DefaultScrollMode = "edge"
	// DefaultScrollEdgeMargin is rows of buffer above/below the cursor before edge mode scrolls.
	DefaultScrollEdgeMargin = 5
	// ScrollEdgeMarginMax is the upper bound for [ui.scroll].edge_margin after Validate.
	ScrollEdgeMarginMax = 50

	// DefaultScreenRenderHashCache skips terminal Show when the logical screen buffer matches the
	// last pushed frame (reduces flicker and I/O on slow links). Set [ui].screen_render_hash_cache = false to always flush.
	DefaultScreenRenderHashCache = true
	// ShrunkenListingRowTextWidthThreshold is the row text width (cells) below which a panel counts as
	// "shrunken" for optional name-only listing (see [ui].shrunken_shows_name_only).
	// 40 targets a 50/50 split on an 80-column terminal (inner listing width 38) with file icons off;
	// with icons on, the text budget is three cells narrower so the gate still trips.
	ShrunkenListingRowTextWidthThreshold = 40

	// MinCarouselPanelInnerWidth is the minimum interior width (cells inside the panel frame) for carousel mode.
	MinCarouselPanelInnerWidth = 72 // 3 × MinCarouselColumnWidth
	// MinCarouselColumnWidth is the minimum width per carousel column (parent, center, child).
	MinCarouselColumnWidth = 24
	// MinCarouselFilePreviewColumnWidth is the minimum child-column width for carousel file preview
	// (unless HideInactivePanel gives the active panel full terminal width).
	MinCarouselFilePreviewColumnWidth = 32

	// DefaultCarouselSplit is the default carousel column width spec (parent | center | child).
	DefaultCarouselSplit0 = "*"
	DefaultCarouselSplit1 = "*"
	DefaultCarouselSplit2 = "*"

	// DefaultUserMenuFileName is the basename of the user menu definition under the config directory
	// when [user_menu].file is empty.
	DefaultUserMenuFileName = "menu.toml"

	// DefaultMetaFileName is the basename of the meta column command definitions file under the
	// config directory when [meta].file is empty.
	DefaultMetaFileName = "meta.toml"

	// DefaultPoolsFileName is the basename of the work-pool definitions file under the config
	// directory when [pools].file is empty.
	DefaultPoolsFileName = "pools.toml"

	// DefaultMetaEntryWorkers is the number of concurrent background workers used per meta column
	// entry when the entry does not specify its own workers value in meta.toml.
	DefaultMetaEntryWorkers = 2

	// DefaultMetaMaxActiveColumns caps how many meta columns may be active per panel at once.
	DefaultMetaMaxActiveColumns = 8

	// DefaultPoolMaxParallel is the upper clamp for [[pools]].max_parallel in pools.toml.
	DefaultPoolMaxParallel = 64

	// DefaultMessageLogMaxEntries caps status/toast lines retained for the Messages view (oldest dropped).
	DefaultMessageLogMaxEntries = 500

	// PreviewModeInternal highlights files in-process with Chroma (default).
	PreviewModeInternal = "internal"
	// PreviewModeExternal runs [preview].command as a subprocess.
	PreviewModeExternal       = "external"
	DefaultPreviewMode        = PreviewModeInternal
	DefaultPreviewStyle       = "catppuccin-frappe"
	DefaultPreviewLineNumbers = true
	DefaultPreviewTabWidth    = 4
	// DefaultMaxPreviewBytes caps internal preview reads (matches cmdrun.MaxStreamBytes).
	DefaultMaxPreviewBytes = 512 * 1024

	// DefaultPreviewGitDiffContextLines is the unified-diff context (-U) for git-dirty
	// file previews (F3 / quick view / carousel). Git's default (~3) shows only local
	// hunks; a large value keeps unchanged lines so the preview is the whole file with
	// +/- markers. Stream size is still capped by cmdrun.MaxStreamBytes.
	DefaultPreviewGitDiffContextLines = 1_000_000

	// DefaultFilePreviewCommand runs bat with line numbers, paging disabled, colors forced on (non-TTY stdout),
	// and wrap/width driven by {terminal_width} so output matches the inactive preview column.
	DefaultFilePreviewCommand = "bat -n --paging=never --color=always --wrap=auto --terminal-width=%w %f"

	// DefaultSFTPIdleTimeoutSecs is how long an unused SFTP connection stays open in the pool.
	DefaultSFTPIdleTimeoutSecs = 60
	// DefaultSFTPDialTimeoutSecs limits connect/handshake time.
	DefaultSFTPDialTimeoutSecs = 30
	// DefaultSFTPListTimeoutSecs limits remote panel directory listing (ReadDir).
	DefaultSFTPListTimeoutSecs = 60

	// DefaultShellSyncCwdOnReturn navigates the active panel to the process cwd after drop-to-shell exits.
	DefaultShellSyncCwdOnReturn = true
	// DefaultShellPersistent keeps one MC-style shell session alive across Ctrl+O toggles (Linux only).
	DefaultShellPersistent = true
	// DefaultShellTerminalPanelHeight is the embedded terminal panel's content row count
	// (excludes the separator row); clamped to a minimum of 3 in Config.Validate.
	DefaultShellTerminalPanelHeight = 10
	// MinShellTerminalPanelHeight is the lower clamp for [shell].terminal_panel_height in Config.Validate.
	MinShellTerminalPanelHeight = 3
)
