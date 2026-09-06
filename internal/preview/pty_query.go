package preview

import "bytes"

// terminalQueryScanner strips a small set of terminal capability queries out of a PTY-connected
// child's output stream and answers them inline, so a tool that checks with the terminal before
// deciding whether to draw graphics (observed: movie-info sending DA1 then CPR) gets a real
// reply instead of hanging/timing out against cmdrun.Run's disconnected pipe. It recognizes
// exactly three literal queries — DA1 ("\x1b[c" / "\x1b[0c") and CPR ("\x1b[6n") — not a general
// escape-sequence parser.
//
// ponytail: three known literal queries, not a full DEC/xterm query grammar. This scanner only
// answers queries a child sends before deciding what to draw — movie-info's own Kitty graphics
// support (chunked transmission, Unicode-placeholder rewriting, PNG-header dimension recovery)
// lives in rules.go and needed no query answered here: it never sends Kitty's own capability
// probe ("\x1b_Gi=<id>,a=q;...\x1b\\", expecting an OK/error reply), only DA1/CPR. Extend with
// that pattern here if a real tool turns out to need it answered.
type terminalQueryScanner struct {
	sixelOK bool
	carry   []byte
}

// da1Reply answers a DA1 "Send Device Attributes" query. Real terminals reply "\x1b[?<params>c";
// param 4 is the DEC-defined signal for Sixel graphics support. There is no equivalent DA1 param
// for Kitty's graphics protocol, so sixelOK is the only capability this can honestly advertise.
func da1Reply(sixelOK bool) []byte {
	if sixelOK {
		return []byte("\x1b[?62;4c")
	}
	return []byte("\x1b[?6c")
}

// cprReply answers a CPR "Cursor Position Report" query with a fixed row/col.
//
// ponytail: static reply, not a real cursor tracker — a tool using CPR deltas to measure how
// many rows a just-drawn image occupied will get a wrong measurement. Revisit if that surfaces.
func cprReply() []byte {
	return []byte("\x1b[1;1R")
}

// knownQueries lists every literal query Scan recognizes, paired with the reply it answers with
// (sixelOK-dependent for the two DA1 spellings). Checked in order, so both the full-match switch
// and the partial-prefix check share one list instead of drifting.
var knownQueries = []struct {
	query []byte
	reply func(sixelOK bool) []byte
}{
	{[]byte("\x1b[0c"), da1Reply},
	{[]byte("\x1b[c"), da1Reply},
	{[]byte("\x1b[6n"), func(bool) []byte { return cprReply() }},
}

// partialQueryPrefixLen reports whether rest could still grow into one of knownQueries: it
// matches a query's prefix exactly and hasn't reached that query's full length yet (a
// full-length match is caught by the caller before this is ever consulted).
func partialQueryPrefixLen(rest []byte) int {
	for _, kq := range knownQueries {
		if len(rest) < len(kq.query) && bytes.Equal(rest, kq.query[:len(rest)]) {
			return len(rest)
		}
	}
	return 0
}

// Scan strips any recognized query out of chunk and returns the cleaned bytes (ready to append
// to the captured output) plus the reply bytes to write back to the pty master, in the order
// encountered. A trailing partial match — a chunk boundary landing inside a query — is held in
// carry and resolved on the next Scan call, or released as-is by Flush. Bytes that are not part
// of a recognized query, including a lone ESC that never completes into one, pass through
// unchanged into clean.
func (s *terminalQueryScanner) Scan(chunk []byte) (clean, reply []byte) {
	buf := chunk
	if len(s.carry) > 0 {
		buf = append(s.carry, chunk...)
		s.carry = nil
	}
	clean = make([]byte, 0, len(buf))
	i := 0
	for i < len(buf) {
		if buf[i] != 0x1b {
			clean = append(clean, buf[i])
			i++
			continue
		}
		rest := buf[i:]
		matched := false
		for _, kq := range knownQueries {
			if bytes.HasPrefix(rest, kq.query) {
				reply = append(reply, kq.reply(s.sixelOK)...)
				i += len(kq.query)
				matched = true
				break
			}
		}
		switch {
		case matched:
		case partialQueryPrefixLen(rest) > 0:
			s.carry = append(s.carry, rest...)
			return clean, reply
		default:
			clean = append(clean, buf[i])
			i++
		}
	}
	return clean, reply
}

// Flush releases any bytes held back as a possible partial query — call once no more input is
// coming (e.g. the child exited) so they aren't silently dropped.
func (s *terminalQueryScanner) Flush() []byte {
	out := s.carry
	s.carry = nil
	return out
}
