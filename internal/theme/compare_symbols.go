package theme

import "strings"

const (
	SymbolKeyCompareEqual         = "compare.equal"
	SymbolKeyCompareRelocated     = "compare.relocated"
	SymbolKeyComparePrimaryOnly   = "compare.primary_only"
	SymbolKeyCompareSecondaryOnly = "compare.secondary_only"
	SymbolKeyCompareContentDiff   = "compare.content_diff"
	SymbolKeyComparePending       = "compare.pending"
	SymbolKeyCompareError         = "compare.error"
)

func (t Theme) SymbolCompareEqual() string { return t.compareSymbol(SymbolKeyCompareEqual, "=") }
func (t Theme) SymbolCompareRelocated() string {
	return t.compareSymbol(SymbolKeyCompareRelocated, "\u2194")
}
func (t Theme) SymbolComparePrimaryOnly() string {
	return t.compareSymbol(SymbolKeyComparePrimaryOnly, "<")
}
func (t Theme) SymbolCompareSecondaryOnly() string {
	return t.compareSymbol(SymbolKeyCompareSecondaryOnly, ">")
}
func (t Theme) SymbolCompareContentDiff() string {
	return t.compareSymbol(SymbolKeyCompareContentDiff, "\u2260")
}
func (t Theme) SymbolComparePending() string { return t.compareSymbol(SymbolKeyComparePending, "?") }
func (t Theme) SymbolCompareError() string   { return t.compareSymbol(SymbolKeyCompareError, "!") }

func (t Theme) compareSymbol(key, fallback string) string {
	if t.Symbols != nil {
		if s := strings.TrimSpace(t.Symbols[key]); s != "" {
			return s
		}
	}
	return fallback
}
