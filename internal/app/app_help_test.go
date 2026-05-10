package app

import (
	"reflect"
	"testing"
)

func TestKeysForHelpDisplay_preferredFirst(t *testing.T) {
	t.Parallel()
	got := keysForHelpDisplay([]string{"C-e", "C-g"}, "C-g")
	want := []string{"C-g", "C-e"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestKeysForHelpDisplay_noPreferred(t *testing.T) {
	t.Parallel()
	in := []string{"C-e", "C-g"}
	got := keysForHelpDisplay(in, "")
	want := []string{"C-e", "C-g"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestKeysForHelpDisplay_preferredAbsent(t *testing.T) {
	t.Parallel()
	got := keysForHelpDisplay([]string{"F5", "F6"}, "C-g")
	want := []string{"F5", "F6"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestJoinKeyDisplay_listsAllBindings(t *testing.T) {
	t.Parallel()
	got := joinKeyDisplay([]string{"C-e", "C-g"}, "C-g")
	want := "Ctrl+G, Ctrl+E"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestJoinKeyDisplay_singleBinding(t *testing.T) {
	t.Parallel()
	got := joinKeyDisplay([]string{"F5"}, "F5")
	if got != "F5" {
		t.Fatalf("got %q want F5", got)
	}
}
