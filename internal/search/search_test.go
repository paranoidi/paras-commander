package search

import "testing"

func TestMatchFuzzyCaseInsensitive(t *testing.T) {
	query := Parse("rdm")
	result := query.Match("README.md", Options{CaseInsensitive: true})
	if !result.Matched {
		t.Fatal("Match() matched = false, want true")
	}
	want := []Range{{Start: 0, End: 1}, {Start: 3, End: 5}}
	assertRanges(t, result.Ranges, want)
}

func TestMatchPrefixSuffixExactAndInverseTerms(t *testing.T) {
	tests := []struct {
		name  string
		query string
		value string
		want  bool
	}{
		{name: "prefix", query: "^doc", value: "docs", want: true},
		{name: "suffix", query: ".go$", value: "panel_test.go", want: true},
		{name: "exact", query: "'main", value: "cmd-main.go", want: true},
		{name: "inverse excludes", query: "!test", value: "panel_test.go", want: false},
		{name: "inverse keeps", query: "!test", value: "panel.go", want: true},
		{name: "and requires all", query: "^main .go$", value: "main_test.go", want: true},
		{name: "and rejects missing term", query: "^main .go$", value: "not-main.go", want: false},
		// fzf parse order: $ stripped before ', so '.git$' ≡ exact '.git'
		{name: "quoted-suffix is exact not literal-dollar", query: "'.git$", value: "repo/.git", want: true},
		{name: "quoted-suffix exact rejects non-substring", query: "'.git$", value: "repo/git", want: false},
		{name: "quoted-suffix matches gitignore too", query: "'.git$", value: "repo/.gitignore", want: true},
		{name: "true suffix matches .git", query: ".git$", value: "repo/.git", want: true},
		{name: "true suffix rejects .gitignore", query: ".git$", value: "repo/.gitignore", want: false},
		{name: "true suffix matches file.git", query: ".git$", value: "repo/file.git", want: true},
		{name: "equal whole string", query: "^foo$", value: "foo", want: true},
		{name: "equal rejects prefix-only", query: "^foo$", value: "foobar", want: false},
		{name: "equal rejects suffix-only", query: "^foo$", value: "xfoo", want: false},
		{name: "boundary match spaced", query: "'foo'", value: "xxx foo xxx", want: true},
		{name: "boundary match underscore", query: "'foo'", value: "xxx_foo_xxx", want: true},
		{name: "boundary rejects glued", query: "'foo'", value: "xxxfooxxx", want: false},
		{name: "inverse suffix excludes", query: "!.git$", value: "repo/.git", want: false},
		{name: "inverse suffix keeps", query: "!.git$", value: "repo/.gitignore", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Parse(tt.query).Match(tt.value, Options{CaseInsensitive: true}).Matched
			if got != tt.want {
				t.Fatalf("Match() matched = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchOrGroupsAndEscapedSpaces(t *testing.T) {
	tests := []struct {
		name  string
		query string
		value string
		want  bool
	}{
		{name: "or first alternative", query: "walnut | harbor", value: "walnut.txt", want: true},
		{name: "or second alternative", query: "walnut | harbor", value: "harbor.txt", want: true},
		{name: "or neither alternative", query: "walnut | harbor", value: "lantern.txt", want: false},
		{name: "or with and, left alternative", query: "^core | ^lib .go$", value: "lib_walnut.go", want: true},
		{name: "or with and, right term fails", query: "^core | ^lib .go$", value: "lib_walnut.rb", want: false},
		{name: "or with and, group fails", query: "^core | ^lib .go$", value: "walnut.go", want: false},
		{name: "or suffixes", query: "^core go$ | rb$ | py$", value: "core_walnut.py", want: true},
		{name: "or suffixes still need prefix", query: "^core go$ | rb$ | py$", value: "walnut.py", want: false},
		{name: "or suffixes reject others", query: "^core go$ | rb$ | py$", value: "core_walnut.txt", want: false},
		{name: "inverse satisfied by absence", query: "!harbor | ^core", value: "walnut.go", want: true},
		{name: "inverse unsatisfied, other fails", query: "!harbor | ^core", value: "harbor_walnut.go", want: false},
		{name: "other term rescues matched inverse", query: "!harbor | ^core", value: "core_harbor.go", want: true},
		{name: "leading pipe ignored", query: "| walnut", value: "walnut", want: true},
		{name: "leading pipe does not widen", query: "| walnut", value: "harbor", want: false},
		{name: "trailing pipe ignored", query: "walnut |", value: "harbor", want: false},
		{name: "doubled pipe joins neighbours", query: "walnut | | harbor", value: "harbor", want: true},
		{name: "attached pipe is literal match", query: "a|b", value: "a|b", want: true},
		{name: "attached pipe is not or", query: "a|b", value: "b", want: false},
		{name: "escaped space matches space", query: `lantern\ walnut`, value: "lantern walnut.txt", want: true},
		{name: "escaped space is not a separator", query: `lantern\ walnut`, value: "lantern_walnut.txt", want: false},
		{name: "plain space still separates", query: "lantern walnut", value: "lantern_walnut.txt", want: true},
		{name: "escaped space with exact prefix", query: `'lantern\ walnut`, value: "old lantern walnut.txt", want: true},
		{name: "escaped space with exact prefix rejects scatter", query: `'lantern\ walnut`, value: "lantern x walnut.txt", want: false},
		{name: "escaped space before pipe is not operator", query: `\ |`, value: "a |b", want: true},
		{name: "other backslash stays literal", query: `lantern\walnut`, value: `lantern\walnut`, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Parse(tt.query).Match(tt.value, Options{CaseInsensitive: true}).Matched
			if got != tt.want {
				t.Fatalf("Parse(%q).Match(%q) matched = %v, want %v", tt.query, tt.value, got, tt.want)
			}
		})
	}
}

func TestEmptyAfterStrayPipes(t *testing.T) {
	for _, q := range []string{"|", "| |", "  |  ", "! |"} {
		if !Parse(q).Empty() {
			t.Fatalf("Parse(%q).Empty() = false, want true", q)
		}
	}
}

func TestMatchOrRangesUseBestAlternative(t *testing.T) {
	value := "core_walnut.rb"
	// The prefix alternative outscores the suffix one, so only its range is reported.
	got := Parse("rb$ | ^core").Match(value, Options{CaseInsensitive: true})
	if !got.Matched {
		t.Fatal("Match() matched = false, want true")
	}
	assertRanges(t, got.Ranges, []Range{{Start: 0, End: 4}})
	want := Parse("^core").Match(value, Options{CaseInsensitive: true}).Score
	if got.Score != want {
		t.Fatalf("Score = %d, want best alternative's %d", got.Score, want)
	}
}

func TestMatchOrInverseOnlySatisfactionScoresNothing(t *testing.T) {
	got := Parse("!harbor | ^core").Match("walnut.go", Options{CaseInsensitive: true})
	if !got.Matched || got.Score != 0 || len(got.Ranges) != 0 {
		t.Fatalf("Match() = %+v, want matched with zero score and no ranges", got)
	}
}

func TestRankOrdersBetterFuzzyMatchesFirst(t *testing.T) {
	results := Parse("abc").Rank([]string{
		"a-long-boring-candidate",
		"abc.txt",
		"x_a_b_c",
	}, Options{CaseInsensitive: true})

	if len(results) != 3 {
		t.Fatalf("len(results) = %d, want 3", len(results))
	}
	if results[0].Index != 1 {
		t.Fatalf("best result index = %d, want 1 for compact early match", results[0].Index)
	}
}

func TestRankPrefersContiguousMatchOverEarlierLooseMatch(t *testing.T) {
	results := Parse("cop").Rank([]string{
		"gen-compare-diff.sh",
		"dedupe-copy",
		"gen-conflict-copy-move.sh",
	}, Options{CaseInsensitive: true})

	if len(results) != 3 {
		t.Fatalf("len(results) = %d, want 3", len(results))
	}
	if results[0].Index != 1 {
		t.Fatalf("best result index = %d, want 1 for contiguous match dedupe-copy", results[0].Index)
	}
}

func TestRankPrefersContiguousMatchOverEarlierScatteredMatch(t *testing.T) {
	// "Duck And Morty": a5 n6 d7 o10 r11 (loose, span 7) vs
	// "Paw Wars Andor": stray a1, then contiguous "Andor" at A9 n10 d11 o12 r13.
	results := Parse("andor").Rank([]string{
		"Duck And Morty",
		"Paw Wars Andor",
	}, Options{CaseInsensitive: true})

	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if results[0].Index != 1 {
		t.Fatalf("best result index = %d, want 1 for contiguous match in Paw Wars Andor", results[0].Index)
	}
	assertRanges(t, results[0].Result.Ranges, []Range{{Start: 9, End: 14}})
}

func TestEmptyQueryMatchesInOriginalOrder(t *testing.T) {
	results := Parse("   ").Rank([]string{"b", "a"}, Options{CaseInsensitive: true})
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if results[0].Index != 0 || results[1].Index != 1 {
		t.Fatalf("result indexes = %d,%d, want original order 0,1", results[0].Index, results[1].Index)
	}
}

func assertRanges(t *testing.T, got, want []Range) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len(ranges) = %d, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ranges[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}
