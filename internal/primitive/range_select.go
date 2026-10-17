package primitive

import "fmt"

// RangeAnchorIndex returns the index in [0, n) whose key equals anchor, or -1 when anchor is
// empty or not among the rows. It is the shared "find the pending range-select start" step of
// the panel, Compare and Dedup range select.
func RangeAnchorIndex(n int, anchor string, key func(i int) string) int {
	if anchor == "" {
		return -1
	}
	for i := 0; i < n; i++ {
		if key(i) == anchor {
			return i
		}
	}
	return -1
}

// RangeResult is the outcome of one range-select press.
type RangeResult int

const (
	RangeNone    RangeResult = iota // nothing to anchor or select
	RangeStarted                    // anchor set; the caller advances the cursor
	RangeDone                       // range selected, anchor cleared
)

// RangeMessage is the toast for a range-select press ("" for RangeNone); keyLabel is the
// action's binding. Shared by the panel, Compare and Dedup.
func RangeMessage(res RangeResult, added int, keyLabel string) string {
	switch res {
	case RangeStarted:
		return fmt.Sprintf("Range start set, move and press %s on the last item", keyLabel)
	case RangeDone:
		return fmt.Sprintf("Selected %d items", added)
	}
	return ""
}
