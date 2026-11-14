package dialog

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/pathpick"
)

func TestPathCompletionSetOpenRules(t *testing.T) {
	var c PathCompletion

	// Typing a fresh segment with 2+ candidates auto-opens the dropdown.
	c.Set("/tmp/f", 5, []pathpick.Candidate{{Name: "foo"}, {Name: "fum"}})
	if !c.Open {
		t.Fatal("expected Open=true when typing a non-empty partial with 2+ candidates")
	}

	// A bare trailing separator (empty partial) does not auto-open, even with candidates.
	var c2 PathCompletion
	c2.Set("/tmp/", 5, []pathpick.Candidate{{Name: "foo"}, {Name: "bar"}})
	if c2.Open {
		t.Fatal("expected Open=false for an empty partial (bare trailing separator)")
	}

	// No candidates always closes, regardless of partialEmpty.
	var c3 PathCompletion
	c3.Set("/tmp/f", 5, nil)
	if c3.Open {
		t.Fatal("expected Open=false when there are no candidates")
	}

	// A single PREFIX candidate never opens the dropdown: ghost text takes over (GhostSuffix).
	var c4 PathCompletion
	c4.Set("/tmp/f", 5, []pathpick.Candidate{{Name: "foo"}})
	if c4.Open {
		t.Fatal("expected Open=false for a single prefix candidate (ghost text instead)")
	}

	// A single FUZZY (non-prefix) candidate still opens the one-row dropdown; it cannot be
	// rendered as a suffix after the caret.
	var c5 PathCompletion
	value := "/tmp/comman"
	start := len([]rune("/tmp/"))
	c5.Set(value, start, []pathpick.Candidate{{Name: "paras-commander"}})
	if !c5.Open {
		t.Fatal("expected Open=true for a single fuzzy (non-prefix) candidate")
	}
}

func TestPathCompletionSetSameValueKeepsState(t *testing.T) {
	var c PathCompletion
	c.Set("/tmp/f", 5, []pathpick.Candidate{{Name: "foo"}, {Name: "fum"}})
	c.Open = true
	c.Selected = 1

	// Re-syncing the SAME value (e.g. an FS-change refresh) keeps Open/Selected.
	c.Set("/tmp/f", 5, []pathpick.Candidate{{Name: "foo"}, {Name: "fum"}})
	if !c.Open || c.Selected != 1 {
		t.Fatalf("same-value resync should keep state: Open=%v Selected=%d", c.Open, c.Selected)
	}

	// Selected clamps down when the new (same-value) list is shorter.
	c.Set("/tmp/f", 5, []pathpick.Candidate{{Name: "foo"}})
	if c.Selected != 0 {
		t.Fatalf("Selected = %d, want clamped to 0", c.Selected)
	}

	// A real edit (different value) resets Selected/Scroll and re-opens (2+ candidates).
	c.Selected = 0
	c.Set("/tmp/fu", 5, []pathpick.Candidate{{Name: "fum"}, {Name: "fuzz"}})
	if c.Selected != 0 || !c.Open {
		t.Fatalf("edit should reset Selected=0 and reopen: Selected=%d Open=%v", c.Selected, c.Open)
	}
}

func TestPathCompletionGhostSuffix(t *testing.T) {
	// Single prefix candidate: GhostSuffix returns the remaining name, no trailing "/".
	var c PathCompletion
	c.Set("/tmp/f", 5, []pathpick.Candidate{{Name: "foo", IsDir: true}})
	if got := c.GhostSuffix("/tmp/f"); got != "oo" {
		t.Fatalf("GhostSuffix = %q, want %q", got, "oo")
	}
	if !c.hasGhost("/tmp/f") {
		t.Fatal("expected HasGhost=true for a single prefix candidate")
	}

	// Single FUZZY (non-prefix) candidate: no ghost suffix.
	var c2 PathCompletion
	value := "/tmp/comman"
	start := len([]rune("/tmp/"))
	c2.Set(value, start, []pathpick.Candidate{{Name: "paras-commander"}})
	if got := c2.GhostSuffix(value); got != "" {
		t.Fatalf("GhostSuffix = %q, want \"\" for a fuzzy (non-prefix) match", got)
	}
	if c2.hasGhost(value) {
		t.Fatal("expected HasGhost=false for a fuzzy (non-prefix) candidate")
	}

	// 2+ candidates: no ghost suffix even when one of them is a prefix match.
	var c3 PathCompletion
	c3.Set("/tmp/f", 5, []pathpick.Candidate{{Name: "foo"}, {Name: "fum"}})
	if got := c3.GhostSuffix("/tmp/f"); got != "" {
		t.Fatalf("GhostSuffix = %q, want \"\" for 2+ candidates", got)
	}
}

func TestPathCompletionMoveClampsNoWrap(t *testing.T) {
	var c PathCompletion
	items := make([]pathpick.Candidate, 3)
	for i := range items {
		items[i] = pathpick.Candidate{Name: string(rune('a' + i))}
	}
	c.Set("x", 0, items)

	c.Move(-1)
	if c.Selected != 0 {
		t.Fatalf("Move(-1) at top: Selected = %d, want 0 (no wrap)", c.Selected)
	}
	c.Move(1)
	if c.Selected != 1 {
		t.Fatalf("Move(1): Selected = %d, want 1", c.Selected)
	}
	c.Move(10)
	if c.Selected != len(items)-1 {
		t.Fatalf("Move(10): Selected = %d, want clamped to %d", c.Selected, len(items)-1)
	}
	c.Move(1)
	if c.Selected != len(items)-1 {
		t.Fatalf("Move(1) at bottom: Selected = %d, want %d (no wrap)", c.Selected, len(items)-1)
	}
}

func TestPathCompletionCycleWraps(t *testing.T) {
	var c PathCompletion
	items := []pathpick.Candidate{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	c.Set("x", 0, items)

	c.Cycle()
	if c.Selected != 1 {
		t.Fatalf("Selected = %d, want 1", c.Selected)
	}
	c.Cycle()
	if c.Selected != 2 {
		t.Fatalf("Selected = %d, want 2", c.Selected)
	}
	c.Cycle()
	if c.Selected != 0 {
		t.Fatalf("Cycle at end should wrap to 0, got %d", c.Selected)
	}
}

func TestPathCompletionAcceptPrefix(t *testing.T) {
	var c PathCompletion
	c.Set("/tmp/f", 5, []pathpick.Candidate{{Name: "foo", IsDir: true}})
	newValue, newCursor := c.Accept("/tmp/f")
	if newValue != "/tmp/foo/" {
		t.Fatalf("Value = %q, want /tmp/foo/", newValue)
	}
	if newCursor != len([]rune("/tmp/foo/")) {
		t.Fatalf("Cursor = %d, want %d", newCursor, len([]rune("/tmp/foo/")))
	}
	if c.Open || len(c.Items) != 0 {
		t.Fatalf("Accept should clear completion state, got %+v", c)
	}
}

func TestPathCompletionAcceptFuzzyReplacesPartial(t *testing.T) {
	var c PathCompletion
	// Query "/tmp/comman" where the candidate is a fuzzy (non-prefix) match "paras-commander".
	value := "/tmp/comman"
	start := len([]rune("/tmp/"))
	c.Set(value, start, []pathpick.Candidate{{Name: "paras-commander", IsDir: true}})
	newValue, newCursor := c.Accept(value)
	want := "/tmp/paras-commander/"
	if newValue != want {
		t.Fatalf("Value = %q, want %q", newValue, want)
	}
	if newCursor != len([]rune(want)) {
		t.Fatalf("Cursor = %d, want %d", newCursor, len([]rune(want)))
	}
}

func TestPathCompletionAcceptNonDirNoTrailingSlash(t *testing.T) {
	var c PathCompletion
	c.Set("/tmp/re", 5, []pathpick.Candidate{{Name: "readme.txt", IsDir: false}})
	newValue, _ := c.Accept("/tmp/re")
	if newValue != "/tmp/readme.txt" {
		t.Fatalf("Value = %q, want /tmp/readme.txt (no trailing slash for a file)", newValue)
	}
}

func TestHandlePathCompletionKeyTabSingleItemAccepts(t *testing.T) {
	var c PathCompletion
	value := "/tmp/f"
	cursor := len([]rune(value))
	c.Set(value, 5, []pathpick.Candidate{{Name: "foo", IsDir: true}})
	c.Open = false // closed dropdown, but a single candidate is known

	handled, outcome := HandlePathCompletionKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone), &c, &value, &cursor)
	if !handled || outcome != CompletionAccepted {
		t.Fatalf("handled=%v outcome=%v, want true,true", handled, outcome)
	}
	if value != "/tmp/foo/" {
		t.Fatalf("value = %q, want /tmp/foo/", value)
	}
}

func TestHandlePathCompletionKeyTabMultiItemOpensThenCycles(t *testing.T) {
	var c PathCompletion
	value := "/tmp/f"
	cursor := len([]rune(value))
	c.Set(value, 5, []pathpick.Candidate{{Name: "foo"}, {Name: "fum"}})
	c.Open = false

	handled, outcome := HandlePathCompletionKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone), &c, &value, &cursor)
	if !handled || outcome != CompletionNone {
		t.Fatalf("first Tab: handled=%v outcome=%v, want true,false (opens, doesn't accept)", handled, outcome)
	}
	if !c.Open || c.Selected != 0 {
		t.Fatalf("expected dropdown open with Selected=0, got Open=%v Selected=%d", c.Open, c.Selected)
	}

	handled, outcome = HandlePathCompletionKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone), &c, &value, &cursor)
	if !handled || outcome != CompletionNone {
		t.Fatal("second Tab should cycle, not accept")
	}
	if c.Selected != 1 {
		t.Fatalf("Selected = %d, want 1 after cycling", c.Selected)
	}
}

func TestHandlePathCompletionKeyEnterAcceptsAndEscClosesOnly(t *testing.T) {
	var c PathCompletion
	value := "/tmp/f"
	cursor := len([]rune(value))
	c.Set(value, 5, []pathpick.Candidate{{Name: "foo"}, {Name: "fum"}})
	if !c.Open {
		t.Fatal("expected auto-open")
	}

	handled, outcome := HandlePathCompletionKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone), &c, &value, &cursor)
	if !handled || outcome != CompletionNone {
		t.Fatal("Esc should be handled without accepting")
	}
	if c.Open {
		t.Fatal("Esc should close the dropdown")
	}
	if len(c.Items) == 0 {
		t.Fatal("Esc should not clear the candidate list, only close the dropdown")
	}

	// Closed + Enter is NOT handled by the completion dropdown (falls through to the dialog).
	handled, _ = HandlePathCompletionKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), &c, &value, &cursor)
	if handled {
		t.Fatal("Enter while closed should not be handled by the completion dropdown")
	}

	// Re-open and accept via Enter.
	c.Open = true
	c.Selected = 1
	handled, outcome = HandlePathCompletionKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), &c, &value, &cursor)
	if !handled || outcome != CompletionAccepted {
		t.Fatalf("handled=%v outcome=%v, want true,true", handled, outcome)
	}
	if value != "/tmp/fum" {
		t.Fatalf("value = %q, want /tmp/fum", value)
	}
}

func TestHandlePathCompletionKeyUpDownMoveWhileOpen(t *testing.T) {
	var c PathCompletion
	value := "/tmp/f"
	cursor := len([]rune(value))
	c.Set(value, 5, []pathpick.Candidate{{Name: "foo"}, {Name: "fum"}, {Name: "fizz"}})

	handled, outcome := HandlePathCompletionKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone), &c, &value, &cursor)
	if !handled || outcome != CompletionNone || c.Selected != 1 {
		t.Fatalf("Down: handled=%v outcome=%v Selected=%d, want true,false,1", handled, outcome, c.Selected)
	}
	handled, outcome = HandlePathCompletionKey(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone), &c, &value, &cursor)
	if !handled || outcome != CompletionNone || c.Selected != 0 {
		t.Fatalf("Up: handled=%v outcome=%v Selected=%d, want true,false,0", handled, outcome, c.Selected)
	}
}

func (c *PathCompletion) hasGhost(value string) bool {
	_, ok := c.ghost(value)
	return ok
}

func TestSetCreateTabFallsThrough(t *testing.T) {
	var c PathCompletion
	value := "/tmp/lantern/"
	cursor := len([]rune(value))
	c.SetCreate(value, "/tmp/lantern")
	if handled, _ := HandlePathCompletionKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone), &c, &value, &cursor); handled {
		t.Fatal("Tab on the create row must fall through to the form (focus move)")
	}
	if c.Open {
		t.Fatal("Tab should close the create row so it is not painted over the next widget")
	}
}

func TestSetCreateOpensAndEnterReportsCreate(t *testing.T) {
	for _, key := range []tcell.Key{tcell.KeyEnter} {
		var c PathCompletion
		value := "/tmp/lantern/"
		cursor := len([]rune(value))
		c.SetCreate(value, "/tmp/lantern")
		if !c.Open || len(c.Items) != 1 || !c.Items[0].Create || c.CreatePath != "/tmp/lantern" {
			t.Fatalf("SetCreate: open=%v items=%+v path=%q", c.Open, c.Items, c.CreatePath)
		}
		if c.GhostSuffix(value) != "" {
			t.Fatal("the create row must not paint ghost text")
		}
		handled, outcome := HandlePathCompletionKey(tcell.NewEventKey(key, 0, tcell.ModNone), &c, &value, &cursor)
		if !handled || outcome != CompletionCreateDir {
			t.Fatalf("key %v: handled=%v outcome=%v, want true, CompletionCreateDir", key, handled, outcome)
		}
		if value != "/tmp/lantern/" || cursor != len([]rune(value)) {
			t.Fatalf("value/cursor changed: %q %d", value, cursor)
		}
	}
}

func TestSetCreateSameValueKeepsClosed(t *testing.T) {
	var c PathCompletion
	c.SetCreate("/tmp/lantern/", "/tmp/lantern")
	c.Open = false
	c.SetCreate("/tmp/lantern/", "/tmp/lantern")
	if c.Open {
		t.Fatal("a same-value resync must keep the dropdown closed after Esc")
	}
	c.Set("/tmp/lantern/x", 13, []pathpick.Candidate{{Name: "xa"}, {Name: "xb"}})
	if c.CreatePath != "" {
		t.Fatal("Set must drop the create path")
	}
}
