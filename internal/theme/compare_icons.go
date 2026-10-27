package theme

const (
	IconKeyCompareEqual         = "compare.equal"
	IconKeyCompareRelocated     = "compare.relocated"
	IconKeyComparePrimaryOnly   = "compare.primary_only"
	IconKeyCompareSecondaryOnly = "compare.secondary_only"
	IconKeyCompareContentDiff   = "compare.content_diff"
	IconKeyComparePending       = "compare.pending"
	IconKeyCompareError         = "compare.error"
)

func (t Theme) IconCompareEqual() string { return t.iconOr(IconKeyCompareEqual, "=") }
func (t Theme) IconCompareRelocated() string {
	return t.iconOr(IconKeyCompareRelocated, "\u2194")
}
func (t Theme) IconComparePrimaryOnly() string {
	return t.iconOr(IconKeyComparePrimaryOnly, "<")
}
func (t Theme) IconCompareSecondaryOnly() string {
	return t.iconOr(IconKeyCompareSecondaryOnly, ">")
}
func (t Theme) IconCompareContentDiff() string {
	return t.iconOr(IconKeyCompareContentDiff, "\u2260")
}
func (t Theme) IconComparePending() string {
	return t.iconOr(IconKeyComparePending, "\U000f02ca")
}
func (t Theme) IconCompareError() string { return t.iconOr(IconKeyCompareError, "!") }
