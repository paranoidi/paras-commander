package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/preview/chromastyles"
)

const (
	appDirName              = "pc"
	fileName                = "config.toml"
	keybindingsFileBaseName = "keybindings.toml"

	ThemeDefault  = "default"
	SortName      = "name"
	SortExtension = "extension"
	SortSize      = "size"
	SortMtime     = "mtime"
	// Listing format root keys (default_listing_format).
	ListingFormatMtime = "mtime"
	ListingFormatPerm  = "perm"
	ListingFormatBrief = "brief"
	SortDiskUsage      = "disk_usage"
	DeletePermanent    = "permanent"
	FilterModeFuzzy    = "fuzzy"
	FilterSyntaxFZF    = "subset-fzf"
	// FilterCycleMatchesVisual orders ↑/↓ among matches by panel row (basename sort order).
	FilterCycleMatchesVisual = "visual"
	// FilterCycleMatchesRanked orders ↑/↓ by fuzzy rank (best match first).
	FilterCycleMatchesRanked = "ranked"
	ScrollModeMinimal        = "minimal"
	ScrollModeCenter         = "center"
	ScrollModeEdge           = "edge"
	PanelScrollbarNone       = "none"
	PanelScrollbarThumb      = "thumb"
	PanelScrollbarBar        = "bar"
)

// Paths identifies configuration files discovered from XDG paths.
type Paths struct {
	ConfigDir        string
	ConfigFile       string
	ThemesDir        string
	PreviewStylesDir string
	KeybindingsFile  string
}

// WithResolvedLocations copies paths and derives ConfigDir or ConfigFile when only one side is set.
// When ThemesDir is empty but ConfigDir is known, ThemesDir becomes filepath.Join(ConfigDir, "themes")
// so themes on disk override embedded themes with the same declared `name` (see theme.Resolve).
func (paths Paths) WithResolvedLocations() Paths {
	out := paths
	f, err := resolvePersistPaths(out)
	if err != nil {
		return out.withDerivedThemesDir()
	}
	out.ConfigFile = f
	if strings.TrimSpace(out.ConfigDir) == "" {
		out.ConfigDir = filepath.Dir(f)
	}
	return out.withDerivedThemesDir()
}

func (paths Paths) withDerivedThemesDir() Paths {
	out := paths
	cd := strings.TrimSpace(out.ConfigDir)
	if strings.TrimSpace(out.ThemesDir) == "" && cd != "" {
		out.ThemesDir = filepath.Join(cd, "themes")
	}
	if strings.TrimSpace(out.PreviewStylesDir) == "" && cd != "" {
		out.PreviewStylesDir = filepath.Join(cd, "themes", "preview")
	}
	return out
}

// Config is the parsed general application configuration.
type Config struct {
	Theme      string           `toml:"theme"`
	Panels     PanelsConfig     `toml:"panels"`
	DiskUsage  DiskUsageConfig  `toml:"disk_usage"`
	FSWalk     FSWalkConfig     `toml:"fs_walk"`
	Carousel   CarouselConfig   `toml:"carousel"`
	UI         UIConfig         `toml:"ui"`
	Filter     FilterConfig     `toml:"filter"`
	Jobs       JobsConfig       `toml:"jobs"`
	Operations OperationsConfig `toml:"operations"`
	Logging    LoggingConfig    `toml:"logging"`
	Bookmarks  BookmarksConfig  `toml:"bookmarks"`
	UserMenu   UserMenuConfig   `toml:"user_menu"`
	Preview    PreviewConfig    `toml:"preview"`
	SFTP       SFTPConfig       `toml:"sftp"`
	Shell      ShellConfig      `toml:"shell"`
	// Meta configures the separate meta.toml command definitions file and execution settings.
	Meta MetaConfig `toml:"meta"`
	// Pools configures discovery of the separate pools.toml work-pool definitions file.
	Pools PoolsConfig `toml:"pools"`
	// MassRename configures discovery of the separate patterns.toml saved mass-rename patterns file.
	MassRename MassRenameConfig `toml:"mass_rename"`
	// Compare configures twin-panel directory compare (content hash diff).
	Compare CompareConfig `toml:"compare"`
	// Dedup configures find-duplicates within a single directory.
	Dedup DedupConfig `toml:"dedup"`
	// StatusCommand runs a shell command on an interval and shows its output at top-left.
	StatusCommand StatusCommandConfig `toml:"status_command"`
}

// PanelsConfig controls file panel browsing, sorting, and listing.
type PanelsConfig struct {
	ShowHidden           bool   `toml:"show_hidden"`
	RespectGitignore     bool   `toml:"respect_gitignore"`
	DefaultSort          string `toml:"default_sort"`
	DefaultListingFormat string `toml:"default_listing_format"`
	SortReverse          bool   `toml:"sort_reverse"`
	DirectoriesFirst     bool   `toml:"directories_first"`
	// RefreshIntervalMS re-reads both panel directories on this interval in background goroutines (0 disables).
	RefreshIntervalMS     int  `toml:"refresh_interval_ms"`
	OpenFilesExternally   bool `toml:"open_files_externally"`
	RunExecutablesOnEnter bool `toml:"run_executables_on_enter"`
}

// DiskUsageConfig controls the disk-usage (F-key) view and its background walk.
type DiskUsageConfig struct {
	IdleSizeSort           bool `toml:"idle_size_sort"`
	IdleSortDelayMS        int  `toml:"idle_sort_delay_ms"`
	DescendIntoMountPoints bool `toml:"descend_into_mount_points"`
}

// FSWalkConfig controls adaptive concurrency for recursive filesystem walks (find + disk-usage).
type FSWalkConfig struct {
	InitialWorkers  int `toml:"initial_workers"`
	MaxWorkers      int `toml:"max_workers"`
	AdaptIntervalMS int `toml:"adapt_interval_ms"`
}

// CarouselConfig controls the carousel (multi-column) panel layout.
type CarouselConfig struct {
	// Split sets parent | center | child column widths: fixed cells ("16"), percent of
	// remaining width after fixed columns ("20%"), or flex remainder ("*"). Exactly 3 entries.
	Split []string `toml:"split"`
	// ShowSize toggles the size column per carousel pane (exactly 3 booleans).
	ShowSize []bool `toml:"show_size"`
	// AutohideInactivePanel hides the inactive twin panel while the active panel is in
	// carousel mode, giving its columns the full width. The panel reappears when Tab makes
	// it the active panel, and hides again when Tab leaves it. Has no effect outside carousel mode.
	AutohideInactivePanel bool `toml:"autohide_inactive_panel"`
}

// DedupConfig controls find-duplicates scanning.
type DedupConfig struct {
	// HashConfirmBytes pauses before hashing when the total byte size of hash
	// candidates exceeds this value. Zero disables the confirmation gate.
	HashConfirmBytes int64 `toml:"hash_confirm_bytes"`
	// FileProgressBytes shows a per-file progress bar in the scan dialog for
	// files at or above this size. Zero disables the per-file bar.
	FileProgressBytes int64 `toml:"file_progress_bytes"`
	// ChunkBytes compares same-size files this many bytes at a time, bailing out
	// of a file as soon as its content diverges. Zero disables chunking.
	ChunkBytes int64 `toml:"chunk_bytes"`
}

// CompareConfig controls panel compare hashing and walk options.
type CompareConfig struct {
	HashConcurrency     int   `toml:"hash_concurrency"`
	ReadBufferKiB       int   `toml:"read_buffer_kib"`
	MaxHashBytes        int64 `toml:"max_hash_bytes"`
	StayOnVolumeDefault bool  `toml:"stay_on_volume_default"`
}

// PoolsConfig controls discovery of the separate pools.toml file.
type PoolsConfig struct {
	// File is an absolute path or ~/… to the global pools.toml. Empty uses
	// filepath.Join(configDir, DefaultPoolsFileName) after paths are resolved.
	File string `toml:"file"`
}

// MassRenameConfig controls discovery of the separate patterns.toml file (saved mass-rename
// find/replace patterns).
type MassRenameConfig struct {
	// File is an absolute path to the global patterns.toml. Empty uses
	// filepath.Join(configDir, DefaultMassRenamePatternsFileName) after paths are resolved.
	File string `toml:"file"`
}

// MetaConfig controls discovery of the separate meta.toml and execution settings.
type MetaConfig struct {
	// File is an absolute path or ~/… to the global meta.toml. Empty uses
	// filepath.Join(configDir, DefaultMetaFileName) after paths are resolved.
	File string `toml:"file"`
	// LocalNames lists basenames probed in the active panel directory before the global file.
	// Empty in config means use built-in default (see Default().Meta).
	LocalNames []string `toml:"local_names"`
	// DefaultEntryWorkers is the number of concurrent background goroutines used per meta
	// entry when the entry does not specify its own workers value in meta.toml.
	// Minimum 1 after Validate. Default DefaultMetaEntryWorkers.
	DefaultEntryWorkers int `toml:"default_entry_workers"`
}

// BookmarksConfig controls fzf-marks compatible directory marks.
type BookmarksConfig struct {
	// File is the marks file path. Empty uses FZF_MARKS_FILE or ~/.fzf-marks.
	File string `toml:"file"`
}

// UserMenuConfig controls discovery of the separate menu.toml (F2 user menu).
type UserMenuConfig struct {
	// File is an absolute path or ~/… to the global menu.toml. Empty uses
	// filepath.Join(configDir, DefaultUserMenuFileName) after paths are resolved.
	File string `toml:"file"`
	// LocalNames lists basenames probed in the active panel directory before the global file.
	// Empty in config means use built-in default (see Default().UserMenu).
	LocalNames []string `toml:"local_names"`
}

// ShellConfig controls open shell (suspend TUI, run interactive shell, resume).
type ShellConfig struct {
	// Command is an optional argv template parsed like shellwords (see cmdrun.ParseCommandArgv).
	// Empty uses $SHELL then bash fallback. Setting it forces the one-shot shell even when
	// Persistent is true (a custom argv is incompatible with the persistent PTY session).
	Command string `toml:"command"`
	// SyncCwdOnReturn navigates the active panel to the shell cwd after returning from the shell.
	SyncCwdOnReturn bool `toml:"sync_cwd_on_return"`
	// Persistent keeps one MC-style shell session alive across Ctrl+O toggles (Linux only;
	// falls back to the one-shot shell elsewhere or when the PTY cannot start).
	Persistent bool `toml:"persistent"`
	// TerminalPanelHeight is the embedded terminal panel's content row count (excludes the
	// separator row). Minimum 3 after Validate. Default DefaultShellTerminalPanelHeight.
	TerminalPanelHeight int `toml:"terminal_panel_height"`
}

// StatusCommandConfig runs a shell command on an interval and shows its first line of
// output at the top-left of the menu bar (menu labels and job bars shift right to make room).
type StatusCommandConfig struct {
	// Command is a shell command line, run via sh -c (no meta.toml-style %f/%d/... macro
	// expansion — there is no per-panel/per-row context for a global command). Empty disables
	// the feature.
	Command string `toml:"command"`
	// IntervalMS is how often Command runs. Minimum StatusCommandIntervalMinMS after Validate.
	// Default DefaultStatusCommandIntervalMS.
	IntervalMS int `toml:"interval_ms"`
	// MaxWidth caps the reserved column width for the displayed text (longer output is
	// ellipsized). Clamped to [StatusCommandMaxWidthMin, StatusCommandMaxWidthMax] after
	// Validate. Default DefaultStatusCommandMaxWidth.
	MaxWidth int `toml:"max_width"`
}

// PreviewConfig controls file preview (internal Chroma or external command).
type PreviewConfig struct {
	// Mode is "internal" (Chroma in-process) or "external" (subprocess command).
	Mode string `toml:"mode"`
	// Style is the Chroma style name for internal mode (default catppuccin-frappe).
	Style string `toml:"style"`
	// LineNumbers prefixes each line with a gutter in internal mode.
	LineNumbers bool `toml:"line_numbers"`
	// Command is a single-line argv template parsed like shellwords (see cmdrun.ParseCommandArgv).
	// Use %f once to insert the absolute file path as one token; use %w for terminal width; if %f is omitted, the path is appended.
	Command string `toml:"command"`
	// Images enables in-process image previews (F3 / quick view / carousel) via sixel or
	// Kitty graphics. When false, image paths and video thumbnails show metadata text instead.
	Images bool `toml:"images"`
	// ImageProtocol selects the terminal graphics protocol: "auto", "sixel", or "kitty".
	// Empty/invalid values normalize to DefaultPreviewImageProtocol ("auto").
	ImageProtocol string `toml:"image_protocol"`
	// TerminalSixel / TerminalKitty / TerminalKittyPlaceholder are tri-state user confirmations
	// ("auto"/"yes"/"no", default "auto") for terminal graphics capabilities that environment/tmux
	// introspection alone cannot always answer — set via the M-F3 image-capabilities dialog rather
	// than by hand-editing this file. "auto" leaves the ResolveImageProtocol/
	// TmuxSupportsKittyUnicodePlaceholders heuristics in charge; "yes"/"no" pin the answer.
	//
	// TerminalSixel / TerminalKitty override which graphics protocol "auto" picks when exactly one
	// is "yes" (or overturn a heuristic guess of the other when it's "no"); TerminalKittyPlaceholder
	// additionally confirms Kitty Unicode-placeholder support under tmux for terminals like WezTerm,
	// where Kitty protocol support is reliable and always assumed (cursor-relative placement) but
	// placeholder support is an experimental, build-specific capability that tmux's client_termtype
	// can't distinguish from a build that lacks it — confirm this only once you've verified your
	// attached build actually supports it; against a build that doesn't, placeholder mode sends
	// cells the terminal can't interpret and nothing renders.
	TerminalSixel            string `toml:"terminal_sixel"`
	TerminalKitty            string `toml:"terminal_kitty"`
	TerminalKittyPlaceholder string `toml:"terminal_kitty_placeholder"`
	// VideoThumbCols / VideoThumbRows set the video thumbnail grid size (default 2×2).
	VideoThumbCols int `toml:"video_thumb_cols"`
	VideoThumbRows int `toml:"video_thumb_rows"`
	// Prefetch enables background decode of nearby images and video thumbnail generation.
	Prefetch bool `toml:"prefetch"`
	// PrefetchAlways, when true, runs prefetch whenever Prefetch is on. When false (default),
	// prefetch only runs while quick view is latched or the active panel is in carousel mode.
	PrefetchAlways bool `toml:"prefetch_always"`
	// QuickViewDisableOnInactiveNav, when true (default), turns off quick view and shows a
	// toast whenever the inactive (non-driver) panel navigates to a new directory — e.g. via
	// Alt+I/Alt+O, a bookmark "open in other panel", or find/compare/dedup/history/SFTP results —
	// since quick view would otherwise immediately overlay the freshly opened listing with a preview.
	QuickViewDisableOnInactiveNav bool `toml:"quick_view_disable_on_inactive_nav"`
	// PrefetchWorkers is the worker-pool size for background prefetch (default 4).
	PrefetchWorkers int `toml:"prefetch_workers"`
	// PrefetchWindow bounds background prefetch to entries within this many positions of the
	// cursor in each direction (default 5, clamped 1-50). Farther entries are never queued, so
	// they can't evict already-warm near-cursor cache entries.
	PrefetchWindow int `toml:"prefetch_window"`
	// ImageMaxEdgePx caps the longest edge of decoded stills, for protocols/contexts that
	// don't need the tmux-sixel payload-safety clamp below (default 0 = unrestricted).
	// Applied even when Prefetch is false.
	ImageMaxEdgePx int `toml:"image_max_edge_px"`
	// TmuxSixelMaxEdgePx caps the longest edge of decoded stills and video-thumb grids for
	// Sixel under tmux (default 1024, floor 64). Keeps tmux graphics payloads under its
	// hardcoded ~1MB input buffer limit.
	TmuxSixelMaxEdgePx int `toml:"tmux_sixel_max_edge_px"`
	// VideoThumbMaxEdgePx caps the longest edge of composited video-thumb grids for
	// protocols/contexts that don't need the tmux-sixel clamp above (default 2048, floor 64).
	// Unlike ImageMaxEdgePx, 0/unset always falls back to a concrete default: a video-thumb
	// grid composites native-resolution frames before this clamp applies, so it can't go
	// unrestricted.
	VideoThumbMaxEdgePx int `toml:"video_thumb_max_edge_px"`
	// PrefetchMemoryMaxMB is the in-memory prefetch LRU budget in MiB (default 256).
	PrefetchMemoryMaxMB int `toml:"prefetch_memory_max_mb"`
	// VideoThumbCacheMaxMB caps the on-disk video thumbnail cache under
	// $XDG_CACHE_HOME/pc/video-thumbs/ (default 512).
	VideoThumbCacheMaxMB int `toml:"video_thumb_cache_max_mb"`
}

type UIConfig struct {
	ShowMenuBar   bool `toml:"show_menu_bar"`
	ShowFileIcons bool `toml:"show_file_icons"`
	// LeaderMenuShowDirectKeys shows the preferred global keybind after each action name in the Esc function menu.
	LeaderMenuShowDirectKeys bool `toml:"leader_menu_show_direct_keys"`
	// ShrunkenShowsNameOnly: when true, narrow panels hide trailing listing columns and show only names
	// (sort and default_listing_format are unchanged; see ShrunkenListingRowTextWidthThreshold in builtin.go).
	ShrunkenShowsNameOnly bool `toml:"shrunken_shows_name_only"`
	// ScreenRenderHashCache, when true, hashes the logical cell buffer after each full render and skips
	// screen.Show when unchanged from the last flush. Default DefaultScreenRenderHashCache.
	ScreenRenderHashCache bool `toml:"screen_render_hash_cache"`
	// KeyRepeatDebounceMS coalesces rapid file-list cursor steps, quick view preview reloads,
	// carousel child preview reloads, and F3 style-picker re-highlighting. Zero disables debouncing.
	// Default DefaultKeyRepeatDebounceMS.
	KeyRepeatDebounceMS int `toml:"key_repeat_debounce_ms"`
	// PathPickerValidateDelayMS waits after the filter changes before checking whether the typed path exists.
	// Default DefaultPathPickerValidateDelayMS. Use 0 to validate on the next scheduler tick (still not per-key synchronous).
	PathPickerValidateDelayMS int `toml:"path_picker_validate_delay_ms"`
	// SelectionsPanelMaxRows caps visible rows in the cross-directory selections strip (0 = default 5).
	SelectionsPanelMaxRows int `toml:"selections_panel_max_rows"`
	// SelectionsPanelActivePercent is the strip share of panel height when focused in side-by-side
	// layout, and of panel width when the strip is shown in stacked layout (default 50; clamped 10–90).
	SelectionsPanelActivePercent int `toml:"selections_panel_active_percent"`

	Zoom   UIZoomConfig   `toml:"zoom"`
	Scroll UIScrollConfig `toml:"scroll"`
	Find   UIFindConfig   `toml:"find"`
	Status UIStatusConfig `toml:"status"`
}

type UIZoomConfig struct {
	// ActivePanel widens the active browser column; inactive column uses the remainder (see ActivePercent/InactivePercent).
	ActivePanel bool `toml:"active_panel"`
	// DisabledAboveWidth: when > 0 and terminal width (cells) is >= this value, zoom is not applied
	// (50/50 split). Zero disables this gate (zoom follows active_panel and runtime toggle only).
	// Default DefaultZoomActivePanelDisabledAboveWidth.
	DisabledAboveWidth int `toml:"disabled_above_width"`
	// DisabledAboveHeight: when > 0 and terminal height (cells) is >= this value, zoom is not
	// applied in stacked (top/bottom) layout. Zero disables this gate.
	// Default DefaultZoomActivePanelDisabledAboveHeight.
	DisabledAboveHeight int `toml:"disabled_above_height"`
	// Orientation selects twin-pane layout: "side_by_side" (default) or "stacked".
	Orientation string `toml:"orientation"`
	// ActivePercent and InactivePercent are the width shares (must sum to 100) when ActivePanel is true.
	ActivePercent   int `toml:"active_percent"`
	InactivePercent int `toml:"inactive_percent"`
}

type UIScrollConfig struct {
	// Mode selects file-list scroll policy: minimal, center, or edge.
	Mode string `toml:"mode"`
	// EdgeMargin is rows of buffer above/below the cursor before edge mode scrolls.
	EdgeMargin int `toml:"edge_margin"`
	// Scrollbar selects the vertical scroll indicator style for panel lists (none, thumb, bar).
	Scrollbar string `toml:"scrollbar"`
	// ScrollbarInactive shows scroll indicators on the inactive panel when true.
	ScrollbarInactive bool `toml:"scrollbar_inactive"`
}

type UIFindConfig struct {
	// QueryDebounceMS waits after the last keystroke in the find dialog query field before
	// re-ranking the result list. Reducing this to 0 ranks on every keystroke (no debounce).
	// Default DefaultFindQueryDebounceMS.
	QueryDebounceMS int `toml:"query_debounce_ms"`
	// MaxResults caps the number of ranked results shown in the find dialog. The full index
	// is always retained; only the top-N scored entries are kept after each rank.
	// Default DefaultFindMaxResults.
	MaxResults int `toml:"max_results"`
	// ListNavIdleMS is the navigation-idle delay before a background rank update is applied
	// to the find result list. Resets on every Up/Down/PgUp/PgDn movement. Zero applies updates
	// immediately regardless of navigation.
	// Default DefaultFindListNavIdleMS.
	ListNavIdleMS int `toml:"list_nav_idle_ms"`
}

type UIStatusConfig struct {
	// MessageTTLSeconds is how long transient status banners stay visible before clearing.
	// Default 4.5. Use 0 to keep messages until replaced or cleared by another action.
	MessageTTLSeconds float64 `toml:"message_ttl_seconds"`
	// LogMaxEntries caps how many status/toast lines are retained for the Messages view (oldest dropped).
	// Zero means use the built-in default (see DefaultMessageLogMaxEntries).
	LogMaxEntries int `toml:"log_max_entries"`
}

type FilterConfig struct {
	Mode              string `toml:"mode"`
	Syntax            string `toml:"syntax"`
	MatchPathSegments bool   `toml:"match_path_segments"`
	// CycleMatches controls Up/Down among quick-filter matches: "visual" (default) or "ranked".
	CycleMatches string `toml:"cycle_matches"`
	// CaseInsensitive controls case sensitivity of the quick filter and find dialog.
	CaseInsensitive bool `toml:"case_insensitive"`
}

type JobsConfig struct {
	ShowFinished    bool `toml:"show_finished"`
	KeepFinished    int  `toml:"keep_finished"`
	AutoshowOnError bool `toml:"autoshow_on_error"`
	AutoshowOnStart bool `toml:"autoshow_on_start"`
	// ProgressUIWakeDebounceMS is minimum spacing between jobsWakePayload interrupts after worker EventProgress.
	ProgressUIWakeDebounceMS int `toml:"progress_ui_wake_debounce_ms"`
	// BlockerDialogNextDebounceMS is the delay before auto-opening the next quick blocker dialog after an answer (0 = immediate).
	BlockerDialogNextDebounceMS int `toml:"blocker_dialog_next_debounce_ms"`
	// WorkerProgressMinBytes is the minimum bytes copied between worker EventProgress emits.
	WorkerProgressMinBytes int `toml:"worker_progress_min_bytes"`
	// WorkerProgressMinIntervalMS is the minimum milliseconds between worker EventProgress emits when copying.
	WorkerProgressMinIntervalMS int `toml:"worker_progress_min_interval_ms"`
	// ThroughputChartWindowSec is the wall-time span shown by the jobs details throughput chart (20–120).
	ThroughputChartWindowSec int `toml:"throughput_chart_window_sec"`
	// ThroughputChartColumnMS is milliseconds per chart column and chart ticker interval; 80–2000 after Validate.
	ThroughputChartColumnMS int `toml:"throughput_chart_column_ms"`
	// ThroughputChartEnabled controls the details-panel throughput strip and chart rendering.
	ThroughputChartEnabled bool `toml:"throughput_chart_enabled"`
	// FreeSpaceOnProgressWake runs async statfs on both panels when a progress UI wake is applied (see applyJobRefreshes).
	FreeSpaceOnProgressWake bool `toml:"free_space_on_progress_wake"`
	// FreeSpacePollIntervalSecs is how often to refresh panel free space while any job is unfinished (0 disables).
	FreeSpacePollIntervalSecs int `toml:"free_space_poll_interval_secs"`
	// ScanYieldIntervalMS is cooperative sleep during pre-scan while a transfer job is running.
	ScanYieldIntervalMS int `toml:"scan_yield_interval_ms"`
	// ScanYieldEveryN triggers cooperative yield every N walk entries during pre-scan while a transfer is active.
	ScanYieldEveryN int `toml:"scan_yield_every_n"`
	// ScanNiceIncrement is added to nice on Linux for pre-scan when a transfer is active (0 uses builtin default).
	ScanNiceIncrement int `toml:"scan_nice_increment"`
	// ScanProgressMinIntervalMS throttles scan-progress events during pre-scan.
	ScanProgressMinIntervalMS int `toml:"scan_progress_min_interval_ms"`
}

type OperationsConfig struct {
	// ConfirmDelete shows a confirmation dialog before deleting files.
	ConfirmDelete bool `toml:"confirm_delete"`
	// DeleteMode selects how delete jobs remove files (currently only "permanent").
	DeleteMode          string `toml:"delete_mode"`
	PreservePermissions bool   `toml:"preserve_permissions"`
	PreserveTimestamps  bool   `toml:"preserve_timestamps"`
	CopyBufferKiB       int    `toml:"copy_buffer_kib"`
	// SyncAfterEachFile fsyncs each copied file before closing (durable; slow for many small files).
	SyncAfterEachFile bool `toml:"sync_after_each_file"`
	// DiskSpaceCheckMinFileBytes: per-file mid-copy disk checks run only when the source file size is >= this value.
	// Zero means run the check before every file.
	DiskSpaceCheckMinFileBytes int64 `toml:"disk_space_check_min_file_bytes"`
	// CowFileCloning enables Linux FICLONE (CoW) when supported (similar to Midnight Commander file cloning).
	CowFileCloning bool `toml:"cow_file_cloning"`
	// CopyFileRange tries Linux copy_file_range(2) after FICLONE before userspace read/write.
	CopyFileRange bool `toml:"copy_file_range"`
	// SparseFileCopy preserves holes on Linux via SEEK_DATA/SEEK_HOLE before userspace read/write.
	SparseFileCopy bool `toml:"sparse_file_copy"`
	// PreallocateDestination reserves destination space (fallocate or truncate) before copy when source size is known.
	PreallocateDestination bool `toml:"preallocate_destination"`
	// PreallocateMinFileBytes applies preallocation only when the source file is at least this large (0 = always).
	PreallocateMinFileBytes int64 `toml:"preallocate_min_file_bytes"`
	// SyncAtJobEnd fsyncs copied local files once after the job when sync_after_each_file is false.
	SyncAtJobEnd bool `toml:"sync_at_job_end"`
	// SyncMinFileKiB skips fsync for copied files smaller than this threshold (0 = no minimum).
	SyncMinFileKiB int `toml:"sync_min_file_kib"`
	// FlattenDefaultLocation is the default destination prefill panel: "active" or "inactive".
	FlattenDefaultLocation string `toml:"flatten_default_location"`
	// FlattenRecursive is the default for the flatten dialog recursive checkbox.
	FlattenRecursive bool `toml:"flatten_recursive"`
	// FlattenRemoveEmptyDirs is the default for the flatten dialog remove-empty checkbox.
	FlattenRemoveEmptyDirs bool `toml:"flatten_remove_empty_dirs"`
	// RenameFocusAfter is the default for the rename dialog focus-after-rename checkbox.
	RenameFocusAfter bool `toml:"rename_focus_after"`
	// RemoveDanglingDirs prompts (delete confirm dialog, default Yes) to remove directories
	// left empty by a browser move or delete job after it completes.
	RemoveDanglingDirs bool `toml:"remove_dangling_directories"`
}

type LoggingConfig struct {
	Enabled bool   `toml:"enabled"`
	Level   string `toml:"level"`
	Path    string `toml:"path"`
}

// Default returns built-in values that match the current application behavior.
func Default() Config {
	return Config{
		Theme: ThemeDefault,
		Panels: PanelsConfig{
			ShowHidden:            false,
			RespectGitignore:      true,
			DefaultSort:           SortName,
			DefaultListingFormat:  DefaultListingFormat,
			SortReverse:           false,
			DirectoriesFirst:      true,
			RefreshIntervalMS:     DefaultRefreshIntervalMS,
			OpenFilesExternally:   true,
			RunExecutablesOnEnter: true,
		},
		DiskUsage: DiskUsageConfig{
			IdleSizeSort:           true,
			IdleSortDelayMS:        500,
			DescendIntoMountPoints: false,
		},
		FSWalk: FSWalkConfig{
			InitialWorkers:  DefaultFSWalkInitialWorkers,
			MaxWorkers:      DefaultFSWalkMaxWorkers,
			AdaptIntervalMS: DefaultFSWalkAdaptIntervalMS,
		},
		Carousel: CarouselConfig{
			Split:                 DefaultCarouselSplit(),
			ShowSize:              DefaultCarouselShowSize(),
			AutohideInactivePanel: DefaultCarouselAutohideInactivePanel,
		},
		UI: UIConfig{
			ShowMenuBar:                  true,
			ShowFileIcons:                true,
			LeaderMenuShowDirectKeys:     DefaultLeaderMenuShowDirectKeys,
			ShrunkenShowsNameOnly:        DefaultShrunkenShowsNameOnly,
			ScreenRenderHashCache:        DefaultScreenRenderHashCache,
			KeyRepeatDebounceMS:          DefaultKeyRepeatDebounceMS,
			PathPickerValidateDelayMS:    DefaultPathPickerValidateDelayMS,
			SelectionsPanelMaxRows:       0,
			SelectionsPanelActivePercent: DefaultSelectionsPanelActivePercent,
			Zoom: UIZoomConfig{
				ActivePanel:         DefaultZoomActivePanel,
				DisabledAboveWidth:  DefaultZoomActivePanelDisabledAboveWidth,
				DisabledAboveHeight: DefaultZoomActivePanelDisabledAboveHeight,
				Orientation:         DefaultPaneSplitOrientation,
				ActivePercent:       DefaultPanelZoomActivePercent,
				InactivePercent:     DefaultPanelZoomInactivePercent,
			},
			Scroll: UIScrollConfig{
				Mode:              DefaultScrollMode,
				EdgeMargin:        DefaultScrollEdgeMargin,
				Scrollbar:         DefaultPanelScrollbar,
				ScrollbarInactive: DefaultPanelScrollbarInactive,
			},
			Find: UIFindConfig{
				QueryDebounceMS: DefaultFindQueryDebounceMS,
				MaxResults:      DefaultFindMaxResults,
				ListNavIdleMS:   DefaultFindListNavIdleMS,
			},
			Status: UIStatusConfig{
				MessageTTLSeconds: 4.5,
				LogMaxEntries:     DefaultMessageLogMaxEntries,
			},
		},
		Filter: FilterConfig{
			Mode:              FilterModeFuzzy,
			Syntax:            FilterSyntaxFZF,
			MatchPathSegments: false,
			CycleMatches:      FilterCycleMatchesVisual,
			CaseInsensitive:   true,
		},
		Jobs: JobsConfig{
			ShowFinished:                true,
			KeepFinished:                20,
			AutoshowOnError:             true,
			AutoshowOnStart:             false,
			ProgressUIWakeDebounceMS:    DefaultProgressUIWakeDebounceMS,
			BlockerDialogNextDebounceMS: DefaultBlockerDialogNextDebounceMS,
			WorkerProgressMinBytes:      DefaultWorkerProgressMinBytes,
			WorkerProgressMinIntervalMS: DefaultWorkerProgressMinIntervalMS,
			ThroughputChartWindowSec:    DefaultThroughputChartWindowSec,
			ThroughputChartColumnMS:     DefaultThroughputChartColumnMS,
			ThroughputChartEnabled:      DefaultThroughputChartEnabled,
			FreeSpaceOnProgressWake:     DefaultFreeSpaceOnProgressWake,
			FreeSpacePollIntervalSecs:   DefaultFreeSpacePollIntervalSecs,
			ScanYieldIntervalMS:         DefaultScanYieldIntervalMS,
			ScanYieldEveryN:             DefaultScanYieldEveryN,
			ScanNiceIncrement:           DefaultScanNiceIncrement,
			ScanProgressMinIntervalMS:   DefaultScanProgressMinIntervalMS,
		},
		Operations: OperationsConfig{
			ConfirmDelete:              true,
			DeleteMode:                 DeletePermanent,
			PreservePermissions:        DefaultPreservePermissions,
			PreserveTimestamps:         DefaultPreserveTimestamps,
			CopyBufferKiB:              DefaultCopyBufferKiB,
			SyncAfterEachFile:          DefaultSyncAfterEachFile,
			DiskSpaceCheckMinFileBytes: DefaultDiskSpaceCheckMinFileBytes,
			CowFileCloning:             DefaultCowFileCloning,
			CopyFileRange:              DefaultCopyFileRange,
			SparseFileCopy:             DefaultSparseFileCopy,
			PreallocateDestination:     DefaultPreallocateDestination,
			PreallocateMinFileBytes:    DefaultPreallocateMinFileBytes,
			SyncAtJobEnd:               DefaultSyncAtJobEnd,
			SyncMinFileKiB:             DefaultSyncMinFileKiB,
			FlattenDefaultLocation:     DefaultFlattenDefaultLocation,
			FlattenRecursive:           DefaultFlattenRecursive,
			FlattenRemoveEmptyDirs:     DefaultFlattenRemoveEmptyDirs,
			RenameFocusAfter:           DefaultRenameFocusAfter,
			RemoveDanglingDirs:         DefaultRemoveDanglingDirs,
		},
		Logging: LoggingConfig{
			Enabled: false,
			Level:   "info",
			Path:    "",
		},
		Bookmarks: BookmarksConfig{
			File: "",
		},
		UserMenu: UserMenuConfig{
			File:       "",
			LocalNames: []string{DefaultUserMenuFileName},
		},
		Preview: PreviewConfig{
			Mode:                          DefaultPreviewMode,
			Style:                         DefaultPreviewStyle,
			LineNumbers:                   DefaultPreviewLineNumbers,
			Command:                       DefaultFilePreviewCommand,
			Images:                        DefaultPreviewImages,
			ImageProtocol:                 DefaultPreviewImageProtocol,
			TerminalSixel:                 DefaultPreviewTerminalSixel,
			TerminalKitty:                 DefaultPreviewTerminalKitty,
			TerminalKittyPlaceholder:      DefaultPreviewTerminalKittyPlaceholder,
			VideoThumbCols:                DefaultPreviewVideoThumbCols,
			VideoThumbRows:                DefaultPreviewVideoThumbRows,
			Prefetch:                      DefaultPreviewPrefetch,
			PrefetchAlways:                DefaultPreviewPrefetchAlways,
			QuickViewDisableOnInactiveNav: DefaultPreviewQuickViewDisableOnInactiveNav,
			PrefetchWorkers:               DefaultPreviewPrefetchWorkers,
			PrefetchWindow:                DefaultPreviewPrefetchWindow,
			ImageMaxEdgePx:                DefaultPreviewImageMaxEdgePx,
			TmuxSixelMaxEdgePx:            DefaultPreviewTmuxSixelMaxEdgePx,
			VideoThumbMaxEdgePx:           DefaultPreviewVideoThumbMaxEdgePx,
			PrefetchMemoryMaxMB:           DefaultPreviewPrefetchMemoryMaxMB,
			VideoThumbCacheMaxMB:          DefaultPreviewVideoThumbCacheMaxMB,
		},
		SFTP: SFTPConfig{
			IdleTimeoutSecs: DefaultSFTPIdleTimeoutSecs,
			DialTimeoutSecs: DefaultSFTPDialTimeoutSecs,
			ListTimeoutSecs: DefaultSFTPListTimeoutSecs,
		},
		Shell: ShellConfig{
			SyncCwdOnReturn:     DefaultShellSyncCwdOnReturn,
			Persistent:          DefaultShellPersistent,
			TerminalPanelHeight: DefaultShellTerminalPanelHeight,
		},
		Meta: MetaConfig{
			File:                "",
			LocalNames:          []string{DefaultMetaFileName},
			DefaultEntryWorkers: DefaultMetaEntryWorkers,
		},
		Pools: PoolsConfig{
			File: "",
		},
		MassRename: MassRenameConfig{
			File: "",
		},
		Compare: CompareConfig{
			HashConcurrency:     DefaultCompareHashConcurrency,
			ReadBufferKiB:       DefaultCompareReadBufferKiB,
			MaxHashBytes:        0,
			StayOnVolumeDefault: DefaultCompareStayOnVolumeDefault,
		},
		Dedup: DedupConfig{
			HashConfirmBytes:  DefaultDedupHashConfirmBytes,
			FileProgressBytes: DefaultDedupFileProgressBytes,
			ChunkBytes:        DefaultDedupChunkBytes,
		},
		StatusCommand: StatusCommandConfig{
			Command:    "",
			IntervalMS: DefaultStatusCommandIntervalMS,
			MaxWidth:   DefaultStatusCommandMaxWidth,
		},
	}
}

// DefaultPaths returns XDG-compliant config locations.
func DefaultPaths() (Paths, error) {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Paths{}, fmt.Errorf("resolve home directory: %w", err)
		}
		configHome = filepath.Join(home, ".config")
	}

	configDir := filepath.Join(configHome, appDirName)
	return Paths{
		ConfigDir:        configDir,
		ConfigFile:       filepath.Join(configDir, fileName),
		ThemesDir:        filepath.Join(configDir, "themes"),
		PreviewStylesDir: filepath.Join(configDir, "themes", "preview"),
		KeybindingsFile:  filepath.Join(configDir, keybindingsFileBaseName),
	}, nil
}

// ConfigFileName returns the basename of the main configuration file (config.toml).
func ConfigFileName() string { return fileName }

// KeybindingsFileName returns the basename of the keybindings file (keybindings.toml).
func KeybindingsFileName() string { return keybindingsFileBaseName }

// Load resolves the default XDG paths and loads config.toml if present.
func Load() (Config, error) {
	paths, err := DefaultPaths()
	if err != nil {
		return Config{}, err
	}
	return LoadFromPaths(paths)
}

// LoadFromPaths loads config.toml from paths, merging it over built-in defaults.
func LoadFromPaths(paths Paths) (Config, error) {
	paths = paths.WithResolvedLocations()
	_ = chromastyles.LoadFromDir(paths.PreviewStylesDir)

	cfg := Default()
	configFile := paths.ConfigFile
	if configFile == "" && paths.ConfigDir != "" {
		configFile = filepath.Join(paths.ConfigDir, fileName)
	}
	if configFile == "" {
		return cfg, cfg.Validate()
	}

	meta, err := toml.DecodeFile(configFile, &cfg)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, cfg.Validate()
		}
		return Config{}, fmt.Errorf("load config %q: %w", configFile, err)
	}
	for _, key := range meta.Undecoded() {
		return Config{}, fmt.Errorf("load config %q: unknown field %q", configFile, key.String())
	}
	if err := (&cfg).Validate(); err != nil {
		return Config{}, fmt.Errorf("validate config %q: %w", configFile, err)
	}
	return cfg, nil
}

// WriteDefaultStub writes the full default configuration as TOML.
func WriteDefaultStub(filename string) error {
	if filename == "" {
		return fmt.Errorf("config stub filename is required")
	}

	var buffer bytes.Buffer
	if err := EncodeDefaultStub(&buffer); err != nil {
		return err
	}
	if err := os.WriteFile(filename, buffer.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write config stub %q: %w", filename, err)
	}
	return nil
}

// EncodeDefaultStub writes the full default configuration as TOML.
func EncodeDefaultStub(w io.Writer) error {
	if err := toml.NewEncoder(w).Encode(Default()); err != nil {
		return fmt.Errorf("encode default config stub: %w", err)
	}
	return nil
}

func resolvePersistPaths(paths Paths) (configFile string, err error) {
	file := strings.TrimSpace(paths.ConfigFile)
	dir := strings.TrimSpace(paths.ConfigDir)
	switch {
	case file != "":
		return file, nil
	case dir != "":
		return filepath.Join(dir, fileName), nil
	default:
		return "", fmt.Errorf("config: ConfigFile or ConfigDir required to persist configuration")
	}
}

// CanPersist reports whether Paths identify a writable config.toml target.
func (paths Paths) CanPersist() bool {
	f, err := resolvePersistPaths(paths)
	return err == nil && f != ""
}

// WriteMergedPartial merges patch keys into config.toml using TOML as a generic document:
// absent files are written with only patched keys (minimal); existing files preserve other
// keys from the decoded map. Nested tables are merged recursively when both sides are maps.
//
// Leading comments in existing files cannot be preserved.
func WriteMergedPartial(paths Paths, patch map[string]interface{}) error {
	if len(patch) == 0 {
		return nil
	}
	configFile, err := resolvePersistPaths(paths)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(configFile), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	var merged map[string]interface{}
	switch _, statErr := os.Stat(configFile); {
	case errors.Is(statErr, os.ErrNotExist):
		merged = shallowCloneRoot(patch)
	case statErr != nil:
		return fmt.Errorf("stat config %q: %w", configFile, statErr)
	default:
		raw, readErr := os.ReadFile(configFile)
		if readErr != nil {
			return fmt.Errorf("read config %q: %w", configFile, readErr)
		}
		rawTrim := strings.TrimSpace(string(raw))
		if rawTrim == "" {
			merged = shallowCloneRoot(patch)
		} else {
			decoded := make(map[string]interface{})
			if _, decErr := toml.Decode(rawTrim, &decoded); decErr != nil {
				return fmt.Errorf("parse config %q: %w", configFile, decErr)
			}
			mergeRoot(decoded, patch)
			merged = decoded
		}
	}

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(merged); err != nil {
		return fmt.Errorf("encode config %q: %w", configFile, err)
	}
	out := bytes.TrimSuffix(buf.Bytes(), []byte{'\n'})
	payload := append(out, '\n')
	if err := atomicWrite(configFile, payload, 0o644); err != nil {
		return fmt.Errorf("write config %q: %w", configFile, err)
	}
	return nil
}

func atomicWrite(dest string, data []byte, perm fs.FileMode) error {
	dir := filepath.Dir(dest)
	temp, err := os.CreateTemp(dir, "."+filepath.Base(dest)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := temp.Name()
	_, copyErr := temp.Write(data)
	syncErr := temp.Sync()
	closeErr := temp.Close()
	if copyErr != nil || syncErr != nil || closeErr != nil {
		_ = os.Remove(tmpPath)
		return errors.Join(copyErr, syncErr, closeErr)
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func shallowCloneRoot(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	out := make(map[string]interface{}, len(m))
	for key, val := range m {
		next, ok := val.(map[string]interface{})
		if ok {
			out[key] = shallowCloneRoot(next)
			continue
		}
		out[key] = val
	}
	return out
}

func mergeRoot(dst map[string]interface{}, patch map[string]interface{}) {
	for key, pv := range patch {
		next, ok := pv.(map[string]interface{})
		if !ok || next == nil {
			dst[key] = pv
			continue
		}
		ex, ok := dst[key].(map[string]interface{})
		if !ok || ex == nil {
			dst[key] = shallowCloneRoot(next)
			continue
		}
		mergeRoot(ex, next)
	}
}

// Validate clamps unsupported values to their valid defaults instead of rejecting
// the config. This allows users to write a forward-looking config.toml that works
// now and will work as features are added.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.Theme) == "" {
		return fmt.Errorf("theme is required")
	}
	builtin := Default()
	c.validateGeneral(&builtin)
	c.validateCompareDedupShell(&builtin)
	c.validateSortAndDelete(&builtin)
	c.validateUI(&builtin)
	c.validateUIZoom(&builtin)
	c.validateFilter(&builtin)
	c.validateJobs(&builtin)
	c.validateJobsWorkerTiming(&builtin)
	c.validateJobsScan(&builtin)
	c.validateOperations(&builtin)
	if !validLoggingLevel(c.Logging.Level) {
		c.Logging.Level = builtin.Logging.Level
	}
	if len(c.UserMenu.LocalNames) == 0 {
		c.UserMenu.LocalNames = append([]string(nil), builtin.UserMenu.LocalNames...)
	}
	c.validatePreview(&builtin)
	c.validateSFTP(&builtin)
	c.validateMeta(&builtin)
	return nil
}

func (c *Config) validateGeneral(builtin *Config) {
	ds := strings.ToLower(strings.TrimSpace(c.Panels.DefaultSort))
	if ds == SortDiskUsage || ds == "disk-usage" {
		c.Panels.DefaultSort = SortName
		c.DiskUsage.IdleSizeSort = true
	}
	if c.DiskUsage.IdleSortDelayMS <= 0 {
		c.DiskUsage.IdleSortDelayMS = 500
	}
	if c.Panels.RefreshIntervalMS < 0 {
		c.Panels.RefreshIntervalMS = DefaultRefreshIntervalMS
	}
	if c.Panels.RefreshIntervalMS > 0 {
		if c.Panels.RefreshIntervalMS < RefreshIntervalMinMS {
			c.Panels.RefreshIntervalMS = RefreshIntervalMinMS
		}
		if c.Panels.RefreshIntervalMS > RefreshIntervalMaxMS {
			c.Panels.RefreshIntervalMS = RefreshIntervalMaxMS
		}
	}
	if c.FSWalk.InitialWorkers < 1 {
		c.FSWalk.InitialWorkers = builtin.FSWalk.InitialWorkers
	}
	if c.FSWalk.MaxWorkers < c.FSWalk.InitialWorkers {
		c.FSWalk.MaxWorkers = c.FSWalk.InitialWorkers
	}
	if c.FSWalk.AdaptIntervalMS < FSWalkAdaptIntervalMinMS {
		c.FSWalk.AdaptIntervalMS = builtin.FSWalk.AdaptIntervalMS
	}
}

func (c *Config) validateCompareDedupShell(builtin *Config) {
	if c.Compare.HashConcurrency < 1 {
		c.Compare.HashConcurrency = builtin.Compare.HashConcurrency
	}
	if c.Compare.ReadBufferKiB < 1 {
		c.Compare.ReadBufferKiB = builtin.Compare.ReadBufferKiB
	}
	if c.Dedup.HashConfirmBytes < 0 {
		c.Dedup.HashConfirmBytes = 0
	}
	if c.Dedup.FileProgressBytes < 0 {
		c.Dedup.FileProgressBytes = 0
	}
	if c.Dedup.ChunkBytes < 0 {
		c.Dedup.ChunkBytes = 0
	}
	if c.Shell.TerminalPanelHeight < MinShellTerminalPanelHeight {
		c.Shell.TerminalPanelHeight = builtin.Shell.TerminalPanelHeight
	}
	if c.StatusCommand.IntervalMS < StatusCommandIntervalMinMS {
		c.StatusCommand.IntervalMS = builtin.StatusCommand.IntervalMS
	}
	if c.StatusCommand.MaxWidth < StatusCommandMaxWidthMin {
		c.StatusCommand.MaxWidth = builtin.StatusCommand.MaxWidth
	}
	if c.StatusCommand.MaxWidth > StatusCommandMaxWidthMax {
		c.StatusCommand.MaxWidth = StatusCommandMaxWidthMax
	}
}

func (c *Config) validateSortAndDelete(builtin *Config) {
	if !c.sortModeValid(c.Panels.DefaultSort) {
		c.Panels.DefaultSort = builtin.Panels.DefaultSort
	}
	if !c.listingFormatValid(c.Panels.DefaultListingFormat) {
		c.Panels.DefaultListingFormat = DefaultListingFormat
	}
	// SortReverse is now supported, no clamping needed
	if c.Operations.DeleteMode != DeletePermanent {
		c.Operations.DeleteMode = builtin.Operations.DeleteMode
	}
}

func (c *Config) validateUI(builtin *Config) {
	if c.UI.Status.MessageTTLSeconds < 0 {
		c.UI.Status.MessageTTLSeconds = builtin.UI.Status.MessageTTLSeconds
	}
	if c.UI.PathPickerValidateDelayMS < 0 {
		c.UI.PathPickerValidateDelayMS = builtin.UI.PathPickerValidateDelayMS
	}
	const pathPickerValidateMaxMS = 30_000
	if c.UI.PathPickerValidateDelayMS > pathPickerValidateMaxMS {
		c.UI.PathPickerValidateDelayMS = pathPickerValidateMaxMS
	}
	if c.UI.KeyRepeatDebounceMS < 0 {
		c.UI.KeyRepeatDebounceMS = builtin.UI.KeyRepeatDebounceMS
	}
	if c.UI.KeyRepeatDebounceMS > KeyRepeatDebounceMaxMS {
		c.UI.KeyRepeatDebounceMS = KeyRepeatDebounceMaxMS
	}
	if c.UI.Find.QueryDebounceMS < 0 {
		c.UI.Find.QueryDebounceMS = builtin.UI.Find.QueryDebounceMS
	}
	if c.UI.Find.QueryDebounceMS > KeyRepeatDebounceMaxMS {
		c.UI.Find.QueryDebounceMS = KeyRepeatDebounceMaxMS
	}
	if c.UI.Find.MaxResults <= 0 {
		c.UI.Find.MaxResults = builtin.UI.Find.MaxResults
	}
	if c.UI.Find.ListNavIdleMS < 0 {
		c.UI.Find.ListNavIdleMS = builtin.UI.Find.ListNavIdleMS
	}
	if c.UI.SelectionsPanelActivePercent < SelectionsPanelActivePercentMin ||
		c.UI.SelectionsPanelActivePercent > SelectionsPanelActivePercentMax {
		c.UI.SelectionsPanelActivePercent = DefaultSelectionsPanelActivePercent
	}
}

func (c *Config) validateUIZoom(builtin *Config) {
	if c.UI.Zoom.ActivePercent <= 0 || c.UI.Zoom.InactivePercent <= 0 ||
		c.UI.Zoom.ActivePercent+c.UI.Zoom.InactivePercent != 100 {
		c.UI.Zoom.ActivePercent = DefaultPanelZoomActivePercent
		c.UI.Zoom.InactivePercent = DefaultPanelZoomInactivePercent
	}
	if c.UI.Zoom.DisabledAboveWidth < 0 {
		c.UI.Zoom.DisabledAboveWidth = builtin.UI.Zoom.DisabledAboveWidth
	}
	if c.UI.Zoom.DisabledAboveHeight < 0 {
		c.UI.Zoom.DisabledAboveHeight = builtin.UI.Zoom.DisabledAboveHeight
	}
	if !c.paneSplitOrientationValid(c.UI.Zoom.Orientation) {
		c.UI.Zoom.Orientation = DefaultPaneSplitOrientation
	}
	if !c.scrollModeValid(c.UI.Scroll.Mode) {
		c.UI.Scroll.Mode = DefaultScrollMode
	}
	if !c.panelScrollbarValid(c.UI.Scroll.Scrollbar) {
		c.UI.Scroll.Scrollbar = DefaultPanelScrollbar
	}
	if c.UI.Scroll.EdgeMargin < 0 {
		c.UI.Scroll.EdgeMargin = DefaultScrollEdgeMargin
	}
	if c.UI.Scroll.EdgeMargin > ScrollEdgeMarginMax {
		c.UI.Scroll.EdgeMargin = ScrollEdgeMarginMax
	}
	if c.UI.Status.LogMaxEntries <= 0 {
		c.UI.Status.LogMaxEntries = builtin.UI.Status.LogMaxEntries
	}
	const messageLogMaxCap = 50_000
	if c.UI.Status.LogMaxEntries > messageLogMaxCap {
		c.UI.Status.LogMaxEntries = messageLogMaxCap
	}
	normalizeCarousel(&c.Carousel)
}

func (c *Config) validateFilter(builtin *Config) {
	if c.Filter.Mode != FilterModeFuzzy {
		c.Filter.Mode = builtin.Filter.Mode
	}
	if c.Filter.Syntax != FilterSyntaxFZF {
		c.Filter.Syntax = builtin.Filter.Syntax
	}
	if c.Filter.MatchPathSegments {
		c.Filter.MatchPathSegments = builtin.Filter.MatchPathSegments
	}
	cm := strings.ToLower(strings.TrimSpace(c.Filter.CycleMatches))
	switch cm {
	case FilterCycleMatchesVisual, FilterCycleMatchesRanked:
		c.Filter.CycleMatches = cm
	default:
		c.Filter.CycleMatches = builtin.Filter.CycleMatches
	}
}

func (c *Config) validateJobs(builtin *Config) {
	if c.Jobs.KeepFinished <= 0 {
		c.Jobs.KeepFinished = builtin.Jobs.KeepFinished
	}
	if c.Jobs.ProgressUIWakeDebounceMS <= 0 {
		c.Jobs.ProgressUIWakeDebounceMS = builtin.Jobs.ProgressUIWakeDebounceMS
	}
	const jobsProgressTimingMinMS = JobsProgressTimingMinMS
	const jobsProgressTimingMaxMS = JobsProgressTimingMaxMS
	if c.Jobs.ProgressUIWakeDebounceMS < jobsProgressTimingMinMS {
		c.Jobs.ProgressUIWakeDebounceMS = jobsProgressTimingMinMS
	}
	if c.Jobs.ProgressUIWakeDebounceMS > jobsProgressTimingMaxMS {
		c.Jobs.ProgressUIWakeDebounceMS = jobsProgressTimingMaxMS
	}
	if c.Jobs.BlockerDialogNextDebounceMS < 0 {
		c.Jobs.BlockerDialogNextDebounceMS = 0
	}
	if c.Jobs.BlockerDialogNextDebounceMS > BlockerDialogNextDebounceMaxMS {
		c.Jobs.BlockerDialogNextDebounceMS = BlockerDialogNextDebounceMaxMS
	}
	const workerProgressBytesMin = WorkerProgressMinBytesMin
	const workerProgressBytesMax = WorkerProgressMinBytesMax
	if c.Jobs.WorkerProgressMinBytes <= 0 {
		c.Jobs.WorkerProgressMinBytes = builtin.Jobs.WorkerProgressMinBytes
	}
	if c.Jobs.WorkerProgressMinBytes < workerProgressBytesMin {
		c.Jobs.WorkerProgressMinBytes = workerProgressBytesMin
	}
	if c.Jobs.WorkerProgressMinBytes > workerProgressBytesMax {
		c.Jobs.WorkerProgressMinBytes = workerProgressBytesMax
	}
}

func (c *Config) validateJobsWorkerTiming(builtin *Config) {
	const jobsProgressTimingMinMS = JobsProgressTimingMinMS
	const jobsProgressTimingMaxMS = JobsProgressTimingMaxMS
	if c.Jobs.WorkerProgressMinIntervalMS <= 0 {
		c.Jobs.WorkerProgressMinIntervalMS = builtin.Jobs.WorkerProgressMinIntervalMS
	}
	if c.Jobs.WorkerProgressMinIntervalMS < jobsProgressTimingMinMS {
		c.Jobs.WorkerProgressMinIntervalMS = jobsProgressTimingMinMS
	}
	if c.Jobs.WorkerProgressMinIntervalMS > jobsProgressTimingMaxMS {
		c.Jobs.WorkerProgressMinIntervalMS = jobsProgressTimingMaxMS
	}
	const throughputChartWindowMinSec = 20
	const throughputChartWindowMaxSec = 120
	if c.Jobs.ThroughputChartWindowSec <= 0 {
		c.Jobs.ThroughputChartWindowSec = builtin.Jobs.ThroughputChartWindowSec
	}
	if c.Jobs.ThroughputChartWindowSec < throughputChartWindowMinSec {
		c.Jobs.ThroughputChartWindowSec = throughputChartWindowMinSec
	}
	if c.Jobs.ThroughputChartWindowSec > throughputChartWindowMaxSec {
		c.Jobs.ThroughputChartWindowSec = throughputChartWindowMaxSec
	}
	const throughputChartColumnMinMS = 80
	const throughputChartColumnMaxMS = 2000
	if c.Jobs.ThroughputChartColumnMS <= 0 {
		c.Jobs.ThroughputChartColumnMS = builtin.Jobs.ThroughputChartColumnMS
	}
	if c.Jobs.ThroughputChartColumnMS < throughputChartColumnMinMS {
		c.Jobs.ThroughputChartColumnMS = throughputChartColumnMinMS
	}
	if c.Jobs.ThroughputChartColumnMS > throughputChartColumnMaxMS {
		c.Jobs.ThroughputChartColumnMS = throughputChartColumnMaxMS
	}
}

func (c *Config) validateJobsScan(builtin *Config) {
	const jobsProgressTimingMinMS = JobsProgressTimingMinMS
	const jobsProgressTimingMaxMS = JobsProgressTimingMaxMS
	if c.Jobs.FreeSpacePollIntervalSecs < 0 {
		c.Jobs.FreeSpacePollIntervalSecs = builtin.Jobs.FreeSpacePollIntervalSecs
	}
	const freeSpacePollIntervalMaxSecs = 3600
	if c.Jobs.FreeSpacePollIntervalSecs > freeSpacePollIntervalMaxSecs {
		c.Jobs.FreeSpacePollIntervalSecs = freeSpacePollIntervalMaxSecs
	}
	if c.Jobs.ScanYieldIntervalMS <= 0 {
		c.Jobs.ScanYieldIntervalMS = builtin.Jobs.ScanYieldIntervalMS
	}
	if c.Jobs.ScanYieldIntervalMS < jobsProgressTimingMinMS {
		c.Jobs.ScanYieldIntervalMS = jobsProgressTimingMinMS
	}
	if c.Jobs.ScanYieldIntervalMS > jobsProgressTimingMaxMS {
		c.Jobs.ScanYieldIntervalMS = jobsProgressTimingMaxMS
	}
	if c.Jobs.ScanYieldEveryN <= 0 {
		c.Jobs.ScanYieldEveryN = builtin.Jobs.ScanYieldEveryN
	}
	if c.Jobs.ScanYieldEveryN > ScanYieldEveryNMax {
		c.Jobs.ScanYieldEveryN = ScanYieldEveryNMax
	}
	if c.Jobs.ScanNiceIncrement < 0 {
		c.Jobs.ScanNiceIncrement = builtin.Jobs.ScanNiceIncrement
	}
	if c.Jobs.ScanNiceIncrement > 19 {
		c.Jobs.ScanNiceIncrement = 19
	}
	if c.Jobs.ScanProgressMinIntervalMS <= 0 {
		c.Jobs.ScanProgressMinIntervalMS = builtin.Jobs.ScanProgressMinIntervalMS
	}
	if c.Jobs.ScanProgressMinIntervalMS < jobsProgressTimingMinMS {
		c.Jobs.ScanProgressMinIntervalMS = jobsProgressTimingMinMS
	}
	if c.Jobs.ScanProgressMinIntervalMS > jobsProgressTimingMaxMS {
		c.Jobs.ScanProgressMinIntervalMS = jobsProgressTimingMaxMS
	}
}

func (c *Config) validateOperations(builtin *Config) {
	if c.Operations.CopyBufferKiB <= 0 {
		c.Operations.CopyBufferKiB = builtin.Operations.CopyBufferKiB
	}
	if c.Operations.DiskSpaceCheckMinFileBytes < 0 {
		c.Operations.DiskSpaceCheckMinFileBytes = builtin.Operations.DiskSpaceCheckMinFileBytes
	}
	loc := strings.ToLower(strings.TrimSpace(c.Operations.FlattenDefaultLocation))
	if loc != FlattenDefaultLocationActive && loc != FlattenDefaultLocationInactive {
		c.Operations.FlattenDefaultLocation = builtin.Operations.FlattenDefaultLocation
	} else {
		c.Operations.FlattenDefaultLocation = loc
	}
}

func (c *Config) validatePreview(builtin *Config) {
	if strings.TrimSpace(c.Preview.Command) == "" {
		c.Preview.Command = builtin.Preview.Command
	}
	mode := strings.ToLower(strings.TrimSpace(c.Preview.Mode))
	switch mode {
	case PreviewModeExternal:
		c.Preview.Mode = PreviewModeExternal
	case "", PreviewModeInternal:
		c.Preview.Mode = PreviewModeInternal
	default:
		c.Preview.Mode = builtin.Preview.Mode
	}
	c.Preview.Style = previewValidateStyle(c.Preview.Style)
	switch strings.ToLower(strings.TrimSpace(c.Preview.ImageProtocol)) {
	case PreviewImageProtocolSixel:
		c.Preview.ImageProtocol = PreviewImageProtocolSixel
	case PreviewImageProtocolKitty:
		c.Preview.ImageProtocol = PreviewImageProtocolKitty
	case "", PreviewImageProtocolAuto:
		c.Preview.ImageProtocol = PreviewImageProtocolAuto
	default:
		c.Preview.ImageProtocol = builtin.Preview.ImageProtocol
	}
	c.Preview.TerminalSixel = validateTerminalCapability(c.Preview.TerminalSixel, builtin.Preview.TerminalSixel)
	c.Preview.TerminalKitty = validateTerminalCapability(c.Preview.TerminalKitty, builtin.Preview.TerminalKitty)
	c.Preview.TerminalKittyPlaceholder = validateTerminalCapability(c.Preview.TerminalKittyPlaceholder, builtin.Preview.TerminalKittyPlaceholder)
	if c.Preview.VideoThumbCols < PreviewVideoThumbGridMin || c.Preview.VideoThumbCols > PreviewVideoThumbGridMax {
		c.Preview.VideoThumbCols = builtin.Preview.VideoThumbCols
	}
	if c.Preview.VideoThumbRows < PreviewVideoThumbGridMin || c.Preview.VideoThumbRows > PreviewVideoThumbGridMax {
		c.Preview.VideoThumbRows = builtin.Preview.VideoThumbRows
	}
	if c.Preview.PrefetchWorkers < PreviewPrefetchWorkersMin || c.Preview.PrefetchWorkers > PreviewPrefetchWorkersMax {
		c.Preview.PrefetchWorkers = builtin.Preview.PrefetchWorkers
	}
	if c.Preview.PrefetchWindow < PreviewPrefetchWindowMin || c.Preview.PrefetchWindow > PreviewPrefetchWindowMax {
		c.Preview.PrefetchWindow = builtin.Preview.PrefetchWindow
	}
	if c.Preview.ImageMaxEdgePx != 0 && c.Preview.ImageMaxEdgePx < PreviewImageMaxEdgePxMin {
		c.Preview.ImageMaxEdgePx = PreviewImageMaxEdgePxMin
	}
	if c.Preview.TmuxSixelMaxEdgePx < PreviewImageMaxEdgePxMin {
		c.Preview.TmuxSixelMaxEdgePx = builtin.Preview.TmuxSixelMaxEdgePx
	}
	if c.Preview.VideoThumbMaxEdgePx < PreviewImageMaxEdgePxMin {
		c.Preview.VideoThumbMaxEdgePx = builtin.Preview.VideoThumbMaxEdgePx
	}
	if c.Preview.PrefetchMemoryMaxMB < 1 {
		c.Preview.PrefetchMemoryMaxMB = builtin.Preview.PrefetchMemoryMaxMB
	}
	if c.Preview.VideoThumbCacheMaxMB < 1 {
		c.Preview.VideoThumbCacheMaxMB = builtin.Preview.VideoThumbCacheMaxMB
	}
	if c.Preview.Mode == PreviewModeExternal {
		if _, err := cmdrun.PreviewCommandArgv(c.Preview.Command, "/tmp/pc-preview-validate", 80); err != nil {
			c.Preview.Command = builtin.Preview.Command
		}
	}
}

// validateTerminalCapability normalizes a [preview] terminal_* tri-state value to
// "auto"/"yes"/"no", falling back to fallback for anything else (mirrors ImageProtocol's
// switch/normalize pattern above).
func validateTerminalCapability(v, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case PreviewTerminalCapabilityYes:
		return PreviewTerminalCapabilityYes
	case PreviewTerminalCapabilityNo:
		return PreviewTerminalCapabilityNo
	case "", PreviewTerminalCapabilityAuto:
		return PreviewTerminalCapabilityAuto
	default:
		return fallback
	}
}

func (c *Config) validateSFTP(builtin *Config) {
	if c.SFTP.IdleTimeoutSecs < 15 {
		c.SFTP.IdleTimeoutSecs = builtin.SFTP.IdleTimeoutSecs
	}
	if c.SFTP.IdleTimeoutSecs > 3600 {
		c.SFTP.IdleTimeoutSecs = 3600
	}
	if c.SFTP.DialTimeoutSecs < 5 {
		c.SFTP.DialTimeoutSecs = builtin.SFTP.DialTimeoutSecs
	}
	if c.SFTP.DialTimeoutSecs > 300 {
		c.SFTP.DialTimeoutSecs = 300
	}
	if c.SFTP.ListTimeoutSecs < 5 {
		c.SFTP.ListTimeoutSecs = builtin.SFTP.ListTimeoutSecs
	}
	if c.SFTP.ListTimeoutSecs > 300 {
		c.SFTP.ListTimeoutSecs = 300
	}
}

func (c *Config) validateMeta(builtin *Config) {
	if len(c.Meta.LocalNames) == 0 {
		c.Meta.LocalNames = append([]string(nil), builtin.Meta.LocalNames...)
	}
	if c.Meta.DefaultEntryWorkers < 1 {
		c.Meta.DefaultEntryWorkers = builtin.Meta.DefaultEntryWorkers
	}
	const metaWorkersMax = 64
	if c.Meta.DefaultEntryWorkers > metaWorkersMax {
		c.Meta.DefaultEntryWorkers = metaWorkersMax
	}
}

func (c Config) sortModeValid(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case SortName, SortExtension, SortSize, SortMtime:
		return true
	default:
		return false
	}
}

func (c Config) scrollModeValid(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ScrollModeMinimal, ScrollModeCenter, ScrollModeEdge:
		return true
	default:
		return false
	}
}

func (c Config) panelScrollbarValid(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case PanelScrollbarNone, PanelScrollbarThumb, PanelScrollbarBar:
		return true
	default:
		return false
	}
}

func (c Config) paneSplitOrientationValid(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case PaneSplitSideBySide, PaneSplitStacked:
		return true
	default:
		return false
	}
}

func (c Config) listingFormatValid(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ListingFormatMtime, "modified", ListingFormatPerm, "permissions", "mode", ListingFormatBrief, "minimal":
		return true
	default:
		return false
	}
}

func validLoggingLevel(level string) bool {
	switch strings.ToLower(level) {
	case "debug", "info", "warn", "error":
		return true
	default:
		return false
	}
}

func previewValidateStyle(name string) string {
	n := strings.TrimSpace(name)
	if n == "" {
		return DefaultPreviewStyle
	}
	if canonical := chromastyles.CanonicalName(n); canonical != "" {
		return canonical
	}
	return DefaultPreviewStyle
}

// NormalizePreviewStyle returns a valid Chroma style registry name.
func NormalizePreviewStyle(name string) string {
	return previewValidateStyle(name)
}
