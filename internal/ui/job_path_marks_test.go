package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/jobs"
)

func TestPathEqualOrUnder(t *testing.T) {
	t.Parallel()
	root := filepath.Join("/tmp", "example", "foo")
	if !pathEqualOrUnder(root, root) {
		t.Fatal("root should match itself")
	}
	child := filepath.Join(root, "bar", "asdf")
	if !pathEqualOrUnder(root, child) {
		t.Fatal("child under root")
	}
	if pathEqualOrUnder(root, filepath.Join("/tmp", "example", "other")) {
		t.Fatal("sibling path should not match")
	}
	if pathEqualOrUnder(filepath.Join("/tmp", "example", "foobar"), filepath.Join("/tmp", "example", "foo")) {
		t.Fatal("prefix segment boundary: foobar is not under foo")
	}
}

func TestEntryPathMarkedByJobs_finishedIgnored(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "src")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(root, "x")
	j := JobEntry{
		Status:      string(jobs.StatusCompleted),
		Sources:     []string{root},
		Destination: t.TempDir(),
	}
	if EntryPathMarkedByJobs(child, []JobEntry{j}) {
		t.Fatal("finished job should not mark")
	}
}

func TestEntryPathMarkedByJobs_sourceSubtree(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	root := filepath.Join(tmp, "foo")
	if err := os.MkdirAll(filepath.Join(root, "bar", "asdf"), 0o755); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(tmp, "dst")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	j := JobEntry{
		Status:      string(jobs.StatusQueued),
		Sources:     []string{root},
		Destination: dst,
		DestIsDir:   true,
	}
	if !EntryPathMarkedByJobs(root, []JobEntry{j}) {
		t.Fatal("source root")
	}
	if !EntryPathMarkedByJobs(filepath.Join(root, "bar", "asdf"), []JobEntry{j}) {
		t.Fatal("nested under source")
	}
	if EntryPathMarkedByJobs(filepath.Join(tmp, "other"), []JobEntry{j}) {
		t.Fatal("outside tree")
	}
}

func TestEntryPathMarkedByJobs_destinationSubtree(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	src := filepath.Join(tmp, "proj")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	dstParent := filepath.Join(tmp, "out")
	if err := os.MkdirAll(dstParent, 0o755); err != nil {
		t.Fatal(err)
	}
	// ResolveDestination joins basename(src) under dstParent.
	dstRoot := filepath.Join(dstParent, filepath.Base(src))
	if err := os.MkdirAll(filepath.Join(dstRoot, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	j := JobEntry{
		Status:      string(jobs.StatusRunning),
		Sources:     []string{src},
		Destination: dstParent,
		DestIsDir:   true,
	}
	if !EntryPathMarkedByJobs(dstRoot, []JobEntry{j}) {
		t.Fatal("resolved dest root")
	}
	if !EntryPathMarkedByJobs(filepath.Join(dstRoot, "nested"), []JobEntry{j}) {
		t.Fatal("under resolved dest root")
	}
	if EntryPathMarkedByJobs(filepath.Join(dstParent, "unrelated"), []JobEntry{j}) {
		t.Fatal("sibling under dst parent not part of this job")
	}
}
