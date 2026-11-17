package cmdrun

import (
	"fmt"
	"strconv"
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
	Priority   Priority
}

// Priority lowers CPU (nice) and disk (ionice) priority of a command; the zero value changes nothing.
type Priority struct {
	// Nice is the nice -n increment (1..19); 0 leaves it unchanged.
	Nice int
	// IOArgs are the ionice arguments (see ParseIONice); empty leaves them unchanged.
	IOArgs []string
}

// ParseIONice turns an ionice setting ("idle", "best-effort" or "best-effort:N" with N 0..7,
// case-insensitive) into ionice arguments.
func ParseIONice(s string) ([]string, error) {
	v := strings.ToLower(strings.TrimSpace(s))
	if v == "idle" {
		return []string{"-c", "3"}, nil
	}
	if v == "best-effort" {
		return []string{"-c", "2"}, nil
	}
	if lvl, ok := strings.CutPrefix(v, "best-effort:"); ok {
		if n, err := strconv.Atoi(lvl); err == nil && n >= 0 && n <= 7 && lvl == strconv.Itoa(n) {
			return []string{"-c", "2", "-n", lvl}, nil
		}
	}
	return nil, fmt.Errorf("invalid ionice %q (want idle, best-effort or best-effort:0-7)", s)
}

// wrap prepends the nice / ionice commands to argv.
func (p Priority) wrap(argv []string) []string {
	var out []string
	if p.Nice != 0 {
		out = append(out, "nice", "-n", strconv.Itoa(p.Nice))
	}
	if len(p.IOArgs) > 0 {
		out = append(out, "ionice")
		out = append(out, p.IOArgs...)
	}
	return append(out, argv...)
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
			Argv:     spec.Priority.wrap(ShellArgv(expanded)),
			Expanded: expanded,
			Display:  display,
		}, nil
	}
	argv, err := expandInvocationArgv(template, spec.Ctx)
	if err != nil {
		return InvocationResult{}, err
	}
	return InvocationResult{
		Argv:     spec.Priority.wrap(argv),
		Expanded: expanded,
		Display:  display,
	}, nil
}

func expandInvocationArgv(template string, ctx cmdmacro.Context) ([]string, error) {
	argv, err := ParseCommandArgv(cmdmacro.ProtectGroups(template))
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
