package gitstatus

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"
)

const gitStatusTimeout = 60 * time.Second

// GitCommand runs git status --porcelain for tests and production.
var GitCommand = defaultGitCommand

// defaultGitCommand scopes the status scan to listDir via a pathspec when it's a strict
// subdirectory of workRoot, rather than always scanning the whole repository. With --ignored,
// an unscoped `git status` must enumerate every ignored file repo-wide (e.g. a large
// node_modules/vendor/build directory, or one backed by slow network storage) just to answer a
// query about one listing directory; every caller here only ever looks up paths within listDir
// (see StatusesForListing/mapFromSnapshot), so the scoped and unscoped results are identical for
// everything actually used — the pathspec just skips work git would otherwise do to describe
// parts of the tree nobody asked about.
func defaultGitCommand(ctx context.Context, workRoot, listDir string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, gitStatusTimeout)
	defer cancel()
	args := []string{"-C", workRoot, "status", "--porcelain=1", "--ignored"}
	if listDir != "" && filepath.Clean(listDir) != filepath.Clean(workRoot) {
		args = append(args, "--", listDir)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("git status: %w: %s", err, bytes.TrimSpace(stderr.Bytes()))
		}
		return "", fmt.Errorf("git status: %w", err)
	}
	return stdout.String(), nil
}

func querySnapshot(ctx context.Context, workRoot, listDir string) (*snapshot, error) {
	out, err := GitCommand(ctx, workRoot, listDir)
	if err != nil {
		return nil, err
	}
	entries := parsePorcelain(out, workRoot)
	return &snapshot{entries: entries}, nil
}

// StatusForFile returns path's Git cell from a `git status` scoped to that one file. It is not
// cached: the Cache keys whole-directory snapshots, and a one-file snapshot would answer every
// other row of that directory as clean. It exists so a synchronous caller (CLI `pc <file>`)
// pays for one pathspec instead of a repo-wide --ignored scan when the file sits at the root.
func StatusForFile(ctx context.Context, workRoot, path string) (Cell, error) {
	path = filepath.Clean(path)
	sn, err := querySnapshot(ctx, filepath.Clean(workRoot), path)
	if err != nil {
		return Cell{}, err
	}
	return sn.fileCell(path), nil
}
