package panel

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/paranoidi/paras-commander/internal/testutil"
)

// fillValue sets every map, slice, pointer and scalar reachable from v to a non-zero value, so a
// field Duplicate forgets to copy deeply is still a shared pointer in the copy. Unexported fields
// are reached through unsafe. Funcs, channels, interfaces and time.Time are left alone: closures
// are shared on purpose, and a time.Time's location pointer is immutable.
func fillValue(v reflect.Value, depth int) {
	if !v.CanSet() {
		if !v.CanAddr() {
			return
		}
		v = reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem()
	}
	if depth > 5 {
		return
	}
	switch v.Kind() {
	case reflect.Struct:
		if v.Type() == reflect.TypeOf(time.Time{}) {
			return
		}
		for i := range v.NumField() {
			fillValue(v.Field(i), depth+1)
		}
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
		fillValue(v.Elem(), depth+1)
	case reflect.Slice:
		v.Set(reflect.MakeSlice(v.Type(), 1, 1))
		fillValue(v.Index(0), depth+1)
	case reflect.Map:
		v.Set(reflect.MakeMap(v.Type()))
		key, val := reflect.New(v.Type().Key()).Elem(), reflect.New(v.Type().Elem()).Elem()
		fillValue(key, depth+1)
		fillValue(val, depth+1)
		v.SetMapIndex(key, val)
	case reflect.String:
		v.SetString("x")
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(1)
	}
}

// sharedReferences collects every map, slice and pointer address reachable from v.
func sharedReferences(v reflect.Value, skip map[string]bool, name string, into map[uintptr]string) {
	if !v.IsValid() || skip[name] {
		return
	}
	switch v.Kind() {
	case reflect.Struct:
		if v.Type() == reflect.TypeOf(time.Time{}) {
			return
		}
		for i := range v.NumField() {
			sharedReferences(v.Field(i), skip, name+"."+v.Type().Field(i).Name, into)
		}
	case reflect.Pointer:
		if v.IsNil() {
			return
		}
		into[v.Pointer()] = name
		sharedReferences(v.Elem(), skip, name, into)
	case reflect.Slice:
		if v.Len() == 0 {
			return
		}
		into[v.Pointer()] = name
		for i := range v.Len() {
			sharedReferences(v.Index(i), skip, name+"[]", into)
		}
	case reflect.Map:
		if v.Len() == 0 {
			return
		}
		into[v.Pointer()] = name
		iter := v.MapRange()
		for iter.Next() {
			sharedReferences(iter.Value(), skip, name+"{}", into)
		}
	}
}

func TestDuplicateSharesNoMemoryWithSource(t *testing.T) {
	var src State
	fillValue(reflect.ValueOf(&src).Elem(), 0)
	// Duplicate rebuilds derived state from the listing, which must be readable.
	src.ListLayout = ListLayoutFlat
	src.ActiveEntryFilter = nil
	dup := src.Duplicate()

	allowed := map[string]bool{"State.Gitignore": true}
	srcRefs, dupRefs := map[uintptr]string{}, map[uintptr]string{}
	sharedReferences(reflect.ValueOf(src), allowed, "State", srcRefs)
	sharedReferences(reflect.ValueOf(dup), allowed, "State", dupRefs)
	for addr, name := range dupRefs {
		if from, ok := srcRefs[addr]; ok {
			t.Errorf("Duplicate shares %s with the source (source field %s)", name, from)
		}
	}
	if dup.Gitignore != src.Gitignore {
		t.Error("Duplicate must share the gitignore cache")
	}
}

func TestDuplicateTreeSharesNoNodes(t *testing.T) {
	s, orchard := orchardTree(t)
	h := newTreeLoadHarness(t, s)
	if err := s.ExpandTreeCursorRow(10); err != nil {
		t.Fatal(err)
	}
	h.settle()
	dup := s.Duplicate()

	srcRefs, dupRefs := map[uintptr]string{}, map[uintptr]string{}
	sharedReferences(reflect.ValueOf(*s), map[string]bool{"State.Gitignore": true}, "State", srcRefs)
	sharedReferences(reflect.ValueOf(dup), map[string]bool{"State.Gitignore": true}, "State", dupRefs)
	for addr, name := range dupRefs {
		if from, ok := srcRefs[addr]; ok {
			t.Errorf("Duplicate shares %s with the source (source field %s)", name, from)
		}
	}
	if dup.VisibleEntryCount() != s.VisibleEntryCount() {
		t.Fatalf("copy shows %d rows, source %d", dup.VisibleEntryCount(), s.VisibleEntryCount())
	}
	// Collapsing the source's folder in place must not touch the copy's tree.
	if n := findTreeNode(s.TreeRoots, orchard); n == nil || len(n.Children) == 0 {
		t.Fatal("orchard children must be loaded")
	}
	s.TreeRoots[0].Children = nil
	if n := findTreeNode(dup.TreeRoots, orchard); n == nil || len(n.Children) == 0 {
		t.Fatal("copy lost its loaded children when the source's node changed")
	}
}

func TestDuplicateClearsLoadingNodes(t *testing.T) {
	s, orchard := orchardTree(t)
	newTreeLoadHarness(t, s)
	if err := s.ExpandTreeCursorRow(10); err != nil {
		t.Fatal(err)
	}
	if n := findTreeNode(s.TreeRoots, orchard); n == nil || !n.Value.Loading {
		t.Fatal("precondition: orchard must be loading")
	}
	dup := s.Duplicate()
	if n := findTreeNode(dup.TreeRoots, orchard); n == nil || n.Value.Loading {
		t.Fatal("a copy must not keep a node loading for the source's fetch")
	}
}

func TestDuplicateDiverges(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"anchor.txt", "breeze.txt", "cinder.txt"} {
		testutil.WriteFile(t, filepath.Join(dir, name))
	}
	src, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	src.Cursor = 1
	src.Sort.Reverse = true
	src.ApplySort()
	src.TogglePathSelection(src.Entries[0].Path)
	src.SelectionStashPaths = []string{"stashed"}
	src.RangeAnchor = "anchor"
	src.OpenFilter(10)
	src.AppendFilterRune('b', 10)
	src.ListingPending = true
	src.ListingPendingPath = "/elsewhere"

	dup := src.Duplicate()
	if dup.PathString() != src.PathString() || dup.Cursor != src.Cursor || !dup.Sort.Reverse {
		t.Fatalf("copy lost path/cursor/sort: %v %d %v", dup.PathString(), dup.Cursor, dup.Sort.Reverse)
	}
	if dup.SelectedPathCount() != 0 || dup.RangeAnchor != "" || !dup.StashEmpty() {
		t.Fatalf("copy starts with selection %d range %q stash %d", dup.SelectedPathCount(), dup.RangeAnchor, dup.StashPathCount())
	}
	if dup.Filter.Active || dup.Filter.Editing || dup.Filter.Query != "" {
		t.Fatalf("copy starts with quick filter %+v", dup.Filter)
	}
	if dup.ListingPending || dup.ListingPendingPath != "" {
		t.Fatal("copy must not wait for the source's load")
	}
	if src.SelectedPathCount() != 1 {
		t.Fatal("duplicating changed the source selection")
	}

	// Edits on one side do not show on the other.
	dup.Entries[0].Name = "changed"
	dup.History = append(dup.History, "/extra")
	dup.TogglePathSelection(dup.Entries[1].Path)
	if src.Entries[0].Name == "changed" || len(src.History) == len(dup.History) || src.SelectedPathCount() != 1 {
		t.Fatal("copy edits leaked into the source")
	}
	if dup.SelectedPathCount() != 1 {
		t.Fatalf("copy selection = %d, want 1", dup.SelectedPathCount())
	}
}

func TestHistoryVisitTokensAreUniqueAcrossStates(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	one, err := New(a)
	if err != nil {
		t.Fatal(err)
	}
	two := one.Duplicate()
	target := b
	tokenOne := one.recordVisit(target)
	tokenTwo := two.recordVisit(target)
	if tokenOne == 0 || tokenOne == tokenTwo {
		t.Fatalf("tokens %d and %d must be unique", tokenOne, tokenTwo)
	}
	// A superseded navigation of one tab cannot undo the other tab's visit.
	one.RevertHistoryVisit(target, tokenTwo)
	if len(one.History) == 0 || one.History[0] != target {
		t.Fatalf("a foreign token reverted the visit: %v", one.History)
	}
	two.RevertHistoryVisit(target, tokenTwo)
	if len(two.History) > 0 && two.History[0] == target {
		t.Fatalf("own token must revert the visit: %v", two.History)
	}
}

func TestDuplicateKeepsRowMarks(t *testing.T) {
	dir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(dir, "anchor.txt"))
	src, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	src.AddNewFileMarks(src.Path, []string{"anchor.txt"})
	dup := src.Duplicate()
	src.ClearNewFileMarks()
	if m := dup.NewFileMarksByDir[dir]; m == nil || len(m.latest) != 1 {
		t.Fatalf("copy lost its new-file marks: %+v", dup.NewFileMarksByDir)
	}
}

func TestBumpGenerationsPastBothStates(t *testing.T) {
	var a, b State
	a.ListingEpoch, a.selectionDerivedGen = 7, 2
	b.ListingEpoch, b.selectionDerivedGen = 3, 9
	a.BumpGenerationsPast(&b)
	if a.ListingEpoch <= 7 || a.SelectionDerivedGen() <= 9 {
		t.Fatalf("epoch %d, selection generation %d must exceed both states'", a.ListingEpoch, a.SelectionDerivedGen())
	}
}
