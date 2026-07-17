package find

// ScrollingQueryEdit is an opaque handle for find-dialog query editing.
// The app host produces and consumes these; find never inspects the payload.
type ScrollingQueryEdit struct {
	v any
}

// NewScrollingQueryEdit wraps a host-owned edit value.
func NewScrollingQueryEdit(v any) ScrollingQueryEdit {
	return ScrollingQueryEdit{v: v}
}

// Value returns the host-owned edit value for type assertion in the app layer.
func (e ScrollingQueryEdit) Value() any { return e.v }
