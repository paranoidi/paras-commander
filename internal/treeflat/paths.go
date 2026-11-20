package treeflat

import (
	"slices"
	"strings"
)

// PathNode is the value of one PathTree node.
type PathNode struct {
	// Path is the caller's id for a leaf, or the directory path of a Group.
	Path string
	// Display is the leaf's last label component, or a Group's folded components joined by "/".
	Display string
	// Group marks a synthetic directory (no path of its own was supplied).
	Group bool
}

// pathTrie is one trie node. built caches the finished Node of the folded chain that starts
// here, valid while dirty is false: Add and Remove mark every node on the changed path dirty,
// so Nodes rebuilds only those and reuses every other subtree as is.
type pathTrie struct {
	name   string
	id     string      // set on a leaf: the id of the label ending here
	any    string      // id of one leaf below (or at) this node
	kids   []*pathTrie // in no particular order: build sorts them in place
	byName map[string]*pathTrie
	value  PathNode         // built: the chain's value ...
	nodes  []Node[PathNode] // ... and its children
	below  int32            // components between this node and that leaf
	leaf   bool
	dirty  bool
}

// pathTrieMapMin is the kid count above which find indexes the kids by name.
// ponytail: a linear scan below it; a node with thousands of kids pays one map build on first lookup.
const pathTrieMapMin = 8

func (t *pathTrie) find(name string) *pathTrie {
	if len(t.kids) <= pathTrieMapMin {
		for _, k := range t.kids {
			if k.name == name {
				return k
			}
		}
		return nil
	}
	if t.byName == nil {
		t.byName = make(map[string]*pathTrie, len(t.kids))
		for _, k := range t.kids {
			t.byName[k.name] = k
		}
	}
	return t.byName[name]
}

// child returns t's kid called name, creating it. sorted means the labels arrive in
// componentCmp order (and only ever append), so a kid's labels are contiguous and only the
// last kid can match: a miss there is a new kid without a lookup.
func (t *pathTrie) child(name string, sorted bool) *pathTrie {
	if n := len(t.kids); n > 0 && t.kids[n-1].name == name {
		return t.kids[n-1]
	}
	if !sorted {
		if k := t.find(name); k != nil {
			return k
		}
	}
	k := &pathTrie{name: name}
	if t.byName != nil {
		t.byName[name] = k
	}
	t.kids = append(t.kids, k)
	return k
}

// component splits the first component off label[pos:]: its name and the next position.
// more is false for the last component. An absolute label keeps its root slash on the first one.
func component(label string, pos int) (name string, next int, more bool) {
	search := pos
	if pos == 0 && strings.HasPrefix(label, "/") {
		search = 1
	}
	if j := strings.IndexByte(label[search:], '/'); j >= 0 {
		return label[pos : search+j], search + j + 1, true
	}
	return label[pos:], len(label), false
}

func (t *pathTrie) add(label, id string, sorted bool) {
	below := strings.Count(label, "/")
	if !strings.HasPrefix(label, "/") {
		below++
	}
	below--
	t.dirty = true
	for pos, more := 0, true; more; below-- {
		var name string
		name, pos, more = component(label, pos)
		t = t.child(name, sorted)
		t.dirty = true
		if t.any == "" {
			t.any, t.below = id, int32(below)
		}
		if !more {
			t.leaf, t.id = true, id
		}
	}
}

// remove drops the leaf at label (nothing when there is none), prunes the nodes left empty and
// repoints the representative leaf of every ancestor that used the removed one.
func (t *pathTrie) remove(label string) {
	stack := make([]*pathTrie, 1, 16)
	stack[0] = t
	for pos, more := 0, true; more; {
		var name string
		name, pos, more = component(label, pos)
		if t = t.find(name); t == nil {
			return
		}
		stack = append(stack, t)
	}
	last := stack[len(stack)-1]
	if !last.leaf {
		return
	}
	gone := last.id
	last.leaf, last.id = false, ""
	for i := len(stack) - 1; i >= 1; i-- {
		n, p := stack[i], stack[i-1]
		p.dirty, n.dirty = true, true
		if !n.leaf && len(n.kids) == 0 {
			p.removeKid(n)
		} else if n.any == gone {
			n.any, n.below = "", 0
			if n.leaf {
				n.any = n.id
			} else {
				n.any, n.below = n.kids[0].any, n.kids[0].below+1
			}
		}
	}
}

func (t *pathTrie) removeKid(k *pathTrie) {
	if i := slices.Index(t.kids, k); i >= 0 {
		t.kids = slices.Delete(t.kids, i, i+1)
	}
	delete(t.byName, k.name)
}

// componentCmp orders labels component by component ("/" sorts below every other byte),
// which keeps all labels below one directory contiguous. Plain string order does not
// ("foo/x" sorts after "foo.txt" but "foo" and "foo/x" are one directory).
func componentCmp(a, b string) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			if a[i] == '/' {
				return -1
			}
			if b[i] == '/' {
				return 1
			}
			return int(a[i]) - int(b[i])
		}
	}
	return len(a) - len(b)
}

// PathTree is a folded directory tree of "/"-separated labels that can change one label at a
// time: Nodes returns exactly what NewPathTree builds for the current label set, rebuilding only
// the nodes a change touched. The zero value is an empty tree. Not safe for concurrent use.
type PathTree struct{ root *pathTrie }

func (p *PathTree) trie() *pathTrie {
	if p.root == nil {
		p.root = &pathTrie{}
	}
	return p.root
}

// Add makes label a leaf identified by id (replacing the id of an existing leaf).
func (p *PathTree) Add(label, id string) { p.trie().add(label, id, false) }

// Remove drops the leaf at label and every directory it leaves empty; a label that is not a
// leaf (never added, or only a directory of others) changes nothing.
func (p *PathTree) Remove(label string) { p.trie().remove(label) }

// Nodes returns the tree's roots. The slices are shared with the tree and with the result of an
// earlier call: read them, never modify them.
func (p *PathTree) Nodes() []Node[PathNode] {
	t := p.trie()
	if t.dirty || t.nodes == nil {
		t.nodes, t.dirty = t.children(), false
	}
	return t.nodes
}

// NewPathTree builds a folded directory tree from "/"-separated labels in one pass (ids[i]
// identifies labels[i]; both are the caller's, e.g. a path relative to a base directory and its
// absolute path). A synthetic directory whose only child is another synthetic directory folds
// into it, so a lone "foo/bar/asdf/file.txt" is the node "foo/bar/asdf" with the single child
// "file.txt". A label that is itself supplied (a selected directory) is a leaf, never a Group.
// Siblings sort directories first, then by name. Node IDs are the node's Path. Labels already in
// component order (the usual case for sorted paths) skip the per-node lookups.
func NewPathTree(labels, ids []string) *PathTree {
	p := &PathTree{root: &pathTrie{}}
	sorted := slices.IsSortedFunc(labels, componentCmp)
	for i, label := range labels {
		p.root.add(label, ids[i], sorted)
	}
	return p
}

func (t *pathTrie) children() []Node[PathNode] {
	slices.SortStableFunc(t.kids, func(a, b *pathTrie) int {
		if da, db := len(a.kids) > 0, len(b.kids) > 0; da != db {
			if da {
				return -1
			}
			return 1
		}
		return strings.Compare(a.name, b.name)
	})
	out := make([]Node[PathNode], len(t.kids))
	for i, k := range t.kids {
		out[i] = k.build()
	}
	return out
}

// build returns the Node of the folded chain starting at t, from the cache while t is clean.
func (t *pathTrie) build() Node[PathNode] {
	if !t.dirty {
		return Node[PathNode]{ID: t.value.Path, Value: t.value, Children: t.nodes}
	}
	e, display := t, t.name
	for !e.leaf && len(e.kids) == 1 && !e.kids[0].leaf {
		e = e.kids[0]
		display += "/" + e.name
	}
	v := PathNode{Display: display, Group: !e.leaf, Path: e.id}
	if v.Group {
		v.Path = e.any
		for range int(e.below) {
			if i := strings.LastIndex(v.Path, "/"); i >= 0 {
				v.Path = v.Path[:i]
			}
		}
	}
	t.value, t.nodes, t.dirty = v, e.children(), false
	return Node[PathNode]{ID: v.Path, Value: v, Children: t.nodes}
}
