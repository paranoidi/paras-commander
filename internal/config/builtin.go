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
	DefaultQuickViewPreviewDebounceMS = 75

	DefaultDiskUsageWalkConcurrency = 4

	DefaultProgressEmitMinBytes      = 512 * 1024
	DefaultProgressEmitMinIntervalMS = 200

	DefaultPreservePermissions        = true
	DefaultPreserveTimestamps         = true
	DefaultCopyBufferKiB              = 256
	DefaultSyncAfterEachFile          = true
	DefaultDiskSpaceCheckMinFileBytes = 50 * 1024 * 1024
	DefaultCowFileCloning             = true

	// DefaultListingFormat is the persisted default_listing_format value (Modified time column).
	DefaultListingFormat = ListingFormatMtime

	// Panel zoom: widen the active browser column when [ui].zoom_active_panel is true.
	DefaultZoomActivePanel          = false
	DefaultPanelZoomActivePercent   = 70
	DefaultPanelZoomInactivePercent = 30
	// DefaultZoomActivePanelDisabledAboveWidth: when > 0 and terminal width (cells) is >= this value,
	// panel zoom is suppressed (even split). Use 0 to never disable zoom based on width.
	DefaultZoomActivePanelDisabledAboveWidth = 155

	// DefaultShrunkenShowsNameOnly: when true, file panels whose list row text width is below
	// ShrunkenListingRowTextWidthThreshold render only the name column (size / meta / mtime / perm hidden).
	DefaultShrunkenShowsNameOnly = false
	// ShrunkenListingRowTextWidthThreshold is the row text width (cells) below which a panel counts as
	// "shrunken" for optional name-only listing (see [ui].shrunken_shows_name_only).
	// 40 targets a 50/50 split on an 80-column terminal (inner listing width 38) with file icons off;
	// with icons on, the text budget is three cells narrower so the gate still trips.
	ShrunkenListingRowTextWidthThreshold = 40

	// DefaultUserMenuFileName is the basename of the user menu definition under the config directory
	// when [user_menu].file is empty.
	DefaultUserMenuFileName = "menu.toml"

	// DefaultMessageLogMaxEntries caps status/toast lines retained for the Messages view (oldest dropped).
	DefaultMessageLogMaxEntries = 500

	// DefaultFilePreviewCommand runs bat with paging disabled, colors forced on (non-TTY stdout),
	// and wrap/width driven by {terminal_width} so output matches the inactive preview column.
	DefaultFilePreviewCommand = "bat --paging=never --color=always --wrap=auto --terminal-width={terminal_width}"
)
