package rsync

import (
	"bufio"
	"reflect"
	"strings"
	"testing"
)

func TestParseProgress(t *testing.T) {
	tests := []struct {
		name string
		line string
		want progressLine
		ok   bool
	}{
		{"mid run", "      1,234,567  45%   12.3MB/s    0:01:02 (xfr#12, to-chk=34/100)", progressLine{1234567, 45, 100, 66}, true},
		{"first line has no file counts", "              3   0%    0.00kB/s    0:00:00  ", progressLine{Bytes: 3}, true},
		{"locale separators", "      3.000.003  99%    1,40GB/s    0:00:00 (xfr#2, to-chk=2/5)", progressLine{3000003, 99, 5, 3}, true},
		{"no-break space separators", "  1 234 567 100%  1,0MB/s  10:00:02 (xfr#9, to-chk=0/9)", progressLine{1234567, 100, 9, 9}, true},
		{"incremental recursion", "  4,096  10%  1.0kB/s  0:00:01 (xfr#1, ir-chk=1004/1010)", progressLine{4096, 10, 1010, 6}, true},
		{"file name", "lantern/willow.txt", progressLine{}, false},
		{"file name with a percent", "harbor 50% done 0:00:01.txt", progressLine{}, false},
		{"itemized line", ">f+++++++++ orchard", progressLine{}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseProgress(tc.line)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("parseProgress = %+v, %v; want %+v, %v", got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestProgressTotalBytesIsALowerBoundThatEndsExact(t *testing.T) {
	const total = 3_000_006
	for _, done := range []int64{40_000, 900_000, 3_000_003} {
		p := progressLine{Bytes: done, Percent: int(100 * done / total)}
		if got := p.TotalBytes(); got > total || (p.Percent >= 1 && got < done) {
			t.Fatalf("done %d (%d%%): total estimate %d, want within [done, %d]", done, p.Percent, got, total)
		}
	}
	if got := (progressLine{Bytes: total, Percent: 100}).TotalBytes(); got != total {
		t.Fatalf("100%% total = %d, want %d", got, total)
	}
	if got := (progressLine{Bytes: 5, Percent: 0}).TotalBytes(); got != 0 {
		t.Fatalf("under 1%% total = %d, want unknown", got)
	}
}

func TestSplitLinesEndsOnCarriageReturnAndNewline(t *testing.T) {
	stream := "meadow.txt\n\r  3  0%  0.00kB/s  0:00:00 \r  3  0%  1.00kB/s  0:00:00 (xfr#1, to-chk=3/5)\nwillow/\n  9 100% 1.0kB/s 0:00:01 (xfr#2, to-chk=0/5)"
	sc := bufio.NewScanner(strings.NewReader(stream))
	sc.Split(splitLines)
	var got []string
	for sc.Scan() {
		if sc.Text() != "" {
			got = append(got, sc.Text())
		}
	}
	want := []string{
		"meadow.txt",
		"  3  0%  0.00kB/s  0:00:00 ",
		"  3  0%  1.00kB/s  0:00:00 (xfr#1, to-chk=3/5)",
		"willow/",
		"  9 100% 1.0kB/s 0:00:01 (xfr#2, to-chk=0/5)",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lines = %q\nwant    %q", got, want)
	}
}

func TestItemizedName(t *testing.T) {
	for line, want := range map[string]string{
		">f+++++++++ lantern.txt":       "lantern.txt",
		".d..t...... ./":                "./",
		">f.st...... orchard/a b":       "orchard/a b",
		"*deleting   thistle.txt":       "thistle.txt",
		"cd+++++++++ harbor/":           "harbor/",
		"sending incremental file list": "",
		"lantern.txt":                   "",
	} {
		got, ok := itemizedName(line)
		if ok != (want != "") || got != want {
			t.Fatalf("itemizedName(%q) = %q, %v; want %q", line, got, ok, want)
		}
	}
}

func TestProgress2FromVersion(t *testing.T) {
	for out, want := range map[string]bool{
		"rsync  version 3.2.7  protocol version 31\nCopyright (C) 1996-2022 by Andrew Tridgell": true,
		"rsync  version 3.1.0  protocol version 31":                                             true,
		"rsync  version 3.0.9  protocol version 30":                                             false,
		"openrsync: protocol version 29\nrsync version 2.6.9 compatible":                        false,
		"rsync  version 4.0.0  protocol version 40":                                             true,
		"something else": false,
	} {
		if got := progress2FromVersion(out); got != want {
			t.Fatalf("progress2FromVersion(%q) = %v, want %v", out, got, want)
		}
	}
}

func TestExitMessage(t *testing.T) {
	if got := exitMessage(23); got != "rsync: partial transfer due to error (exit 23)" {
		t.Fatalf("23 = %q", got)
	}
	if got := exitMessage(11); !strings.Contains(got, "file I/O") {
		t.Fatalf("11 = %q", got)
	}
	if got := exitMessage(99); got != "rsync failed (exit 99)" {
		t.Fatalf("99 = %q", got)
	}
	if !partialExit(23) || !partialExit(24) || partialExit(11) {
		t.Fatal("only 23 and 24 are partial transfers")
	}
}
