package app

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// reloadedConfiguration is everything a configuration reload applies, loaded and validated
// before any of it touches the running app.
type reloadedConfiguration struct {
	config  config.Config
	keys    *keymap.Bundle
	styles  theme.Theme
	choices []theme.NamedTheme
}

// loadConfiguration reads and validates config.toml, keybindings.toml and the active theme
// (resolved from the new config) without changing the app. Unlike startup, a theme that does not
// resolve is an error rather than a fallback to the default theme.
func (a *App) loadConfiguration() (reloadedConfiguration, error) {
	cfg, err := config.LoadFromPaths(a.paths)
	if err != nil {
		return reloadedConfiguration{}, err
	}
	keys, err := resolveKeymapBundle(Options{Paths: a.paths})
	if err != nil {
		return reloadedConfiguration{}, err
	}
	styles, err := theme.Resolve(cfg.Theme, a.paths.ThemesDir)
	if err != nil {
		return reloadedConfiguration{}, err
	}
	choices, err := theme.ThemeChoices(a.paths.ThemesDir)
	if err != nil {
		return reloadedConfiguration{}, fmt.Errorf("load theme choices: %w", err)
	}
	return reloadedConfiguration{config: cfg, keys: keys, styles: styles, choices: choices}, nil
}

// reloadConfiguration re-reads config.toml, keybindings.toml and the themes, and applies them
// only when all of them load and validate.
func (a *App) reloadConfiguration() {
	next, err := a.loadConfiguration()
	if err != nil {
		a.setErrorMessage("Reload configuration", err)
		return
	}
	restart := a.applyLoadedConfig(next.config)
	a.keys.ReplaceFrom(next.keys)
	if a.model.ViewMode == ui.ViewBrowser {
		a.model.MenuDefinitions = a.browserMenuDefinitions()
	}
	a.themes, next.choices = themeRegistry(next.styles, next.choices)
	a.model.ThemeDialog.Choices = uiThemeChoices(next.choices)
	a.model.ThemeDialog.CurrentName = next.styles.Name
	a.setStyles(next.styles)
	a.setReloadedMessage("Configuration reloaded", restart)
}

// applyLoadedConfig makes cfg the running configuration and pushes it to everything that reads
// it: the model, the handlers' snapshots and the live appliers. Settings only read at startup
// keep their running values (see restartOnlySettings); their names are returned.
func (a *App) applyLoadedConfig(cfg config.Config) []string {
	restart := pinRestartOnlySettings(a.config, &cfg)
	a.config = cfg
	a.jobsCtrl.SetConfig(cfg)
	a.findCtrl.SetConfig(cfg)
	a.metaCtrl.SetConfig(cfg)
	a.compareCtrl.SetConfig(cfg)
	a.dedupCtrl.SetConfig(cfg)
	a.fileindexCtrl.SetConfig(cfg)
	a.applyLiveConfig()
	return restart
}

// applyLiveConfig applies a.config to the model and live subsystems. Session-only overrides of
// saved settings (zoom, pane split) are cleared so the loaded values show.
func (a *App) applyLiveConfig() {
	a.zoomActivePanelOverride = nil
	a.paneSplitOrientationOverride = nil
	applyConfigToModel(&a.model, a.config)
	a.syncScrollFromConfig()
	a.syncCarouselQuickViewOrdering()
	a.applyMouseConfig()
	a.applyDiskUsageConfig()
	a.setStyles(a.styles) // re-stamp the config-driven theme flags (icons, tooltips)
	a.ensurePanelsVisible()
}

// applyConfigToModel sets the model fields that mirror configuration values. Startup and every
// configuration reload use it.
func applyConfigToModel(m *ui.Model, cfg config.Config) {
	m.SelectedPanelMaxRows = cfg.UI.SelectedPanelMaxRows
	m.SelectedPanelActivePercent = cfg.UI.SelectedPanelActivePercent
	m.HideMenuBar = !cfg.UI.ShowMenuBar
	m.UseNerdfontIcons = cfg.UI.UseNerdfontIcons
	m.CarouselLayout = carouselLayoutFromConfig(cfg.Carousel)
	m.NarrowPanelsNameOnly = cfg.UI.NarrowPanelsNameOnly
	m.OtherPanelMatchMarks = cfg.UI.OtherPanelMatchMarks
	m.JobsFilterCaseInsensitive = cfg.Filter.CaseInsensitive
	m.TerminalPanel.Rows = cfg.Shell.TerminalPanelHeight
}

// setReloadedMessage toasts msg, as a warning naming the settings that need a restart when any
// changed.
func (a *App) setReloadedMessage(msg string, restart []string) {
	if len(restart) == 0 {
		a.setTransientMessage(msg, ui.MessageUrgencyInfo)
		return
	}
	a.setTransientMessage(msg+"; restart to apply: "+strings.Join(restart, ", "), ui.MessageUrgencyWarn)
}

// restartOnlySetting is a configuration value wired into long-lived machinery at startup (job
// engine, disk-usage engine, fileindex, work pools, watcher and tickers) that a reload cannot
// change.
type restartOnlySetting struct {
	name  string
	field func(*config.Config) any // pointer to the value inside the config
}

// restartOnlySettings lists them in the order a reload names them.
var restartOnlySettings = []restartOnlySetting{
	{"jobs", func(c *config.Config) any { return &c.Jobs }},
	{"operations", func(c *config.Config) any { return &c.Operations }},
	{"compare.read_buffer_kib", func(c *config.Config) any { return &c.Compare.ReadBufferKiB }},
	{"adaptive_io", func(c *config.Config) any { return &c.AdaptiveIO }},
	{"fileindex", func(c *config.Config) any { return &c.FileIndex }},
	{"pools", func(c *config.Config) any { return &c.Pools }},
	{"commands.force_color", func(c *config.Config) any { return &c.Commands.ForceColor }},
	{"panels.watch_directories", func(c *config.Config) any { return &c.Panels.WatchDirectories }},
	{"panels.refresh_interval_ms", func(c *config.Config) any { return &c.Panels.RefreshIntervalMS }},
	{"panels.respect_gitignore", func(c *config.Config) any { return &c.Panels.RespectGitignore }},
}

// pinRestartOnlySettings copies the running value of every restart-only setting that differs in
// loaded back into loaded, so the whole app keeps seeing the values its machinery runs with, and
// returns the names of those settings.
func pinRestartOnlySettings(running config.Config, loaded *config.Config) []string {
	var changed []string
	for _, s := range restartOnlySettings {
		cur := reflect.ValueOf(s.field(&running)).Elem()
		next := reflect.ValueOf(s.field(loaded)).Elem()
		if reflect.DeepEqual(cur.Interface(), next.Interface()) {
			continue
		}
		next.Set(cur)
		changed = append(changed, s.name)
	}
	return changed
}
