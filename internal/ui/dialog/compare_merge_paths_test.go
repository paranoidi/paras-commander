package dialog

import "testing"

func TestFormatCompareMergePaths(t *testing.T) {
	home := "/home/alice"
	tests := []struct {
		name       string
		primary    string
		secondary  string
		wantShared string
		wantLeft   string
		wantRight  string
	}{
		{
			name:       "compare roots under shared parent",
			primary:    "/home/alice/projects/paras-commander/test-cases/diff-b",
			secondary:  "/home/alice/projects/paras-commander/test-cases/diff-a",
			wantShared: "~/projects/paras-commander/test-cases/",
			wantLeft:   "diff-b",
			wantRight:  "diff-a",
		},
		{
			name:       "no common segments",
			primary:    "/left-root/alpha",
			secondary:  "/right-root/beta",
			wantShared: "",
			wantLeft:   "/left-root/alpha",
			wantRight:  "/right-root/beta",
		},
		{
			name:       "ancestor and descendant",
			primary:    "/home/alice/a/b",
			secondary:  "/home/alice/a/b/c/file.txt",
			wantShared: "~/a/b/",
			wantLeft:   compareMergePathHereLabel,
			wantRight:  "c/file.txt",
		},
		{
			name:       "identical paths",
			primary:    "/home/alice/work/repo",
			secondary:  "/home/alice/work/repo",
			wantShared: "",
			wantLeft:   "~/work/repo",
			wantRight:  "~/work/repo",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotShared, gotLeft, gotRight := FormatCompareMergePaths(tc.primary, tc.secondary, home)
			if gotShared != tc.wantShared || gotLeft != tc.wantLeft || gotRight != tc.wantRight {
				t.Fatalf("FormatCompareMergePaths(%q, %q) = (%q, %q, %q), want (%q, %q, %q)",
					tc.primary, tc.secondary, gotShared, gotLeft, gotRight, tc.wantShared, tc.wantLeft, tc.wantRight)
			}
		})
	}
}
