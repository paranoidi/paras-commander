// Package zoxide lists the directories tracked by the zoxide CLI.
package zoxide

import (
	"bytes"
	"os/exec"
	"path/filepath"
)

// List returns zoxide's directories (highest score first) by running
// `zoxide query --list`. The database lives at $_ZO_DATA_DIR/db.zo, else
// $XDG_DATA_HOME/zoxide/db.zo (~/.local/share/zoxide/db.zo); asking the binary
// keeps its exclusions, aging and binary format out of this package.
// installed is false when the zoxide binary is not on PATH; a failed query returns no paths.
// Blocking: call off the UI goroutine.
func List() (paths []string, installed bool) {
	bin, err := exec.LookPath("zoxide")
	if err != nil {
		return nil, false
	}
	out, err := exec.Command(bin, "query", "--list").Output()
	if err != nil {
		return nil, true
	}
	return parseList(out), true
}

func parseList(out []byte) []string {
	var paths []string
	for line := range bytes.Lines(out) {
		p := string(bytes.TrimRight(line, "\r\n"))
		if filepath.IsAbs(p) {
			paths = append(paths, filepath.Clean(p))
		}
	}
	return paths
}
