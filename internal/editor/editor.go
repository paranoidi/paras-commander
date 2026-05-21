package editor

import (
	"context"
	"fmt"
	"os"
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

// RunInteractive runs argv with stdin/stdout/stderr attached to the terminal.
func RunInteractive(ctx context.Context, argv []string, dir string) error {
	if len(argv) == 0 {
		return fmt.Errorf("editor argv is empty")
	}
	if err := cmdrun.RunInteractive(ctx, argv, dir); err != nil {
		return fmt.Errorf("run editor: %w", err)
	}
	return nil
}
