package panel

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

// MetaAlertLevel is how strongly a meta cell asks for attention.
type MetaAlertLevel uint8

// Meta alert levels, weakest first.
const (
	MetaAlertNone MetaAlertLevel = iota
	MetaAlertWarn
	MetaAlertCritical
)

type metaAlertOp uint8

const (
	metaAlertContains metaAlertOp = iota
	metaAlertGT
	metaAlertGE
	metaAlertLT
	metaAlertLE
	metaAlertEQ
	metaAlertNE
)

// metaAlertOps lists the operator prefixes, longer ones first so ">=" is not read as ">".
var metaAlertOps = []struct {
	prefix string
	op     metaAlertOp
}{
	{">=", metaAlertGE},
	{"<=", metaAlertLE},
	{"!=", metaAlertNE},
	{">", metaAlertGT},
	{"<", metaAlertLT},
	{"=", metaAlertEQ},
}

// metaAlertRule is one compiled rule: text is the lowercased operand, num its value when
// numeric is set.
type metaAlertRule struct {
	op      metaAlertOp
	text    string
	num     float64
	numeric bool
}

// MetaAlerts holds a meta column's compiled warn and critical rules. The zero value never alerts.
type MetaAlerts struct {
	warn, critical []metaAlertRule
}

// ParseMetaAlerts compiles the warn and critical rule lists of a meta column. A rule starting
// with >, >=, <, <=, = or != compares against its operand; any other text is a substring of the
// value. Text compares case-insensitively.
func ParseMetaAlerts(warn, critical []string) (MetaAlerts, error) {
	w, err := parseMetaAlertRules(warn)
	if err != nil {
		return MetaAlerts{}, fmt.Errorf("warn: %w", err)
	}
	c, err := parseMetaAlertRules(critical)
	if err != nil {
		return MetaAlerts{}, fmt.Errorf("critical: %w", err)
	}
	return MetaAlerts{warn: w, critical: c}, nil
}

// MustMetaAlerts is ParseMetaAlerts for built-in rules, which are constants: it panics on an
// invalid rule.
func MustMetaAlerts(warn, critical []string) MetaAlerts {
	a, err := ParseMetaAlerts(warn, critical)
	if err != nil {
		panic(err)
	}
	return a
}

func parseMetaAlertRules(rules []string) ([]metaAlertRule, error) {
	if len(rules) == 0 {
		return nil, nil
	}
	out := make([]metaAlertRule, len(rules))
	for i, s := range rules {
		r, err := parseMetaAlertRule(s)
		if err != nil {
			return nil, fmt.Errorf("rule %q: %w", s, err)
		}
		out[i] = r
	}
	return out, nil
}

func parseMetaAlertRule(s string) (metaAlertRule, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return metaAlertRule{}, errors.New("empty rule")
	}
	r := metaAlertRule{op: metaAlertContains}
	operand := s
	for _, o := range metaAlertOps {
		if rest, ok := strings.CutPrefix(s, o.prefix); ok {
			r.op = o.op
			operand = strings.TrimSpace(rest)
			break
		}
	}
	if operand == "" {
		return metaAlertRule{}, errors.New("missing value")
	}
	r.text = strings.ToLower(operand)
	if r.op == metaAlertContains {
		return r, nil
	}
	r.num, r.numeric = metaAlertNumber(operand)
	if !r.numeric && r.op != metaAlertEQ && r.op != metaAlertNE {
		return metaAlertRule{}, errors.New("value is not a number")
	}
	return r, nil
}

// metaAlertNumber is MetaNumber without NaN, which compares unequal to everything.
func metaAlertNumber(s string) (float64, bool) {
	f, ok := MetaNumber(s)
	return f, ok && !math.IsNaN(f)
}

func (r metaAlertRule) matches(value, lower string) bool {
	if r.op == metaAlertContains {
		return strings.Contains(lower, r.text)
	}
	v, numeric := metaAlertNumber(value)
	switch r.op {
	case metaAlertEQ, metaAlertNE:
		same := lower == r.text
		if numeric && r.numeric {
			same = v == r.num
		}
		return same == (r.op == metaAlertEQ)
	}
	if !numeric {
		return false
	}
	switch r.op {
	case metaAlertGT:
		return v > r.num
	case metaAlertGE:
		return v >= r.num
	case metaAlertLT:
		return v < r.num
	default:
		return v <= r.num
	}
}

// Empty reports whether no rule is set, so a column can skip evaluating its cells.
func (a MetaAlerts) Empty() bool { return len(a.warn) == 0 && len(a.critical) == 0 }

// Level returns the level of a raw meta value: critical when any critical rule matches, else
// warn when any warn rule does. Blank and multi-field (tab or newline split) values never alert.
func (a MetaAlerts) Level(raw string) MetaAlertLevel {
	value := strings.TrimSpace(raw)
	if value == "" || strings.ContainsAny(value, "\t\r\n") {
		return MetaAlertNone
	}
	lower := strings.ToLower(value)
	matchesAny := func(rules []metaAlertRule) bool {
		for _, r := range rules {
			if r.matches(value, lower) {
				return true
			}
		}
		return false
	}
	switch {
	case matchesAny(a.critical):
		return MetaAlertCritical
	case matchesAny(a.warn):
		return MetaAlertWarn
	}
	return MetaAlertNone
}
