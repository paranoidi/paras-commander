package jobs

import (
	"testing"
	"time"
)

func TestFormatHumanDuration(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		d    time.Duration
		want string
	}{
		{name: "zero", d: 0, want: "0s"},
		{name: "subsecond", d: 400 * time.Millisecond, want: "<1s"},
		{name: "seconds_only", d: 45 * time.Second, want: "45s"},
		{name: "minute_second", d: 90 * time.Second, want: "1min 30s"},
		{name: "minutes_only", d: 20 * time.Minute, want: "20min"},
		{name: "hour_minute", d: time.Hour + 20*time.Minute, want: "1h 20min"},
		{name: "hour_second_remainder", d: time.Hour + 5*time.Second, want: "1h 5s"},
		{name: "exact_hour", d: time.Hour, want: "1h"},
		{name: "negative_clamped", d: -time.Minute, want: "0s"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := FormatHumanDuration(tc.d)
			if got != tc.want {
				t.Fatalf("FormatHumanDuration(%v) = %q, want %q", tc.d, got, tc.want)
			}
		})
	}
}
