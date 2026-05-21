package usermenu

import "testing"

func TestEntryIndexForKey(t *testing.T) {
	entries := []MenuEntry{
		{Key: "g", Title: "lazygit"},
		{Key: "P", Title: "pwd"},
	}
	if i, ok := EntryIndexForKey(entries, 'g'); !ok || i != 0 {
		t.Fatalf("g: got %d %v", i, ok)
	}
	if i, ok := EntryIndexForKey(entries, 'G'); !ok || i != 0 {
		t.Fatalf("G: got %d %v", i, ok)
	}
	if i, ok := EntryIndexForKey(entries, 'p'); !ok || i != 1 {
		t.Fatalf("p: got %d %v", i, ok)
	}
	if _, ok := EntryIndexForKey(entries, 'x'); ok {
		t.Fatal("x should not match")
	}
}
