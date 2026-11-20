package treeflat

import (
	"fmt"
	"math/rand"
	"slices"
	"testing"
)

// benchLabels is n labels shaped like the dedup strip's: six levels of a few dozen words.
func benchLabels(n int) (labels, ids []string) {
	words := []string{"amber", "birch", "cedar", "dune", "ember", "fjord", "grove", "harbor", "iris", "juniper", "kettle", "lantern"}
	for g := range n {
		l := fmt.Sprintf("%s/%s/%s/%s/%s/%s-%d.bin", words[g%12], words[(g/12)%12], words[(g/144)%12], words[(g/1728)%12], words[(g/7)%12], words[g%11], g)
		labels, ids = append(labels, l), append(ids, "/scan/root/"+l)
	}
	return labels, ids
}

func BenchmarkFromPathsSorted(b *testing.B) {
	labels, ids := benchLabels(27000)
	slices.SortFunc(labels, componentCmp)
	for i, l := range labels {
		ids[i] = "/scan/root/" + l
	}
	b.ReportAllocs()
	for b.Loop() {
		FromPaths(labels, ids)
	}
}

func BenchmarkFromPathsShuffled(b *testing.B) {
	labels, ids := benchLabels(27000)
	rand.New(rand.NewSource(1)).Shuffle(len(labels), func(i, j int) {
		labels[i], labels[j] = labels[j], labels[i]
		ids[i], ids[j] = ids[j], ids[i]
	})
	b.ReportAllocs()
	for b.Loop() {
		FromPaths(labels, ids)
	}
}

// BenchmarkPathTreeEdit is one Add and one Remove followed by Nodes on a built 27k-label tree.
func BenchmarkPathTreeEdit(b *testing.B) {
	labels, ids := benchLabels(27000)
	tree := NewPathTree(labels, ids)
	tree.Nodes()
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		j := i % len(labels)
		tree.Remove(labels[j])
		tree.Nodes()
		tree.Add(labels[j], ids[j])
		tree.Nodes()
	}
}
