package compare

import "testing"

func TestRelDir(t *testing.T) {
	tests := []struct {
		rel  string
		want string
	}{
		{"nested/ledger.txt", "nested"},
		{"a/b/c/file.dat", "a/b/c"},
		{"ledger.txt", ""},
		{"", ""},
	}
	for _, tc := range tests {
		if got := RelDir(tc.rel); got != tc.want {
			t.Errorf("RelDir(%q) = %q, want %q", tc.rel, got, tc.want)
		}
	}
}

func TestRelBase(t *testing.T) {
	tests := []struct {
		rel  string
		want string
	}{
		{"nested/ledger.txt", "ledger.txt"},
		{"a/b/c/file.dat", "file.dat"},
		{"ledger.txt", "ledger.txt"},
	}
	for _, tc := range tests {
		if got := RelBase(tc.rel); got != tc.want {
			t.Errorf("RelBase(%q) = %q, want %q", tc.rel, got, tc.want)
		}
	}
}
