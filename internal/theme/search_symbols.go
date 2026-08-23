package theme

import "strings"

const SymbolKeySearchIcon = "search.icon"

// SymbolSearchIcon returns the leading glyph painted inside scrollquery-based filter/search
// input rows (find, history, path picker, help, mass-rename pattern picker, run-for-each
// history picker, SFTP connect, F3 style picker).
// When UseNerdfontIcons is false, returns the ASCII "/" marker.
// Otherwise consults the theme's [symbols.search] section or uses the Nerd Font search glyph.
func (t Theme) SymbolSearchIcon() string {
	if !t.UseNerdfontIcons {
		return "/"
	}
	return t.searchSymbol(SymbolKeySearchIcon, "\uF002")
}

func (t Theme) searchSymbol(key, fallback string) string {
	if t.Symbols != nil {
		if s := strings.TrimSpace(t.Symbols[key]); s != "" {
			return s
		}
	}
	return fallback
}
