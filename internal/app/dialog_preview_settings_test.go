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

// TestToggleKittySupportedClearsPlaceholder covers unchecking Kitty support also clearing the
// (now-inconsistent) placeholder checkbox, since placeholder display requires Kitty protocol.
func TestToggleKittySupportedClearsPlaceholder(t *testing.T) {
	st := &dialog.PreviewSettingsDialogState{KittySupported: true, KittyPlaceholderSupported: true}
	toggleKittySupported(st)
	if st.KittySupported {
		t.Fatal("KittySupported should be false after toggling from true")
	}
	if st.KittyPlaceholderSupported {
		t.Fatal("KittyPlaceholderSupported should be cleared when Kitty support is unchecked")
	}

	toggleKittySupported(st)
	if !st.KittySupported {
		t.Fatal("KittySupported should be true after toggling from false")
	}
	if st.KittyPlaceholderSupported {
		t.Fatal("KittyPlaceholderSupported should stay unchecked when only Kitty support is re-checked")
	}
}

// TestToggleKittyPlaceholderSupportedImpliesKitty covers checking placeholder support also
// implicitly checking Kitty support, since placeholder is a Kitty-only display mode.
func TestToggleKittyPlaceholderSupportedImpliesKitty(t *testing.T) {
	st := &dialog.PreviewSettingsDialogState{}
	toggleKittyPlaceholderSupported(st)
	if !st.KittyPlaceholderSupported {
		t.Fatal("KittyPlaceholderSupported should be true after toggling from false")
	}
	if !st.KittySupported {
		t.Fatal("KittySupported should be implicitly checked when placeholder support is checked")
	}

	toggleKittyPlaceholderSupported(st)
	if st.KittyPlaceholderSupported {
		t.Fatal("KittyPlaceholderSupported should be false after toggling from true")
	}
	if !st.KittySupported {
		t.Fatal("KittySupported should remain checked when only placeholder support is unchecked")
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
