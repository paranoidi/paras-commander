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
