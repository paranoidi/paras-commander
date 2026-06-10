package panel

import "testing"

func TestMergeNavigationHistoriesPassiveFirstDedupes(t *testing.T) {
	passive := []string{"/tmp/beta", "/tmp/alpha"}
	active := []string{"/tmp/gamma", "/tmp/beta"}
	got := MergeNavigationHistories(passive, active)
	want := []string{"/tmp/beta", "/tmp/alpha", "/tmp/gamma"}
	if len(got) != len(want) {
		t.Fatalf("len = %d want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("[%d] = %q want %q (full %v)", i, got[i], want[i], got)
		}
	}
}

func TestMergeNavigationHistoriesSkipsEmptyPaths(t *testing.T) {
	got := MergeNavigationHistories([]string{"", "."}, []string{"/tmp/ok"})
	if len(got) != 1 || got[0] != "/tmp/ok" {
		t.Fatalf("got %v want [/tmp/ok]", got)
	}
}
