package ops

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

// ChmodPlan describes a validated chmod operation.
type ChmodPlan struct {
	Entries []localfs.Entry
	Mode    os.FileMode
	ModeStr string // display representation
}

// PlanChmod validates a chmod operation.
func PlanChmod(source Source, modeExpr string) (ChmodPlan, error) {
	if len(source.Entries) == 0 {
		return ChmodPlan{}, &Error{Op: "chmod", Text: "no entries to change"}
	}
	if modeExpr == "" {
		return ChmodPlan{}, &Error{Op: "chmod", Text: "mode expression is empty"}
	}

	mode, err := parseMode(modeExpr)
	if err != nil {
		return ChmodPlan{}, &Error{Op: "chmod", Text: err.Error()}
	}

	return ChmodPlan{
		Entries: source.Entries,
		Mode:    mode,
		ModeStr: modeExpr,
	}, nil
}

// ExecuteChmod applies the mode to each entry.
func ExecuteChmod(plan ChmodPlan) error {
	for _, entry := range plan.Entries {
		if err := localfs.Chmod(entry.Path, plan.Mode); err != nil {
			return &Error{Op: "chmod", Text: "failed to change mode for " + entry.Name, Err: err}
		}
	}
	return nil
}

// parseMode parses a chmod expression which may be numeric (e.g. "755", "0644")
// or symbolic (e.g. "u+r", "g-w", "a+rx", "u=rw,go=r").
func parseMode(expr string) (os.FileMode, error) {
	if expr == "" {
		return 0, fmt.Errorf("mode expression is empty")
	}

	// Try numeric mode first.
	if mode, err := parseNumericMode(expr); err == nil {
		return mode, nil
	}

	// Try symbolic mode.
	return parseSymbolicMode(expr)
}

// parseNumericMode parses a numeric mode like "755" or "0644".
func parseNumericMode(expr string) (os.FileMode, error) {
	// Remove leading 0o or 0 prefix.
	cleaned := expr
	if strings.HasPrefix(cleaned, "0o") || strings.HasPrefix(cleaned, "0O") {
		cleaned = cleaned[2:]
	}

	if len(cleaned) < 3 || len(cleaned) > 4 {
		return 0, fmt.Errorf("invalid numeric mode %q", expr)
	}

	for _, r := range cleaned {
		if r < '0' || r > '7' {
			return 0, fmt.Errorf("invalid numeric mode %q", expr)
		}
	}

	val, err := strconv.ParseUint(cleaned, 8, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid numeric mode %q: %w", expr, err)
	}

	return os.FileMode(val), nil
}

// parseSymbolicMode parses symbolic chmod expressions like "u+r", "g-w", "a+rx".
// Multiple comma-separated clauses are supported (e.g. "u=rw,go=r").
func parseSymbolicMode(expr string) (os.FileMode, error) {
	// Start with current mode-like defaults for the computation.
	// Symbolic chmod interprets relative to the current mode, but we don't have
	// the current mode here. We compute a mode change mask that can be applied.
	// For simplicity, we treat "u", "g", "o" as who clauses, and
	// build up a target mode from scratch using "=" and modify it with "+"/"-".

	clauses := strings.Split(expr, ",")
	var result os.FileMode

	for _, clause := range clauses {
		clause = strings.TrimSpace(clause)
		if clause == "" {
			continue
		}

		who, op, permStr, err := parseSymbolicClause(clause)
		if err != nil {
			return 0, err
		}

		perms := parsePermString(permStr)
		mask := buildWhoMask(who)

		switch op {
		case '=':
			// Clear bits in mask, then set.
			result &^= mask
			result |= perms & mask
		case '+':
			result |= perms & mask
		case '-':
			result &^= perms & mask
		default:
			return 0, fmt.Errorf("invalid symbolic mode %q: unknown operator %c", expr, op)
		}
	}

	return result, nil
}

// parseSymbolicClause parses one clause like "u+r" or "go-w" or "a=rx".
func parseSymbolicClause(clause string) (who string, op byte, permStr string, err error) {
	if len(clause) < 2 {
		return "", 0, "", fmt.Errorf("invalid symbolic clause %q", clause)
	}

	// Extract who (u/g/o/a), optionally combined.
	whoEnd := 0
	for whoEnd < len(clause) {
		switch clause[whoEnd] {
		case 'u', 'g', 'o', 'a':
			whoEnd++
		default:
			goto parseOp
		}
	}
parseOp:
	if whoEnd == 0 {
		// Default who is "a" if no who specified.
		who = "a"
		whoEnd = 0
	} else {
		who = clause[:whoEnd]
	}

	if whoEnd >= len(clause) {
		return "", 0, "", fmt.Errorf("invalid symbolic clause %q: missing operator", clause)
	}

	op = clause[whoEnd]
	switch op {
	case '+', '-', '=':
	default:
		return "", 0, "", fmt.Errorf("invalid symbolic clause %q: unknown operator %c", clause, op)
	}

	permStr = clause[whoEnd+1:]
	return who, op, permStr, nil
}

// parsePermString extracts the permission bits from a string like "rwx" or "rx".
func parsePermString(s string) os.FileMode {
	var mode os.FileMode
	for _, r := range s {
		switch r {
		case 'r':
			mode |= 0o444
		case 'w':
			mode |= 0o222
		case 'x':
			mode |= 0o111
		case 's':
			// setuid/setgid - added to owner/group execute position
			mode |= 0o6000
		case 't':
			// sticky bit
			mode |= 0o1000
		}
	}
	return mode
}

// buildWhoMask creates a file mode mask for the given who string.
// "u" -> owner bits, "g" -> group bits, "o" -> other bits, "a" -> all.
func buildWhoMask(who string) os.FileMode {
	var mask os.FileMode
	for _, r := range who {
		switch r {
		case 'u':
			mask |= 0o700
		case 'g':
			mask |= 0o070
		case 'o':
			mask |= 0o007
		case 'a':
			mask |= 0o777
		}
	}
	return mask
}
