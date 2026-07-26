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

func TestDefaultDedupHashConfirmBytes(t *testing.T) {
	if got := Default().Dedup.HashConfirmBytes; got != DefaultDedupHashConfirmBytes {
		t.Fatalf("Dedup.HashConfirmBytes = %d, want %d", got, DefaultDedupHashConfirmBytes)
	}
}

func TestValidateClampsNegativeDedupHashConfirmBytes(t *testing.T) {
	cfg := Default()
	cfg.Dedup.HashConfirmBytes = -1
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.Dedup.HashConfirmBytes != 0 {
		t.Fatalf("Dedup.HashConfirmBytes = %d, want 0", cfg.Dedup.HashConfirmBytes)
	}
}

func TestDefaultDedupFileProgressBytes(t *testing.T) {
	if got := Default().Dedup.FileProgressBytes; got != DefaultDedupFileProgressBytes {
		t.Fatalf("Dedup.FileProgressBytes = %d, want %d", got, DefaultDedupFileProgressBytes)
	}
}

func TestValidateClampsNegativeDedupFileProgressBytes(t *testing.T) {
	cfg := Default()
	cfg.Dedup.FileProgressBytes = -1
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.Dedup.FileProgressBytes != 0 {
		t.Fatalf("Dedup.FileProgressBytes = %d, want 0", cfg.Dedup.FileProgressBytes)
	}
}

func TestDefaultDedupChunkBytes(t *testing.T) {
	if got := Default().Dedup.ChunkBytes; got != DefaultDedupChunkBytes {
		t.Fatalf("Dedup.ChunkBytes = %d, want %d", got, DefaultDedupChunkBytes)
	}
}

func TestValidateClampsNegativeDedupChunkBytes(t *testing.T) {
	cfg := Default()
	cfg.Dedup.ChunkBytes = -1
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.Dedup.ChunkBytes != 0 {
		t.Fatalf("Dedup.ChunkBytes = %d, want 0", cfg.Dedup.ChunkBytes)
	}
}

func TestValidateClampsDiskUsageWalkConcurrency(t *testing.T) {
	cfg := Default()
	cfg.DiskUsage.WalkConcurrency = 0
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.DiskUsage.WalkConcurrency != DefaultDiskUsageWalkConcurrency {
		t.Fatalf("DiskUsage.WalkConcurrency = %d, want %d", cfg.DiskUsage.WalkConcurrency, DefaultDiskUsageWalkConcurrency)
	}
	cfg.DiskUsage.WalkConcurrency = 2
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.DiskUsage.WalkConcurrency != 2 {
		t.Fatalf("DiskUsage.WalkConcurrency = %d, want 2", cfg.DiskUsage.WalkConcurrency)
	}
}

func TestValidateClampsShellTerminalPanelHeight(t *testing.T) {
	cfg := Default()
	if cfg.Shell.TerminalPanelHeight != DefaultShellTerminalPanelHeight {
		t.Fatalf("Default() Shell.TerminalPanelHeight = %d, want %d", cfg.Shell.TerminalPanelHeight, DefaultShellTerminalPanelHeight)
	}
	cfg.Shell.TerminalPanelHeight = 1
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.Shell.TerminalPanelHeight != DefaultShellTerminalPanelHeight {
		t.Fatalf("Shell.TerminalPanelHeight = %d, want reset to default %d", cfg.Shell.TerminalPanelHeight, DefaultShellTerminalPanelHeight)
	}
	cfg.Shell.TerminalPanelHeight = 5
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.Shell.TerminalPanelHeight != 5 {
		t.Fatalf("Shell.TerminalPanelHeight = %d, want 5 (unchanged, above minimum)", cfg.Shell.TerminalPanelHeight)
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
	if got := Default().UI.Zoom.ActivePercent; got != DefaultPanelZoomActivePercent {
		t.Fatalf("Zoom.ActivePercent = %d, want %d", got, DefaultPanelZoomActivePercent)
	}
	if got := Default().UI.Zoom.InactivePercent; got != DefaultPanelZoomInactivePercent {
		t.Fatalf("Zoom.InactivePercent = %d, want %d", got, DefaultPanelZoomInactivePercent)
	}
	if got := Default().UI.Zoom.DisabledAboveWidth; got != DefaultZoomActivePanelDisabledAboveWidth {
		t.Fatalf("Zoom.DisabledAboveWidth = %d, want %d", got, DefaultZoomActivePanelDisabledAboveWidth)
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
	cfg.UI.Zoom.ActivePercent = 60
	cfg.UI.Zoom.InactivePercent = 50
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.UI.Zoom.ActivePercent != DefaultPanelZoomActivePercent ||
		cfg.UI.Zoom.InactivePercent != DefaultPanelZoomInactivePercent {
		t.Fatalf("got %d/%d, want defaults %d/%d",
			cfg.UI.Zoom.ActivePercent, cfg.UI.Zoom.InactivePercent,
			DefaultPanelZoomActivePercent, DefaultPanelZoomInactivePercent)
	}
}

func TestDefaultCarouselSplit(t *testing.T) {
	got := Default().Carousel.Split
	want := DefaultCarouselSplit()
	if len(got) != len(want) {
		t.Fatalf("Carousel.Split len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Carousel.Split[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	show := Default().Carousel.ShowSize
	if len(show) != 3 || show[0] || !show[1] || !show[2] {
		t.Fatalf("Carousel.ShowSize = %v, want [false true true]", show)
	}
	if !Default().Carousel.AutohideInactivePanel {
		t.Fatal("Carousel.AutohideInactivePanel = false, want true")
	}
}

func TestValidateResetsInvalidCarouselSplit(t *testing.T) {
	cfg := Default()
	cfg.Carousel.Split = []string{"20%", "bad", "*"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	want := DefaultCarouselSplit()
	for i := range want {
		if cfg.Carousel.Split[i] != want[i] {
			t.Fatalf("Carousel.Split[%d] = %q, want %q after invalid token", i, cfg.Carousel.Split[i], want[i])
		}
	}

	cfg.Carousel.Split = []string{"20%", "30%", "*"}
	cfg.Carousel.ShowSize = []bool{true}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(cfg.Carousel.ShowSize) != 3 {
		t.Fatalf("Carousel.ShowSize len = %d, want 3", len(cfg.Carousel.ShowSize))
	}
}

func TestValidateAcceptsFitSplitTokensAtParentAndCenter(t *testing.T) {
	cfg := Default()
	cfg.Carousel.Split = []string{"<<16", "<33%", "*"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	want := []string{"<<16", "<33%", "*"}
	for i := range want {
		if cfg.Carousel.Split[i] != want[i] {
			t.Fatalf("Carousel.Split[%d] = %q, want %q (fit tokens should survive validation)", i, cfg.Carousel.Split[i], want[i])
		}
	}
}

func TestValidateResetsFitSplitTokenAtChildColumn(t *testing.T) {
	cfg := Default()
	cfg.Carousel.Split = []string{"*", "*", "<16"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	want := DefaultCarouselSplit()
	for i := range want {
		if cfg.Carousel.Split[i] != want[i] {
			t.Fatalf("Carousel.Split[%d] = %q, want %q after fit-mode token on child column", i, cfg.Carousel.Split[i], want[i])
		}
	}
}

func TestValidateResetsMalformedFitSplitTokens(t *testing.T) {
	for _, tok := range []string{"<", "<<", "<abc", "<<abc", "<0", "<-1", "<150%", "<%", "<<%", "<<<33%"} {
		cfg := Default()
		cfg.Carousel.Split = []string{tok, "*", "*"}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate(%q): %v", tok, err)
		}
		want := DefaultCarouselSplit()
		for i := range want {
			if cfg.Carousel.Split[i] != want[i] {
				t.Fatalf("token %q: Carousel.Split[%d] = %q, want %q", tok, i, cfg.Carousel.Split[i], want[i])
			}
		}
	}
}

func TestValidateResetsInvalidPanelScrollbar(t *testing.T) {
	cfg := Default()
	cfg.UI.Scroll.Scrollbar = "invalid"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.UI.Scroll.Scrollbar != DefaultPanelScrollbar {
		t.Fatalf("Scroll.Scrollbar = %q, want %q", cfg.UI.Scroll.Scrollbar, DefaultPanelScrollbar)
	}
}

func TestValidateClampsNegativeZoomActivePanelDisabledAboveWidth(t *testing.T) {
	cfg := Default()
	cfg.UI.Zoom.DisabledAboveWidth = -10
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.UI.Zoom.DisabledAboveWidth != DefaultZoomActivePanelDisabledAboveWidth {
		t.Fatalf("Zoom.DisabledAboveWidth = %d, want %d",
			cfg.UI.Zoom.DisabledAboveWidth, DefaultZoomActivePanelDisabledAboveWidth)
	}
	cfg.UI.Zoom.DisabledAboveWidth = 0
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.UI.Zoom.DisabledAboveWidth != 0 {
		t.Fatalf("Zoom.DisabledAboveWidth = %d, want 0", cfg.UI.Zoom.DisabledAboveWidth)
	}
}

func TestValidateClampsRefreshIntervalMS(t *testing.T) {
	cfg := Default()
	cfg.Panels.RefreshIntervalMS = -1
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.Panels.RefreshIntervalMS != DefaultRefreshIntervalMS {
		t.Fatalf("negative = %d, want default %d", cfg.Panels.RefreshIntervalMS, DefaultRefreshIntervalMS)
	}
	cfg.Panels.RefreshIntervalMS = 0
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.Panels.RefreshIntervalMS != 0 {
		t.Fatalf("zero = %d, want 0 (disabled)", cfg.Panels.RefreshIntervalMS)
	}
	cfg.Panels.RefreshIntervalMS = 100
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.Panels.RefreshIntervalMS != 200 {
		t.Fatalf("100 = %d, want clamp 200", cfg.Panels.RefreshIntervalMS)
	}
	cfg.Panels.RefreshIntervalMS = 120_000
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.Panels.RefreshIntervalMS != 60_000 {
		t.Fatalf("120000 = %d, want clamp 60000", cfg.Panels.RefreshIntervalMS)
	}
}

func TestValidateClampsScrollMode(t *testing.T) {
	cfg := Default()
	cfg.UI.Scroll.Mode = "bogus"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.UI.Scroll.Mode != ScrollModeEdge {
		t.Fatalf("Scroll.Mode = %q, want %q", cfg.UI.Scroll.Mode, ScrollModeEdge)
	}
	cfg.UI.Scroll.Mode = ScrollModeEdge
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.UI.Scroll.Mode != ScrollModeEdge {
		t.Fatalf("Scroll.Mode = %q, want %q", cfg.UI.Scroll.Mode, ScrollModeEdge)
	}
}

func TestValidateClampsScrollEdgeMargin(t *testing.T) {
	cfg := Default()
	cfg.UI.Scroll.EdgeMargin = -3
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.UI.Scroll.EdgeMargin != DefaultScrollEdgeMargin {
		t.Fatalf("Scroll.EdgeMargin = %d, want %d", cfg.UI.Scroll.EdgeMargin, DefaultScrollEdgeMargin)
	}
	cfg.UI.Scroll.EdgeMargin = 999
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if cfg.UI.Scroll.EdgeMargin != ScrollEdgeMarginMax {
		t.Fatalf("Scroll.EdgeMargin = %d, want %d", cfg.UI.Scroll.EdgeMargin, ScrollEdgeMarginMax)
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
[panels]
show_hidden = true
directories_first = false

[jobs]
keep_finished = 5

[operations]
confirm_delete = false
copy_buffer_kib = 512

[logging]
level = "debug"
`)

	cfg, err := LoadFromPaths(Paths{ConfigFile: path})
	if err != nil {
		t.Fatalf("LoadFromPaths() error = %v", err)
	}

	if !cfg.Panels.ShowHidden {
		t.Fatal("Panels.ShowHidden = false, want true")
	}
	if cfg.Operations.ConfirmDelete {
		t.Fatal("Operations.ConfirmDelete = true, want false")
	}
	if cfg.Panels.DirectoriesFirst {
		t.Fatal("Panels.DirectoriesFirst = true, want false")
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
}

func TestLoadFromPathsMergesStatusMessageTTL(t *testing.T) {
	path := writeConfig(t, `
[ui.status]
message_ttl_seconds = 12.25
`)
	cfg, err := LoadFromPaths(Paths{ConfigFile: path})
	if err != nil {
		t.Fatalf("LoadFromPaths() error = %v", err)
	}
	if cfg.UI.Status.MessageTTLSeconds != 12.25 {
		t.Fatalf("Status.MessageTTLSeconds = %v, want 12.25", cfg.UI.Status.MessageTTLSeconds)
	}
}

func TestLoadFromPathsReportsInvalidTOML(t *testing.T) {
	path := writeConfig(t, `theme =`)

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
			content: "[panels]\ndefault_sort = \"unknown_mode\"\n",
			testFn: func(t *testing.T, cfg Config) {
				if cfg.Panels.DefaultSort != SortName {
					t.Fatalf("Panels.DefaultSort = %q, want clamped to %q", cfg.Panels.DefaultSort, SortName)
				}
			},
		},
		{
			name:    "default_sort disk_usage migrates",
			content: "[panels]\ndefault_sort = \"disk_usage\"\n",
			testFn: func(t *testing.T, cfg Config) {
				if cfg.Panels.DefaultSort != SortName {
					t.Fatalf("Panels.DefaultSort = %q, want migrated to %q", cfg.Panels.DefaultSort, SortName)
				}
				if !cfg.DiskUsage.IdleSizeSort {
					t.Fatal("DiskUsage.IdleSizeSort = false, want true after disk_usage migration")
				}
			},
		},
		{
			name:    "default_sort disk-usage migrates",
			content: "[panels]\ndefault_sort = \"disk-usage\"\n",
			testFn: func(t *testing.T, cfg Config) {
				if cfg.Panels.DefaultSort != SortName {
					t.Fatalf("Panels.DefaultSort = %q, want migrated to %q", cfg.Panels.DefaultSort, SortName)
				}
				if !cfg.DiskUsage.IdleSizeSort {
					t.Fatal("DiskUsage.IdleSizeSort = false, want true after disk-usage migration")
				}
			},
		},
		{
			name:    "listing format invalid",
			content: "[panels]\ndefault_listing_format = \"wide\"\n",
			testFn: func(t *testing.T, cfg Config) {
				if cfg.Panels.DefaultListingFormat != ListingFormatBrief {
					t.Fatalf("Panels.DefaultListingFormat = %q, want clamped to %q", cfg.Panels.DefaultListingFormat, ListingFormatBrief)
				}
			},
		},
		{
			name:    "flatten default location invalid",
			content: "[operations]\nflatten_default_location = \"other\"\n",
			testFn: func(t *testing.T, cfg Config) {
				if cfg.Operations.FlattenDefaultLocation != FlattenDefaultLocationActive {
					t.Fatalf("FlattenDefaultLocation = %q, want clamped to %q", cfg.Operations.FlattenDefaultLocation, FlattenDefaultLocationActive)
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
			content: "[ui.status]\nmessage_ttl_seconds = -1\n",
			testFn: func(t *testing.T, cfg Config) {
				want := Default().UI.Status.MessageTTLSeconds
				if cfg.UI.Status.MessageTTLSeconds != want {
					t.Fatalf("Status.MessageTTLSeconds = %v, want clamped to default %v", cfg.UI.Status.MessageTTLSeconds, want)
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
	for _, want := range []string{"theme = \"default\"", "[panels]", "show_hidden = false", "[disk_usage]", "[carousel]", "[ui]", "[ui.status]", "message_ttl_seconds", "log_max_entries", "[filter]", "[jobs]", "[operations]", "[logging]"} {
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
	if cfg.Theme != "mytheme" || cfg.Panels.ShowHidden != Default().Panels.ShowHidden {
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
