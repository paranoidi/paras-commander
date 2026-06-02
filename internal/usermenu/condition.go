package usermenu

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
)

// EvalContext carries panel state for condition evaluation.
type EvalContext struct {
	Active *panel.State
	Other  *panel.State
	// ShellPatterns true → filepath.Match for f/F/d/D; false → regexp on basename or full path.
	ShellPatterns bool
}

// EvalWhen evaluates a visibility expression; empty string is true.
func EvalWhen(expr string, ctx *EvalContext) (bool, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return true, nil
	}
	expr = stripLeadingConditionPrefix(expr)
	p := &condParser{s: expr, ctx: ctx}
	return p.parseOr()
}

// EvalWhenAny evaluates multiple visibility expressions with OR semantics.
//
// Rules:
//   - Empty slice (or only empty strings) ⇒ true.
//   - Otherwise returns true if any expression matches.
//   - If any expression fails to parse/compile, returns an error immediately.
func EvalWhenAny(exprs []string, ctx *EvalContext) (bool, error) {
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
	if strings.HasPrefix(s, "?") { // debug prefix from MC — ignored
		s = strings.TrimSpace(s[1:])
	}
	return strings.TrimSpace(s)
}

type condParser struct {
	s   string
	i   int
	ctx *EvalContext
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
		return false, fmt.Errorf("user menu condition: unexpected end")
	}
	if p.s[p.i] == '(' {
		p.i++
		v, err := p.parseOr()
		if err != nil {
			return false, err
		}
		p.skipSpace()
		if p.i >= len(p.s) || p.s[p.i] != ')' {
			return false, fmt.Errorf("user menu condition: expected ')'")
		}
		p.i++
		return v, nil
	}
	return p.parsePredicate()
}

func (p *condParser) parsePredicate() (bool, error) {
	p.skipSpace()
	if p.i >= len(p.s) {
		return false, fmt.Errorf("user menu condition: expected predicate")
	}
	k := p.s[p.i]
	p.i++
	arg := strings.TrimSpace(p.readArg())
	switch k {
	case 'f':
		return matchFilePattern(p.ctx.Active, arg, p.ctx.ShellPatterns)
	case 'F':
		return matchFilePattern(p.ctx.Other, arg, p.ctx.ShellPatterns)
	case 'd':
		return matchDirPattern(p.ctx.Active, arg, p.ctx.ShellPatterns)
	case 'D':
		return matchDirPattern(p.ctx.Other, arg, p.ctx.ShellPatterns)
	case 't':
		return matchTypes(p.ctx.Active, arg)
	case 'T':
		return matchTypes(p.ctx.Other, arg)
	default:
		return false, fmt.Errorf("user menu condition: unknown predicate %q (want fFdDtT)", k)
	}
}

// readArg reads pattern / type letters until top-level & | ) or end.
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

func matchFilePattern(ps *panel.State, pattern string, shell bool) (bool, error) {
	if ps == nil {
		return false, nil
	}
	ent, ok := ps.CurrentEntry()
	if !ok {
		return false, nil
	}
	name := ent.Name
	return matchPattern(name, pattern, shell)
}

func matchDirPattern(ps *panel.State, pattern string, shell bool) (bool, error) {
	if ps == nil {
		return false, nil
	}
	dir := filepath.Clean(ps.PathString())
	return matchPattern(dir, pattern, shell)
}

func matchPattern(value, pattern string, shell bool) (bool, error) {
	if pattern == "" {
		return false, nil
	}
	if shell {
		ok, _ := filepath.Match(pattern, value)
		return ok, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, err
	}
	return re.MatchString(value), nil
}

func matchTypes(ps *panel.State, types string) (bool, error) {
	if ps == nil || types == "" {
		return false, nil
	}
	ent, ok := ps.CurrentEntry()
	if !ok {
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
			acc = acc || hasTaggedInPanelDir(ps)
		default:
			// ignore unknown letters like MC debug noise
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
