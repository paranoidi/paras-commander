package app

import (
	"bytes"

	"github.com/gdamore/tcell/v2"
)

// Terminals without an extended-key mode send Alt+Shift+O as the bytes ESC O, which tcell reads as the
// start of an application-mode function-key sequence (SS3, e.g. ESC O A for Up). tcell then waits for a
// third byte with no timeout and drops the pair when a non-function-key byte (or nothing useful) follows,
// so Alt+Shift+O, the default "Other copies" key (M-O), never reached the app. Inside tmux it worked only
// because tmux re-encodes the key.
//
// altShiftTty fixes that below tcell: ESC O not followed by a byte tcell knows as an SS3 final becomes
// the modifyOtherKeys form of Alt+Shift+O, which tcell decodes as rune 'O' with ModAlt, the same event
// the legacy ESC-prefix path gives Alt+o.
//
// ponytail: a terminal that splits a real ESC O A across two reads would turn into Alt+Shift+O then 'A';
// terminals write one key per write, so this is a ceiling, not an observed case. Paste content holding a
// literal ESC O is rewritten too.

// altShiftOSeq is Alt+Shift+O as xterm modifyOtherKeys: ESC [ 27 ; 3 ; 79 ~ (3 = Alt, 79 = 'O').
var altShiftOSeq = []byte("\x1b[27;3;79~")

// ss3Finals are the bytes tcell accepts after ESC O (its ss3Keys table); anything else is not an SS3 key.
const ss3Finals = "ABCDFHPQRStuvlwx"

// rewriteAltShiftO returns b with every bare ESC O (see above) replaced by altShiftOSeq. It returns b itself
// when there is nothing to rewrite.
func rewriteAltShiftO(b []byte) []byte {
	if !bytes.Contains(b, []byte("\x1bO")) {
		return b
	}
	out := make([]byte, 0, len(b)+len(altShiftOSeq))
	for i := 0; i < len(b); i++ {
		if b[i] == 0x1b && i+1 < len(b) && b[i+1] == 'O' {
			if i+2 < len(b) && bytes.IndexByte([]byte(ss3Finals), b[i+2]) >= 0 {
				out = append(out, b[i], b[i+1]) // a real application-mode key: leave it to tcell
			} else {
				out = append(out, altShiftOSeq...)
			}
			i++ // consumed the 'O' as well
			continue
		}
		out = append(out, b[i])
	}
	return out
}

// altShiftTty applies rewriteAltShiftO to everything tcell reads from the terminal. The rewrite can grow a
// read past tcell's buffer, so the overflow waits in pending for the next Read.
type altShiftTty struct {
	tcell.Tty
	pending []byte
}

func (t *altShiftTty) Read(p []byte) (int, error) {
	if len(t.pending) == 0 {
		raw := make([]byte, len(p))
		n, err := t.Tty.Read(raw)
		if n == 0 {
			return 0, err
		}
		t.pending = rewriteAltShiftO(raw[:n])
		if len(t.pending) == n { // unchanged: avoid the copy-out dance below
			c := copy(p, t.pending)
			t.pending = t.pending[c:]
			return c, err
		}
	}
	c := copy(p, t.pending)
	t.pending = t.pending[c:]
	return c, nil
}
