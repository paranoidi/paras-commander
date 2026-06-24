package entrymatch

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// Context carries state for when-expression evaluation.
// Cursor mode: set Active (and optionally Other). Row mode: set Row and PanelDir.
type Context struct {
	Active *panel.State
	Other  *panel.State
	// Row mode (meta column filtering).
	Row      *localfs.Entry
	PanelDir string
	// ShellPatterns true → filepath.Match for f/F/d/D; false → regexp.
	ShellPatterns bool
}

// EvalWhen evaluates a visibility expression; empty string is true.
func EvalWhen(expr string, ctx *Context) (bool, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return true, nil
	}
	expr = stripLeadingConditionPrefix(expr)
	expr = normalizeBareGlob(expr)
	p := &condParser{s: expr, ctx: ctx}
	return p.parseOr()
}

// EvalWhenAny evaluates multiple visibility expressions with OR semantics.
func EvalWhenAny(exprs []string, ctx *Context) (bool, error) {
	var any bool
	for _, expr := range exprs {
		expr = strings.TrimSpace(expr)
		if expr == "" {
			continue
		}
		any = true
		ok, err := EvalWhen(expr, ctx)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	if !any {
		return true, nil
	}
	return false, nil
}

func stripLeadingConditionPrefix(s string) string {
	s = strings.TrimSpace(s)
	for strings.HasPrefix(s, "=") || strings.HasPrefix(s, "+") {
		s = strings.TrimSpace(s[1:])
	}
	if strings.HasPrefix(s, "?") {
		s = strings.TrimSpace(s[1:])
	}
	return strings.TrimSpace(s)
}

// normalizeBareGlob turns "*.py" into "f *.py" when no predicate prefix is present.
func normalizeBareGlob(expr string) string {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return expr
	}
	if expr[0] == '!' || expr[0] == '(' {
		return expr
	}
	if len(expr) >= 2 && (expr[0] == 'f' || expr[0] == 'F' || expr[0] == 'd' || expr[0] == 'D' || expr[0] == 't' || expr[0] == 'T') {
		if unicode.IsSpace(rune(expr[1])) {
			return expr
		}
	}
	return "f " + expr
}

type condParser struct {
	s   string
	i   int
	ctx *Context
}

func (p *condParser) skipSpace() {
	for p.i < len(p.s) && (p.s[p.i] == ' ' || p.s[p.i] == '\t') {
		p.i++
	}
}

func (p *condParser) parseOr() (bool, error) {
	left, err := p.parseAnd()
	if err != nil {
		return false, err
	}
	for {
		p.skipSpace()
		if p.i >= len(p.s) {
			return left, nil
		}
		if p.s[p.i] != '|' {
			return left, nil
		}
		p.i++
		right, err := p.parseAnd()
		if err != nil {
			return false, err
		}
		left = left || right
	}
}

func (p *condParser) parseAnd() (bool, error) {
	left, err := p.parseUnary()
	if err != nil {
		return false, err
	}
	for {
		p.skipSpace()
		if p.i >= len(p.s) {
			return left, nil
		}
		if p.s[p.i] != '&' {
			return left, nil
		}
		p.i++
		right, err := p.parseUnary()
		if err != nil {
			return false, err
		}
		left = left && right
	}
}

func (p *condParser) parseUnary() (bool, error) {
	p.skipSpace()
	if p.i < len(p.s) && p.s[p.i] == '!' {
		p.i++
		v, err := p.parseUnary()
		return !v, err
	}
	return p.parsePrimary()
}

func (p *condParser) parsePrimary() (bool, error) {
	p.skipSpace()
	if p.i >= len(p.s) {
		return false, fmt.Errorf("when condition: unexpected end")
	}
	if p.s[p.i] == '(' {
		p.i++
		v, err := p.parseOr()
		if err != nil {
			return false, err
		}
		p.skipSpace()
		if p.i >= len(p.s) || p.s[p.i] != ')' {
			return false, fmt.Errorf("when condition: expected ')'")
		}
		p.i++
		return v, nil
	}
	return p.parsePredicate()
}

func (p *condParser) parsePredicate() (bool, error) {
	p.skipSpace()
	if p.i >= len(p.s) {
		return false, fmt.Errorf("when condition: expected predicate")
	}
	k := p.s[p.i]
	p.i++
	arg := strings.TrimSpace(p.readArg())
	switch k {
	case 'f':
		return matchFilePattern(p.ctx, arg)
	case 'F':
		if p.ctx.Row != nil {
			return false, nil
		}
		return matchFilePatternOther(p.ctx, arg)
	case 'd':
		return matchDirPattern(p.ctx, arg)
	case 'D':
		if p.ctx.Row != nil {
			return false, nil
		}
		return matchDirPatternOther(p.ctx, arg)
	case 't':
		return matchTypes(p.ctx, arg, false)
	case 'T':
		if p.ctx.Row != nil {
			return false, nil
		}
		return matchTypes(p.ctx, arg, true)
	default:
		return false, fmt.Errorf("when condition: unknown predicate %q (want fFdDtT)", k)
	}
}

func (p *condParser) readArg() string {
	p.skipSpace()
	start := p.i
	depth := 0
	for p.i < len(p.s) {
		c := p.s[p.i]
		switch c {
		case '(':
			depth++
		case ')':
			if depth == 0 {
				goto out
			}
			depth--
		case '&', '|':
			if depth == 0 {
				goto out
			}
		}
		p.i++
	}
out:
	return strings.TrimSpace(p.s[start:p.i])
}

func matchFilePattern(ctx *Context, pattern string) (bool, error) {
	if ctx.Row != nil {
		return matchPattern(ctx.Row.Name, pattern, ctx.ShellPatterns)
	}
	if ctx.Active == nil {
		return false, nil
	}
	ent, ok := ctx.Active.CurrentEntry()
	if !ok {
		return false, nil
	}
	return matchPattern(ent.Name, pattern, ctx.ShellPatterns)
}

func matchFilePatternOther(ctx *Context, pattern string) (bool, error) {
	if ctx.Other == nil {
		return false, nil
	}
	ent, ok := ctx.Other.CurrentEntry()
	if !ok {
		return false, nil
	}
	return matchPattern(ent.Name, pattern, ctx.ShellPatterns)
}

func matchDirPattern(ctx *Context, pattern string) (bool, error) {
	var dir string
	if ctx.Row != nil {
		dir = filepath.Clean(ctx.PanelDir)
	} else if ctx.Active != nil {
		dir = filepath.Clean(ctx.Active.PathString())
	} else {
		return false, nil
	}
	return matchPattern(dir, pattern, ctx.ShellPatterns)
}

func matchDirPatternOther(ctx *Context, pattern string) (bool, error) {
	if ctx.Other == nil {
		return false, nil
	}
	dir := filepath.Clean(ctx.Other.PathString())
	return matchPattern(dir, pattern, ctx.ShellPatterns)
}

func matchPattern(value, pattern string, shell bool) (bool, error) {
	if pattern == "" {
		return false, nil
	}
	if shell {
		ok, err := filepath.Match(pattern, value)
		if err != nil {
			return false, err
		}
		return ok, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, err
	}
	return re.MatchString(value), nil
}

func matchTypes(ctx *Context, types string, otherPanel bool) (bool, error) {
	if types == "" {
		return false, nil
	}
	if ctx.Row != nil {
		return matchEntryTypes(ctx.Row, types, ctx)
	}
	var ps *panel.State
	if otherPanel {
		ps = ctx.Other
	} else {
		ps = ctx.Active
	}
	if ps == nil {
		return false, nil
	}
	ent, ok := ps.CurrentEntry()
	if !ok {
		return false, nil
	}
	row := &localfs.Entry{Name: ent.Name, Path: ent.Path, Type: ent.Type, Mode: ent.Mode}
	return matchEntryTypes(row, types, ctx)
}

func matchEntryTypes(ent *localfs.Entry, types string, ctx *Context) (bool, error) {
	if ent == nil || types == "" {
		return false, nil
	}
	var acc bool
	for _, r := range types {
		switch r {
		case 'n':
			acc = acc || ent.Type != localfs.EntryDirectory
		case 'r':
			acc = acc || ent.Type == localfs.EntryFile
		case 'd':
			acc = acc || ent.Type == localfs.EntryDirectory
		case 'l':
			acc = acc || ent.Type == localfs.EntrySymlink
		case 'c', 'b', 'f', 's':
			acc = acc || ent.Type == localfs.EntryOther && modeLetterMatch(ent.Mode, r)
		case 'x':
			acc = acc || ent.Mode&0o111 != 0
		case 't':
			if ctx.Row != nil {
				// tagged predicate N/A in row mode
			} else if ctx.Active != nil {
				acc = acc || hasTaggedInPanelDir(ctx.Active)
			}
		default:
			// ignore unknown letters
		}
	}
	return acc, nil
}

func modeLetterMatch(m fs.FileMode, r rune) bool {
	switch r {
	case 'c':
		return m&fs.ModeCharDevice != 0
	case 'b':
		return m&fs.ModeDevice != 0 && m&fs.ModeCharDevice == 0
	case 'f':
		return m&fs.ModeNamedPipe != 0
	case 's':
		return m&fs.ModeSocket != 0
	default:
		return false
	}
}

// ValidateWhenExprs checks syntax and pattern compilability for when expressions.
func ValidateWhenExprs(exprs []string, shellPatterns bool) error {
	ctx := validationEvalContext(shellPatterns)
	for _, expr := range exprs {
		expr = strings.TrimSpace(expr)
		if expr == "" {
			continue
		}
		if _, err := EvalWhen(expr, ctx); err != nil {
			return err
		}
	}
	return nil
}

func validationEvalContext(shellPatterns bool) *Context {
	activeDir := "/menu-validate/active"
	otherDir := "/menu-validate/other"
	sampleFile := localfs.Entry{
		Name: "sample.txt",
		Path: filepath.Join(activeDir, "sample.txt"),
		Type: localfs.EntryFile,
		Mode: 0o644,
	}
	otherFile := localfs.Entry{
		Name: "other.txt",
		Path: filepath.Join(otherDir, "other.txt"),
		Type: localfs.EntryFile,
		Mode: 0o644,
	}
	active := &panel.State{
		Path:          pathloc.MustParse(activeDir),
		Entries:       []localfs.Entry{sampleFile},
		Cursor:        0,
		SelectedPaths: map[string]bool{sampleFile.Path: true},
	}
	other := &panel.State{
		Path:          pathloc.MustParse(otherDir),
		Entries:       []localfs.Entry{otherFile},
		Cursor:        0,
		SelectedPaths: map[string]bool{otherFile.Path: true},
	}
	return &Context{
		Active:        active,
		Other:         other,
		ShellPatterns: shellPatterns,
	}
}

func hasTaggedInPanelDir(ps *panel.State) bool {
	if ps == nil || len(ps.SelectedPaths) == 0 {
		return false
	}
	base := filepath.Clean(ps.PathString())
	for p := range ps.SelectedPaths {
		if filepath.Clean(filepath.Dir(p)) == base {
			return true
		}
	}
	return false
}
