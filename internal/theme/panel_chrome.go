package theme

import "github.com/gdamore/tcell/v2"

// PanelChrome holds resolved frame/title/surface/header styles for one panel chrome state.
type PanelChrome struct {
	Frame             tcell.Style
	Title             tcell.Style
	Surface           tcell.Style
	Header            tcell.Style
	HeaderCarousel    tcell.Style
	Text              tcell.Style
	DiskUsageOverview tcell.Style
}

// PanelChrome resolves panel chrome styles from active vs inactive vs blocked state.
func (t Theme) PanelChrome(active, blocked bool) PanelChrome {
	if blocked {
		return PanelChrome{
			Frame:             t.PanelBlockedFrame,
			Title:             t.PanelBlockedTitle,
			Surface:           t.PanelBlockedSurface,
			Header:            t.PanelBlockedHeader,
			HeaderCarousel:    t.PanelBlockedHeaderCarousel,
			Text:              t.PanelBlockedText,
			DiskUsageOverview: t.PanelBlockedDiskUsageOverview,
		}
	}
	if active {
		return PanelChrome{
			Frame:             t.PanelActiveFrame,
			Title:             t.PanelActiveTitle,
			Surface:           t.PanelActiveSurface,
			Header:            t.PanelActiveHeader,
			HeaderCarousel:    t.PanelActiveHeaderCarousel,
			Text:              t.PanelText,
			DiskUsageOverview: t.PanelActiveDiskUsageOverview,
		}
	}
	return PanelChrome{
		Frame:             t.PanelInactiveFrame,
		Title:             t.PanelInactiveTitle,
		Surface:           t.PanelInactiveSurface,
		Header:            t.PanelInactiveHeader,
		HeaderCarousel:    t.PanelInactiveHeaderCarousel,
		Text:              t.PanelText,
		DiskUsageOverview: t.PanelInactiveDiskUsageOverview,
	}
}
