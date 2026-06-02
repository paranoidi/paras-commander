package usermenu

// FilterVisible returns entries whose When condition passes and the index
// of the row to highlight (first Default entry, else 0).
func FilterVisible(m *MenuFile, ctx *EvalContext) ([]MenuEntry, int, error) {
	if m == nil {
		return nil, 0, nil
	}
	ctxCopy := *ctx
	ctxCopy.ShellPatterns = m.ShellPatterns
	var out []MenuEntry
	defaultIdx := -1
	for _, e := range m.Entries {
		ok, err := EvalWhenAny(e.When, &ctxCopy)
		if err != nil {
			return nil, -1, err
		}
		if !ok {
			continue
		}
		if e.Default && defaultIdx < 0 {
			defaultIdx = len(out)
		}
		out = append(out, e)
	}
	if len(out) == 0 {
		return out, 0, nil
	}
	if defaultIdx < 0 {
		defaultIdx = 0
	}
	return out, defaultIdx, nil
}
