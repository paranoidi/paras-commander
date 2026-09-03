package preview

import (
	"bytes"
	"testing"
)

func TestTerminalQueryScannerStripsAndAnswers(t *testing.T) {
	s := &terminalQueryScanner{sixelOK: true}
	clean, reply := s.Scan([]byte("hello \x1b[0c world"))
	if got := string(clean); got != "hello  world" {
		t.Fatalf("clean = %q, want %q", got, "hello  world")
	}
	if !bytes.Equal(reply, da1Reply(true)) {
		t.Fatalf("reply = %q, want %q", reply, da1Reply(true))
	}
}

func TestTerminalQueryScannerRealWorldCapture(t *testing.T) {
	// Exact byte sequence captured from movie-info: DA1, a stray lone ESC, then CPR.
	input := []byte("\x1b[0c\x1b\x1b[6n")
	s := &terminalQueryScanner{sixelOK: false}
	clean, reply := s.Scan(input)
	if got := string(clean); got != "\x1b" {
		t.Fatalf("clean = %q, want the stray ESC to pass through untouched", got)
	}
	want := append(append([]byte{}, da1Reply(false)...), cprReply()...)
	if !bytes.Equal(reply, want) {
		t.Fatalf("reply = %q, want %q", reply, want)
	}
}

func TestTerminalQueryScannerSplitAcrossChunks(t *testing.T) {
	s := &terminalQueryScanner{sixelOK: false}
	clean1, reply1 := s.Scan([]byte("\x1b["))
	if len(clean1) != 0 || len(reply1) != 0 {
		t.Fatalf("first chunk: clean=%q reply=%q, want both empty (held as partial)", clean1, reply1)
	}
	clean2, reply2 := s.Scan([]byte("6n"))
	if len(clean2) != 0 {
		t.Fatalf("second chunk: clean=%q, want empty", clean2)
	}
	if !bytes.Equal(reply2, cprReply()) {
		t.Fatalf("second chunk: reply=%q, want %q", reply2, cprReply())
	}
}

func TestTerminalQueryScannerDivergingPrefixReleased(t *testing.T) {
	s := &terminalQueryScanner{}
	// "\x1b[9" is a common CSI prefix but diverges from every recognized query at the 3rd byte;
	// it and the rest of the ordinary SGR sequence must pass through unchanged.
	clean1, reply1 := s.Scan([]byte("\x1b["))
	if len(clean1) != 0 || len(reply1) != 0 {
		t.Fatalf("first chunk: clean=%q reply=%q, want both empty (held as partial)", clean1, reply1)
	}
	clean2, reply2 := s.Scan([]byte("97m"))
	if len(reply2) != 0 {
		t.Fatalf("reply = %q, want empty (not a recognized query)", reply2)
	}
	if got := string(clean2); got != "\x1b[97m" {
		t.Fatalf("clean = %q, want %q", got, "\x1b[97m")
	}
}

func TestTerminalQueryScannerFlushReleasesUnresolvedPartial(t *testing.T) {
	s := &terminalQueryScanner{}
	s.Scan([]byte("\x1b[")) // held as a possible partial query, stream ends here
	if got := string(s.Flush()); got != "\x1b[" {
		t.Fatalf("Flush = %q, want %q", got, "\x1b[")
	}
	if got := string(s.Flush()); got != "" {
		t.Fatalf("second Flush = %q, want empty", got)
	}
}

func TestDA1ReplySignalsSixelOnlyWhenSupported(t *testing.T) {
	if got := string(da1Reply(true)); got != "\x1b[?62;4c" {
		t.Fatalf("da1Reply(true) = %q, want %q", got, "\x1b[?62;4c")
	}
	if got := string(da1Reply(false)); got != "\x1b[?6c" {
		t.Fatalf("da1Reply(false) = %q, want %q", got, "\x1b[?6c")
	}
}
