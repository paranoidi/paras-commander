package subshell

import "testing"

func TestQuoteArg(t *testing.T) {
	cases := map[string]string{
		"/plain":          "'/plain'",
		"/with space/dir": "'/with space/dir'",
		"/it's here":      `'/it'\''s here'`,
		"":                "''",
	}
	for in, want := range cases {
		if got := QuoteArg(in); got != want {
			t.Errorf("QuoteArg(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestChdirCommand(t *testing.T) {
	if got, want := string(chdirCommand("/tmp/maple leaf", 0)), " cd '/tmp/maple leaf'\n"; got != want {
		t.Errorf("chdirCommand(no ack) = %q, want %q", got, want)
	}
	if got, want := string(chdirCommand("/tmp/maple leaf", 7)), " cd '/tmp/maple leaf'; echo "+chdirAckPrefix+"7 >&3\n"; got != want {
		t.Errorf("chdirCommand(ack 7) = %q, want %q", got, want)
	}
}
