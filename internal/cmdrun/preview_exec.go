package cmdrun

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// filePreviewExecutableCandidates is the order used when resolving the default
// bat-style preview command (Debian/Ubuntu often install bat as batcat).
var filePreviewExecutableCandidates = []string{"bat", "batcat", "cat"}

// ResolveFilePreviewExecutable returns the first preview highlighter found on PATH.
func ResolveFilePreviewExecutable() (string, error) {
	for _, name := range filePreviewExecutableCandidates {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("no file preview executable found (tried %v)", filePreviewExecutableCandidates)
}

// BuildFilePreviewArgv parses the preview command template and resolves bat/batcat/cat.
func BuildFilePreviewArgv(commandLine, absPath string, terminalWidth int) ([]string, error) {
	argv, err := PreviewCommandArgv(commandLine, absPath, terminalWidth)
	if err != nil {
		return nil, err
	}
	return FinalizePreviewArgv(argv, absPath)
}

// FinalizePreviewArgv picks bat, batcat, or cat on PATH for argv whose program is bat or batcat.
// When only cat is available, bat-specific flags are omitted.
func FinalizePreviewArgv(argv []string, absPath string) ([]string, error) {
	exe, err := ResolveFilePreviewExecutable()
	if err != nil {
		return nil, err
	}
	return FinalizePreviewArgvWithExecutable(argv, absPath, exe), nil
}

// FinalizePreviewArgvWithExecutable applies a resolved executable path (for tests).
func FinalizePreviewArgvWithExecutable(argv []string, absPath, exe string) []string {
	if len(argv) == 0 {
		return []string{exe, absPath}
	}
	base := filepath.Base(argv[0])
	if base != "bat" && base != "batcat" {
		return argv
	}
	if filepath.Base(exe) == "cat" {
		return []string{exe, absPath}
	}
	out := append([]string(nil), argv...)
	out[0] = exe
	return out
}
