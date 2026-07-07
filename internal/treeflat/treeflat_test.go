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
	if r := byID["a"]; r.Depth != 0 || !r.HasChildren || !r.Expanded {
		t.Fatalf("row a = %+v, want depth 0, expandable, expanded", r)
	}
	if r := byID["a/y/z"]; r.Depth != 2 || r.HasChildren || r.Expanded {
		t.Fatalf("row a/y/z = %+v, want depth 2 leaf", r)
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
