package dialog

import "testing"

func TestApplyRenameSanitize(t *testing.T) {
	if got := ApplyRenameSanitize("a.b_c", true, true); got != "a b c" {
		t.Fatalf("dots+underscore: got %q want %q", got, "a b c")
	}
	if got := ApplyRenameSanitize("a.b_c", true, false); got != "a b_c" {
		t.Fatalf("dots only: got %q", got)
	}
	if got := ApplyRenameSanitize("a.b_c", false, true); got != "a.b c" {
		t.Fatalf("underscore only: got %q", got)
	}
	if got := ApplyRenameSanitize("x", false, false); got != "x" {
		t.Fatalf("none: got %q", got)
	}
}

func TestApplyRenameSlugify(t *testing.T) {
	if got := ApplyRenameSlugify("a b c", RenameSlugifyDot); got != "a.b.c" {
		t.Fatalf("dot: got %q", got)
	}
	if got := ApplyRenameSlugify("a b c", RenameSlugifyUnderscore); got != "a_b_c" {
		t.Fatalf("underscore: got %q", got)
	}
	if got := ApplyRenameSlugify("no-spaces", RenameSlugifyDot); got != "no-spaces" {
		t.Fatalf("unchanged: got %q", got)
	}
}
