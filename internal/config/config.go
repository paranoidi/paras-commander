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
)

const (
	appDirName = "paras-commander"
	fileName   = "config.toml"

	// actionKeysTable and jobsActionKeysTable are optional top-level
	// TOML tables holding keybindings inside config.toml. They mirror
	// the canonical contents of keybindings.toml and are owned by the
	// keymap package; config only tolerates them as pass-through so a
	// single bootstrap file can carry both general settings and the
	// full shortcut map (global plus jobs-view overlay).
	actionKeysTable         = "action_keys"
	jobsActionKeysTable     = "jobs_action_keys"
	commandsActionKeysTable = "commands_action_keys"

	ThemeDefault    = "default"
	StartupPathCWD  = "cwd"
	SortName        = "name"
	SortExtension   = "extension"
	SortSize        = "size"
	SortMtime       = "mtime"
	SortDiskUsage   = "disk_usage"
	DeletePermanent = "permanent"
	FilterModeFuzzy = "fuzzy"
	FilterSyntaxFZF = "subset-fzf"
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
	Theme                           string `toml:"theme"`
	ShowHidden                      bool   `toml:"show_hidden"`
	ConfirmDelete                   bool   `toml:"confirm_delete"`
	ConfirmOverwrite                bool   `toml:"confirm_overwrite"`
	CaseInsensitiveFilter           bool   `toml:"case_insensitive_filter"`
	JobConcurrency                  int    `toml:"job_concurrency"`
	StartupPathMode                 string `toml:"startup_path_mode"`
	DefaultSort                     string `toml:"default_sort"`
	SortReverse                     bool   `toml:"sort_reverse"`
	DirectoriesFirst                bool   `toml:"directories_first"`
	DiskUsageIdleSizeSort           bool   `toml:"disk_usage_idle_size_sort"`
	DiskUsageIdleSortDelayMS        int    `toml:"disk_usage_idle_sort_delay_ms"`
	DiskUsageDescendIntoMountPoints bool   `toml:"disk_usage_descend_into_mount_points"`
	// DiskUsageWalkConcurrency limits concurrent subdirectory branches during a disk-usage walk (minimum 1 after Validate).
	// Default is DefaultDiskUsageWalkConcurrency.
	// Low values spare HDD/NAS; raise for fast local SSDs.
	DiskUsageWalkConcurrency   int              `toml:"disk_usage_walk_concurrency"`
	FollowSymlinksOnNavigation bool             `toml:"follow_symlinks_on_navigation"`
	OpenFilesExternally        bool             `toml:"open_files_externally"`
	DeleteMode                 string           `toml:"delete_mode"`
	UI                         UIConfig         `toml:"ui"`
	Filter                     FilterConfig     `toml:"filter"`
	Jobs                       JobsConfig       `toml:"jobs"`
	Operations                 OperationsConfig `toml:"operations"`
	Logging                    LoggingConfig    `toml:"logging"`
	Bookmarks                  BookmarksConfig  `toml:"bookmarks"`
	// Meta holds named per-entry shell commands shown in the panel Meta column.
	// Each key is the command name; command receives the entry path as $1.
	Meta map[string]MetaCommandDef `toml:"meta"`
}

// MetaCommandDef is one named meta command entry.
// File runs for regular files; Dirs runs for directories; $1 = absolute path.
// Output exceeding one line or 20 characters is replaced with "too long".
type MetaCommandDef struct {
	Description string `toml:"description"`
	File        string `toml:"file"`
	Dirs        string `toml:"dirs"`
}

// BookmarksConfig controls fzf-marks compatible directory marks.
type BookmarksConfig struct {
	// File is the marks file path. Empty uses FZF_MARKS_FILE or ~/.fzf-marks.
	File string `toml:"file"`
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
	// Default 3.5. Use 0 to keep messages until replaced or cleared by another action.
	StatusMessageTTLSeconds float64 `toml:"status_message_ttl_seconds"`
}

type FilterConfig struct {
	Mode              string `toml:"mode"`
	Syntax            string `toml:"syntax"`
	MatchPathSegments bool   `toml:"match_path_segments"`
	// CycleMatches controls Up/Down among quick-filter matches: "visual" (default) or "ranked".
	CycleMatches string `toml:"cycle_matches"`
}

type JobsConfig struct {
	ShowFinished      bool `toml:"show_finished"`
	KeepFinished      int  `toml:"keep_finished"`
	AutoshowOnError   bool `toml:"autoshow_on_error"`
	AutoshowOnStart   bool `toml:"autoshow_on_start"`
	RefreshDebounceMS int  `toml:"refresh_debounce_ms"`
	// ProgressEmitMinBytes is the minimum bytes copied between optional worker progress events.
	ProgressEmitMinBytes int `toml:"progress_emit_min_bytes"`
	// ProgressEmitMinIntervalMS is the minimum milliseconds between optional progress events when copying.
	ProgressEmitMinIntervalMS int `toml:"progress_emit_min_interval_ms"`
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
		ConfirmDelete:                   true,
		ConfirmOverwrite:                true,
		CaseInsensitiveFilter:           true,
		JobConcurrency:                  1,
		StartupPathMode:                 StartupPathCWD,
		DefaultSort:                     SortName,
		SortReverse:                     false,
		DirectoriesFirst:                true,
		DiskUsageIdleSizeSort:           true,
		DiskUsageIdleSortDelayMS:        500,
		DiskUsageDescendIntoMountPoints: false,
		DiskUsageWalkConcurrency:        DefaultDiskUsageWalkConcurrency,
		FollowSymlinksOnNavigation:      true,
		OpenFilesExternally:             true,
		DeleteMode:                      DeletePermanent,
		UI: UIConfig{
			ShowMenuBar:             true,
			ShowFooter:              true,
			ShowStatusLine:          true,
			ShowJobsLine:            true,
			ShowFileIcons:           true,
			BorderStyle:             BorderStyleSingle,
			Clock:                   false,
			StatusMessageTTLSeconds: 3.5,
		},
		Filter: FilterConfig{
			Mode:              FilterModeFuzzy,
			Syntax:            FilterSyntaxFZF,
			MatchPathSegments: false,
			CycleMatches:      FilterCycleMatchesVisual,
		},
		Jobs: JobsConfig{
			ShowFinished:              true,
			KeepFinished:              20,
			AutoshowOnError:           true,
			AutoshowOnStart:           false,
			RefreshDebounceMS:         150,
			ProgressEmitMinBytes:      DefaultProgressEmitMinBytes,
			ProgressEmitMinIntervalMS: DefaultProgressEmitMinIntervalMS,
		},
		Operations: OperationsConfig{
			PreservePermissions:        DefaultPreservePermissions,
			PreserveTimestamps:         DefaultPreserveTimestamps,
			CopyBufferKiB:              DefaultCopyBufferKiB,
			SyncAfterEachFile:          DefaultSyncAfterEachFile,
			DiskSpaceCheckMinFileBytes: DefaultDiskSpaceCheckMinFileBytes,
			CowFileCloning:             DefaultCowFileCloning,
		},
		Logging: LoggingConfig{
			Enabled: false,
			Level:   "info",
			Path:    "",
		},
		Bookmarks: BookmarksConfig{
			File: "",
		},
		Meta: map[string]MetaCommandDef{
			"count-items": {
				Description: "Count files and folders",
				Dirs:        `echo "$(find "$1" -maxdepth 1 -mindepth 1 -type f | wc -l) $(find "$1" -maxdepth 1 -mindepth 1 -type d | wc -l) "`,
			},
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
		KeybindingsFile: filepath.Join(configDir, "keybindings.toml"),
	}, nil
}

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
		if len(key) > 0 && isShortcutTable(key[0]) {
			continue
		}
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

// ReadActionKeys parses the optional [action_keys] table from a config.toml
// style file and returns the action -> chord-strings map it contains.
//
// When the file is missing or has no [action_keys] table, the returned map
// is nil and the error is nil. Other read or parse errors propagate.
//
// This lets a single config.toml bootstrap file carry both general
// settings and shortcut overrides; the keymap loader prefers a dedicated
// keybindings.toml when present and falls back to this table otherwise.
func ReadActionKeys(filename string) (map[string][]string, error) {
	return readShortcutTable(filename, actionKeysTable)
}

// ReadJobsActionKeys parses the optional [jobs_action_keys] table from a
// config.toml style file. Same nil/error semantics as ReadActionKeys.
//
// The jobs-view overlay (see internal/keymap/jobs_overlay.go) only
// permits jobs.* action IDs; this function preserves whatever keys the
// file contains and leaves validation to the keymap loader so error
// messages remain centralized in one place.
func ReadJobsActionKeys(filename string) (map[string][]string, error) {
	return readShortcutTable(filename, jobsActionKeysTable)
}

// ReadCommandsActionKeys parses the optional [commands_action_keys] table from a config.toml
// style file. Same nil/error semantics as ReadActionKeys.
func ReadCommandsActionKeys(filename string) (map[string][]string, error) {
	return readShortcutTable(filename, commandsActionKeysTable)
}

// readShortcutTable extracts a single named keybindings table from a
// TOML file. It is shared by ReadActionKeys / ReadJobsActionKeys so that
// adding a future bundle (another top-level "*_action_keys" table) is a
// one-liner: register the table name and add a thin wrapper.
func readShortcutTable(filename, table string) (map[string][]string, error) {
	if strings.TrimSpace(filename) == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read config %q: %w", filename, err)
	}
	var top map[string]interface{}
	if err := toml.Unmarshal(raw, &top); err != nil {
		return nil, fmt.Errorf("parse config %q: %w", filename, err)
	}
	rawTable, ok := top[table].(map[string]interface{})
	if !ok || len(rawTable) == 0 {
		return nil, nil
	}
	out := make(map[string][]string)
	if err := flattenShortcutTable(rawTable, table, "", out); err != nil {
		return nil, fmt.Errorf("parse config %q: %w", filename, err)
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// isShortcutTable reports whether a top-level TOML key is one of the
// keybindings pass-through tables tolerated inside config.toml.
func isShortcutTable(name string) bool {
	switch name {
	case actionKeysTable, jobsActionKeysTable, commandsActionKeysTable:
		return true
	}
	return false
}

// flattenShortcutTable converts the nested map produced by TOML's
// dotted-key parsing (e.g. action_keys.app.quit) into a flat map keyed
// by the dotted action ID with string-list values.
func flattenShortcutTable(node map[string]interface{}, table, prefix string, out map[string][]string) error {
	for key, value := range node {
		full := key
		if prefix != "" {
			full = prefix + "." + key
		}
		switch typed := value.(type) {
		case map[string]interface{}:
			if err := flattenShortcutTable(typed, table, full, out); err != nil {
				return err
			}
		case []interface{}:
			items := make([]string, 0, len(typed))
			for _, item := range typed {
				s, ok := item.(string)
				if !ok {
					return fmt.Errorf("[%s.%s]: expected string list elements", table, full)
				}
				items = append(items, s)
			}
			out[full] = items
		default:
			return fmt.Errorf("[%s.%s]: unsupported type %T", table, full, value)
		}
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
		c.DiskUsageIdleSortDelayMS = builtin.DiskUsageIdleSortDelayMS
	}
	if c.DiskUsageWalkConcurrency < 1 {
		c.DiskUsageWalkConcurrency = builtin.DiskUsageWalkConcurrency
	}
	if !c.sortModeValid(c.DefaultSort) {
		c.DefaultSort = builtin.DefaultSort
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
	if c.Jobs.RefreshDebounceMS <= 0 {
		c.Jobs.RefreshDebounceMS = builtin.Jobs.RefreshDebounceMS
	}
	const jobsRefreshMinMS = 50
	const jobsRefreshMaxMS = 5000
	if c.Jobs.RefreshDebounceMS < jobsRefreshMinMS {
		c.Jobs.RefreshDebounceMS = jobsRefreshMinMS
	}
	if c.Jobs.RefreshDebounceMS > jobsRefreshMaxMS {
		c.Jobs.RefreshDebounceMS = jobsRefreshMaxMS
	}
	const progressEmitBytesMin = 64 * 1024
	const progressEmitBytesMax = 64 * 1024 * 1024
	if c.Jobs.ProgressEmitMinBytes <= 0 {
		c.Jobs.ProgressEmitMinBytes = builtin.Jobs.ProgressEmitMinBytes
	}
	if c.Jobs.ProgressEmitMinBytes < progressEmitBytesMin {
		c.Jobs.ProgressEmitMinBytes = progressEmitBytesMin
	}
	if c.Jobs.ProgressEmitMinBytes > progressEmitBytesMax {
		c.Jobs.ProgressEmitMinBytes = progressEmitBytesMax
	}
	if c.Jobs.ProgressEmitMinIntervalMS <= 0 {
		c.Jobs.ProgressEmitMinIntervalMS = builtin.Jobs.ProgressEmitMinIntervalMS
	}
	if c.Jobs.ProgressEmitMinIntervalMS < jobsRefreshMinMS {
		c.Jobs.ProgressEmitMinIntervalMS = jobsRefreshMinMS
	}
	if c.Jobs.ProgressEmitMinIntervalMS > jobsRefreshMaxMS {
		c.Jobs.ProgressEmitMinIntervalMS = jobsRefreshMaxMS
	}
	if c.Operations.CopyBufferKiB <= 0 {
		c.Operations.CopyBufferKiB = builtin.Operations.CopyBufferKiB
	}
	if c.Operations.DiskSpaceCheckMinFileBytes < 0 {
		c.Operations.DiskSpaceCheckMinFileBytes = builtin.Operations.DiskSpaceCheckMinFileBytes
	}
	if !validLoggingLevel(c.Logging.Level) {
		c.Logging.Level = builtin.Logging.Level
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

func validLoggingLevel(level string) bool {
	switch strings.ToLower(level) {
	case "debug", "info", "warn", "error":
		return true
	default:
		return false
	}
}
