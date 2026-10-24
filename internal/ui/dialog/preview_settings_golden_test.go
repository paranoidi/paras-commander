package dialog

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Render goldens for the preview settings dialog (see simple_dialog_golden_test.go for the
// harness). previewGoldenFocus is the only code that knows how the state encodes focus.

func init() { goldenBandRows["preview_settings"] = 26 }

var previewGoldenFocusNames = []string{
	"sixel-auto", "sixel-yes", "sixel-no",
	"kitty-auto", "kitty-yes", "kitty-no",
	"placeholder-auto", "placeholder-yes", "placeholder-no",
	"protocol-" + config.PreviewImageProtocolAuto, "protocol-" + config.PreviewImageProtocolSixel,
	"protocol-" + config.PreviewImageProtocolKitty, "protocol-" + config.PreviewImageProtocolUeberzug,
	"metadata-" + config.PreviewImageMetadataOff, "metadata-" + config.PreviewImageMetadataBasic,
	"metadata-" + config.PreviewImageMetadataEssentials, "metadata-" + config.PreviewImageMetadataFull,
	"video", "ok", "cancel",
}

func previewGoldenFocus(t *testing.T, st *PreviewSettingsDialogState, pos string) {
	t.Helper()
	row, value, _ := strings.Cut(pos, "-")
	switch row {
	case "sixel", "kitty", "placeholder":
		st.Focus = PreviewCapabilityID(row, value)
	case "protocol":
		st.Focus = PreviewProtocolID(value)
	case "metadata":
		st.Focus = PreviewMetadataID(value)
	case "video":
		st.Focus = PreviewVideoMetadata
	case "ok":
		st.Focus = dlg.OK
	case "cancel":
		st.Focus = dlg.Cancel
	default:
		t.Fatalf("unknown preview settings focus %q", pos)
	}
}

func previewGoldenState(t *testing.T, variant, pos string) PreviewSettingsDialogState {
	t.Helper()
	st := PreviewSettingsDialogState{
		Open:             true,
		Sixel:            config.PreviewTerminalCapabilityAuto,
		Kitty:            config.PreviewTerminalCapabilityAuto,
		KittyPlaceholder: config.PreviewTerminalCapabilityAuto,
		Protocol:         config.PreviewImageProtocolAuto,
		ImageMetadata:    config.PreviewImageMetadataEssentials,
	}
	if variant == "mixed" {
		st.Sixel = config.PreviewTerminalCapabilityNo
		st.Kitty = config.PreviewTerminalCapabilityYes
		st.KittyPlaceholder = config.PreviewTerminalCapabilityYes
		st.Protocol = config.PreviewImageProtocolKitty
		st.ImageMetadata = config.PreviewImageMetadataFull
		st.VideoMetadata = true
	}
	previewGoldenFocus(t, &st, pos)
	return st
}

func TestPreviewSettingsGoldenRender(t *testing.T) {
	var cases []simpleGolden
	add := func(w, h int, variant, pos string) {
		st := previewGoldenState(t, variant, pos)
		cases = append(cases, simpleGolden{fmt.Sprintf("%dx%d_%s_focus-%s", w, h, variant, pos), w, h, func(s tcell.Screen, l Layout) {
			DrawPreviewSettingsDialog(s, l, st, asciiDialogStyles())
		}})
	}
	for _, pos := range previewGoldenFocusNames {
		add(80, 31, "default", pos)
	}
	for _, pos := range []string{"sixel-no", "kitty-yes", "placeholder-yes", "protocol-kitty", "metadata-full", "video", "ok", "cancel"} {
		add(80, 31, "mixed", pos)
	}
	add(60, 31, "mixed", "kitty-no")
	add(44, 31, "mixed", "metadata-basic")
	add(80, 20, "mixed", "protocol-sixel")
	runSimpleGoldens(t, "preview_settings", cases)
}
