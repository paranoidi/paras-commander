package app

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
)

func TestRewriteAltShiftO(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"bare", "\x1bO", "\x1b[27;3;79~"},
		{"followed by non-SS3 key", "\x1bOz1", "\x1b[27;3;79~z1"},
		{"F10 final x is a real SS3 key", "\x1bOx", "\x1bOx"},
		{"arrow up (application mode)", "\x1bOA", "\x1bOA"},
		{"F1", "\x1bOP", "\x1bOP"},
		{"alt+o lowercase untouched", "\x1bo", "\x1bo"},
		{"csi untouched", "\x1b[27;5;79~", "\x1b[27;5;79~"},
		{"two in a row", "\x1bO\x1bO", "\x1b[27;3;79~\x1b[27;3;79~"},
		{"between keys", "a\x1bOb", "a\x1b[27;3;79~b"},
		{"arrow then alt+shift+o", "\x1bOA\x1bO", "\x1bOA\x1b[27;3;79~"},
	}
	for _, c := range cases {
		if got := string(rewriteAltShiftO([]byte(c.in))); got != c.want {
			t.Errorf("%s: rewrite(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

// scriptedTty feeds scripted reads to altShiftTty.
type scriptedTty struct {
	tcell.Tty
	reads [][]byte
}

func (f *scriptedTty) Read(p []byte) (int, error) {
	r := f.reads[0]
	f.reads = f.reads[1:]
	return copy(p, r), nil
}

// A rewrite longer than the caller's buffer must come out across several Reads, in order, losing nothing.
func TestAltShiftTtyReadOverflow(t *testing.T) {
	tty := &altShiftTty{Tty: &scriptedTty{reads: [][]byte{[]byte("\x1bO")}}}
	var got []byte
	buf := make([]byte, 2) // 2 in, 9 out; read it back 2 bytes at a time
	for first := true; first || len(tty.pending) > 0; first = false {
		n, _ := tty.Read(buf)
		got = append(got, buf[:n]...)
	}
	if !bytes.Equal(got, altShiftOSeq) {
		t.Fatalf("reassembled %q, want %q", got, altShiftOSeq)
	}
}

// tcellKeyEvents feeds raw to tcell's real input parser and returns the key events it posts.
func tcellKeyEvents(raw []byte) []*tcell.EventKey {
	ch := make(chan tcell.Event, 16)
	ip := tcell.NewInputProcessor(ch)
	ip.ScanUTF8(raw)
	time.Sleep(120 * time.Millisecond) // past tcell's 50ms ESC timeout
	close(ch)
	var out []*tcell.EventKey
	for ev := range ch {
		if k, ok := ev.(*tcell.EventKey); ok {
			out = append(out, k)
		}
	}
	return out
}

// TestTcellStillDropsBareEscO is a tripwire, not a regression test. It PASSES while tcell still has the
// bug the Alt+Shift+O workaround exists for, and FAILS once a tcell upgrade fixes it, which is the cue to
// delete the workaround. See commit c5170a25 "fix(input): deliver Alt+Shift+O outside tmux".
func TestTcellStillDropsBareEscO(t *testing.T) {
	if got := tcellKeyEvents([]byte("\x1bO")); len(got) != 0 {
		desc := make([]string, len(got))
		for i, k := range got {
			desc[i] = fmt.Sprintf("{key=%v rune=%q mod=%v}", k.Key(), k.Rune(), k.Modifiers())
		}
		t.Fatalf(`tcell now handles a bare ESC O itself (it posted %d key event(s): %s), so the Alt+Shift+O workaround is probably obsolete.

It was added in commit c5170a25 ("fix(input): deliver Alt+Shift+O outside tmux"): tcell used to enter its
SS3 state on ESC O, wait with no timeout and drop the key, so M-O ("Other copies") never fired outside tmux.

To retire it (git show c5170a25 lists the files):
  1. delete internal/app/altshift_tty.go, newscreen_unix.go, newscreen_other.go and this test file
  2. in internal/app/app.go use tcell.NewScreen() instead of newTerminalScreen()
  3. before deleting, run kb-compare.sh in WezTerm without tmux and confirm Alt+Shift+O reaches the app
     (F1, then F4, then Alt+Shift+O should list "Other copies"); this test checks the parser, not a terminal`,
			len(got), strings.Join(desc, ", "))
	}
}

// End to end: the bytes WezTerm sends for Alt+Shift+O, run through the rewrite and tcell's real input
// parser, resolve to the default "Other copies" binding. Without the rewrite tcell posts no event at all.
func TestAltShiftOReachesOtherCopiesBinding(t *testing.T) {
	got := tcellKeyEvents(rewriteAltShiftO([]byte("\x1bO")))
	if len(got) != 1 || got[0].Key() != tcell.KeyRune || got[0].Rune() != 'O' || got[0].Modifiers()&tcell.ModAlt == 0 {
		t.Fatalf("rewritten ESC O decoded to %v, want one Alt+'O' key", got)
	}

	want, err := keymap.ParseKey("M-O")
	if err != nil {
		t.Fatal(err)
	}
	if ch := keymap.CanonicalChord(keymap.EventChord(got[0])); ch != keymap.CanonicalChord(want) {
		t.Fatalf("event chord %+v does not match the M-O binding %+v", ch, keymap.CanonicalChord(want))
	}
}
