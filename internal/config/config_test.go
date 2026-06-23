package config

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/preview/chromastyles"
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

func TestValidateClampsJobsWorkerProgress(t *testing.T) {
	cfg := Default()
	cfg.Jobs.WorkerProgressMinBytes = 100
	cfg.Jobs.WorkerProgressMinIntervalMS = 10
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.Jobs.WorkerProgressMinBytes != 64*1024 {
		t.Fatalf("WorkerProgressMinBytes = %d, want %d", cfg.Jobs.WorkerProgressMinBytes, 64*1024)
	}
	if cfg.Jobs.WorkerProgressMinIntervalMS != 50 {
		t.Fatalf("WorkerProgressMinIntervalMS = %d, want 50", cfg.Jobs.WorkerProgressMinIntervalMS)
	}
	cfg.Jobs.WorkerProgressMinBytes = 128 * 1024 * 1024
	cfg.Jobs.WorkerProgressMinIntervalMS = 999999
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.Jobs.WorkerProgressMinBytes != 64*1024*1024 {
		t.Fatalf("WorkerProgressMinBytes = %d, want max 64MiB", cfg.Jobs.WorkerProgressMinBytes)
	}
	if cfg.Jobs.WorkerProgressMinIntervalMS != 5000 {
		t.Fatalf("WorkerProgressMinIntervalMS = %d, want 5000", cfg.Jobs.WorkerProgressMinIntervalMS)
	}
}

func TestDefaultPathPickerValidateDelayMS(t *testing.T) {
	if got := Default().UI.PathPickerValidateDelayMS; got != DefaultPathPickerValidateDelayMS {
		t.Fatalf("PathPickerValidateDelayMS = %d, want %d", got, DefaultPathPickerValidateDelayMS)
	}
	if got := Default().UI.PanelZoomActivePercent; got != DefaultPanelZoomActivePercent {
		t.Fatalf("PanelZoomActivePercent = %d, want %d", got, DefaultPanelZoomActivePercent)
	}
	if got := Default().UI.PanelZoomInactivePercent; got != DefaultPanelZoomInactivePercent {
		t.Fatalf("PanelZoomInactivePercent = %d, want %d", got, DefaultPanelZoomInactivePercent)
	}
	if got := Default().UI.ZoomActivePanelDisabledAboveWidth; got != DefaultZoomActivePanelDisabledAboveWidth {
		t.Fatalf("ZoomActivePanelDisabledAboveWidth = %d, want %d", got, DefaultZoomActivePanelDisabledAboveWidth)
	}
	if got := Default().UI.ShrunkenShowsNameOnly; got != DefaultShrunkenShowsNameOnly {
		t.Fatalf("ShrunkenShowsNameOnly = %v, want %v", got, DefaultShrunkenShowsNameOnly)
	}
	if got := Default().UI.ScreenRenderHashCache; got != DefaultScreenRenderHashCache {
		t.Fatalf("ScreenRenderHashCache = %v, want %v", got, DefaultScreenRenderHashCache)
	}
}

func TestValidateResetsInvalidPanelZoomPercents(t *testing.T) {
	cfg := Default()
	cfg.UI.PanelZoomActivePercent = 60
	cfg.UI.PanelZoomInactivePercent = 50
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.UI.PanelZoomActivePercent != DefaultPanelZoomActivePercent ||
		cfg.UI.PanelZoomInactivePercent != DefaultPanelZoomInactivePercent {
		t.Fatalf("got %d/%d, want defaults %d/%d",
			cfg.UI.PanelZoomActivePercent, cfg.UI.PanelZoomInactivePercent,
			DefaultPanelZoomActivePercent, DefaultPanelZoomInactivePercent)
	}
}

func TestDefaultCarouselSplit(t *testing.T) {
	got := Default().UI.CarouselSplit
	want := DefaultCarouselSplit()
	if len(got) != len(want) {
		t.Fatalf("CarouselSplit len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("CarouselSplit[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	show := Default().UI.CarouselShowSize
	if len(show) != 3 || !show[0] || !show[1] || !show[2] {
		t.Fatalf("CarouselShowSize = %v, want [true true true]", show)
	}
}

func TestValidateResetsInvalidCarouselSplit(t *testing.T) {
	cfg := Default()
	cfg.UI.CarouselSplit = []string{"20%", "bad", "*"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	want := DefaultCarouselSplit()
	for i := range want {
		if cfg.UI.CarouselSplit[i] != want[i] {
			t.Fatalf("CarouselSplit[%d] = %q, want %q after invalid token", i, cfg.UI.CarouselSplit[i], want[i])
		}
	}

	cfg.UI.CarouselSplit = []string{"20%", "30%", "*"}
	cfg.UI.CarouselShowSize = []bool{true}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(cfg.UI.CarouselShowSize) != 3 {
		t.Fatalf("CarouselShowSize len = %d, want 3", len(cfg.UI.CarouselShowSize))
	}
}

func TestValidateResetsInvalidPanelScrollbar(t *testing.T) {
	cfg := Default()
	cfg.UI.PanelScrollbar = "invalid"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.UI.PanelScrollbar != DefaultPanelScrollbar {
		t.Fatalf("PanelScrollbar = %q, want %q", cfg.UI.PanelScrollbar, DefaultPanelScrollbar)
	}
}

func TestValidateClampsNegativeZoomActivePanelDisabledAboveWidth(t *testing.T) {
	cfg := Default()
	cfg.UI.ZoomActivePanelDisabledAboveWidth = -10
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.UI.ZoomActivePanelDisabledAboveWidth != DefaultZoomActivePanelDisabledAboveWidth {
		t.Fatalf("ZoomActivePanelDisabledAboveWidth = %d, want %d",
			cfg.UI.ZoomActivePanelDisabledAboveWidth, DefaultZoomActivePanelDisabledAboveWidth)
	}
	cfg.UI.ZoomActivePanelDisabledAboveWidth = 0
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.UI.ZoomActivePanelDisabledAboveWidth != 0 {
		t.Fatalf("ZoomActivePanelDisabledAboveWidth = %d, want 0", cfg.UI.ZoomActivePanelDisabledAboveWidth)
	}
}

func TestValidateClampsRefreshIntervalMS(t *testing.T) {
	cfg := Default()
	cfg.RefreshIntervalMS = -1
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.RefreshIntervalMS != DefaultRefreshIntervalMS {
		t.Fatalf("negative = %d, want default %d", cfg.RefreshIntervalMS, DefaultRefreshIntervalMS)
	}
	cfg.RefreshIntervalMS = 0
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.RefreshIntervalMS != 0 {
		t.Fatalf("zero = %d, want 0 (disabled)", cfg.RefreshIntervalMS)
	}
	cfg.RefreshIntervalMS = 100
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.RefreshIntervalMS != 200 {
		t.Fatalf("100 = %d, want clamp 200", cfg.RefreshIntervalMS)
	}
	cfg.RefreshIntervalMS = 120_000
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.RefreshIntervalMS != 60_000 {
		t.Fatalf("120000 = %d, want clamp 60000", cfg.RefreshIntervalMS)
	}
}

func TestValidateClampsScrollMode(t *testing.T) {
	cfg := Default()
	cfg.UI.ScrollMode = "bogus"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.UI.ScrollMode != ScrollModeEdge {
		t.Fatalf("ScrollMode = %q, want %q", cfg.UI.ScrollMode, ScrollModeEdge)
	}
	cfg.UI.ScrollMode = ScrollModeEdge
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.UI.ScrollMode != ScrollModeEdge {
		t.Fatalf("ScrollMode = %q, want %q", cfg.UI.ScrollMode, ScrollModeEdge)
	}
}

func TestValidateClampsScrollEdgeMargin(t *testing.T) {
	cfg := Default()
	cfg.UI.ScrollEdgeMargin = -3
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.UI.ScrollEdgeMargin != DefaultScrollEdgeMargin {
		t.Fatalf("ScrollEdgeMargin = %d, want %d", cfg.UI.ScrollEdgeMargin, DefaultScrollEdgeMargin)
	}
	cfg.UI.ScrollEdgeMargin = 999
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.UI.ScrollEdgeMargin != ScrollEdgeMarginMax {
		t.Fatalf("ScrollEdgeMargin = %d, want %d", cfg.UI.ScrollEdgeMargin, ScrollEdgeMarginMax)
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
			name:    "flatten default location invalid",
			content: "[operations]\nflatten_default_location = \"other\"\n",
			testFn: func(t *testing.T, cfg Config) {
				if cfg.Operations.FlattenDefaultLocation != FlattenDefaultLocationInactive {
					t.Fatalf("FlattenDefaultLocation = %q, want clamped to %q", cfg.Operations.FlattenDefaultLocation, FlattenDefaultLocationInactive)
				}
			},
		},
		{
			name:    "flatten default location active",
			content: "[operations]\nflatten_default_location = \"active\"\n",
			testFn: func(t *testing.T, cfg Config) {
				if cfg.Operations.FlattenDefaultLocation != FlattenDefaultLocationActive {
					t.Fatalf("FlattenDefaultLocation = %q, want %q", cfg.Operations.FlattenDefaultLocation, FlattenDefaultLocationActive)
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
	for _, want := range []string{"theme = \"default\"", "show_hidden = false", "[ui]", "status_message_ttl_seconds", "message_log_max_entries", "[filter]", "[jobs]", "[operations]", "[logging]"} {
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

func TestLoadFromPathsAcceptsJobsSettingsTable(t *testing.T) {
	path := writeConfig(t, `theme = "default"
[jobs]
keep_finished = 25
`)
	cfg, err := LoadFromPaths(Paths{ConfigFile: path})
	if err != nil {
		t.Fatalf("LoadFromPaths() error = %v, want success with [jobs] settings", err)
	}
	if cfg.Theme != "default" {
		t.Fatalf("Theme = %q, want default", cfg.Theme)
	}
	if cfg.Jobs.KeepFinished != 25 {
		t.Fatalf("Jobs.KeepFinished = %d, want 25", cfg.Jobs.KeepFinished)
	}
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
	if got.PreviewStylesDir != filepath.Join(dir, "themes", "preview") {
		t.Fatalf("PreviewStylesDir = %q, want %q", got.PreviewStylesDir, filepath.Join(dir, "themes", "preview"))
	}
	customPreview := filepath.Join(base, "my-preview-styles")
	gotPreview := Paths{ConfigDir: base, PreviewStylesDir: customPreview}.WithResolvedLocations()
	if gotPreview.PreviewStylesDir != customPreview {
		t.Fatalf("PreviewStylesDir must preserve explicit value = %q, want %q", gotPreview.PreviewStylesDir, customPreview)
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

func TestValidatePreviewModeAndStyle(t *testing.T) {
	cfg := Default()
	cfg.Preview.Mode = "bogus"
	cfg.Preview.Style = "not-a-real-style"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.Preview.Mode != PreviewModeInternal {
		t.Fatalf("Mode = %q, want %q", cfg.Preview.Mode, PreviewModeInternal)
	}
	if cfg.Preview.Style != DefaultPreviewStyle {
		t.Fatalf("Style = %q, want %q", cfg.Preview.Style, DefaultPreviewStyle)
	}
	cfg.Preview.Mode = PreviewModeExternal
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate external: %v", err)
	}
	if cfg.Preview.Mode != PreviewModeExternal {
		t.Fatalf("Mode = %q, want external", cfg.Preview.Mode)
	}
}

func TestLoadFromPathsValidatesCustomPreviewStyle(t *testing.T) {
	t.Cleanup(chromastyles.ResetForTest)
	dir := t.TempDir()
	previewDir := filepath.Join(dir, "themes", "preview")
	if err := os.MkdirAll(previewDir, 0o755); err != nil {
		t.Fatalf("mkdir preview styles: %v", err)
	}
	xml := `<style name="cedar-glow-preview"><entry type="Keyword" style="#aabbcc"/></style>`
	if err := os.WriteFile(filepath.Join(previewDir, "cedar-glow-preview.xml"), []byte(xml), 0o644); err != nil {
		t.Fatalf("write preview style: %v", err)
	}
	configPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(configPath, []byte("[preview]\nstyle = \"cedar-glow-preview\"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := LoadFromPaths(Paths{
		ConfigFile:       configPath,
		ConfigDir:        dir,
		PreviewStylesDir: previewDir,
	})
	if err != nil {
		t.Fatalf("LoadFromPaths: %v", err)
	}
	if cfg.Preview.Style != "cedar-glow-preview" {
		t.Fatalf("Preview.Style = %q, want cedar-glow-preview", cfg.Preview.Style)
	}
}

func TestValidateKeyRepeatDebounceMS(t *testing.T) {
	cfg := Default()
	cfg.UI.KeyRepeatDebounceMS = -1
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.UI.KeyRepeatDebounceMS != DefaultKeyRepeatDebounceMS {
		t.Fatalf("KeyRepeatDebounceMS = %d, want default %d", cfg.UI.KeyRepeatDebounceMS, DefaultKeyRepeatDebounceMS)
	}
	cfg.UI.KeyRepeatDebounceMS = 20_000
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.UI.KeyRepeatDebounceMS != KeyRepeatDebounceMaxMS {
		t.Fatalf("KeyRepeatDebounceMS = %d, want clamp %d", cfg.UI.KeyRepeatDebounceMS, KeyRepeatDebounceMaxMS)
	}
}
