package theme

import "github.com/gdamore/tcell/v2"

// fileIndexStates are the fileindex row states of "browse with fileindex" (localfs.IndexStatus*
// values): each has an icon ([icons.fileindex_state]) and a style ([panel.fileindex]). The
// fallback glyph is the Nerd Font icon, ascii the one shown without Nerd Font icons.
var fileIndexStates = []struct {
	key, icon, ascii string
	fg               tcell.Color
}{
	{"indexed", "\U000F0AA9", "=", tcell.ColorGray},   // nf-md-database_check
	{"changed", "\U000F0B86", "~", tcell.ColorYellow}, // nf-md-database_edit
	{"new", "\U000F01BB", "+", tcell.ColorGreen},      // nf-md-database_minus
	{"missing", "\U000F01BA", "-", tcell.ColorRed},    // nf-md-database_plus
	{"excluded", "\U000F1640", "x", tcell.ColorGray},  // nf-md-database_off
	{"scanning", "\U000F0CFF", "*", tcell.ColorTeal},  // nf-md-database_sync
	{"differs", "\U000F163A", "!", tcell.ColorYellow}, // nf-md-database_alert
	{"tagged", "\U000F12F6", "#", tcell.ColorFuchsia}, // nf-md-database_marker
}

// fileIndexStateStyleKeys are the optional [panel.fileindex] style keys.
func fileIndexStateStyleKeys() []string {
	keys := make([]string, len(fileIndexStates))
	for i, s := range fileIndexStates {
		keys[i] = "panel.fileindex." + s.key
	}
	return keys
}

// IconFileIndexStateKey is the [icons.fileindex_state] key of a state.
func IconFileIndexStateKey(state string) string { return "fileindex_state." + state }

// IconFileIndexState is the one-cell glyph of a fileindex row state ([icons.fileindex_state]),
// an ASCII letter without Nerd Font icons; "" for an unknown state.
func (t Theme) IconFileIndexState(state string) rune {
	for _, s := range fileIndexStates {
		if s.key == state {
			if !t.UseNerdfontIcons {
				return rune(s.ascii[0])
			}
			return t.iconRuneOr(IconFileIndexStateKey(state), []rune(s.icon)[0])
		}
	}
	return 0
}

// PanelFileIndexStyle is the foreground style of a fileindex row state ([panel.fileindex]).
func (t Theme) PanelFileIndexStyle(state string) tcell.Style {
	if st, ok := t.PanelFileIndex[state]; ok {
		return st
	}
	return tcell.StyleDefault
}

// fileIndexStateStyles resolves the optional [panel.fileindex] styles, falling back to a plain
// foreground per state.
func fileIndexStateStyles(specs map[string]styleSpec, palette map[string]tcell.Color) (map[string]tcell.Style, error) {
	out := make(map[string]tcell.Style, len(fileIndexStates))
	for _, s := range fileIndexStates {
		st, err := optionalStyle(specs, palette, "panel.fileindex."+s.key, tcell.StyleDefault.Foreground(s.fg))
		if err != nil {
			return nil, err
		}
		out[s.key] = st
	}
	return out, nil
}
