package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunGitIndex(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}
	root := t.TempDir()
	git := func(args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	git("config", "user.email", "t@example.com")
	git("config", "user.name", "T")
	f := filepath.Join(root, "harbor-lantern.txt")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runGitIndex(root, gitStage, []string{f}); err != nil {
		t.Fatal(err)
	}
	if got := git("diff", "--cached", "--name-only"); got != "harbor-lantern.txt" {
		t.Fatalf("staged = %q", got)
	}
	if err := runGitIndex(root, gitUnstage, []string{f}); err != nil {
		t.Fatal(err)
	}
	if got := git("diff", "--cached", "--name-only"); got != "" {
		t.Fatalf("still staged: %q", got)
	}
	git("add", "--", f)
	git("commit", "-q", "-m", "init")
	if err := runGitIndex(root, gitUntrack, []string{f}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(f); err != nil {
		t.Fatalf("untrack removed the file from disk: %v", err)
	}
	if got := git("ls-files", "--cached"); got != "" {
		t.Fatalf("still tracked: %q", got)
	}
}
