package primitive

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestFitPathForWidthPreservesBasenameTypicalColumn(t *testing.T) {
	const longRel = "langfilter/.venv/lib/python3.13/site-packages/pygments/lexers/snobol.py"
	const width = 55
	got := FitPathForWidth(longRel, width)
	if !strings.HasSuffix(got, "snobol.py") {
		t.Fatalf("basename must remain intact: got %q", got)
	}
	if utf8.RuneCountInString(got) > width {
		t.Fatalf("length %d > width %d: %q", utf8.RuneCountInString(got), width, got)
	}
	if strings.ContainsRune(got, '~') {
		t.Fatalf("path fit should not use tilde marker: %q", got)
	}
}

func TestFitPathForWidthShortColumnStillKeepsExtensionTail(t *testing.T) {
	const longRel = "langfilter/.venv/lib/python3.13/site-packages/pygments/lexers/snobol.py"
	got := FitPathForWidth(longRel, 40)
	if !strings.HasSuffix(got, "snobol.py") {
		t.Fatalf("want suffix snobol.py, got %q", got)
	}
	if utf8.RuneCountInString(got) > 40 {
		t.Fatalf("overflow: %q len=%d", got, utf8.RuneCountInString(got))
	}
}

func TestFitPathForWidthBasenameOnlyLongerThanMax(t *testing.T) {
	got := FitPathForWidth("dir/abcdefghijklmnopqrstuvwxyz0123456789.go", 12)
	if utf8.RuneCountInString(got) != 12 {
		t.Fatalf("want length 12, got %d (%q)", utf8.RuneCountInString(got), got)
	}
	if !strings.ContainsRune(got, Ellipsis) {
		t.Fatalf("expected middle ellipsis: %q", got)
	}
}

func TestFitPathForWidthSingleSegment(t *testing.T) {
	if got := FitPathForWidth("readme.txt", 80); got != "readme.txt" {
		t.Fatalf("got %q", got)
	}
	if got := FitPathForWidth("readme.txt", 10); got != "readme.txt" {
		t.Fatalf("exact fit: got %q", got)
	}
}

func TestFitPathForWidthAbsoluteRoot(t *testing.T) {
	if got := FitPathForWidth("/", 1); got != "/" {
		t.Fatalf("got %q", got)
	}
	if got := FitPathForWidth("/", 0); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestFitPathForWidthHomePrefixOnly(t *testing.T) {
	if got := FitPathForWidth("~/", 2); got != "~/" {
		t.Fatalf("got %q", got)
	}
}

func TestFitPathForWidthEllipsisInTruncation(t *testing.T) {
	got := FitPathForWidth("aaaa/bbbb/cccc", 7)
	if utf8.RuneCountInString(got) != 7 {
		t.Fatalf("len=%d got %q", utf8.RuneCountInString(got), got)
	}
}

func TestFitPathForWidthEmptyPath(t *testing.T) {
	for _, w := range []int{1, 4, 20} {
		if got := FitPathForWidth("", w); got != "" {
			t.Fatalf("FitPathForWidth('', %d) = %q, want empty", w, got)
		}
	}
}

func TestFitPathForWidthMaxZero(t *testing.T) {
	if got := FitPathForWidth("any/path", 0); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestFitPathForWidthAbsolutePath(t *testing.T) {
	p := "/usr/share/doc/readme"
	got := FitPathForWidth(p, 80)
	if got != p {
		t.Fatalf("got %q", got)
	}
	narrow := FitPathForWidth(p, 18)
	if !strings.HasSuffix(narrow, "readme") {
		t.Fatalf("want basename readme: %q", narrow)
	}
	if !strings.HasPrefix(narrow, "/") {
		t.Fatalf("want absolute prefix: %q", narrow)
	}
}
