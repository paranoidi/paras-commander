package helpkeys

import (
	"reflect"
	"testing"

	"github.com/paranoidi/paras-commander/internal/ui"
)

func TestForDisplay_preferredFirst(t *testing.T) {
	t.Parallel()
	got := ForDisplay([]string{"C-e", "C-g"}, "C-g")
	want := []string{"C-g", "C-e"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestForDisplay_noPreferred(t *testing.T) {
	t.Parallel()
	in := []string{"C-e", "C-g"}
	got := ForDisplay(in, "")
	want := []string{"C-e", "C-g"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestForDisplay_preferredAbsent(t *testing.T) {
	t.Parallel()
	got := ForDisplay([]string{"F5", "F6"}, "C-g")
	want := []string{"F5", "F6"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestJoinDisplay_listsAllBindings(t *testing.T) {
	t.Parallel()
	got := JoinDisplay([]string{"C-e", "C-g"}, "C-g")
	want := "Ctrl+G, Ctrl+E"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestJoinDisplay_singleBinding(t *testing.T) {
	t.Parallel()
	got := JoinDisplay([]string{"F5"}, "F5")
	if got != "F5" {
		t.Fatalf("got %q want F5", got)
	}
}

func TestCanonicalRankOrdersKeysSectionTitle(t *testing.T) {
	ent := ui.HelpEntry{Keys: "Alt+O", Section: "Navigation", Title: "Open", FuzzyExtra: "panel.open-dir-in-other search"}
	got := CanonicalRankText(ent)
	want := "Alt+O Navigation Open panel.open-dir-in-other search"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
