package rsync

import (
	"os/exec"
	"regexp"
	"strconv"
	"sync"
)

var versionRe = regexp.MustCompile(`rsync\s+version\s+(\d+)\.(\d+)`)

// progress2Supported reports whether the installed rsync has --info=progress2 (3.1 and newer).
// Older rsync (macOS ships 2.6.9) runs without it, so a job shows file names but no byte progress.
// It asks rsync for its version once (a process: call it from a worker).
var progress2Supported = sync.OnceValue(func() bool {
	out, err := exec.Command("rsync", "--version").Output()
	return err == nil && progress2FromVersion(string(out))
})

// progress2FromVersion reads the first "rsync  version X.Y" of `rsync --version` output; an
// unrecognized output is the conservative answer (not supported).
func progress2FromVersion(out string) bool {
	m := versionRe.FindStringSubmatch(out)
	if m == nil {
		return false
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	return major > 3 || (major == 3 && minor >= 1)
}
