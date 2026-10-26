package dialog

import (
	"strconv"
	"testing"
)

func TestFormatDedupByteSizeBinaryOneDecimal(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1024 * 1024, "1.0 MiB"},
		{1024*1024 + 524288, "1.5 MiB"}, // MiB + 0.5 MiB
		{-1, "0 B"},
	}
	for _, tt := range tests {
		t.Run(strconv.FormatInt(tt.bytes, 10), func(t *testing.T) {
			if got := formatDedupByteSize(tt.bytes); got != tt.want {
				t.Fatalf("formatDedupByteSize(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}
