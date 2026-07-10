package primitive

import "testing"

func TestStripCommonPathPrefix(t *testing.T) {
	home := "/home/alice"
	tests := []struct {
		name      string
		left      string
		right     string
		wantLeft  string
		wantRight string
	}{
		{
			name:      "compare roots under shared parent",
			left:      PathWithHomeTilde("/home/alice/projects/paras-commander/test-cases/diff-b", home),
			right:     PathWithHomeTilde("/home/alice/projects/paras-commander/test-cases/diff-a", home),
			wantLeft:  "diff-b",
			wantRight: "diff-a",
		},
		{
			name:      "files under different compare roots",
			left:      PathWithHomeTilde("/home/alice/projects/paras-commander/test-cases/diff-b/foo.txt", home),
			right:     PathWithHomeTilde("/home/alice/projects/paras-commander/test-cases/diff-a/foo.txt", home),
			wantLeft:  "diff-b/foo.txt",
			wantRight: "diff-a/foo.txt",
		},
		{
			name:      "identical paths",
			left:      "~/work/repo/file.go",
			right:     "~/work/repo/file.go",
			wantLeft:  "",
			wantRight: "",
		},
		{
			name:      "empty side unchanged",
			left:      "~/solo/file.txt",
			right:     "",
			wantLeft:  "~/solo/file.txt",
			wantRight: "",
		},
		{
			name:      "no common segments",
			left:      "/left-root/alpha.txt",
			right:     "/right-root/beta.txt",
			wantLeft:  "/left-root/alpha.txt",
			wantRight: "/right-root/beta.txt",
		},
		{
			name:      "parent and child",
			left:      "~/a/b",
			right:     "~/a/b/c/file.txt",
			wantLeft:  "",
			wantRight: "c/file.txt",
		},
		{
			name:      "different prefix roots",
			left:      "~/a/b",
			right:     "/home/alice/a/b",
			wantLeft:  "~/a/b",
			wantRight: "/home/alice/a/b",
		},
		{
			name:      "relocated different leaf names",
			left:      "~/proj/cases/diff-b/old.txt",
			right:     "~/proj/cases/diff-a/new.txt",
			wantLeft:  "diff-b/old.txt",
			wantRight: "diff-a/new.txt",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotLeft, gotRight := StripCommonPathPrefix(tc.left, tc.right)
			if gotLeft != tc.wantLeft || gotRight != tc.wantRight {
				t.Fatalf("StripCommonPathPrefix(%q, %q) = (%q, %q), want (%q, %q)",
					tc.left, tc.right, gotLeft, gotRight, tc.wantLeft, tc.wantRight)
			}
		})
	}
}

func TestCommonDisplayPathPrefix(t *testing.T) {
	home := "/home/alice"
	tests := []struct {
		name  string
		left  string
		right string
		want  string
	}{
		{
			name:  "compare roots under shared parent",
			left:  PathWithHomeTilde("/home/alice/projects/paras-commander/test-cases/diff-b", home),
			right: PathWithHomeTilde("/home/alice/projects/paras-commander/test-cases/diff-a", home),
			want:  "~/projects/paras-commander/test-cases/",
		},
		{
			name:  "no common segments",
			left:  "/left-root/alpha",
			right: "/right-root/beta",
			want:  "",
		},
		{
			name:  "parent and child",
			left:  "~/a/b",
			right: "~/a/b/c/file.txt",
			want:  "~/a/b/",
		},
		{
			name:  "different prefix roots",
			left:  "~/a/b",
			right: "/home/alice/a/b",
			want:  "",
		},
		{
			name:  "empty side",
			left:  "~/solo",
			right: "",
			want:  "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CommonDisplayPathPrefix(tc.left, tc.right)
			if got != tc.want {
				t.Fatalf("CommonDisplayPathPrefix(%q, %q) = %q, want %q", tc.left, tc.right, got, tc.want)
			}
		})
	}
}
