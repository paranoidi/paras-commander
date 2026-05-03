package menu

// FindItemByFKeyLabel returns the first non-separator item whose KeyLabel equals label (e.g. "F5").
func FindItemByFKeyLabel(defs []Definition, label string) (def Definition, item Item, ok bool) {
	if label == "" {
		return Definition{}, Item{}, false
	}
	for _, d := range defs {
		for _, it := range d.Items {
			if it.Separator || it.KeyLabel != label {
				continue
			}
			return d, it, true
		}
	}
	return Definition{}, Item{}, false
}
