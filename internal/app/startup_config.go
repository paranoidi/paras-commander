package app

import (
	"fmt"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/theme"
)

type startupConfig struct {
	Config       config.Config
	Theme        theme.Theme
	ThemeChoices []theme.NamedTheme
	Keymap       *keymap.Bundle
}

func loadUserStartupConfig(paths config.Paths) (startupConfig, error) {
	paths = paths.WithResolvedLocations()

	conf, err := config.LoadFromPaths(paths)
	if err != nil {
		return startupConfig{}, err
	}
	styles, _ := theme.Resolve(conf.Theme, paths.ThemesDir)
	themeChoices, err := theme.ThemeChoices(paths.ThemesDir)
	if err != nil {
		return startupConfig{}, err
	}
	bundle, err := keymap.LoadFromPaths(paths)
	if err != nil {
		return startupConfig{}, err
	}
	return startupConfig{
		Config:       conf,
		Theme:        styles,
		ThemeChoices: themeChoices,
		Keymap:       bundle,
	}, nil
}

func builtInStartupConfig() (startupConfig, error) {
	conf := config.Default()
	if err := conf.Validate(); err != nil {
		return startupConfig{}, fmt.Errorf("validate built-in config: %w", err)
	}
	styles := theme.Default()
	themeChoices, err := theme.BuiltInThemes()
	if err != nil {
		return startupConfig{}, fmt.Errorf("load built-in themes: %w", err)
	}
	bundle, err := keymap.DefaultBundle()
	if err != nil {
		return startupConfig{}, fmt.Errorf("load built-in keybindings: %w", err)
	}
	return startupConfig{
		Config:       conf,
		Theme:        styles,
		ThemeChoices: themeChoices,
		Keymap:       bundle,
	}, nil
}

func resolveStartupConfig(paths config.Paths) (startupConfig, bool, error) {
	startup, loadErr := loadUserStartupConfig(paths)
	if loadErr == nil {
		return startup, false, nil
	}
	yes, err := startupDefaultsPrompt(startupPromptIn, startupPromptOut, loadErr)
	if err != nil {
		return startupConfig{}, false, err
	}
	if !yes {
		return startupConfig{}, false, loadErr
	}
	startup, err = builtInStartupConfig()
	if err != nil {
		return startupConfig{}, false, err
	}
	return startup, true, nil
}
