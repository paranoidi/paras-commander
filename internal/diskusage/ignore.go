package diskusage

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// LoadGoduIgnoreBasenames reads ~/.goduignore (newline-separated basename rules, like upstream godu)
// and returns a ShouldIgnoreFolder that matches on filepath.Base(absPath).
func LoadGoduIgnoreBasenames(goduIgnorePath string) (ShouldIgnoreFolder, error) {
	ignored := map[string]struct{}{}
	if filepath.Clean(strings.TrimSpace(goduIgnorePath)) == "" || goduIgnorePath == "" {
		return func(string) bool { return false }, nil
	}

	f, err := os.Open(goduIgnorePath)
	if err != nil {
		if os.IsNotExist(err) {
			return func(string) bool { return false }, nil
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		ignored[line] = struct{}{}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return func(absPath string) bool {
		base := filepath.Base(absPath)
		_, skip := ignored[base]
		return skip
	}, nil
}
