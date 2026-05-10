package jobs

import (
	"fmt"
	"strings"
	"time"
)

// FormatHumanDuration formats elapsed wall time for jobs UI (e.g. "1h 20min", "45s", "<1s").
func FormatHumanDuration(d time.Duration) string {
	orig := d
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	sec := int(d / time.Second)
	if sec == 0 {
		if orig > 0 {
			return "<1s"
		}
		return "0s"
	}
	h := sec / 3600
	sec %= 3600
	m := sec / 60
	s := sec % 60
	var parts []string
	if h > 0 {
		parts = append(parts, fmt.Sprintf("%dh", h))
	}
	if m > 0 {
		parts = append(parts, fmt.Sprintf("%dmin", m))
	}
	if s > 0 {
		parts = append(parts, fmt.Sprintf("%ds", s))
	}
	return strings.Join(parts, " ")
}
