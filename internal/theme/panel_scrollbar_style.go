package theme

import "github.com/gdamore/tcell/v2"

// PanelScrollbarStyles returns track and thumb styles for a panel list scrollbar.
// Inactive or blocked panels use muted foreground colors.
func (t Theme) PanelScrollbarStyles(fileListActive, chromeBlocked bool) (track, thumb tcell.Style) {
	track = t.PanelScrollbarTrack
	thumb = t.PanelScrollbarThumb
	if chromeBlocked {
		fg, bg, _ := t.PanelBlockedText.Decompose()
		return track.Foreground(fg).Background(bg),
			thumb.Foreground(fg).Background(bg)
	}
	if !fileListActive {
		fg, bg, _ := t.PanelInactiveTitle.Decompose()
		return track.Foreground(fg).Background(bg),
			thumb.Foreground(fg).Background(bg)
	}
	return track, thumb
}

// DialogScrollbarStyles returns panel.scrollbar.track/thumb recolored onto the dialog surface
// background, for painting a scrollbar inside a dialog list. panel.scrollbar.* only defines a
// foreground, so left as PanelScrollbarStyles returns it the background would leak the terminal
// default instead of matching the dialog; the panel active/blocked semantics don't apply here.
func (t Theme) DialogScrollbarStyles() (track, thumb tcell.Style) {
	return mergeForegroundOnSurface(t.PanelScrollbarTrack, t.DialogSurface),
		mergeForegroundOnSurface(t.PanelScrollbarThumb, t.DialogSurface)
}
