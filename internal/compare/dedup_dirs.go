package compare

import (
	"cmp"
	"crypto/sha256"
	"encoding/binary"
	"slices"
	"strings"
)

// DedupDirKind classifies a duplicate directory group.
type DedupDirKind int

const (
	DirNone    DedupDirKind = iota
	DirExact                // names, structure and content all match
	DirContent              // same multiset of file contents; names and layout ignored
)

// DedupDirGroup is a set of directories whose whole recursive content matches.
type DedupDirGroup struct {
	Kind   DedupDirKind
	Digest [32]byte
	Size   int64
	Files  int
	Rels   []string // sorted rel paths of the member directories
	// Hidden is parallel to Rels: the member's subtree holds entries the walk
	// dropped (dotfiles, ignored, symlinks, unreadable) or empty subdirectories,
	// which the verdict did not cover.
	Hidden []bool
}

type dedupDirNode struct {
	files   map[string][32]byte // direct files by name
	subs    map[string]bool     // direct subdirectory names
	bad     bool                // subtree holds a file without a full hash
	hidden  bool
	nfiles  int
	size    int64
	hashes  [][32]byte
	exact   [32]byte
	content [32]byte
}

// groupDirsByDigest folds file hashes into per-directory Merkle digests and
// returns the maximal duplicate directory groups of both tiers. files is every
// walked file; hashOf holds full hashes of the files that have one. skipped holds
// rel dirs that lost walk-dropped entries and dirs every entered directory.
func groupDirsByDigest(files []FileRecord, hashOf map[string][32]byte, skipped, dirs map[string]bool, only []string) []DedupDirGroup {
	nodes := map[string]*dedupDirNode{}
	node := func(rel string) *dedupDirNode {
		n := nodes[rel]
		if n == nil {
			n = &dedupDirNode{files: map[string][32]byte{}, subs: map[string]bool{}}
			nodes[rel] = n
		}
		return n
	}
	link := func(rel string) {
		for rel != "" {
			parent := RelDir(rel)
			node(parent).subs[RelBase(rel)] = true
			node(rel)
			rel = parent
		}
	}
	for rel := range dirs {
		link(rel)
	}
	for _, f := range files {
		dir := RelDir(f.Rel)
		link(dir)
		n := node(dir)
		h, ok := hashOf[f.Rel]
		if !ok {
			n.bad = true
		}
		n.files[RelBase(f.Rel)] = h
		n.nfiles++
		n.size += f.Size
	}
	for rel := range skipped {
		link(rel)
		node(rel).hidden = true
	}

	order := make([]string, 0, len(nodes))
	for rel := range nodes {
		order = append(order, rel)
	}
	// Deepest first so children are folded before their parent.
	slices.SortFunc(order, func(a, b string) int {
		return cmp.Or(cmp.Compare(dirDepth(b), dirDepth(a)), cmp.Compare(a, b))
	})
	for _, rel := range order {
		n := nodes[rel]
		names := make([]string, 0, len(n.files)+len(n.subs))
		for name := range n.files {
			names = append(names, name)
			n.hashes = append(n.hashes, n.files[name])
		}
		for name := range n.subs {
			names = append(names, name)
		}
		slices.Sort(names)
		h := sha256.New()
		for _, name := range names {
			var child [32]byte
			kind := byte('f')
			if n.subs[name] {
				kind = 'd'
				c := nodes[joinDirRel(rel, name)]
				child = c.exact
				n.bad = n.bad || c.bad
				n.hidden = n.hidden || c.hidden || c.nfiles == 0
				n.nfiles += c.nfiles
				n.size += c.size
				n.hashes = append(n.hashes, c.hashes...)
				c.hashes = nil
			} else {
				child = n.files[name]
			}
			var lenBuf [8]byte
			binary.BigEndian.PutUint64(lenBuf[:], uint64(len(name)))
			h.Write([]byte{kind})
			h.Write(lenBuf[:])
			h.Write([]byte(name))
			h.Write(child[:])
		}
		copy(n.exact[:], h.Sum(nil))
		// ponytail: per-dir sort of subtree hashes is O(files×depth×log); switch to an additive multiset hash if deep trees bite.
		slices.SortFunc(n.hashes, func(a, b [32]byte) int { return slices.Compare(a[:], b[:]) })
		ch := sha256.New()
		for _, x := range n.hashes {
			ch.Write(x[:])
		}
		copy(n.content[:], ch.Sum(nil))
	}

	exactBy := map[[32]byte][]string{}
	contentBy := map[[32]byte][]string{}
	for rel, n := range nodes {
		if rel == "" || n.bad || n.nfiles == 0 {
			continue
		}
		if len(only) > 0 {
			if inside, _ := walkScope(rel, only); !inside {
				continue
			}
		}
		exactBy[n.exact] = append(exactBy[n.exact], rel)
		contentBy[n.content] = append(contentBy[n.content], rel)
	}
	exact := dirGroupsOf(DirExact, exactBy, nodes)
	content := dirGroupsOf(DirContent, contentBy, nodes)

	// Exact implies content: drop a content group whose member set an exact group already covers.
	exactSets := map[string]bool{}
	for _, g := range exact {
		exactSets[strings.Join(g.Rels, "\x00")] = true
	}
	content = slices.DeleteFunc(content, func(g DedupDirGroup) bool { return exactSets[strings.Join(g.Rels, "\x00")] })

	out := append(maximalDirGroups(exact), maximalDirGroups(content)...)
	slices.SortFunc(out, func(a, b DedupDirGroup) int {
		return cmp.Or(cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.Rels[0], b.Rels[0]))
	})
	return out
}

func dirDepth(rel string) int {
	if rel == "" {
		return 0
	}
	return strings.Count(rel, "/") + 1
}

func joinDirRel(dir, name string) string {
	if dir == "" {
		return name
	}
	return dir + "/" + name
}

// dirGroupsOf turns digest buckets into groups. A member nested inside another
// member of the same group (a dir holding only one subdir matches that subdir's
// content) is dropped so members never overlap.
func dirGroupsOf(kind DedupDirKind, by map[[32]byte][]string, nodes map[string]*dedupDirNode) []DedupDirGroup {
	var groups []DedupDirGroup
	for digest, rels := range by {
		members := map[string]bool{}
		for _, r := range rels {
			members[r] = true
		}
		kept := rels[:0:0]
		for _, r := range rels {
			nested := false
			for p := RelDir(r); p != "" && !nested; p = RelDir(p) {
				nested = members[p]
			}
			if !nested {
				kept = append(kept, r)
			}
		}
		if len(kept) < 2 {
			continue
		}
		slices.Sort(kept)
		g := DedupDirGroup{Kind: kind, Digest: digest, Size: nodes[kept[0]].size, Files: nodes[kept[0]].nfiles, Rels: kept}
		for _, r := range kept {
			g.Hidden = append(g.Hidden, nodes[r].hidden)
		}
		groups = append(groups, g)
	}
	return groups
}

// maximalDirGroups drops a group when every member's parent sits in one other
// group of the same tier (that parent group already implies this one).
func maximalDirGroups(groups []DedupDirGroup) []DedupDirGroup {
	idx := map[string]int{}
	for gi, g := range groups {
		for _, r := range g.Rels {
			idx[r] = gi
		}
	}
	return slices.DeleteFunc(groups, func(g DedupDirGroup) bool {
		parent := -1
		for i, r := range g.Rels {
			pi, ok := idx[RelDir(r)]
			if !ok || (i > 0 && pi != parent) {
				return false
			}
			parent = pi
		}
		return true
	})
}
