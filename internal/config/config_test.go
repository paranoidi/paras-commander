package config

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestValidateClampsDiskUsageWalkConcurrency(t *testing.T) {
	cfg := Default()
	cfg.DiskUsageWalkConcurrency = 0
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.DiskUsageWalkConcurrency != DefaultDiskUsageWalkConcurrency {
		t.Fatalf("DiskUsageWalkConcurrency = %d, want %d", cfg.DiskUsageWalkConcurrency, DefaultDiskUsageWalkConcurrency)
	}
	cfg.DiskUsageWalkConcurrency = 2
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.DiskUsageWalkConcurrency != 2 {
		t.Fatalf("DiskUsageWalkConcurrency = %d, want 2", cfg.DiskUsageWalkConcurrency)
	}
}

func TestValidateClampsNegativeDiskSpaceCheckMinFileBytes(t *testing.T) {
	cfg := Default()
	cfg.Operations.DiskSpaceCheckMinFileBytes = -1
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	want := int64(DefaultDiskSpaceCheckMinFileBytes)
	if cfg.Operations.DiskSpaceCheckMinFileBytes != want {
		t.Fatalf("DiskSpaceCheckMinFileBytes = %d, want %d", cfg.Operations.DiskSpaceCheckMinFileBytes, want)
	}
}

func TestValidateClampsJobsProgressEmit(t *testing.T) {
	cfg := Default()
	cfg.Jobs.ProgressEmitMinBytes = 100
	cfg.Jobs.ProgressEmitMinIntervalMS = 10
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.Jobs.ProgressEmitMinBytes != 64*1024 {
		t.Fatalf("ProgressEmitMinBytes = %d, want %d", cfg.Jobs.ProgressEmitMinBytes, 64*1024)
	}
	if cfg.Jobs.ProgressEmitMinIntervalMS != 50 {
		t.Fatalf("ProgressEmitMinIntervalMS = %d, want 50", cfg.Jobs.ProgressEmitMinIntervalMS)
	}
	cfg.Jobs.ProgressEmitMinBytes = 128 * 1024 * 1024
	cfg.Jobs.ProgressEmitMinIntervalMS = 999999
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.Jobs.ProgressEmitMinBytes != 64*1024*1024 {
		t.Fatalf("ProgressEmitMinBytes = %d, want max 64MiB", cfg.Jobs.ProgressEmitMinBytes)
	}
	if cfg.Jobs.ProgressEmitMinIntervalMS != 5000 {
		t.Fatalf("ProgressEmitMinIntervalMS = %d, want 5000", cfg.Jobs.ProgressEmitMinIntervalMS)
	}
}

func TestDefaultPathPickerValidateDelayMS(t *testing.T) {
	if got := Default().UI.PathPickerValidateDelayMS; got != DefaultPathPickerValidateDelayMS {
		t.Fatalf("PathPickerValidateDelayMS = %d, want %d", got, DefaultPathPickerValidateDelayMS)
	}
}

func TestValidateClampsPathPickerValidateDelayMS(t *testing.T) {
	cfg := Default()
	cfg.UI.PathPickerValidateDelayMS = -5
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.UI.PathPickerValidateDelayMS != DefaultPathPickerValidateDelayMS {
		t.Fatalf("negative delay = %d, want default %d", cfg.UI.PathPickerValidateDelayMS, DefaultPathPickerValidateDelayMS)
	}
	cfg.UI.PathPickerValidateDelayMS = 999999
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.UI.PathPickerValidateDelayMS != 30000 {
		t.Fatalf("huge delay = %d, want 30000", cfg.UI.PathPickerValidateDelayMS)
	}
}

func TestLoadFromPathsUsesDefaultsWhenConfigIsMissing(t *testing.T) {
	cfg, err := LoadFromPaths(Paths{ConfigFile: filepath.Join(t.TempDir(), "config.toml")})
	if err != nil {
		t.Fatalf("LoadFromPaths() error = %v", err)
	}

	if !reflect.DeepEqual(cfg, Default()) {
		t.Fatalf("LoadFromPaths() = %+v, want defaults %+v", cfg, Default())
	}
}

func TestLoadFromPathsMergesConfigOverDefaults(t *testing.T) {
	path := writeConfig(t, `
show_hidden = true
confirm_delete = false
directories_first = false

[jobs]
keep_finished = 5

[operations]
copy_buffer_kib = 512

[logging]
level = "debug"
`)

	cfg, err := LoadFromPaths(Paths{ConfigFile: path})
	if err != nil {
		t.Fatalf("LoadFromPaths() error = %v", err)
	}

	if !cfg.ShowHidden {
		t.Fatal("ShowHidden = false, want true")
	}
	if cfg.ConfirmDelete {
		t.Fatal("ConfirmDelete = true, want false")
	}
	if cfg.DirectoriesFirst {
		t.Fatal("DirectoriesFirst = true, want false")
	}
	if cfg.Jobs.KeepFinished != 5 {
		t.Fatalf("Jobs.KeepFinished = %d, want 5", cfg.Jobs.KeepFinished)
	}
	if cfg.Operations.CopyBufferKiB != 512 {
		t.Fatalf("Operations.CopyBufferKiB = %d, want 512", cfg.Operations.CopyBufferKiB)
	}
	if cfg.Logging.Level != "debug" {
		t.Fatalf("Logging.Level = %q, want debug", cfg.Logging.Level)
	}
	if !cfg.ConfirmOverwrite {
		t.Fatal("ConfirmOverwrite = false, want default true")
	}
}

func TestLoadFromPathsMergesStatusMessageTTL(t *testing.T) {
	path := writeConfig(t, `
[ui]
status_message_ttl_seconds = 12.25
`)
	cfg, err := LoadFromPaths(Paths{ConfigFile: path})
	if err != nil {
		t.Fatalf("LoadFromPaths() error = %v", err)
	}
	if cfg.UI.StatusMessageTTLSeconds != 12.25 {
		t.Fatalf("StatusMessageTTLSeconds = %v, want 12.25", cfg.UI.StatusMessageTTLSeconds)
	}
}

func TestLoadFromPathsReportsInvalidTOML(t *testing.T) {
	path := writeConfig(t, `show_hidden =`)

	_, err := LoadFromPaths(Paths{ConfigFile: path})
	if err == nil {
		t.Fatal("LoadFromPaths() error = nil, want parse error")
	}
	if !strings.Contains(err.Error(), "load config") {
		t.Fatalf("LoadFromPaths() error = %v, want load config context", err)
	}
}

func TestLoadFromPathsRejectsUnknownFields(t *testing.T) {
	path := writeConfig(t, `unknown_setting = true`)

	_, err := LoadFromPaths(Paths{ConfigFile: path})
	if err == nil {
		t.Fatal("LoadFromPaths() error = nil, want unknown field error")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("LoadFromPaths() error = %v, want unknown field", err)
	}
}

func TestLoadFromPathsClampsUnsupportedValuesToDefaults(t *testing.T) {
	tests := []struct {
		name    string
		content string
		testFn  func(t *testing.T, cfg Config)
	}{
		{
			name:    "sort mode",
			content: `default_sort = "unknown_mode"`,
			testFn: func(t *testing.T, cfg Config) {
				if cfg.DefaultSort != SortName {
					t.Fatalf("DefaultSort = %q, want clamped to %q", cfg.DefaultSort, SortName)
				}
			},
		},
		{
			name:    "default_sort disk_usage migrates",
			content: `default_sort = "disk_usage"`,
			testFn: func(t *testing.T, cfg Config) {
				if cfg.DefaultSort != SortName {
					t.Fatalf("DefaultSort = %q, want migrated to %q", cfg.DefaultSort, SortName)
				}
				if !cfg.DiskUsageIdleSizeSort {
					t.Fatal("DiskUsageIdleSizeSort = false, want true after disk_usage migration")
				}
			},
		},
		{
			name:    "default_sort disk-usage migrates",
			content: `default_sort = "disk-usage"`,
			testFn: func(t *testing.T, cfg Config) {
				if cfg.DefaultSort != SortName {
					t.Fatalf("DefaultSort = %q, want migrated to %q", cfg.DefaultSort, SortName)
				}
				if !cfg.DiskUsageIdleSizeSort {
					t.Fatal("DiskUsageIdleSizeSort = false, want true after disk-usage migration")
				}
			},
		},
		{
			name:    "listing format invalid",
			content: `default_listing_format = "wide"`,
			testFn: func(t *testing.T, cfg Config) {
				if cfg.DefaultListingFormat != ListingFormatMtime {
					t.Fatalf("DefaultListingFormat = %q, want clamped to %q", cfg.DefaultListingFormat, ListingFormatMtime)
				}
			},
		},
		{
			name:    "job concurrency",
			content: `job_concurrency = 2`,
			testFn: func(t *testing.T, cfg Config) {
				if cfg.JobConcurrency != 1 {
					t.Fatalf("JobConcurrency = %d, want clamped to 1", cfg.JobConcurrency)
				}
			},
		},
		{
			name:    "startup mode",
			content: `startup_path_mode = "last-session"`,
			testFn: func(t *testing.T, cfg Config) {
				if cfg.StartupPathMode != StartupPathCWD {
					t.Fatalf("StartupPathMode = %q, want clamped to %q", cfg.StartupPathMode, StartupPathCWD)
				}
			},
		},
		{
			name:    "path filter",
			content: "[filter]\nmatch_path_segments = true\n",
			testFn: func(t *testing.T, cfg Config) {
				if cfg.Filter.MatchPathSegments {
					t.Fatal("MatchPathSegments = true, want clamped to false")
				}
			},
		},
		{
			name:    "filter cycle_matches invalid",
			content: "[filter]\ncycle_matches = \"nope\"\n",
			testFn: func(t *testing.T, cfg Config) {
				if cfg.Filter.CycleMatches != FilterCycleMatchesVisual {
					t.Fatalf("CycleMatches = %q, want clamped visual", cfg.Filter.CycleMatches)
				}
			},
		},
		{
			name:    "finished retention",
			content: "[jobs]\nkeep_finished = 0\n",
			testFn: func(t *testing.T, cfg Config) {
				if cfg.Jobs.KeepFinished <= 0 {
					t.Fatalf("KeepFinished = %d, want clamped to default positive value", cfg.Jobs.KeepFinished)
				}
			},
		},
		{
			name:    "copy buffer",
			content: "[operations]\ncopy_buffer_kib = 0\n",
			testFn: func(t *testing.T, cfg Config) {
				if cfg.Operations.CopyBufferKiB <= 0 {
					t.Fatalf("CopyBufferKiB = %d, want clamped to default positive value", cfg.Operations.CopyBufferKiB)
				}
			},
		},
		{
			name:    "status message ttl negative",
			content: "[ui]\nstatus_message_ttl_seconds = -1\n",
			testFn: func(t *testing.T, cfg Config) {
				want := Default().UI.StatusMessageTTLSeconds
				if cfg.UI.StatusMessageTTLSeconds != want {
					t.Fatalf("StatusMessageTTLSeconds = %v, want clamped to default %v", cfg.UI.StatusMessageTTLSeconds, want)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeConfig(t, tt.content)
			cfg, err := LoadFromPaths(Paths{ConfigFile: path})
			if err != nil {
				t.Fatalf("LoadFromPaths() error = %v, want success with clamped values", err)
			}
			tt.testFn(t, cfg)
		})
	}
}

func TestDefaultPathsUsesXDGConfigHome(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	paths, err := DefaultPaths()
	if err != nil {
		t.Fatalf("DefaultPaths() error = %v", err)
	}

	wantDir := filepath.Join(configHome, appDirName)
	if paths.ConfigDir != wantDir {
		t.Fatalf("ConfigDir = %q, want %q", paths.ConfigDir, wantDir)
	}
	if paths.ConfigFile != filepath.Join(wantDir, fileName) {
		t.Fatalf("ConfigFile = %q, want config.toml under config dir", paths.ConfigFile)
	}
}

func TestEncodeDefaultStubWritesLoadableDefaults(t *testing.T) {
	var buffer bytes.Buffer
	if err := EncodeDefaultStub(&buffer); err != nil {
		t.Fatalf("EncodeDefaultStub() error = %v", err)
	}

	content := buffer.String()
	for _, want := range []string{"theme = \"default\"", "show_hidden = false", "[ui]", "status_message_ttl_seconds", "[filter]", "[jobs]", "[operations]", "[logging]"} {
		if !strings.Contains(content, want) {
			t.Fatalf("encoded config missing %q:\n%s", want, content)
		}
	}

	path := writeConfig(t, content)
	cfg, err := LoadFromPaths(Paths{ConfigFile: path})
	if err != nil {
		t.Fatalf("LoadFromPaths() error = %v", err)
	}
	if !reflect.DeepEqual(cfg, Default()) {
		t.Fatalf("decoded config = %+v, want defaults %+v", cfg, Default())
	}
}

func TestWriteDefaultStubWritesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := WriteDefaultStub(path); err != nil {
		t.Fatalf("WriteDefaultStub() error = %v", err)
	}

	cfg, err := LoadFromPaths(Paths{ConfigFile: path})
	if err != nil {
		t.Fatalf("LoadFromPaths() error = %v", err)
	}
	if !reflect.DeepEqual(cfg, Default()) {
		t.Fatalf("decoded config = %+v, want defaults %+v", cfg, Default())
	}
}

// TestLoadFromPathsAcceptsActionKeysTable verifies that an [action_keys]
// table inside config.toml is tolerated (not flagged as an unknown
// field). The table is owned by the keymap package and consumed via
// ReadActionKeys; config only needs to pass it through cleanly.
func TestLoadFromPathsAcceptsActionKeysTable(t *testing.T) {
	path := writeConfig(t, `theme = "default"
[action_keys]
"app.quit" = ["F12"]
"panel.refresh" = ["F2"]
`)
	cfg, err := LoadFromPaths(Paths{ConfigFile: path})
	if err != nil {
		t.Fatalf("LoadFromPaths() error = %v, want success with [action_keys]", err)
	}
	if cfg.Theme != "default" {
		t.Fatalf("Theme = %q, want default", cfg.Theme)
	}
}

// TestLoadFromPathsAcceptsJobsActionKeysTable verifies the same
// pass-through tolerance for [jobs_action_keys] (the jobs-view overlay
// added with the keymap Bundle feature). Without this, a stub written
// by --config-stub could not be loaded once the user starts customising
// jobs-view chords inside config.toml.
func TestLoadFromPathsAcceptsJobsActionKeysTable(t *testing.T) {
	path := writeConfig(t, `theme = "default"
[action_keys]
"app.quit" = ["F12"]
[jobs_action_keys]
"jobs.clear-finished" = ["C-k"]
`)
	cfg, err := LoadFromPaths(Paths{ConfigFile: path})
	if err != nil {
		t.Fatalf("LoadFromPaths() error = %v, want success with [jobs_action_keys]", err)
	}
	if cfg.Theme != "default" {
		t.Fatalf("Theme = %q, want default", cfg.Theme)
	}
}

func TestLoadFromPathsAcceptsPathPickerHostActionKeysTable(t *testing.T) {
	path := writeConfig(t, `theme = "default"
[action_keys]
"app.quit" = ["F12"]
[path_picker_host_action_keys]
"ui.open-path-picker" = ["C-p"]
`)
	cfg, err := LoadFromPaths(Paths{ConfigFile: path})
	if err != nil {
		t.Fatalf("LoadFromPaths() error = %v, want success with [path_picker_host_action_keys]", err)
	}
	if cfg.Theme != "default" {
		t.Fatalf("Theme = %q, want default", cfg.Theme)
	}
}

func TestReadPathPickerHostActionKeysExtractsTable(t *testing.T) {
	path := writeConfig(t, `theme = "default"
[path_picker_host_action_keys]
"ui.open-path-picker" = ["C-p"]
`)
	keys, err := ReadPathPickerHostActionKeys(path)
	if err != nil {
		t.Fatalf("ReadPathPickerHostActionKeys() error = %v", err)
	}
	if got, want := keys["ui.open-path-picker"], []string{"C-p"}; !equalStringSlice(got, want) {
		t.Fatalf("ui.open-path-picker = %v, want %v", got, want)
	}
}

func TestLoadFromPathsAcceptsDialogInputActionKeysTable(t *testing.T) {
	path := writeConfig(t, `theme = "default"
[action_keys]
"app.quit" = ["F12"]
[dialog_input_action_keys]
"ui.input.backward-word" = ["M-B"]
`)
	cfg, err := LoadFromPaths(Paths{ConfigFile: path})
	if err != nil {
		t.Fatalf("LoadFromPaths() error = %v, want success with [dialog_input_action_keys]", err)
	}
	if cfg.Theme != "default" {
		t.Fatalf("Theme = %q, want default", cfg.Theme)
	}
}

func TestReadDialogInputActionKeysExtractsTable(t *testing.T) {
	path := writeConfig(t, `theme = "default"
[dialog_input_action_keys]
"ui.input.forward-word" = ["M-f", "C-M-f"]
`)
	keys, err := ReadDialogInputActionKeys(path)
	if err != nil {
		t.Fatalf("ReadDialogInputActionKeys() error = %v", err)
	}
	if got, want := keys["ui.input.forward-word"], []string{"M-f", "C-M-f"}; !equalStringSlice(got, want) {
		t.Fatalf("ui.input.forward-word = %v, want %v", got, want)
	}
}

func TestReadActionKeysReturnsNilWhenMissing(t *testing.T) {
	keys, err := ReadActionKeys(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatalf("ReadActionKeys() error = %v", err)
	}
	if keys != nil {
		t.Fatalf("ReadActionKeys = %v, want nil for missing file", keys)
	}
}

func TestReadActionKeysReturnsNilWhenTableAbsent(t *testing.T) {
	path := writeConfig(t, `theme = "default"`)
	keys, err := ReadActionKeys(path)
	if err != nil {
		t.Fatalf("ReadActionKeys() error = %v", err)
	}
	if keys != nil {
		t.Fatalf("ReadActionKeys = %v, want nil when [action_keys] missing", keys)
	}
}

func TestReadActionKeysExtractsDottedActionIDs(t *testing.T) {
	path := writeConfig(t, `theme = "default"
[action_keys]
"app.quit" = ["F12"]
"panel.refresh" = ["F2", "C-r"]
`)
	keys, err := ReadActionKeys(path)
	if err != nil {
		t.Fatalf("ReadActionKeys() error = %v", err)
	}
	if got, want := keys["app.quit"], []string{"F12"}; !equalStringSlice(got, want) {
		t.Fatalf("app.quit = %v, want %v", got, want)
	}
	if got, want := keys["panel.refresh"], []string{"F2", "C-r"}; !equalStringSlice(got, want) {
		t.Fatalf("panel.refresh = %v, want %v", got, want)
	}
}

func TestReadJobsActionKeysReturnsNilWhenMissing(t *testing.T) {
	keys, err := ReadJobsActionKeys(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatalf("ReadJobsActionKeys() error = %v", err)
	}
	if keys != nil {
		t.Fatalf("ReadJobsActionKeys = %v, want nil for missing file", keys)
	}
}

func TestReadJobsActionKeysExtractsTable(t *testing.T) {
	path := writeConfig(t, `theme = "default"
[jobs_action_keys]
"jobs.clear-finished" = ["C-k"]
`)
	keys, err := ReadJobsActionKeys(path)
	if err != nil {
		t.Fatalf("ReadJobsActionKeys() error = %v", err)
	}
	if got, want := keys["jobs.clear-finished"], []string{"C-k"}; !equalStringSlice(got, want) {
		t.Fatalf("jobs.clear-finished = %v, want %v", got, want)
	}
	plainKeys, err := ReadActionKeys(path)
	if err != nil {
		t.Fatalf("ReadActionKeys() error = %v", err)
	}
	if plainKeys != nil {
		t.Fatalf("ReadActionKeys = %v, want nil when only [jobs_action_keys] present", plainKeys)
	}
}

func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestWriteMergedPartialRequiresPersistPaths(t *testing.T) {
	err := WriteMergedPartial(Paths{}, map[string]interface{}{"theme": "default"})
	if err == nil || !strings.Contains(err.Error(), "persist") {
		t.Fatalf("WriteMergedPartial err = %v, want persistence path error", err)
	}
}

func TestWriteMergedPartialThemeCreatesMinimalTomlWhenMissing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "deep", "config")
	paths := Paths{ConfigDir: dir}
	if err := WriteMergedPartial(paths, map[string]interface{}{"theme": "mytheme"}); err != nil {
		t.Fatalf("WriteMergedPartial error = %v", err)
	}
	configPath := filepath.Join(dir, fileName)
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", configPath, err)
	}
	if strings.TrimSpace(string(raw)) != `theme = "mytheme"` {
		t.Fatalf("file contents:\n%s", string(raw))
	}
	cfg, err := LoadFromPaths(Paths{ConfigFile: configPath})
	if err != nil {
		t.Fatalf("LoadFromPaths(): %v", err)
	}
	if cfg.Theme != "mytheme" || cfg.ShowHidden != Default().ShowHidden {
		t.Fatalf("merged config %+v differs from expectation", cfg)
	}
}

func TestWriteMergedPartialMergesExistingFile(t *testing.T) {
	path := writeConfig(t, `[jobs]
keep_finished = 7
`)

	paths := Paths{ConfigFile: path}
	if err := WriteMergedPartial(paths, map[string]interface{}{"theme": "mytheme"}); err != nil {
		t.Fatalf("WriteMergedPartial: %v", err)
	}
	cfg, err := LoadFromPaths(paths)
	if err != nil {
		t.Fatalf("LoadFromPaths(): %v", err)
	}
	if cfg.Theme != "mytheme" {
		t.Fatalf("Theme = %q", cfg.Theme)
	}
	if cfg.Jobs.KeepFinished != 7 {
		t.Fatalf("Jobs.KeepFinished = %d, want merged 7", cfg.Jobs.KeepFinished)
	}
}

func TestPathsWithResolvedLocations(t *testing.T) {
	base := filepath.Join(t.TempDir(), "xcfg")
	got := Paths{ConfigDir: base}.WithResolvedLocations()
	wantFile := filepath.Join(base, fileName)
	if got.ConfigFile != wantFile {
		t.Fatalf("WithResolvedLocations ConfigFile = %q, want %q", got.ConfigFile, wantFile)
	}
	wantThemes := filepath.Join(base, "themes")
	if got.ThemesDir != wantThemes {
		t.Fatalf("ThemesDir = %q, want %q", got.ThemesDir, wantThemes)
	}
	customThemes := filepath.Join(base, "my-themes")
	gotCustom := Paths{ConfigDir: base, ThemesDir: customThemes}.WithResolvedLocations()
	if gotCustom.ThemesDir != customThemes {
		t.Fatalf("ThemesDir must preserve explicit value = %q, want %q", gotCustom.ThemesDir, customThemes)
	}
	dir := filepath.Join(base, "sub")
	got = Paths{ConfigFile: filepath.Join(dir, fileName)}.WithResolvedLocations()
	if got.ConfigDir != dir {
		t.Fatalf("ConfigDir = %q, want %q", got.ConfigDir, dir)
	}
	if got.ThemesDir != filepath.Join(dir, "themes") {
		t.Fatalf("ThemesDir = %q, want %q", got.ThemesDir, filepath.Join(dir, "themes"))
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
	return path
}
