package cmdmacro

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// PanelSnapshot holds panel fields needed for macro expansion (avoids importing panel).
type PanelSnapshot struct {
	Dir         string
	CurrentName string
	HasCurrent  bool
	TaggedInDir []string // absolute tagged paths: panel directory (flat) or anywhere under it (tree)
}

// Context carries state for macro expansion.
type Context struct {
	Active    *PanelSnapshot
	Other     *PanelSnapshot
	FOverride string // run-for-each iterated absolute path
	RowPath   string // meta row absolute path
}

// QuoteShellArg POSIX-single-quotes s so it is safe to embed in `sh -c`.
func QuoteShellArg(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// ExpandCommandLine substitutes %% %f %F %d %D %t %T into template.
// Path and name values are POSIX-single-quoted so the result is safe for sh -c.
func ExpandCommandLine(template string, ctx Context) (string, error) {
	if ctx.RowPath == "" && ctx.Active == nil {
		return "", fmt.Errorf("cmdmacro: no expansion context")
	}
	var b strings.Builder
	err := walkMacros(template, ctx, func(values []string) {
		for i, v := range values {
			if i > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(QuoteShellArg(v))
		}
	}, func(s string) {
		b.WriteString(s)
	})
	if err != nil {
		return "", err
	}
	return b.String(), nil
}

type argvFrag struct {
	lit  string
	vals []string
}

// ExpandArgvToken substitutes path macros in one argv token using raw values.
// A lone %t/%T becomes one argument per tagged path; surrounding text is glued
// to the first and last path when the token is not a lone multi-value macro.
func ExpandArgvToken(token string, ctx Context) ([]string, error) {
	var frags []argvFrag
	err := walkMacros(token, ctx, func(values []string) {
		frags = append(frags, argvFrag{vals: values})
	}, func(s string) {
		if s != "" {
			frags = append(frags, argvFrag{lit: s})
		}
	})
	if err != nil {
		return nil, err
	}
	return joinArgvFrags(frags), nil
}

func joinArgvFrags(frags []argvFrag) []string {
	var prefix, suffix strings.Builder
	var values []string
	seenMulti := false
	for _, f := range frags {
		if len(f.vals) == 0 {
			if !seenMulti {
				prefix.WriteString(f.lit)
			} else {
				suffix.WriteString(f.lit)
			}
			continue
		}
		if len(f.vals) == 1 && !seenMulti {
			prefix.WriteString(f.vals[0])
			continue
		}
		if len(f.vals) == 1 {
			suffix.WriteString(f.vals[0])
			continue
		}
		values = append(values, f.vals...)
		seenMulti = true
	}
	if len(values) == 0 {
		return []string{prefix.String() + suffix.String()}
	}
	out := append([]string(nil), values...)
	out[0] = prefix.String() + out[0]
	out[len(out)-1] += suffix.String()
	return out
}

func walkMacros(template string, ctx Context, onMacro func([]string), onLit func(string)) error {
	var lit strings.Builder
	flushLit := func() {
		if lit.Len() == 0 {
			return
		}
		onLit(lit.String())
		lit.Reset()
	}
	for i := 0; i < len(template); i++ {
		if template[i] != '%' || i+1 >= len(template) {
			lit.WriteByte(template[i])
			continue
		}
		switch template[i+1] {
		case '%':
			lit.WriteByte('%')
			i++
		case 'f', 'F', 'd', 'D', 't', 'T':
			vals, err := macroValues(template[i+1], ctx)
			if err != nil {
				return err
			}
			flushLit()
			onMacro(vals)
			i++
		default:
			lit.WriteByte('%')
		}
	}
	flushLit()
	return nil
}

func macroValues(letter byte, ctx Context) ([]string, error) {
	switch letter {
	case 'f':
		v, err := expandF(ctx)
		if err != nil {
			return nil, err
		}
		return []string{v}, nil
	case 'F':
		if ctx.Active == nil {
			return nil, fmt.Errorf("cmdmacro: %%F: no active panel")
		}
		if ctx.Other == nil || !ctx.Other.HasCurrent {
			return nil, fmt.Errorf("cmdmacro: %%F: no current file on other panel")
		}
		return []string{ctx.Other.CurrentName}, nil
	case 'd':
		if ctx.Active == nil {
			return nil, fmt.Errorf("cmdmacro: %%d: no active panel")
		}
		return []string{filepath.Clean(ctx.Active.Dir)}, nil
	case 'D':
		if ctx.Active == nil {
			return nil, fmt.Errorf("cmdmacro: %%D: no active panel")
		}
		if ctx.Other == nil {
			return nil, fmt.Errorf("cmdmacro: %%D: no other panel")
		}
		return []string{filepath.Clean(ctx.Other.Dir)}, nil
	case 't':
		if ctx.Active == nil {
			return nil, fmt.Errorf("cmdmacro: %%t: no active panel")
		}
		return taggedPaths(ctx.Active)
	case 'T':
		if ctx.Active == nil {
			return nil, fmt.Errorf("cmdmacro: %%T: no active panel")
		}
		if ctx.Other == nil {
			return nil, fmt.Errorf("cmdmacro: %%T: no other panel")
		}
		return taggedPaths(ctx.Other)
	default:
		return nil, fmt.Errorf("cmdmacro: unknown macro %%%c", letter)
	}
}

func expandF(ctx Context) (string, error) {
	if strings.TrimSpace(ctx.RowPath) != "" {
		return ctx.RowPath, nil
	}
	if strings.TrimSpace(ctx.FOverride) != "" {
		return ctx.FOverride, nil
	}
	if ctx.Active == nil || !ctx.Active.HasCurrent {
		return "", fmt.Errorf("cmdmacro: %%f: no current file")
	}
	return ctx.Active.CurrentName, nil
}

func taggedPaths(ps *PanelSnapshot) ([]string, error) {
	if ps == nil || len(ps.TaggedInDir) == 0 {
		return nil, fmt.Errorf("cmdmacro: %%t: no tagged files in current directory")
	}
	paths := append([]string(nil), ps.TaggedInDir...)
	sort.Strings(paths)
	return paths, nil
}

// CommandRequiresMacro reports whether template contains macro %<letter> (not %%).
func CommandRequiresMacro(template string, letter byte) bool {
	for i := 0; i < len(template)-1; i++ {
		if template[i] != '%' {
			continue
		}
		if template[i+1] == '%' {
			i++
			continue
		}
		if template[i+1] == letter {
			return true
		}
	}
	return false
}

// ErrRunForEachRequiresF is returned when a run-for-each command omits %f.
const ErrRunForEachRequiresF = "command must include %f to represent the selected item"
