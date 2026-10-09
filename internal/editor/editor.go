package editor

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/cmdrun"
)

const defaultEditor = "vi"

// ResolveEditor returns VISUAL, then EDITOR, then "vi".
func ResolveEditor() string {
	for _, key := range []string{"VISUAL", "EDITOR"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return defaultEditor
}

// EditorArgv parses editorLine and appends filePath as the final argument.
func EditorArgv(editorLine, filePath string) ([]string, error) {
	editorLine = strings.TrimSpace(editorLine)
	if editorLine == "" {
		editorLine = defaultEditor
	}
	argv, err := cmdrun.ParseCommandArgv(editorLine)
	if err != nil {
		return nil, fmt.Errorf("parse editor: %w", err)
	}
	if len(argv) == 0 {
		argv = []string{defaultEditor}
	}
	return append(argv, filePath), nil
}

// accessDenied is the write-permission probe; tests replace it.
var accessDenied = needsWriteElevation

// Command builds the argv and extra env for editing path. When elevate is true and the
// current user cannot write path (or, for a missing file, its parent directory), it returns
// a sudoedit invocation (`sudo -e`) with SUDO_EDITOR set to the resolved editor line, and
// elevated is true.
func Command(editorLine, path string, elevate bool) (argv, env []string, elevated bool, err error) {
	if elevate {
		target := path
		if _, statErr := os.Stat(path); errors.Is(statErr, fs.ErrNotExist) {
			target = filepath.Dir(path)
		}
		if accessDenied(target) {
			return []string{"sudo", "-e", "--", path}, []string{"SUDO_EDITOR=" + editorLine}, true, nil
		}
	}
	argv, err = EditorArgv(editorLine, path)
	return argv, nil, false, err
}

// RunInteractive runs argv with stdin/stdout/stderr attached to the terminal.
// extraEnv entries (KEY=VALUE) are appended to the inherited environment.
func RunInteractive(ctx context.Context, argv, extraEnv []string, dir string) error {
	if len(argv) == 0 {
		return fmt.Errorf("editor argv is empty")
	}
	if err := cmdrun.RunInteractiveEnv(ctx, argv, extraEnv, dir); err != nil {
		return fmt.Errorf("run editor: %w", err)
	}
	return nil
}
