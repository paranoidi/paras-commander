package theme

import "github.com/gdamore/tcell/v2"

const (
	IconKeyPanelTabStart = "panel.tab.start"
	IconKeyPanelTabEnd   = "panel.tab.end"

	// PanelTabSquareCap is the start and end cap of a square panel tab (" N label ").
	PanelTabSquareCap = " "
)

// PanelTabCaps returns the strings drawn before and after a panel tab's "N" / "N label"
// ([icons.panel.tab] start / end). They are not trimmed: their spaces are part of the tab. When
// UseNerdfontIcons is false, both are PanelTabSquareCap.
func (t Theme) PanelTabCaps() (start, end string) {
	if !t.UseNerdfontIcons {
		return PanelTabSquareCap, PanelTabSquareCap
	}
	return t.Icons[IconKeyPanelTabStart], t.Icons[IconKeyPanelTabEnd]
}

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

// DialogOnPanelChrome returns the theme with the dialog surface, frame, title and text replaced by
// the active file-list panel's, for a dialog showing content themed for that panel. Dialog widgets
// that paint their own background (buttons, inputs) keep their dialog styles.
func (t Theme) DialogOnPanelChrome() Theme {
	chrome := t.PanelChrome(true, false)
	t.DialogSurface = chrome.Surface
	t.DialogFrame = chrome.Frame
	// The panel title's own colour would differ from the frame's; the dialog title keeps the frame colour.
	frameFg, _, _ := chrome.Frame.Decompose()
	t.DialogTitle = chrome.Title.Foreground(frameFg)
	t.DialogText = t.PanelRowFile
	return t
}
