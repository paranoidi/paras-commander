package cmdrun

import (
	"fmt"
	"strings"

	"github.com/paranoidi/paras-commander/internal/cmdmacro"
)

// InvocationMode selects how an expanded command line is turned into argv.
type InvocationMode int

const (
	// ModeAuto uses sh -c when NeedsShellFromLine reports true after expansion.
	ModeAuto InvocationMode = iota
	// ModeExecParsed always parses expanded line into argv (no shell).
	ModeExecParsed
	// ModeShellScript always runs expanded line via sh -c.
	ModeShellScript
)

// InvocationSpec describes a templated command to build into argv.
type InvocationSpec struct {
	Template   string
	Mode       InvocationMode
	ForceShell bool
	Ctx        cmdmacro.Context
}

// InvocationResult holds argv and display strings for UI/logging.
type InvocationResult struct {
	Argv     []string
	Expanded string
	Display  string
}

// BuildInvocation expands macros in Template and returns argv for exec.Command or sh -c.
func BuildInvocation(spec InvocationSpec) (InvocationResult, error) {
	template := strings.TrimSpace(spec.Template)
	if template == "" {
		return InvocationResult{}, fmt.Errorf("empty command")
	}
	expanded, err := cmdmacro.ExpandCommandLine(template, spec.Ctx)
	if err != nil {
		return InvocationResult{}, err
	}
	display := template
	if strings.TrimSpace(expanded) != template {
		display = template + " → " + expanded
	}
	useShell := spec.ForceShell || spec.Mode == ModeShellScript
	if !useShell && spec.Mode != ModeExecParsed {
		useShell = NeedsShellFromLine(template)
	}
	if useShell {
		return InvocationResult{
			Argv:     ShellArgv(expanded),
			Expanded: expanded,
			Display:  display,
		}, nil
	}
	argv, err := expandInvocationArgv(template, spec.Ctx)
	if err != nil {
		return InvocationResult{}, err
	}
	return InvocationResult{
		Argv:     argv,
		Expanded: expanded,
		Display:  display,
	}, nil
}

func expandInvocationArgv(template string, ctx cmdmacro.Context) ([]string, error) {
	argv, err := ParseCommandArgv(template)
	if err != nil {
		return nil, err
	}
	if len(argv) == 0 {
		return nil, fmt.Errorf("command is empty after parsing")
	}
	out := make([]string, 0, len(argv))
	for _, tok := range argv {
		parts, err := cmdmacro.ExpandArgvToken(tok, ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, parts...)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("command is empty after parsing")
	}
	return out, nil
}
