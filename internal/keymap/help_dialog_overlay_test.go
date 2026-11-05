package keymap

import "testing"

func TestHelpDialogOverlayDefaultF1Legend(t *testing.T) {
	if got := DefaultHelpDialogOverlayKeys()[ActionHelpLegend]; len(got) != 1 || got[0] != "F1" {
		t.Fatalf("default = %v", got)
	}
	if !AllowedInHelpDialogOverlay(ActionHelpLegend) {
		t.Fatal("legend not allowed in [dialog.help]")
	}
}

func TestHelpDialogOverlayDefaultF2(t *testing.T) {
	if got := DefaultHelpDialogOverlayKeys()[ActionHelpTextEditKeys]; len(got) != 1 || got[0] != "F2" {
		t.Fatalf("default = %v", got)
	}
	if AllowedInHelpDialogOverlay(ActionPanelHistoryBothPanels) || !AllowedInHelpDialogOverlay(ActionHelpTextEditKeys) {
		t.Fatal("Allowed mismatch")
	}
}
