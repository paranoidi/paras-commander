package cmdmacro

import (
	"fmt"
	"iter"
	"path/filepath"
	"slices"
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

// macroLetters are the single-letter macros; a fallback group %{a|b|...} takes only these.
const macroLetters = "fFdDtT"

// protectedGroupSep stands in for | inside a fallback group while a command line goes through
// shell-operator detection and argv parsing, which both stop at an unquoted |.
const protectedGroupSep = "\x1f"

// macroAt parses the macro at template[i], which must be '%'. letters holds the macro's
// alternatives in order: one letter for %f, several for %{t|d}. letters is empty when the text
// is no macro: a literal % that spans width bytes (2 for %%, 1 otherwise). Anything but a strict
// %{a|b|...} of macro letters stays literal, so curl's %{http_code} passes through unchanged.
func macroAt(template string, i int) (letters string, width int) {
	if i+1 >= len(template) {
		return "", 1
	}
	c := template[i+1]
	switch {
	case c == '%':
		return "", 2
	case c == '{':
		return groupAt(template, i)
	case strings.IndexByte(macroLetters, c) >= 0:
		return template[i+1 : i+2], 2
	}
	return "", 1
}

func groupAt(template string, i int) (letters string, width int) {
	var b strings.Builder
	for j := i + 2; j+1 < len(template); j += 2 {
		if strings.IndexByte(macroLetters, template[j]) < 0 {
			return "", 1
		}
		b.WriteByte(template[j])
		switch template[j+1] {
		case '}':
			return b.String(), j + 2 - i
		case '|', protectedGroupSep[0]:
		default:
			return "", 1
		}
	}
	return "", 1
}

// resolveMacro returns the first alternative of letters that has a value.
func resolveMacro(letters string, ctx Context) (byte, []string, error) {
	var err error
	for i := 0; i < len(letters); i++ {
		var vals []string
		if vals, err = macroValues(letters[i], ctx); err == nil {
			return letters[i], vals, nil
		}
	}
	if len(letters) > 1 {
		err = fmt.Errorf("cmdmacro: %%{%s}: no value", strings.Join(strings.Split(letters, ""), "|"))
	}
	return 0, nil, err
}

// ProtectGroups hides the | of fallback groups so template can go through shell-operator
// detection and argv parsing. ExpandArgvToken accepts the protected form.
func ProtectGroups(template string) string {
	var b strings.Builder
	for i := 0; i < len(template); {
		if template[i] != '%' {
			b.WriteByte(template[i])
			i++
			continue
		}
		_, width := macroAt(template, i)
		token := template[i : i+width]
		if width > 2 {
			token = strings.ReplaceAll(token, "|", protectedGroupSep)
		}
		b.WriteString(token)
		i += width
	}
	return b.String()
}

// ExpandCommandLine substitutes %% %f %F %d %D %t %T and fallback groups into template.
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
		if template[i] != '%' {
			lit.WriteByte(template[i])
			continue
		}
		letters, width := macroAt(template, i)
		i += width - 1
		if letters == "" {
			lit.WriteByte('%')
			continue
		}
		_, vals, err := resolveMacro(letters, ctx)
		if err != nil {
			return err
		}
		flushLit()
		onMacro(vals)
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

// macroRefs yields the alternatives of each macro in template, in order.
func macroRefs(template string) iter.Seq[string] {
	return func(yield func(string) bool) {
		for i := 0; i < len(template); {
			if template[i] != '%' {
				i++
				continue
			}
			letters, width := macroAt(template, i)
			i += width
			if letters != "" && !yield(letters) {
				return
			}
		}
	}
}

// CommandRequiresMacro reports whether template always reads macro %<letter> when it has a
// value: a plain %<letter>, or a fallback group that tries it first. %% is no macro. It needs no
// panel state; ResolvedMacros tells which alternative of a group a launch actually uses.
func CommandRequiresMacro(template string, letter byte) bool {
	for letters := range macroRefs(template) {
		if letters[0] == letter {
			return true
		}
	}
	return false
}

// ErrRunForEachRequiresF is returned when a run-for-each command omits %f.
const ErrRunForEachRequiresF = "command must include %f to represent the selected item"

// MacroUse is one distinct macro a template references, with the values it expands to.
type MacroUse struct {
	Letter byte
	Values []string
}

// ResolvedMacros returns the distinct macros template reads under ctx, in first-use order, with
// their raw values: a fallback group contributes the alternative that won, and a macro without a
// value is omitted. It is the one answer to "what does this launch read".
func ResolvedMacros(template string, ctx Context) []MacroUse {
	var out []MacroUse
	seen := map[byte]bool{}
	for letters := range macroRefs(template) {
		letter, vals, err := resolveMacro(letters, ctx)
		if err != nil || seen[letter] {
			continue
		}
		seen[letter] = true
		out = append(out, MacroUse{Letter: letter, Values: vals})
	}
	return out
}

// UsesMacro reports whether uses (from ResolvedMacros) holds letter.
func UsesMacro(uses []MacroUse, letter byte) bool {
	return slices.ContainsFunc(uses, func(u MacroUse) bool { return u.Letter == letter })
}

// MacroDisplayValues returns ResolvedMacros for display: %f/%F bare names are joined with their
// panel's directory so every value is a path.
func MacroDisplayValues(template string, ctx Context) []MacroUse {
	out := ResolvedMacros(template, ctx)
	for i, u := range out {
		if dir := macroDir(u.Letter, ctx); dir != "" && !filepath.IsAbs(u.Values[0]) {
			out[i].Values = []string{filepath.Join(dir, u.Values[0])}
		}
	}
	return out
}

func macroDir(letter byte, ctx Context) string {
	switch {
	case letter == 'f' && ctx.Active != nil:
		return ctx.Active.Dir
	case letter == 'F' && ctx.Other != nil:
		return ctx.Other.Dir
	}
	return ""
}
