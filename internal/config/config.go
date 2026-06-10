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
)

const (
	appDirName              = "pc"
	fileName                = "config.toml"
	keybindingsFileBaseName = "keybindings.toml"

	ThemeDefault   = "default"
	StartupPathCWD = "cwd"
	SortName       = "name"
	SortExtension  = "extension"
	SortSize       = "size"
	SortMtime      = "mtime"
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
	BorderStyleSingle        = "single"
)

// Paths identifies configuration files discovered from XDG paths.
type Paths struct {
	ConfigDir       string
	ConfigFile      string
	ThemesDir       string
	KeybindingsFile string
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
	if strings.TrimSpace(out.ThemesDir) != "" {
		return out
	}
	if cd := strings.TrimSpace(out.ConfigDir); cd != "" {
		out.ThemesDir = filepath.Join(cd, "themes")
	}
	return out
}

// Config is the parsed general application configuration.
type Config struct {
	Theme                    string `toml:"theme"`
	ShowHidden               bool   `toml:"show_hidden"`
	RespectGitignore         bool   `toml:"respect_gitignore"`
	ConfirmDelete            bool   `toml:"confirm_delete"`
	ConfirmOverwrite         bool   `toml:"confirm_overwrite"`
	CaseInsensitiveFilter    bool   `toml:"case_insensitive_filter"`
	JobConcurrency           int    `toml:"job_concurrency"`
	StartupPathMode          string `toml:"startup_path_mode"`
	DefaultSort              string `toml:"default_sort"`
	DefaultListingFormat     string `toml:"default_listing_format"`
	SortReverse              bool   `toml:"sort_reverse"`
	DirectoriesFirst         bool   `toml:"directories_first"`
	DiskUsageIdleSizeSort    bool   `toml:"disk_usage_idle_size_sort"`
	DiskUsageIdleSortDelayMS int    `toml:"disk_usage_idle_sort_delay_ms"`
	// RefreshIntervalMS re-reads both panel directories on this interval in background goroutines (0 disables).
	RefreshIntervalMS               int  `toml:"refresh_interval_ms"`
	DiskUsageDescendIntoMountPoints bool `toml:"disk_usage_descend_into_mount_points"`
	// DiskUsageWalkConcurrency limits concurrent subdirectory branches during a disk-usage walk (minimum 1 after Validate).
	// Default is DefaultDiskUsageWalkConcurrency.
	// Low values spare HDD/NAS; raise for fast local SSDs.
	DiskUsageWalkConcurrency   int              `toml:"disk_usage_walk_concurrency"`
	FollowSymlinksOnNavigation bool             `toml:"follow_symlinks_on_navigation"`
	OpenFilesExternally        bool             `toml:"open_files_externally"`
	RunExecutablesOnEnter      bool             `toml:"run_executables_on_enter"`
	DeleteMode                 string           `toml:"delete_mode"`
	UI                         UIConfig         `toml:"ui"`
	Filter                     FilterConfig     `toml:"filter"`
	Jobs                       JobsConfig       `toml:"jobs"`
	Operations                 OperationsConfig `toml:"operations"`
	Logging                    LoggingConfig    `toml:"logging"`
	Bookmarks                  BookmarksConfig  `toml:"bookmarks"`
	UserMenu                   UserMenuConfig   `toml:"user_menu"`
	Preview                    PreviewConfig    `toml:"preview"`
	SFTP                       SFTPConfig       `toml:"sftp"`
	Shell                      ShellConfig      `toml:"shell"`
	// Meta configures the separate meta.toml command definitions file and execution settings.
	Meta MetaConfig `toml:"meta"`
	// Pools configures discovery of the separate pools.toml work-pool definitions file.
	Pools PoolsConfig `toml:"pools"`
}

// PoolsConfig controls discovery of the separate pools.toml file.
type PoolsConfig struct {
	// File is an absolute path or ~/… to the global pools.toml. Empty uses
	// filepath.Join(configDir, DefaultPoolsFileName) after paths are resolved.
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

// ShellConfig controls drop-to-shell (suspend TUI, run interactive shell, resume).
type ShellConfig struct {
	// Command is an optional argv template parsed like shellwords (see cmdrun.ParseCommandArgv).
	// Empty uses $SHELL then bash fallback.
	Command string `toml:"command"`
	// SyncCwdOnReturn navigates the active panel to the process cwd after the shell exits.
	SyncCwdOnReturn bool `toml:"sync_cwd_on_return"`
}

// PreviewConfig controls inactive-panel file preview (external highlighter command).
type PreviewConfig struct {
	// Command is a single-line argv template parsed like shellwords (see cmdrun.ParseCommandArgv).
	// Use %f once to insert the absolute file path as one token; use %w for terminal width; if %f is omitted, the path is appended.
	Command string `toml:"command"`
}

type UIConfig struct {
	ShowMenuBar    bool   `toml:"show_menu_bar"`
	ShowFooter     bool   `toml:"show_footer"`
	ShowStatusLine bool   `toml:"show_status_line"`
	ShowJobsLine   bool   `toml:"show_jobs_line"`
	ShowFileIcons  bool   `toml:"show_file_icons"`
	BorderStyle    string `toml:"border_style"`
	Clock          bool   `toml:"clock"`
	// SelectionsPanelMaxRows caps visible rows in the cross-directory selections strip (0 = default 5).
	SelectionsPanelMaxRows int `toml:"selections_panel_max_rows"`
	// StatusMessageTTLSeconds is how long transient status banners stay visible before clearing.
	// Default 4.5. Use 0 to keep messages until replaced or cleared by another action.
	StatusMessageTTLSeconds float64 `toml:"status_message_ttl_seconds"`
	// PathPickerValidateDelayMS waits after the filter changes before checking whether the typed path exists.
	// Default DefaultPathPickerValidateDelayMS. Use 0 to validate on the next scheduler tick (still not per-key synchronous).
	PathPickerValidateDelayMS int `toml:"path_picker_validate_delay_ms"`
	// PanelSyncFollowNavDebounceMS, when latched panel sync is on, waits this long after the last file-list
	// cursor step (Up/Down/PgUp/PgDn/Home/End) before loading the follower's directory. Zero syncs every tick.
	// Default DefaultPanelSyncFollowNavDebounceMS.
	PanelSyncFollowNavDebounceMS int `toml:"panel_sync_follow_nav_debounce_ms"`
	// QuickViewPreviewDebounceMS waits after the last highlight change before re-running the preview
	// subprocess while Quick view is enabled. Zero disables debouncing. Default DefaultQuickViewPreviewDebounceMS.
	QuickViewPreviewDebounceMS int `toml:"quick_view_preview_debounce_ms"`
	// CarouselPreviewDebounceMS waits after the last file-list cursor step before reloading carousel
	// parent/child directory previews. Zero disables debouncing. Default DefaultCarouselPreviewDebounceMS.
	CarouselPreviewDebounceMS int `toml:"carousel_preview_debounce_ms"`
	// FindQueryDebounceMS waits after the last keystroke in the find dialog query field before
	// re-ranking the result list. Reducing this to 0 ranks on every keystroke (no debounce).
	// Default DefaultFindQueryDebounceMS.
	FindQueryDebounceMS int `toml:"find_query_debounce_ms"`
	// FindMaxResults caps the number of ranked results shown in the find dialog. The full index
	// is always retained; only the top-N scored entries are kept after each rank.
	// Default DefaultFindMaxResults.
	FindMaxResults int `toml:"find_max_results"`
	// FindListNavIdleMS is the navigation-idle delay before a background rank update is applied
	// to the find result list. Resets on every Up/Down/PgUp/PgDn movement. Zero applies updates
	// immediately regardless of navigation.
	// Default DefaultFindListNavIdleMS.
	FindListNavIdleMS int `toml:"find_list_nav_idle_ms"`
	// ZoomActivePanel widens the active browser column; inactive column uses the remainder (see panel_zoom_*_percent).
	ZoomActivePanel bool `toml:"zoom_active_panel"`
	// ZoomActivePanelDisabledAboveWidth: when > 0 and terminal width (cells) is >= this value, zoom is not applied
	// (50/50 split). Zero disables this gate (zoom follows zoom_active_panel and runtime toggle only).
	// Default DefaultZoomActivePanelDisabledAboveWidth.
	ZoomActivePanelDisabledAboveWidth int `toml:"zoom_active_panel_disabled_above_width"`
	// PanelZoomActivePercent and PanelZoomInactivePercent are the width shares (must sum to 100) when ZoomActivePanel is true.
	PanelZoomActivePercent   int `toml:"panel_zoom_active_percent"`
	PanelZoomInactivePercent int `toml:"panel_zoom_inactive_percent"`
	// ShrunkenShowsNameOnly: when true, narrow panels hide trailing listing columns and show only names
	// (sort and default_listing_format are unchanged; see ShrunkenListingRowTextWidthThreshold in builtin.go).
	ShrunkenShowsNameOnly bool `toml:"shrunken_shows_name_only"`
	// CenterScrolling: when true, file-list navigation keeps the highlight row centered in the viewport.
	CenterScrolling bool `toml:"center_scrolling"`
	// MessageLogMaxEntries caps how many status/toast lines are retained for the Messages view (oldest dropped).
	// Zero means use the built-in default (see DefaultMessageLogMaxEntries).
	MessageLogMaxEntries int `toml:"message_log_max_entries"`
	// ScreenRenderHashCache, when true, hashes the logical cell buffer after each full render and skips
	// screen.Show when unchanged from the last flush. Default DefaultScreenRenderHashCache.
	ScreenRenderHashCache bool `toml:"screen_render_hash_cache"`
}

type FilterConfig struct {
	Mode              string `toml:"mode"`
	Syntax            string `toml:"syntax"`
	MatchPathSegments bool   `toml:"match_path_segments"`
	// CycleMatches controls Up/Down among quick-filter matches: "visual" (default) or "ranked".
	CycleMatches string `toml:"cycle_matches"`
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
	PreservePermissions bool `toml:"preserve_permissions"`
	PreserveTimestamps  bool `toml:"preserve_timestamps"`
	CopyBufferKiB       int  `toml:"copy_buffer_kib"`
	// SyncAfterEachFile fsyncs each copied file before closing (durable; slow for many small files).
	SyncAfterEachFile bool `toml:"sync_after_each_file"`
	// DiskSpaceCheckMinFileBytes: per-file mid-copy disk checks run only when the source file size is >= this value.
	// Zero means run the check before every file.
	DiskSpaceCheckMinFileBytes int64 `toml:"disk_space_check_min_file_bytes"`
	// CowFileCloning enables Linux FICLONE (CoW) when supported (similar to Midnight Commander file cloning).
	CowFileCloning bool `toml:"cow_file_cloning"`
	// FlattenDefaultLocation is the default destination prefill panel: "active" or "inactive".
	FlattenDefaultLocation string `toml:"flatten_default_location"`
	// FlattenRecursive is the default for the flatten dialog recursive checkbox.
	FlattenRecursive bool `toml:"flatten_recursive"`
	// FlattenRemoveEmptyDirs is the default for the flatten dialog remove-empty checkbox.
	FlattenRemoveEmptyDirs bool `toml:"flatten_remove_empty_dirs"`
	// RenameFocusAfter is the default for the rename dialog focus-after-rename checkbox.
	RenameFocusAfter bool `toml:"rename_focus_after"`
}

type LoggingConfig struct {
	Enabled bool   `toml:"enabled"`
	Level   string `toml:"level"`
	Path    string `toml:"path"`
}

// Default returns built-in values that match the current application behavior.
func Default() Config {
	return Config{
		Theme:                           ThemeDefault,
		ShowHidden:                      false,
		RespectGitignore:                true,
		ConfirmDelete:                   true,
		ConfirmOverwrite:                true,
		CaseInsensitiveFilter:           true,
		JobConcurrency:                  1,
		StartupPathMode:                 StartupPathCWD,
		DefaultSort:                     SortName,
		DefaultListingFormat:            DefaultListingFormat,
		SortReverse:                     false,
		DirectoriesFirst:                true,
		DiskUsageIdleSizeSort:           true,
		DiskUsageIdleSortDelayMS:        500,
		RefreshIntervalMS:               DefaultRefreshIntervalMS,
		DiskUsageDescendIntoMountPoints: false,
		DiskUsageWalkConcurrency:        DefaultDiskUsageWalkConcurrency,
		FollowSymlinksOnNavigation:      true,
		OpenFilesExternally:             true,
		RunExecutablesOnEnter:           true,
		DeleteMode:                      DeletePermanent,
		UI: UIConfig{
			ShowMenuBar:                       true,
			ShowFooter:                        true,
			ShowStatusLine:                    true,
			ShowJobsLine:                      true,
			ShowFileIcons:                     true,
			BorderStyle:                       BorderStyleSingle,
			Clock:                             false,
			StatusMessageTTLSeconds:           4.5,
			PathPickerValidateDelayMS:         DefaultPathPickerValidateDelayMS,
			PanelSyncFollowNavDebounceMS:      DefaultPanelSyncFollowNavDebounceMS,
			QuickViewPreviewDebounceMS:        DefaultQuickViewPreviewDebounceMS,
			CarouselPreviewDebounceMS:         DefaultCarouselPreviewDebounceMS,
			FindQueryDebounceMS:               DefaultFindQueryDebounceMS,
			FindMaxResults:                    DefaultFindMaxResults,
			FindListNavIdleMS:                 DefaultFindListNavIdleMS,
			ZoomActivePanel:                   DefaultZoomActivePanel,
			ZoomActivePanelDisabledAboveWidth: DefaultZoomActivePanelDisabledAboveWidth,
			PanelZoomActivePercent:            DefaultPanelZoomActivePercent,
			PanelZoomInactivePercent:          DefaultPanelZoomInactivePercent,
			ShrunkenShowsNameOnly:             DefaultShrunkenShowsNameOnly,
			CenterScrolling:                   DefaultCenterScrolling,
			MessageLogMaxEntries:              DefaultMessageLogMaxEntries,
			ScreenRenderHashCache:             DefaultScreenRenderHashCache,
		},
		Filter: FilterConfig{
			Mode:              FilterModeFuzzy,
			Syntax:            FilterSyntaxFZF,
			MatchPathSegments: false,
			CycleMatches:      FilterCycleMatchesVisual,
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
			PreservePermissions:        DefaultPreservePermissions,
			PreserveTimestamps:         DefaultPreserveTimestamps,
			CopyBufferKiB:              DefaultCopyBufferKiB,
			SyncAfterEachFile:          DefaultSyncAfterEachFile,
			DiskSpaceCheckMinFileBytes: DefaultDiskSpaceCheckMinFileBytes,
			CowFileCloning:             DefaultCowFileCloning,
			FlattenDefaultLocation:     DefaultFlattenDefaultLocation,
			FlattenRecursive:           DefaultFlattenRecursive,
			FlattenRemoveEmptyDirs:     DefaultFlattenRemoveEmptyDirs,
			RenameFocusAfter:           DefaultRenameFocusAfter,
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
			Command: DefaultFilePreviewCommand,
		},
		SFTP: SFTPConfig{
			IdleTimeoutSecs: DefaultSFTPIdleTimeoutSecs,
			DialTimeoutSecs: DefaultSFTPDialTimeoutSecs,
			ListTimeoutSecs: DefaultSFTPListTimeoutSecs,
		},
		Shell: ShellConfig{
			SyncCwdOnReturn: DefaultShellSyncCwdOnReturn,
		},
		Meta: MetaConfig{
			File:                "",
			LocalNames:          []string{DefaultMetaFileName},
			DefaultEntryWorkers: DefaultMetaEntryWorkers,
		},
		Pools: PoolsConfig{
			File: "",
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
		ConfigDir:       configDir,
		ConfigFile:      filepath.Join(configDir, fileName),
		ThemesDir:       filepath.Join(configDir, "themes"),
		KeybindingsFile: filepath.Join(configDir, keybindingsFileBaseName),
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
	ds := strings.ToLower(strings.TrimSpace(c.DefaultSort))
	if ds == SortDiskUsage || ds == "disk-usage" {
		c.DefaultSort = SortName
		c.DiskUsageIdleSizeSort = true
	}
	if c.JobConcurrency != 1 {
		c.JobConcurrency = builtin.JobConcurrency
	}
	if c.StartupPathMode != StartupPathCWD {
		c.StartupPathMode = builtin.StartupPathMode
	}
	if c.DiskUsageIdleSortDelayMS <= 0 {
		c.DiskUsageIdleSortDelayMS = 500
	}
	if c.RefreshIntervalMS < 0 {
		c.RefreshIntervalMS = DefaultRefreshIntervalMS
	}
	if c.RefreshIntervalMS > 0 {
		if c.RefreshIntervalMS < RefreshIntervalMinMS {
			c.RefreshIntervalMS = RefreshIntervalMinMS
		}
		if c.RefreshIntervalMS > RefreshIntervalMaxMS {
			c.RefreshIntervalMS = RefreshIntervalMaxMS
		}
	}
	if c.DiskUsageWalkConcurrency < 1 {
		c.DiskUsageWalkConcurrency = builtin.DiskUsageWalkConcurrency
	}
	if !c.sortModeValid(c.DefaultSort) {
		c.DefaultSort = builtin.DefaultSort
	}
	if !c.listingFormatValid(c.DefaultListingFormat) {
		c.DefaultListingFormat = DefaultListingFormat
	}
	// SortReverse is now supported, no clamping needed
	if !c.FollowSymlinksOnNavigation {
		c.FollowSymlinksOnNavigation = builtin.FollowSymlinksOnNavigation
	}
	if c.DeleteMode != DeletePermanent {
		c.DeleteMode = builtin.DeleteMode
	}
	if c.UI.BorderStyle != BorderStyleSingle {
		c.UI.BorderStyle = builtin.UI.BorderStyle
	}
	if c.UI.StatusMessageTTLSeconds < 0 {
		c.UI.StatusMessageTTLSeconds = builtin.UI.StatusMessageTTLSeconds
	}
	if c.UI.PathPickerValidateDelayMS < 0 {
		c.UI.PathPickerValidateDelayMS = builtin.UI.PathPickerValidateDelayMS
	}
	const pathPickerValidateMaxMS = 30_000
	if c.UI.PathPickerValidateDelayMS > pathPickerValidateMaxMS {
		c.UI.PathPickerValidateDelayMS = pathPickerValidateMaxMS
	}
	if c.UI.PanelSyncFollowNavDebounceMS < 0 {
		c.UI.PanelSyncFollowNavDebounceMS = builtin.UI.PanelSyncFollowNavDebounceMS
	}
	const panelSyncFollowNavDebounceMaxMS = 10_000
	if c.UI.PanelSyncFollowNavDebounceMS > panelSyncFollowNavDebounceMaxMS {
		c.UI.PanelSyncFollowNavDebounceMS = panelSyncFollowNavDebounceMaxMS
	}
	if c.UI.QuickViewPreviewDebounceMS < 0 {
		c.UI.QuickViewPreviewDebounceMS = builtin.UI.QuickViewPreviewDebounceMS
	}
	if c.UI.QuickViewPreviewDebounceMS > panelSyncFollowNavDebounceMaxMS {
		c.UI.QuickViewPreviewDebounceMS = panelSyncFollowNavDebounceMaxMS
	}
	if c.UI.CarouselPreviewDebounceMS < 0 {
		c.UI.CarouselPreviewDebounceMS = builtin.UI.CarouselPreviewDebounceMS
	}
	if c.UI.CarouselPreviewDebounceMS > panelSyncFollowNavDebounceMaxMS {
		c.UI.CarouselPreviewDebounceMS = panelSyncFollowNavDebounceMaxMS
	}
	if c.UI.FindQueryDebounceMS < 0 {
		c.UI.FindQueryDebounceMS = builtin.UI.FindQueryDebounceMS
	}
	if c.UI.FindQueryDebounceMS > panelSyncFollowNavDebounceMaxMS {
		c.UI.FindQueryDebounceMS = panelSyncFollowNavDebounceMaxMS
	}
	if c.UI.FindMaxResults <= 0 {
		c.UI.FindMaxResults = builtin.UI.FindMaxResults
	}
	if c.UI.FindListNavIdleMS < 0 {
		c.UI.FindListNavIdleMS = builtin.UI.FindListNavIdleMS
	}
	if c.UI.PanelZoomActivePercent <= 0 || c.UI.PanelZoomInactivePercent <= 0 ||
		c.UI.PanelZoomActivePercent+c.UI.PanelZoomInactivePercent != 100 {
		c.UI.PanelZoomActivePercent = DefaultPanelZoomActivePercent
		c.UI.PanelZoomInactivePercent = DefaultPanelZoomInactivePercent
	}
	if c.UI.ZoomActivePanelDisabledAboveWidth < 0 {
		c.UI.ZoomActivePanelDisabledAboveWidth = builtin.UI.ZoomActivePanelDisabledAboveWidth
	}
	if c.UI.MessageLogMaxEntries <= 0 {
		c.UI.MessageLogMaxEntries = builtin.UI.MessageLogMaxEntries
	}
	const messageLogMaxCap = 50_000
	if c.UI.MessageLogMaxEntries > messageLogMaxCap {
		c.UI.MessageLogMaxEntries = messageLogMaxCap
	}
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
	if !validLoggingLevel(c.Logging.Level) {
		c.Logging.Level = builtin.Logging.Level
	}
	if len(c.UserMenu.LocalNames) == 0 {
		c.UserMenu.LocalNames = append([]string(nil), builtin.UserMenu.LocalNames...)
	}
	if strings.TrimSpace(c.Preview.Command) == "" {
		c.Preview.Command = builtin.Preview.Command
	}
	if _, err := cmdrun.PreviewCommandArgv(c.Preview.Command, "/tmp/pc-preview-validate", 80); err != nil {
		c.Preview.Command = builtin.Preview.Command
	}
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
	return nil
}

func (c Config) sortModeValid(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case SortName, SortExtension, SortSize, SortMtime:
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
