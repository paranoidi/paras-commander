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
