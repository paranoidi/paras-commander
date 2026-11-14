package rsync

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// progressRe is an --info=progress2 line:
//
//	1,234,567  45%  12.3MB/s  0:01:02 (xfr#12, to-chk=34/100)
//
// The byte count carries the locale's thousands separators (any of . , ' and the no-break
// spaces), the rate its decimal mark and unit, so only the digits of the first field are read.
// The parenthesized part is missing from the first line of a run.
var progressRe = regexp.MustCompile(
	`^\s*(\d[\d.,'\s\x{00a0}\x{202f}]*)\s+(\d{1,3})%\s+\S+\s+\d+:\d\d:\d\d(?:\s+\((?:xfr#\d+,\s*)?(?:to|ir)-chk=(\d+)/(\d+)\))?\s*$`)

// progressLine is one parsed progress2 line.
type progressLine struct {
	Bytes   int64
	Percent int
	// TotalFiles is the number of files rsync will look at and DoneFiles how many it has gone
	// through; both 0 when the line carries no file counts.
	TotalFiles, DoneFiles int
}

// parseProgress parses a progress2 line; ok is false for anything else (a file name).
func parseProgress(line string) (progressLine, bool) {
	m := progressRe.FindStringSubmatch(line)
	if m == nil {
		return progressLine{}, false
	}
	var digits strings.Builder
	for _, r := range m[1] {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	b, err := strconv.ParseInt(digits.String(), 10, 64)
	if err != nil {
		return progressLine{}, false
	}
	p := progressLine{Bytes: b}
	p.Percent, _ = strconv.Atoi(m[2])
	if m[3] != "" {
		left, _ := strconv.Atoi(m[3])
		p.TotalFiles, _ = strconv.Atoi(m[4])
		p.DoneFiles = max(p.TotalFiles-left, 0)
	}
	return p, true
}

// TotalBytes estimates the byte total of the run. rsync prints the percentage truncated to a whole
// number, so bytes*100/(percent+1) is a lower bound that never has to shrink; the 100% line gives
// the exact total. 0 while under 1%, when the estimate would be too coarse to show.
func (p progressLine) TotalBytes() int64 {
	switch {
	case p.Percent >= 100:
		return p.Bytes
	case p.Percent >= 1:
		return p.Bytes * 100 / int64(p.Percent+1)
	}
	return 0
}

// splitLines is a bufio.SplitFunc that ends a token at \n or \r: rsync redraws its progress line
// with \r, so each redraw is a line of its own.
func splitLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	for i, b := range data {
		if b == '\n' || b == '\r' {
			return i + 1, data[:i], nil
		}
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// itemizeRe is an itemized change line of a dry run ("%i %n%L"): the 11-character change flags,
// or "*deleting", then the name.
var itemizeRe = regexp.MustCompile(`^(?:[<>ch.][a-zA-Z.][a-zA-Z.+? ]{9} |\*\w+ +)(.*)$`)

// itemizedName returns the name of an itemized change line; ok is false for any other line.
func itemizedName(line string) (name string, ok bool) {
	m := itemizeRe.FindStringSubmatch(line)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// exitMeanings are rsync's documented exit statuses (rsync(1), EXIT VALUES).
var exitMeanings = map[int]string{
	1:   "syntax or usage error",
	2:   "protocol incompatibility",
	3:   "errors selecting input/output files or directories",
	4:   "requested action not supported",
	5:   "error starting client-server protocol",
	6:   "daemon unable to append to log file",
	10:  "error in socket I/O",
	11:  "error in file I/O",
	12:  "error in rsync protocol data stream",
	13:  "errors with program diagnostics",
	14:  "error in IPC code",
	20:  "received SIGUSR1 or SIGINT",
	21:  "some error returned by waitpid()",
	22:  "error allocating core memory buffers",
	23:  "partial transfer due to error",
	24:  "partial transfer due to vanished source files",
	25:  "the --max-delete limit stopped deletions",
	30:  "timeout in data send/receive",
	35:  "timeout waiting for daemon connection",
	255: "unexplained error (ssh failed?)",
}

// exitMessage is the failure text of an rsync exit status.
func exitMessage(code int) string {
	if m, ok := exitMeanings[code]; ok {
		return fmt.Sprintf("rsync: %s (exit %d)", m, code)
	}
	return fmt.Sprintf("rsync failed (exit %d)", code)
}

// partialExit reports whether code means some files were transferred and others were not (the
// stderr lines then name the files, and are kept as the job's warnings).
func partialExit(code int) bool { return code == 23 || code == 24 }
