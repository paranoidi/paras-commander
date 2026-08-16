package app

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// TestToggleKittySupportedClearsPlaceholder covers unchecking Kitty support also clearing the
// (now-inconsistent) placeholder checkbox, since placeholder display requires Kitty protocol.
func TestToggleKittySupportedClearsPlaceholder(t *testing.T) {
	st := &dialog.ImageCapabilityDialogState{KittySupported: true, KittyPlaceholderSupported: true}
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
	st := &dialog.ImageCapabilityDialogState{}
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
