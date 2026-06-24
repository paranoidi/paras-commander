package dialog

import "testing"

func TestMkdirActionForAltShortcut(t *testing.T) {
	t.Parallel()
	tests := []struct {
		rune   rune
		action MkdirAction
		idx    int
		ok     bool
	}{
		{'r', MkdirActionCreate, 0, true},
		{'R', MkdirActionCreate, 0, true},
		{'y', MkdirActionCreateCopySelect, 1, true},
		{'Y', MkdirActionCreateCopySelect, 1, true},
		{'m', MkdirActionCreateMoveSelect, 2, true},
		{'M', MkdirActionCreateMoveSelect, 2, true},
		{'x', 0, 0, false},
	}
	for _, tc := range tests {
		action, idx, ok := MkdirActionForAltShortcut(tc.rune)
		if ok != tc.ok || action != tc.action || idx != tc.idx {
			t.Errorf("MkdirActionForAltShortcut(%q) = (%v, %d, %v), want (%v, %d, %v)",
				string(tc.rune), action, idx, ok, tc.action, tc.idx, tc.ok)
		}
	}
}
