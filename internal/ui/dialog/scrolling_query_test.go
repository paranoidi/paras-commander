package dialog

import "testing"

func TestScrollingQueryInsertBackspaceClear(t *testing.T) {
	q := &ScrollingQuery{Value: "ab", Cursor: 2}
	q.InsertRune('c')
	if q.Value != "abc" || q.Cursor != 3 {
		t.Fatalf("insert: value=%q cursor=%d", q.Value, q.Cursor)
	}
	q.Backspace()
	if q.Value != "ab" || q.Cursor != 2 {
		t.Fatalf("backspace: value=%q cursor=%d", q.Value, q.Cursor)
	}
	q.Clear()
	if q.Value != "" || q.Cursor != 0 || q.Scroll != 0 {
		t.Fatalf("clear: value=%q cursor=%d scroll=%d", q.Value, q.Cursor, q.Scroll)
	}
}

func TestScrollingQueryKillWordBackward(t *testing.T) {
	q := &ScrollingQuery{Value: "/foo/bar", Cursor: len([]rune("/foo/bar"))}
	q.KillWordBackward()
	if q.Value != "/foo/" || q.Cursor != 5 {
		t.Fatalf("kill word: value=%q cursor=%d", q.Value, q.Cursor)
	}
}

func TestScrollingQueryKillThenYank(t *testing.T) {
	q := &ScrollingQuery{Value: "foo bar", Cursor: 7}
	q.KillWordBackward()
	q.Yank()
	if q.Value != "foo bar" || q.Cursor != 7 {
		t.Fatalf("got %q %d", q.Value, q.Cursor)
	}
}

func TestScrollingQueryKillLineThenYank(t *testing.T) {
	q := &ScrollingQuery{Value: "foo bar", Cursor: 3}
	q.KillLine()
	q.KillLine() // empty: buffer untouched
	q.Yank()
	if q.Value != "foo bar" || q.Cursor != 7 {
		t.Fatalf("got %q %d", q.Value, q.Cursor)
	}
}

func TestScrollingQueryKillLineBackwardThenYank(t *testing.T) {
	q := &ScrollingQuery{Value: "foo bar", Cursor: 4}
	q.KillLineBackward()
	if q.Value != "bar" || q.Cursor != 0 {
		t.Fatalf("after kill %q %d", q.Value, q.Cursor)
	}
	q.KillLineBackward() // at start: buffer untouched
	q.Yank()
	if q.Value != "foo bar" || q.Cursor != 4 {
		t.Fatalf("after yank %q %d", q.Value, q.Cursor)
	}
}

func TestScrollingQueryKillLineForwardThenYank(t *testing.T) {
	q := &ScrollingQuery{Value: "foo bar", Cursor: 3}
	q.KillLineForward()
	if q.Value != "foo" || q.Cursor != 3 {
		t.Fatalf("after kill %q %d", q.Value, q.Cursor)
	}
	q.KillLineForward() // at end: buffer untouched
	q.Yank()
	if q.Value != "foo bar" || q.Cursor != 7 {
		t.Fatalf("after yank %q %d", q.Value, q.Cursor)
	}
}

func TestScrollingQueryKillWordForwardThenYank(t *testing.T) {
	q := &ScrollingQuery{Value: "foo bar", Cursor: 3}
	q.KillWordForward()
	if q.Value != "foo" || q.Cursor != 3 {
		t.Fatalf("after kill %q %d", q.Value, q.Cursor)
	}
	q.Yank()
	if q.Value != "foo bar" || q.Cursor != 7 {
		t.Fatalf("after yank %q %d", q.Value, q.Cursor)
	}
}

func TestScrollingQueryCaseWordForward(t *testing.T) {
	q := &ScrollingQuery{Value: "foo bar", Cursor: 3}
	q.CaseWordForward(true)
	if q.Value != "foo BAR" || q.Cursor != 7 {
		t.Fatalf("got %q %d", q.Value, q.Cursor)
	}
}
