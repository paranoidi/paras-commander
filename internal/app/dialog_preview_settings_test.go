package app

import (
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// TestSetKittyCapabilityClearsPlaceholderYes covers setting Kitty to anything other than
// "yes" also dropping a "yes" placeholder, since placeholder display requires Kitty protocol.
func TestSetKittyCapabilityClearsPlaceholderYes(t *testing.T) {
	st := &dialog.PreviewSettingsDialogState{
		Kitty:            config.PreviewTerminalCapabilityYes,
		KittyPlaceholder: config.PreviewTerminalCapabilityYes,
	}
	setKittyCapability(st, config.PreviewTerminalCapabilityNo)
	if st.Kitty != config.PreviewTerminalCapabilityNo {
		t.Fatalf("Kitty = %q, want %q", st.Kitty, config.PreviewTerminalCapabilityNo)
	}
	if st.KittyPlaceholder != config.PreviewTerminalCapabilityNo {
		t.Fatalf("KittyPlaceholder = %q, want %q when Kitty is no", st.KittyPlaceholder, config.PreviewTerminalCapabilityNo)
	}

	setKittyCapability(st, config.PreviewTerminalCapabilityYes)
	if st.Kitty != config.PreviewTerminalCapabilityYes {
		t.Fatalf("Kitty = %q, want %q", st.Kitty, config.PreviewTerminalCapabilityYes)
	}
	if st.KittyPlaceholder != config.PreviewTerminalCapabilityNo {
		t.Fatalf("KittyPlaceholder = %q, want to stay %q", st.KittyPlaceholder, config.PreviewTerminalCapabilityNo)
	}
}

// TestSetKittyPlaceholderCapabilityImpliesKittyYes covers setting placeholder to "yes"
// also forcing Kitty to "yes", since placeholder is a Kitty-only display mode.
func TestSetKittyPlaceholderCapabilityImpliesKittyYes(t *testing.T) {
	st := &dialog.PreviewSettingsDialogState{
		Kitty:            config.PreviewTerminalCapabilityAuto,
		KittyPlaceholder: config.PreviewTerminalCapabilityAuto,
	}
	setKittyPlaceholderCapability(st, config.PreviewTerminalCapabilityYes)
	if st.KittyPlaceholder != config.PreviewTerminalCapabilityYes {
		t.Fatalf("KittyPlaceholder = %q, want %q", st.KittyPlaceholder, config.PreviewTerminalCapabilityYes)
	}
	if st.Kitty != config.PreviewTerminalCapabilityYes {
		t.Fatalf("Kitty = %q, want %q when placeholder is yes", st.Kitty, config.PreviewTerminalCapabilityYes)
	}

	setKittyPlaceholderCapability(st, config.PreviewTerminalCapabilityNo)
	if st.KittyPlaceholder != config.PreviewTerminalCapabilityNo {
		t.Fatalf("KittyPlaceholder = %q, want %q", st.KittyPlaceholder, config.PreviewTerminalCapabilityNo)
	}
	if st.Kitty != config.PreviewTerminalCapabilityYes {
		t.Fatalf("Kitty = %q, want to stay %q", st.Kitty, config.PreviewTerminalCapabilityYes)
	}
}

func TestOptionsMenuOpensPreviewSettingsDialog(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	app := newTestApp(t, screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: config.Default(),
		Theme:  theme.Default(),
	})

	app.dispatch(keymap.ActionAppOpenMenu)
	app.moveMenu(3) // File → Command → Display → Options
	app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'p', tcell.ModNone))

	if quit {
		t.Fatal("handleKey() quit = true, want false")
	}
	if app.model.Menu.Open {
		t.Fatal("menu open = true, want closed")
	}
	if !app.model.PreviewSettingsDialog.Open {
		t.Fatal("preview settings dialog open = false, want true")
	}
}

func TestPreviewSettingsOpenAndSavePreservesCapabilityTriState(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	fields := []struct {
		name string
		set  func(*config.PreviewConfig, string)
		get  func(config.PreviewConfig) string
	}{
		{
			name: "terminal_sixel",
			set:  func(p *config.PreviewConfig, v string) { p.TerminalSixel = v },
			get:  func(p config.PreviewConfig) string { return p.TerminalSixel },
		},
		{
			name: "terminal_kitty",
			set:  func(p *config.PreviewConfig, v string) { p.TerminalKitty = v },
			get:  func(p config.PreviewConfig) string { return p.TerminalKitty },
		},
		{
			name: "terminal_kitty_placeholder",
			set:  func(p *config.PreviewConfig, v string) { p.TerminalKittyPlaceholder = v },
			get:  func(p config.PreviewConfig) string { return p.TerminalKittyPlaceholder },
		},
	}
	values := []string{
		config.PreviewTerminalCapabilityAuto,
		config.PreviewTerminalCapabilityYes,
		config.PreviewTerminalCapabilityNo,
	}

	for _, field := range fields {
		for _, value := range values {
			t.Run(field.name+"/"+value, func(t *testing.T) {
				screen := tcell.NewSimulationScreen("UTF-8")
				if err := screen.Init(); err != nil {
					t.Fatalf("Init() error = %v", err)
				}
				defer screen.Fini()
				screen.SetSize(80, 20)

				cfg := config.Default()
				field.set(&cfg.Preview, value)
				appPaths := config.Paths{ConfigDir: filepath.Join(t.TempDir(), "preview-tristate")}.WithResolvedLocations()
				app := newTestApp(t, screen, Options{
					CWD: func() (string, error) {
						return dir, nil
					},
					Config: cfg,
					Paths:  appPaths,
					Theme:  theme.Default(),
				})

				app.openPreviewSettingsDialog()
				quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModAlt))
				if quit {
					t.Fatal("handleKey() quit = true, want false")
				}
				if app.model.PreviewSettingsDialog.Open {
					t.Fatal("preview settings dialog should close after apply")
				}
				if got := field.get(app.config.Preview); got != value {
					t.Fatalf("in-memory %s = %q, want %q", field.name, got, value)
				}
				reloaded, err := config.LoadFromPaths(appPaths)
				if err != nil {
					t.Fatalf("LoadFromPaths after persist: %v", err)
				}
				if got := field.get(reloaded.Preview); got != value {
					t.Fatalf("persisted %s = %q, want %q", field.name, got, value)
				}
			})
		}
	}
}

func TestPreviewSettingsDialogCanPersistCapabilityNo(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	fields := []struct {
		name  string
		cycle rune
		get   func(config.PreviewConfig) string
	}{
		{
			name:  "terminal_sixel",
			cycle: 's',
			get:   func(p config.PreviewConfig) string { return p.TerminalSixel },
		},
		{
			name:  "terminal_kitty",
			cycle: 'k',
			get:   func(p config.PreviewConfig) string { return p.TerminalKitty },
		},
		{
			name:  "terminal_kitty_placeholder",
			cycle: 'p',
			get:   func(p config.PreviewConfig) string { return p.TerminalKittyPlaceholder },
		},
	}

	for _, field := range fields {
		t.Run(field.name, func(t *testing.T) {
			screen := tcell.NewSimulationScreen("UTF-8")
			if err := screen.Init(); err != nil {
				t.Fatalf("Init() error = %v", err)
			}
			defer screen.Fini()
			screen.SetSize(80, 20)

			appPaths := config.Paths{ConfigDir: filepath.Join(t.TempDir(), "preview-set-no")}.WithResolvedLocations()
			app := newTestApp(t, screen, Options{
				CWD: func() (string, error) {
					return dir, nil
				},
				Config: config.Default(),
				Paths:  appPaths,
				Theme:  theme.Default(),
			})

			app.openPreviewSettingsDialog()
			app.handlePreviewSettingsDialogKey(tcell.NewEventKey(tcell.KeyRune, field.cycle, tcell.ModNone))
			app.handlePreviewSettingsDialogKey(tcell.NewEventKey(tcell.KeyRune, field.cycle, tcell.ModNone))
			quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModAlt))
			if quit {
				t.Fatal("handleKey() quit = true, want false")
			}
			if app.model.PreviewSettingsDialog.Open {
				t.Fatal("preview settings dialog should close after apply")
			}
			if got := field.get(app.config.Preview); got != config.PreviewTerminalCapabilityNo {
				t.Fatalf("in-memory %s = %q, want %q", field.name, got, config.PreviewTerminalCapabilityNo)
			}
			reloaded, err := config.LoadFromPaths(appPaths)
			if err != nil {
				t.Fatalf("LoadFromPaths after persist: %v", err)
			}
			if got := field.get(reloaded.Preview); got != config.PreviewTerminalCapabilityNo {
				t.Fatalf("persisted %s = %q, want %q", field.name, got, config.PreviewTerminalCapabilityNo)
			}
		})
	}
}

func TestPreviewSettingsDialogApplyPersists(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"))

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	defer screen.Fini()
	screen.SetSize(80, 20)

	appPaths := config.Paths{ConfigDir: filepath.Join(t.TempDir(), "persist-preview-settings")}.WithResolvedLocations()
	app := newTestApp(t, screen, Options{
		CWD: func() (string, error) {
			return dir, nil
		},
		Config: config.Default(),
		Paths:  appPaths,
		Theme:  theme.Default(),
	})

	app.openPreviewSettingsDialog()
	app.handlePreviewSettingsDialogKey(tcell.NewEventKey(tcell.KeyRune, 'k', tcell.ModNone))
	app.handlePreviewSettingsDialogKey(tcell.NewEventKey(tcell.KeyRune, 'p', tcell.ModNone))
	app.handlePreviewSettingsDialogKey(tcell.NewEventKey(tcell.KeyRune, 't', tcell.ModNone))
	app.handlePreviewSettingsDialogKey(tcell.NewEventKey(tcell.KeyRune, 'u', tcell.ModNone))
	app.handlePreviewSettingsDialogKey(tcell.NewEventKey(tcell.KeyRune, 'v', tcell.ModNone))
	quit, _ := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModAlt))

	if quit {
		t.Fatal("handleKey() quit = true, want false")
	}
	if app.model.PreviewSettingsDialog.Open {
		t.Fatal("preview settings dialog should close after apply")
	}
	if app.config.Preview.TerminalKitty != config.PreviewTerminalCapabilityYes {
		t.Fatalf("TerminalKitty = %q, want %q", app.config.Preview.TerminalKitty, config.PreviewTerminalCapabilityYes)
	}
	if app.config.Preview.TerminalKittyPlaceholder != config.PreviewTerminalCapabilityYes {
		t.Fatalf("TerminalKittyPlaceholder = %q, want %q", app.config.Preview.TerminalKittyPlaceholder, config.PreviewTerminalCapabilityYes)
	}
	if app.config.Preview.ImageProtocol != config.PreviewImageProtocolKitty {
		t.Fatalf("ImageProtocol = %q, want %q", app.config.Preview.ImageProtocol, config.PreviewImageProtocolKitty)
	}
	if app.config.Preview.ImageMetadata != config.PreviewImageMetadataFull {
		t.Fatalf("ImageMetadata = %q, want %q", app.config.Preview.ImageMetadata, config.PreviewImageMetadataFull)
	}
	if app.config.Preview.VideoMetadata != !config.DefaultPreviewVideoMetadata {
		t.Fatalf("VideoMetadata = %v, want %v (toggled from default)", app.config.Preview.VideoMetadata, !config.DefaultPreviewVideoMetadata)
	}

	reloaded, err := config.LoadFromPaths(appPaths)
	if err != nil {
		t.Fatalf("LoadFromPaths after persist: %v", err)
	}
	if reloaded.Preview.TerminalKitty != config.PreviewTerminalCapabilityYes {
		t.Fatalf("persisted terminal_kitty = %q, want %q", reloaded.Preview.TerminalKitty, config.PreviewTerminalCapabilityYes)
	}
	if reloaded.Preview.TerminalKittyPlaceholder != config.PreviewTerminalCapabilityYes {
		t.Fatalf("persisted terminal_kitty_placeholder = %q, want %q", reloaded.Preview.TerminalKittyPlaceholder, config.PreviewTerminalCapabilityYes)
	}
	if reloaded.Preview.ImageProtocol != config.PreviewImageProtocolKitty {
		t.Fatalf("persisted image_protocol = %q, want %q", reloaded.Preview.ImageProtocol, config.PreviewImageProtocolKitty)
	}
	if reloaded.Preview.ImageMetadata != config.PreviewImageMetadataFull {
		t.Fatalf("persisted image_metadata = %q, want %q", reloaded.Preview.ImageMetadata, config.PreviewImageMetadataFull)
	}
	if reloaded.Preview.VideoMetadata != !config.DefaultPreviewVideoMetadata {
		t.Fatalf("persisted video_metadata = %v, want %v", reloaded.Preview.VideoMetadata, !config.DefaultPreviewVideoMetadata)
	}
}
