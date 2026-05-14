package panel

import "testing"

func TestListingFormatTOMLValue(t *testing.T) {
	t.Parallel()
	tests := []struct {
		f    ListFormat
		want string
	}{
		{ListFormatMtime, "mtime"},
		{ListFormatPerm, "perm"},
		{ListFormatBrief, "brief"},
		{ListFormat(99), "mtime"},
	}
	for _, tt := range tests {
		if got := ListingFormatTOMLValue(tt.f); got != tt.want {
			t.Fatalf("ListingFormatTOMLValue(%v) = %q, want %q", tt.f, got, tt.want)
		}
	}
}
