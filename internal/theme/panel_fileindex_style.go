package theme

import "github.com/gdamore/tcell/v2"

// fileIndexStates are the fileindex row states of "browse with fileindex" (localfs.IndexStatus*
// values): each has an icon ([icons.fileindex_state]) and a style ([panel.fileindex]), except
// the loading states (fileIndexStateLoading), styled by [status] scanning (Theme.StatusScanning).
// ascii is the glyph shown without Nerd Font icons; the Nerd Font glyph is [icons.fileindex_state].
var fileIndexStates = []struct{ key, ascii string }{
	{"indexed", "="},
	{"changed", "~"},
	{"new", "+"},
	{"missing", "-"},
	{"excluded", "x"},
	{"scanning", "*"},
	{"checking", "."},
	{"differs", "!"},
	{"unchecked", "?"},
	{"unhashed", "h"},
	{"tagged", "#"},
}

// fileIndexStateLoading reports a state styled by StatusScanning instead of [panel.fileindex].
func fileIndexStateLoading(state string) bool { return state == "scanning" || state == "checking" }

// fileIndexStateStyleKeys are the [panel.fileindex] style keys.
func fileIndexStateStyleKeys() []string {
	keys := make([]string, 0, len(fileIndexStates))
	for _, s := range fileIndexStates {
		if !fileIndexStateLoading(s.key) {
			keys = append(keys, "panel.fileindex."+s.key)
		}
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
			return t.iconRune(IconFileIndexStateKey(state))
		}
	}
	return 0
}

// PanelFileIndexStyle is the foreground style of a fileindex row state ([panel.fileindex];
// StatusScanning for the loading states).
func (t Theme) PanelFileIndexStyle(state string) tcell.Style {
	if fileIndexStateLoading(state) {
		return t.StatusScanning
	}
	if st, ok := t.PanelFileIndex[state]; ok {
		return st
	}
	return tcell.StyleDefault
}

// fileIndexStateStyles collects the [panel.fileindex] styles out of the resolved style table.
func fileIndexStateStyles(styles map[string]tcell.Style) map[string]tcell.Style {
	out := make(map[string]tcell.Style, len(fileIndexStates))
	for _, s := range fileIndexStates {
		if !fileIndexStateLoading(s.key) {
			out[s.key] = styles["panel.fileindex."+s.key]
		}
	}
	return out
}
