package cmdmacro

import (
	"strings"
	"testing"
)

func TestExpandCommandLineSpecialCharsInName(t *testing.T) {
	got, err := ExpandCommandLine(`gzip -9 %f`, Context{
		Active: &PanelSnapshot{
			Dir:         "/tmp",
			HasCurrent:  true,
			CurrentName: `/tmp/say "hi".gz`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"`) {
		t.Fatalf("expected quoted expansion, got %q", got)
	}
}

func TestExpandCommandLineRowPath(t *testing.T) {
	got, err := ExpandCommandLine(`wc -l < %f`, Context{RowPath: `/tmp/a b/spaced.txt`})
	if err != nil {
		t.Fatal(err)
	}
	if got != `wc -l < '/tmp/a b/spaced.txt'` {
		t.Fatalf("got %q", got)
	}
}

func TestExpandCommandLineShellHostileNames(t *testing.T) {
	dir := "/tmp/orchard"
	cases := []struct {
		name string
		path string
	}{
		{"command substitution", dir + "/beacon$(echo INJECTED)"},
		{"backticks", dir + "/lantern`echo INJECTED`"},
		{"dollar home", dir + "/meadow$HOME"},
		{"double quotes", dir + `/harbor "quoted"`},
		{"single quote", dir + "/harbor's-lantern"},
		{"spaces", dir + "/orchard meadow.txt"},
		{"embedded newline", dir + "/meadow\norchard"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ExpandCommandLine(`printf '%s\n' %f`, Context{RowPath: tc.path})
			if err != nil {
				t.Fatal(err)
			}
			want := `printf '%s\n' ` + QuoteShellArg(tc.path)
			if got != want {
				t.Fatalf("got %q want %q", got, want)
			}
		})
	}
}

func TestExpandArgvTokenStructuredValues(t *testing.T) {
	path := "/tmp/orchard/meadow\norchard"
	got, err := ExpandArgvToken("%f", Context{RowPath: path})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != path {
		t.Fatalf("got %#v", got)
	}
	flag, err := ExpandArgvToken("--file=%f", Context{RowPath: `/tmp/orchard meadow.txt`})
	if err != nil {
		t.Fatal(err)
	}
	if len(flag) != 1 || flag[0] != `--file=/tmp/orchard meadow.txt` {
		t.Fatalf("got %#v", flag)
	}
	tagged, err := ExpandArgvToken("%t", Context{
		Active: &PanelSnapshot{
			Dir:         "/tmp/orchard",
			HasCurrent:  true,
			CurrentName: "/tmp/orchard/cursor",
			TaggedInDir: []string{"/tmp/orchard/beacon", "/tmp/orchard/lantern"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(tagged) != 2 || tagged[0] != "/tmp/orchard/beacon" || tagged[1] != "/tmp/orchard/lantern" {
		t.Fatalf("got %#v", tagged)
	}
}

func TestQuoteShellArg(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"/plain", "'/plain'"},
		{"/with space/dir", "'/with space/dir'"},
		{"/it's here", `'/it'\''s here'`},
		{"", "''"},
		{"$(echo INJECTED)", "'$(echo INJECTED)'"},
		{"`echo INJECTED`", "'`echo INJECTED`'"},
		{"$HOME", "'$HOME'"},
		{"meadow\norchard", "'meadow\norchard'"},
	}
	for _, tc := range cases {
		if got := QuoteShellArg(tc.in); got != tc.want {
			t.Errorf("QuoteShellArg(%q) = %s, want %s", tc.in, got, tc.want)
		}
	}
}

func TestReplaceTerminalWidth(t *testing.T) {
	got := ReplaceTerminalWidth(`bat --tw=%w --w=%w`, 42)
	want := `bat --tw=42 --w=42`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
