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
		useShell = NeedsShellFromLine(expanded)
	}
	if useShell {
		return InvocationResult{
			Argv:     ShellArgv(expanded),
			Expanded: expanded,
			Display:  display,
		}, nil
	}
	argv, err := ParseCommandArgv(expanded)
	if err != nil {
		return InvocationResult{}, err
	}
	if len(argv) == 0 {
		return InvocationResult{}, fmt.Errorf("command is empty after parsing")
	}
	return InvocationResult{
		Argv:     argv,
		Expanded: expanded,
		Display:  display,
	}, nil
}
