package gitstatus

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

const gitStatusTimeout = 60 * time.Second

// GitCommand runs git status --porcelain for tests and production.
var GitCommand = defaultGitCommand

func defaultGitCommand(ctx context.Context, workRoot string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, gitStatusTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", workRoot, "status", "--porcelain=1", "--ignored")
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

func querySnapshot(ctx context.Context, workRoot string) (*snapshot, error) {
	out, err := GitCommand(ctx, workRoot)
	if err != nil {
		return nil, err
	}
	entries := parsePorcelain(out, workRoot)
	return &snapshot{entries: entries}, nil
}
