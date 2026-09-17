package theme

import "strings"

const (
	IconKeyCompareEqual         = "compare.equal"
	IconKeyCompareRelocated     = "compare.relocated"
	IconKeyComparePrimaryOnly   = "compare.primary_only"
	IconKeyCompareSecondaryOnly = "compare.secondary_only"
	IconKeyCompareContentDiff   = "compare.content_diff"
	IconKeyComparePending       = "compare.pending"
	IconKeyCompareError         = "compare.error"
)

func (t Theme) IconCompareEqual() string { return t.compareIcon(IconKeyCompareEqual, "=") }
func (t Theme) IconCompareRelocated() string {
	return t.compareIcon(IconKeyCompareRelocated, "\u2194")
}
func (t Theme) IconComparePrimaryOnly() string {
	return t.compareIcon(IconKeyComparePrimaryOnly, "<")
}
func (t Theme) IconCompareSecondaryOnly() string {
	return t.compareIcon(IconKeyCompareSecondaryOnly, ">")
}
func (t Theme) IconCompareContentDiff() string {
	return t.compareIcon(IconKeyCompareContentDiff, "\u2260")
}
func (t Theme) IconComparePending() string {
	return t.compareIcon(IconKeyComparePending, "\U000f02ca")
}
func (t Theme) IconCompareError() string { return t.compareIcon(IconKeyCompareError, "!") }

func (t Theme) compareIcon(key, fallback string) string {
	if t.Icons != nil {
		if s := strings.TrimSpace(t.Icons[key]); s != "" {
			return s
		}
	}
	return fallback
}
