package cmdrun

import (
	"fmt"

	"github.com/mattn/go-shellwords"
)

// ParseCommandArgv parses a single-line shell-style command into argv tokens using POSIX-like rules.
// Backticks and environment expansion are disabled so parsing cannot execute side effects.
func ParseCommandArgv(line string) ([]string, error) {
	p := shellwords.NewParser()
	p.ParseEnv = false
	p.ParseBacktick = false
	args, err := p.Parse(line)
	if err != nil {
		return nil, fmt.Errorf("parse command: %w", err)
	}
	return args, nil
}
