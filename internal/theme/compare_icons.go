package theme

const (
	IconKeyCompareEqual         = "compare.equal"
	IconKeyCompareRelocated     = "compare.relocated"
	IconKeyComparePrimaryOnly   = "compare.primary_only"
	IconKeyCompareSecondaryOnly = "compare.secondary_only"
	IconKeyCompareContentDiff   = "compare.content_diff"
	IconKeyComparePending       = "compare.pending"
	IconKeyCompareError         = "compare.error"
	IconKeyCompareNameLeft      = "compare.name_left"
	IconKeyCompareNameRight     = "compare.name_right"
)

func (t Theme) IconCompareEqual() string { return t.icon(IconKeyCompareEqual) }
func (t Theme) IconCompareRelocated() string {
	return t.icon(IconKeyCompareRelocated)
}
func (t Theme) IconComparePrimaryOnly() string {
	return t.icon(IconKeyComparePrimaryOnly)
}
func (t Theme) IconCompareSecondaryOnly() string {
	return t.icon(IconKeyCompareSecondaryOnly)
}
func (t Theme) IconCompareContentDiff() string {
	return t.icon(IconKeyCompareContentDiff)
}
func (t Theme) IconComparePending() string {
	return t.icon(IconKeyComparePending)
}
func (t Theme) IconCompareError() string { return t.icon(IconKeyCompareError) }

// IconCompareNameLeft replaces the relocated icon on a row whose merge takes the left path.
func (t Theme) IconCompareNameLeft() string {
	return t.icon(IconKeyCompareNameLeft)
}

// IconCompareNameRight replaces the relocated icon on a row whose merge takes the right path.
func (t Theme) IconCompareNameRight() string {
	return t.icon(IconKeyCompareNameRight)
}
