package dialog

import "testing"

func TestDefaultBookmarkName(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{path: "/", want: "root"},
		{path: "/home/user/projects", want: "projects"},
		{path: ".", want: "root"},
		{path: "", want: "root"},
	}
	for _, tt := range tests {
		if got := DefaultBookmarkName(tt.path); got != tt.want {
			t.Fatalf("DefaultBookmarkName(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
