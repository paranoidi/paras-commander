package app

import "testing"

// TestIsWithinDir covers the guard applyGitStatusLoad uses to accept git-status results for a
// tree-mode expanded child directory (a descendant of the panel's current listing) while
// rejecting results left over from navigating to an unrelated directory.
func TestIsWithinDir(t *testing.T) {
	cases := []struct {
		child, parent string
		want          bool
	}{
		{"/repo/src/pkg", "/repo/src", true},
		{"/repo/src", "/repo/src", true},
		{"/repo/other", "/repo/src", false},
		{"/repo/src2", "/repo/src", false}, // sibling with parent as string-prefix, not path-prefix
		{"/repo", "/repo/src", false},
		{"/repo/src/pkg", "", false},
	}
	for _, c := range cases {
		if got := isWithinDir(c.child, c.parent); got != c.want {
			t.Errorf("isWithinDir(%q, %q) = %v, want %v", c.child, c.parent, got, c.want)
		}
	}
}
