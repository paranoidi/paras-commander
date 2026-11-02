package findcrit

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Age units: s, m (minutes), h, d, w, mo (30 days), y (365 days).
var ageUnits = map[string]time.Duration{
	"s":  time.Second,
	"m":  time.Minute,
	"h":  time.Hour,
	"d":  24 * time.Hour,
	"w":  7 * 24 * time.Hour,
	"mo": 30 * 24 * time.Hour,
	"y":  365 * 24 * time.Hour,
}

var (
	ageTermRx  = regexp.MustCompile(`(\d+(?:\.\d+)?)(mo|[smhdwy])`)
	ageWholeRx = regexp.MustCompile(`^(?:\d+(?:\.\d+)?(?:mo|[smhdwy]))+$`)
	sizeRx     = regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*([kmgtp]?)(i?b)?$`)
	ageLayouts = []string{"2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"}
)

// ParseAge turns an age field into the cutoff instant it names (unix nanoseconds): a relative
// age before now ("30m", "2h", "3d", "2w", "6mo", "1y", or a sum such as "1w3d") or a local
// date ("2026-10-01", "2026-10-01 14:30"). An empty field returns 0 and no error.
func ParseAge(s string, now time.Time) (int64, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return 0, nil
	}
	if ageWholeRx.MatchString(s) {
		var total time.Duration
		for _, m := range ageTermRx.FindAllStringSubmatch(s, -1) {
			n, err := strconv.ParseFloat(m[1], 64)
			if err != nil {
				return 0, fmt.Errorf("age %q: %w", s, err)
			}
			total += time.Duration(n * float64(ageUnits[m[2]]))
		}
		return now.Add(-total).UnixNano(), nil
	}
	for _, layout := range ageLayouts {
		if t, err := time.ParseInLocation(layout, s, now.Location()); err == nil {
			return t.UnixNano(), nil
		}
	}
	return 0, fmt.Errorf("age %q: want 30m, 2h, 3d, 2w, 6mo, 1y or YYYY-MM-DD", s)
}

// ParseSize turns a size field into bytes: a number with an optional binary unit (k, M, G, T,
// P; "kb", "KiB" and "K" all mean 1024). An empty field returns ok false and no error.
func ParseSize(s string) (n int64, ok bool, err error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return 0, false, nil
	}
	m := sizeRx.FindStringSubmatch(s)
	if m == nil {
		return 0, false, fmt.Errorf("size %q: want a number with an optional k, M, G, T unit", s)
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false, fmt.Errorf("size %q: %w", s, err)
	}
	shift := 0
	if m[2] != "" {
		shift = strings.Index("kmgtp", m[2]) + 1
	}
	v *= math.Pow(1024, float64(shift))
	if v > math.MaxInt64 {
		return 0, false, fmt.Errorf("size %q: too large", s)
	}
	return int64(v), true, nil
}
