package ops

import "testing"

func TestFormatConflictSize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		n    int64
		want string
	}{
		{-1, "—"},
		{0, "0 (0 B)"},
		{512, "512 (512 B)"},
		{1536, "1536 (1.5 KiB)"},
		{12357674, "12357674 (11.8 MiB)"},
	}
	for _, tt := range tests {
		if got := FormatConflictSize(tt.n); got != tt.want {
			t.Errorf("FormatConflictSize(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}
