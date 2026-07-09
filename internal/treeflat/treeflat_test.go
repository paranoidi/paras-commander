package treeflat

import (
	"reflect"
	"testing"
)

func testTree() []Node[string] {
	return []Node[string]{
		{ID: "a", Value: "a", Children: []Node[string]{
			{ID: "a/x", Value: "x"},
			{ID: "a/y", Value: "y", Children: []Node[string]{
				{ID: "a/y/z", Value: "z"},
			}},
		}},
		{ID: "b", Value: "b"},
	}
}

func ids[T any](rows []Row[T]) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.ID
	}
	return out
}

func TestFlatten(t *testing.T) {
	tests := []struct {
		name      string
		collapsed map[string]bool
		want      []string
	}{
		{"all expanded", nil, []string{"a", "a/x", "a/y", "a/y/z", "b"}},
		{"inner collapsed", map[string]bool{"a/y": true}, []string{"a", "a/x", "a/y", "b"}},
		{"root collapsed hides subtree", map[string]bool{"a": true}, []string{"a", "b"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rows := Flatten(testTree(), func(id string) bool { return !tc.collapsed[id] })
			if got := ids(rows); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("rows = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFlattenRowFields(t *testing.T) {
	rows := Flatten(testTree(), nil)
	byID := map[string]Row[string]{}
	for _, r := range rows {
		byID[r.ID] = r
	}
	if r := byID["a"]; r.Depth != 0 || !r.HasChildren || !r.Expanded || r.LastChild {
		t.Fatalf("row a = %+v, want depth 0, expandable, expanded, not last root", r)
	}
	if r := byID["a/x"]; r.Depth != 1 || r.HasChildren || r.Expanded || r.LastChild {
		t.Fatalf("row a/x = %+v, want depth 1 first child, leaf, not last", r)
	}
	if got := byID["a/x"].AncestorHasNext; len(got) != 0 {
		t.Fatalf("row a/x AncestorHasNext = %v, want []", got)
	}
	if r := byID["a/y"]; r.Depth != 1 || !r.HasChildren || !r.Expanded || !r.LastChild || len(r.AncestorHasNext) != 0 {
		t.Fatalf("row a/y = %+v, want depth 1 last child, expandable, expanded, no ancestor continues", r)
	}
	if r := byID["a/y/z"]; r.Depth != 2 || r.HasChildren || r.Expanded || !r.LastChild {
		t.Fatalf("row a/y/z = %+v, want depth 2 leaf, last child", r)
	}
	if got := byID["a/y/z"].AncestorHasNext; len(got) != 1 || got[0] {
		t.Fatalf("row a/y/z AncestorHasNext = %v, want [false] (parent y is last sibling)", got)
	}
	if r := byID["b"]; r.Depth != 0 || r.HasChildren || r.Expanded || !r.LastChild {
		t.Fatalf("row b = %+v, want depth 0 leaf, last root", r)
	}

	rows = Flatten(testTree(), func(id string) bool { return id != "a/y" })
	for _, r := range rows {
		if r.ID == "a/y" && (r.Expanded || !r.HasChildren) {
			t.Fatalf("collapsed row a/y = %+v, want HasChildren && !Expanded", r)
		}
	}
}

func TestExpandableIDs(t *testing.T) {
	got := ExpandableIDs(testTree())
	want := []string{"a", "a/y"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ExpandableIDs = %v, want %v", got, want)
	}
}
