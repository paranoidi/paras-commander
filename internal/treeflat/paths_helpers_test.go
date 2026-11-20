package treeflat

// fromPaths is the one-shot tree over labels: the reference the incremental PathTree is
// checked against.
func fromPaths(labels, ids []string) []Node[PathNode] {
	return NewPathTree(labels, ids).Nodes()
}
