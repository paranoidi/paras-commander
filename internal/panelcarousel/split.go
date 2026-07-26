package panelcarousel

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/paranoidi/paras-commander/internal/config"
)

// ColumnSplitKind identifies how a carousel column width is specified.
type ColumnSplitKind int

const (
	SplitFixed ColumnSplitKind = iota
	SplitPercent
	SplitFlex
	SplitFitChars   // "<N" / "<<N": min(measured content, N chars)
	SplitFitPercent // "<N%" / "<<N%": min(measured content, N% of innerW)
)

// ColumnSplitSpec is one carousel column width token after parsing.
type ColumnSplitSpec struct {
	Kind          ColumnSplitKind
	Value         int  // cap in chars for Fixed/FitChars; percent 0–100 for Percent/FitPercent; unused for Flex
	IgnoreOutlier bool // "<<N" / "<<N%": measure with fitListingTextLen outlier ignore
}

// Layout holds resolved carousel column split and per-column size visibility.
type Layout struct {
	Splits   [3]ColumnSplitSpec
	ShowSize [3]bool
}

// DefaultLayout returns equal-thirds split with size visible in all columns.
func DefaultLayout() Layout {
	return Layout{
		Splits: [3]ColumnSplitSpec{
			{Kind: SplitFlex},
			{Kind: SplitFlex},
			{Kind: SplitFlex},
		},
		ShowSize: [3]bool{true, true, true},
	}
}

// ParseLayout builds a Layout from config strings. split must have exactly 3 entries;
// showSize must be empty (all true) or exactly 3 booleans.
func ParseLayout(split []string, showSize []bool) (Layout, error) {
	if len(split) != 3 {
		return Layout{}, fmt.Errorf("carousel.split: want 3 entries, got %d", len(split))
	}
	out := DefaultLayout()
	for i, tok := range split {
		spec, err := parseSplitToken(strings.TrimSpace(tok), i)
		if err != nil {
			return Layout{}, fmt.Errorf("carousel.split[%d]: %w", i, err)
		}
		out.Splits[i] = spec
	}
	switch len(showSize) {
	case 0:
		// defaults already true
	case 3:
		out.ShowSize = [3]bool{showSize[0], showSize[1], showSize[2]}
	default:
		return Layout{}, fmt.Errorf("carousel.show_size: want 0 or 3 entries, got %d", len(showSize))
	}
	return out, nil
}

func parseSplitToken(tok string, index int) (ColumnSplitSpec, error) {
	if tok == "*" {
		return ColumnSplitSpec{Kind: SplitFlex}, nil
	}
	if strings.HasPrefix(tok, "<") {
		if index == 2 {
			return ColumnSplitSpec{}, fmt.Errorf("fit-mode %q not allowed on child column", tok)
		}
		ignoreOutlier := strings.HasPrefix(tok, "<<")
		rest := strings.TrimPrefix(tok, "<")
		if ignoreOutlier {
			rest = strings.TrimPrefix(rest, "<")
		}
		if strings.HasSuffix(rest, "%") {
			pctStr := strings.TrimSpace(strings.TrimSuffix(rest, "%"))
			if pctStr == "" {
				return ColumnSplitSpec{}, fmt.Errorf("invalid percent %q", tok)
			}
			pct, err := strconv.Atoi(pctStr)
			if err != nil || pct < 0 || pct > 100 {
				return ColumnSplitSpec{}, fmt.Errorf("invalid percent %q", tok)
			}
			return ColumnSplitSpec{Kind: SplitFitPercent, Value: pct, IgnoreOutlier: ignoreOutlier}, nil
		}
		n, err := strconv.Atoi(rest)
		if err != nil || n < 1 {
			return ColumnSplitSpec{}, fmt.Errorf("invalid width %q", tok)
		}
		return ColumnSplitSpec{Kind: SplitFitChars, Value: n, IgnoreOutlier: ignoreOutlier}, nil
	}
	if strings.HasSuffix(tok, "%") {
		pctStr := strings.TrimSpace(strings.TrimSuffix(tok, "%"))
		if pctStr == "" {
			return ColumnSplitSpec{}, fmt.Errorf("invalid percent %q", tok)
		}
		pct, err := strconv.Atoi(pctStr)
		if err != nil || pct < 0 || pct > 100 {
			return ColumnSplitSpec{}, fmt.Errorf("invalid percent %q", tok)
		}
		return ColumnSplitSpec{Kind: SplitPercent, Value: pct}, nil
	}
	n, err := strconv.Atoi(tok)
	if err != nil || n < 1 {
		return ColumnSplitSpec{}, fmt.Errorf("invalid width %q", tok)
	}
	return ColumnSplitSpec{Kind: SplitFixed, Value: n}, nil
}

// Resolve computes parent, center, and child column widths for inner panel width innerW.
// When showChild is false, the child width is folded into the center column.
//
// This is a thin wrapper over ResolveMeasured with no measured content, so any fit-mode
// column resolves to its configured worst-case cap. Use ResolveMeasured directly at real
// render call sites where live listing content is available to measure.
func (l Layout) Resolve(innerW int, showChild bool) [3]int {
	return l.ResolveMeasured(innerW, showChild, [3]int{})
}

// ResolveMeasured computes parent, center, and child column widths for inner panel width
// innerW, same as Resolve, but lets fit-mode columns (SplitFitChars/SplitFitPercent) size to
// the real measured content width in measuredFitWidth when it's smaller than the configured
// cap. measuredFitWidth[i] == 0 means "not measured" — the column falls back to its cap.
func (l Layout) ResolveMeasured(innerW int, showChild bool, measuredFitWidth [3]int) [3]int {
	if innerW < 1 {
		return [3]int{}
	}
	widths := resolveWidths(innerW, l.Splits, measuredFitWidth)
	if !showChild {
		widths[1] += widths[2]
		widths[2] = 0
	}
	return widths
}

func resolveWidths(innerW int, splits [3]ColumnSplitSpec, measuredFitWidth [3]int) [3]int {
	var out [3]int
	fixedSum := 0
	for i, s := range splits {
		switch s.Kind {
		case SplitFixed:
			out[i] = s.Value
			fixedSum += s.Value
		case SplitFitChars:
			w := s.Value // cap
			if measuredFitWidth[i] > 0 && measuredFitWidth[i] < w {
				w = measuredFitWidth[i]
			}
			out[i] = w
			fixedSum += w
		case SplitFitPercent:
			capW := (innerW*s.Value + 50) / 100
			w := capW
			if measuredFitWidth[i] > 0 && measuredFitWidth[i] < w {
				w = measuredFitWidth[i]
			}
			out[i] = w
			fixedSum += w
		}
	}
	remaining := innerW - fixedSum
	if remaining < 0 {
		remaining = 0
	}

	percentUsed := 0
	for i, s := range splits {
		if s.Kind == SplitPercent {
			w := (remaining*s.Value + 50) / 100
			out[i] = w
			percentUsed += w
		}
	}
	afterPercent := remaining - percentUsed
	if afterPercent < 0 {
		afterPercent = 0
	}

	flexIdx := make([]int, 0, 3)
	for i, s := range splits {
		if s.Kind == SplitFlex {
			flexIdx = append(flexIdx, i)
		}
	}
	if len(flexIdx) > 0 {
		base := afterPercent / len(flexIdx)
		extra := afterPercent - base*len(flexIdx)
		for j, i := range flexIdx {
			out[i] = base
			if j == len(flexIdx)-1 {
				out[i] += extra
			}
		}
	}

	// Absorb any rounding drift into the last non-fixed, non-fit column, else last column.
	sum := out[0] + out[1] + out[2]
	if sum != innerW {
		drift := innerW - sum
		for i := 2; i >= 0; i-- {
			k := splits[i].Kind
			if k != SplitFixed && k != SplitFitChars && k != SplitFitPercent {
				out[i] += drift
				break
			}
		}
	}
	return out
}

// MinInnerWidth returns the minimum interior width required for carousel layout.
//
// ponytail: uses Resolve (unmeasured), so fit-mode columns are sized at their configured
// worst-case cap here — this is a structural, pre-render check with no live listing to
// measure. Real render call sites use ResolveMeasured with actual content widths instead.
func (l Layout) MinInnerWidth(showChild bool) int {
	minCol := config.MinCarouselColumnWidth
	lo, hi := 1, minCol
	for !l.widthMeetsMinimum(hi, showChild, minCol) {
		hi *= 2
		if hi > 4096 {
			return hi
		}
	}
	for lo < hi {
		mid := (lo + hi) / 2
		if l.widthMeetsMinimum(mid, showChild, minCol) {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return lo
}

func (l Layout) widthMeetsMinimum(innerW int, showChild bool, minCol int) bool {
	if innerW < 1 {
		return false
	}
	widths := l.Resolve(innerW, showChild)
	if !showChild {
		return widths[1] >= minCol
	}
	for i, s := range l.Splits {
		if s.Kind == SplitFlex && widths[i] < minCol {
			return false
		}
	}
	return true
}
