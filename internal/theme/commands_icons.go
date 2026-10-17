package theme

import "strings"

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
