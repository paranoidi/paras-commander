package cmdrun

import (
	"os/exec"
	"strconv"
	"strings"
)

// shellOperatorTokens are argv tokens that require a real shell (redirection, pipelines, etc.).
var shellOperatorTokens = map[string]bool{
	">>": true, ">": true, "<<": true, "<": true,
	"|": true, "||": true, "&&": true,
	";": true, "&": true,
}

// ArgvNeedsShell reports whether argv contains shell operator tokens from POSIX parsing.
func ArgvNeedsShell(argv []string) bool {
	for _, a := range argv {
		if shellOperatorTokens[a] {
			return true
		}
	}
	return false
}

// NeedsShellFromLine reports whether a command line needs a shell to interpret operators.
func NeedsShellFromLine(line string) bool {
	if lineNeedsShellOutsideQuotes(line) {
		return true
	}
	argv, err := ParseCommandArgv(line)
	if err != nil {
		return false
	}
	return ArgvNeedsShell(argv)
}

// ShellArgv returns argv to run script via sh -c.
func ShellArgv(script string) []string {
	sh := "/bin/sh"
	if p, err := exec.LookPath("sh"); err == nil {
		sh = p
	}
	return []string{sh, "-c", script}
}

// FormatShellDisplay returns a compact display string for sh -c invocations.
func FormatShellDisplay(script string) string {
	script = strings.TrimSpace(script)
	if script == "" {
		return "sh -c"
	}
	return "sh -c " + strconv.Quote(script)
}

func lineNeedsShellOutsideQuotes(line string) bool {
	inSingle := false
	inDouble := false
	escaped := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if escaped {
			escaped = false
			continue
		}
		if inSingle {
			if c == '\'' {
				inSingle = false
			}
			continue
		}
		if inDouble {
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				inDouble = false
			}
			continue
		}
		switch c {
		case '\'':
			inSingle = true
		case '"':
			inDouble = true
		case '|', ';', '>', '<':
			return true
		case '&':
			if i+1 < len(line) && line[i+1] == '&' {
				return true
			}
			if i+1 < len(line) && line[i+1] == '>' {
				return true
			}
			if i > 0 && (i+1 >= len(line) || line[i+1] == ' ') {
				return true
			}
		}
	}
	return false
}
