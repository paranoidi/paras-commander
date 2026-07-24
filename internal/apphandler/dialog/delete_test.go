package dialog

import "testing"

func TestDeleteDialogDescendIntoMountPoints(t *testing.T) {
	t.Parallel()
	if !deleteDialogDescendIntoMountPoints {
		t.Fatal("delete dialog must scan across mount points (Samba, etc.)")
	}
}
