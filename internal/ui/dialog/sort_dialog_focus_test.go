package dialog

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestSortDialogMoveFocus(t *testing.T) {
	st := func(n int) SortDialogState {
		var s SortDialogState
		for range n {
			s.MetaRadios = append(s.MetaRadios, SortDialogMetaRadio{Title: "harbor", Name: "harbor"})
		}
		return s
	}
	two, four := st(2), st(4)
	cases := []struct {
		name   string
		s      SortDialogState
		focus  int
		key    tcell.Key
		want   int
		wantOK bool
	}{
		{"right name to meta0", two, 0, tcell.KeyRight, 4, true},
		{"right size without meta stays", two, 2, tcell.KeyRight, 2, true},
		{"right mtime to meta3", four, 3, tcell.KeyRight, 7, true},
		{"left meta1 to extension", two, 5, tcell.KeyLeft, 1, true},
		{"down mtime to disk usage", two, 3, tcell.KeyDown, two.CheckboxFocus(), true},
		{"down mtime without meta", st(0), 3, tcell.KeyDown, 4, true},
		{"down last meta to disk usage", two, 5, tcell.KeyDown, two.CheckboxFocus(), true},
		{"down meta0 falls through", two, 4, tcell.KeyDown, 0, false},
		{"up meta0 stays", two, 4, tcell.KeyUp, 4, true},
		{"up meta1 falls through", two, 5, tcell.KeyUp, 0, false},
		{"up disk usage to mtime", four, four.CheckboxFocus(), tcell.KeyUp, 3, true},
		{"down disk usage falls through", two, two.CheckboxFocus(), tcell.KeyDown, 0, false},
		{"left on button falls through", two, two.OKFocus(), tcell.KeyLeft, 0, false},
	}
	for _, c := range cases {
		got, ok := c.s.MoveFocus(c.focus, c.key)
		if got != c.want || ok != c.wantOK {
			t.Errorf("%s: got (%d, %v), want (%d, %v)", c.name, got, ok, c.want, c.wantOK)
		}
	}
}
