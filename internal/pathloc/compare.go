package pathloc

// EqualOrUnder reports whether loc is the same as or a descendant of root.
func EqualOrUnder(root, loc Path) bool {
	if root.IsZero() || loc.IsZero() {
		return false
	}
	return loc.HasPrefix(root)
}

// TreesOverlap reports whether two locations share a common prefix tree (either may be an ancestor of the other).
func TreesOverlap(a, b Path) bool {
	if a.IsZero() || b.IsZero() {
		return false
	}
	if a.Scheme() != b.Scheme() {
		return false
	}
	return a.HasPrefix(b) || b.HasPrefix(a)
}

// EqualOrUnderStrings parses root and loc strings then calls EqualOrUnder.
func EqualOrUnderStrings(rootStr, locStr string) bool {
	if rootStr == "" || locStr == "" {
		return false
	}
	root, err1 := Parse(rootStr)
	loc, err2 := Parse(locStr)
	if err1 != nil || err2 != nil {
		return false
	}
	return EqualOrUnder(root, loc)
}

// TreesOverlapStrings parses a and b then calls TreesOverlap.
func TreesOverlapStrings(aStr, bStr string) bool {
	if aStr == "" || bStr == "" {
		return false
	}
	a, err1 := Parse(aStr)
	b, err2 := Parse(bStr)
	if err1 != nil || err2 != nil {
		return false
	}
	return TreesOverlap(a, b)
}
