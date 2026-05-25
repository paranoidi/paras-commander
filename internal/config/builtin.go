package config

// Built-in defaults for fields populated by Default() and used elsewhere (e.g. ops, diskusage).
// Single source of truth: do not duplicate these in internal/ops.

const (
	// DefaultPathPickerValidateDelayMS is the debounce before filesystem checks on the path-picker filter.
	DefaultPathPickerValidateDelayMS = 200

	// DefaultPanelSyncFollowNavDebounceMS delays mirroring the driver's file-list highlight into the
	// follower while latched panel sync is on. Terminals do not report key-up; after this many
	// milliseconds without another list cursor step, one sync runs (reduces disk I/O during key repeat).
	// Zero disables coalescing (sync every event, previous behavior).
	DefaultPanelSyncFollowNavDebounceMS = 100

	// DefaultQuickViewPreviewDebounceMS waits after the last listing highlight change before
	// re-running the preview command while Quick view is on. Zero runs immediately every reconcile.
	// Default matches DefaultPanelSyncFollowNavDebounceMS for consistent file-list scroll coalescing.
	DefaultQuickViewPreviewDebounceMS = 100

	// DefaultCarouselPreviewDebounceMS waits after the last file-list cursor step before reloading
	// the carousel child (next) column preview. Zero loads on every render (previous behavior).
	DefaultCarouselPreviewDebounceMS = 100

	DefaultDiskUsageWalkConcurrency = 4
	// DefaultFindWalkConcurrency limits concurrent directory reads during panel find indexing.
	DefaultFindWalkConcurrency = 4

	DefaultWorkerProgressMinBytes      = 512 * 1024
	DefaultWorkerProgressMinIntervalMS = 200

	// DefaultProgressUIWakeDebounceMS is minimum spacing between main-loop wakes after worker EventProgress.
	DefaultProgressUIWakeDebounceMS = 150

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

	// DefaultFlattenRecursive is the initial state of the flatten dialog recursive checkbox.
	DefaultFlattenRecursive = false
	// DefaultFlattenRemoveEmptyDirs is the initial state of the flatten dialog remove-empty checkbox.
	DefaultFlattenRemoveEmptyDirs = true

	// DefaultListingFormat is the persisted default_listing_format value (Modified time column).
	DefaultListingFormat = ListingFormatMtime

	// Panel zoom: widen the active browser column when [ui].zoom_active_panel is true.
	DefaultZoomActivePanel          = true
	DefaultPanelZoomActivePercent   = 70
	DefaultPanelZoomInactivePercent = 30
	// DefaultZoomActivePanelDisabledAboveWidth: when > 0 and terminal width (cells) is >= this value,
	// panel zoom is suppressed (even split). Use 0 to never disable zoom based on width.
	DefaultZoomActivePanelDisabledAboveWidth = 155

	// DefaultShrunkenShowsNameOnly: when true, file panels whose list row text width is below
	// ShrunkenListingRowTextWidthThreshold render only the name column (size / meta / mtime / perm hidden).
	DefaultShrunkenShowsNameOnly = true

	// DefaultCenterScrolling: when true, file-list navigation keeps the highlight row centered in the viewport.
	DefaultCenterScrolling = false

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

	// DefaultUserMenuFileName is the basename of the user menu definition under the config directory
	// when [user_menu].file is empty.
	DefaultUserMenuFileName = "menu.toml"

	// DefaultMetaFileName is the basename of the meta column command definitions file under the
	// config directory when [meta].file is empty.
	DefaultMetaFileName = "meta.toml"

	// DefaultMetaWorkers is the number of concurrent background workers used to run meta column
	// commands. Each worker processes one entry (file or directory) at a time.
	DefaultMetaWorkers = 4

	// DefaultMessageLogMaxEntries caps status/toast lines retained for the Messages view (oldest dropped).
	DefaultMessageLogMaxEntries = 500

	// DefaultFilePreviewCommand runs bat with paging disabled, colors forced on (non-TTY stdout),
	// and wrap/width driven by {terminal_width} so output matches the inactive preview column.
	DefaultFilePreviewCommand = "bat --paging=never --color=always --wrap=auto --terminal-width={terminal_width}"

	// DefaultSFTPIdleTimeoutSecs is how long an unused SFTP connection stays open in the pool.
	DefaultSFTPIdleTimeoutSecs = 60
	// DefaultSFTPDialTimeoutSecs limits connect/handshake time.
	DefaultSFTPDialTimeoutSecs = 30
	// DefaultSFTPListTimeoutSecs limits remote panel directory listing (ReadDir).
	DefaultSFTPListTimeoutSecs = 60
)
