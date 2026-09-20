package ui

import "testing"

// TestCalculateLayoutWithOrientationMenuRowInvariant pins MenuBarLayoutReserved() as the single
// source of truth for "row 0 is the menu strip": fullscreen file preview reclaims it (no menu
// row, panels start at row 0), the browser view reserves it (panels start at row 1).
func TestCalculateLayoutWithOrientationMenuRowInvariant(t *testing.T) {
	t.Parallel()
	split := PanelPaneSplit{ActivePercent: 50, InactivePercent: 50}

	preview := Model{ViewMode: ViewFilePreview, HideMenuBar: false}
	layout := CalculateLayoutWithOrientation(80, 24, preview.MenuBarLayoutReserved(), split, SplitHorizontal, 0, 0)
	if layout.Menu.Height != 0 {
		t.Fatalf("fullscreen preview: Menu.Height = %d, want 0", layout.Menu.Height)
	}
	if layout.Primary.Y != 0 {
		t.Fatalf("fullscreen preview: Primary.Y = %d, want 0", layout.Primary.Y)
	}

	browser := Model{ViewMode: ViewBrowser, HideMenuBar: false}
	layout = CalculateLayoutWithOrientation(80, 24, browser.MenuBarLayoutReserved(), split, SplitHorizontal, 0, 0)
	if layout.Menu.Height != 1 {
		t.Fatalf("browser: Menu.Height = %d, want 1", layout.Menu.Height)
	}
	if layout.Primary.Y != 1 {
		t.Fatalf("browser: Primary.Y = %d, want 1", layout.Primary.Y)
	}
}
