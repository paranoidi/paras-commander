package app

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

var (
	startupPromptIn  io.Reader = os.Stdin
	startupPromptOut io.Writer = os.Stderr
)

// startupDefaultsPrompt asks on stderr whether to launch with built-in defaults; tests may replace it.
var startupDefaultsPrompt = promptLaunchWithBuiltInDefaults

func promptLaunchWithBuiltInDefaults(in io.Reader, out io.Writer, loadErr error) (bool, error) {
	if loadErr != nil {
		if _, err := fmt.Fprintf(out, "Configuration error:\n%v\n\n", loadErr); err != nil {
			return false, err
		}
	}
	if _, err := fmt.Fprint(out, "Launch with built-in defaults anyway? [Y/n]: "); err != nil {
		return false, err
	}

	reader := bufio.NewReader(in)
	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return false, fmt.Errorf("read confirmation: %w", err)
		}
		answer := strings.TrimSpace(strings.ToLower(line))
		switch answer {
		case "", "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			if _, werr := fmt.Fprint(out, "Please answer Y or n: "); werr != nil {
				return false, werr
			}
		}
		if err == io.EOF {
			return false, nil
		}
	}
}
