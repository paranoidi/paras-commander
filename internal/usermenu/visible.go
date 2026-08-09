package usermenu

// FilterVisible returns entries whose When condition passes and the index
// of the row to highlight (first Default entry, else 0). A submenu is dropped
// entirely when all of its children are filtered out (no dead-end rows).
func FilterVisible(m *MenuFile, ctx *EvalContext) ([]MenuEntry, int, error) {
	if m == nil {
		return nil, 0, nil
	}
	out, defaultIdx, err := filterEntries(m.Entries, ctx)
	if err != nil {
		return nil, -1, err
	}
	if len(out) == 0 {
		return out, 0, nil
	}
	return out, defaultIdx, nil
}

// filterEntries recursively filters entries (and, for submenus, their children) by When,
// returning the visible slice and the index of the row to highlight (first Default entry
// at this level, else 0).
func filterEntries(entries []MenuEntry, ctx *EvalContext) ([]MenuEntry, int, error) {
	ctxCopy := *ctx
	var out []MenuEntry
	defaultIdx := -1
	for _, e := range entries {
		ctxCopy.ShellPatterns = e.ShellPatterns
		ok, err := EvalWhenAny(e.When, &ctxCopy)
		if err != nil {
			return nil, -1, err
		}
		if !ok {
			continue
		}
		if e.IsSubmenu() {
			children, _, err := filterEntries(e.Entries, ctx)
			if err != nil {
				return nil, -1, err
			}
			if len(children) == 0 {
				continue
			}
			e.Entries = children
		}
		if e.Default && defaultIdx < 0 {
			defaultIdx = len(out)
		}
		out = append(out, e)
	}
	if defaultIdx < 0 {
		defaultIdx = 0
	}
	return out, defaultIdx, nil
}
