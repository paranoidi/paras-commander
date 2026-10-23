package theme

import (
	"strings"

	"github.com/gdamore/tcell/v2"
)

const IconKeyCommandsCommand = "commands.command"

// IconCommandsCommand returns the leading glyph of a Commands-view row's command line
// ("$" when UseNerdfontIcons is false).
func (t Theme) IconCommandsCommand() string {
	return t.commandsIcon(IconKeyCommandsCommand, "\U000F1183", "$")
}

func (t Theme) commandsIcon(key, fallback, ascii string) string {
	if !t.UseNerdfontIcons {
		return ascii
	}
	if t.Icons != nil {
		if s := strings.TrimSpace(t.Icons[key]); s != "" {
			return s
		}
	}
	return fallback
}

const IconKeyEmpty = "empty"

// panelEmptyFallback is the panel.empty style when the theme omits it.
var panelEmptyFallback = tcell.StyleDefault.Foreground(tcell.PaletteColor(8))

// EmptyText returns the plain text shown in empty lists; it is not a glyph, so it is
// returned whether or not Nerd Font icons are enabled.
func (t Theme) EmptyText() string {
	if s := strings.TrimSpace(t.Icons[IconKeyEmpty]); s != "" {
		return s
	}
	return "[empty]"
}

// PanelEmptyStyle returns the empty-list text style (foreground only), falling back to
// bright black when the theme omits panel.empty.
func (t Theme) PanelEmptyStyle() tcell.Style {
	if t.PanelEmpty != (tcell.Style{}) {
		return t.PanelEmpty
	}
	return panelEmptyFallback
}
