// Package treeflat is the shared tree→visible-rows model used by tree-style
// list views (find-duplicates modes today; the planned file-list tree layout
// later). It owns no rendering and no input handling: callers build a Node
// tree, flatten it against an expanded-state predicate, and run their own
// cursor/scroll/render over the resulting rows.
package treeflat

// Node is one tree node. ID must be stable across rebuilds (abs path, group
// hash, …) so expand state and cursor position survive re-flattening.
type Node[T any] struct {
	ID       string
	Value    T
	Children []Node[T]
}

// Row is one visible row produced by Flatten.
type Row[T any] struct {
	ID          string
	Value       T
	Depth       int
	HasChildren bool
	Expanded    bool
	// LastChild is true when this node is the last among its siblings.
	LastChild bool
	// AncestorHasNext has length Depth-1. AncestorHasNext[i] is true when the
	// ancestor at depth i+1 on the path to this row had younger siblings.
	AncestorHasNext []bool
}

// Flatten walks roots depth-first and returns the visible rows. expanded
// decides whether a node with children shows them; callers keep either a
// collapsed-set (default expanded) or an expanded-set (default collapsed)
// behind the predicate. A nil predicate expands everything.
func Flatten[T any](roots []Node[T], expanded func(id string) bool) []Row[T] {
	var out []Row[T]
	var walk func(nodes []Node[T], depth int, parentContinues []bool)
	walk = func(nodes []Node[T], depth int, parentContinues []bool) {
		for i := range nodes {
			n := &nodes[i]
			last := i == len(nodes)-1
			hasKids := len(n.Children) > 0
			open := hasKids && (expanded == nil || expanded(n.ID))
			continues := append([]bool(nil), parentContinues...)
			out = append(out, Row[T]{
				ID:              n.ID,
				Value:           n.Value,
				Depth:           depth,
				HasChildren:     hasKids,
				Expanded:        open,
				LastChild:       last,
				AncestorHasNext: continues,
			})
			if open {
				var childContinues []bool
				if depth > 0 {
					childContinues = append(append([]bool(nil), parentContinues...), !last)
				}
				walk(n.Children, depth+1, childContinues)
			}
		}
	}
	walk(roots, 0, nil)
	return out
}

// ExpandableIDs returns the IDs of every node that has children, in
// depth-first order (collapse-all helpers).
func ExpandableIDs[T any](roots []Node[T]) []string {
	var out []string
	var walk func(nodes []Node[T])
	walk = func(nodes []Node[T]) {
		for i := range nodes {
			if len(nodes[i].Children) > 0 {
				out = append(out, nodes[i].ID)
				walk(nodes[i].Children)
			}
		}
	}
	walk(roots)
	return out
}
