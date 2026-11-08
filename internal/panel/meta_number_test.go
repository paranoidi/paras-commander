package panel

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

func TestMetaNumber(t *testing.T) {
	for v, want := range map[string]float64{
		"42": 42, "-1.5": -1.5, "512 B": 512, "800K": 800 * 1024, "5.6G": 5.6 * (1 << 30),
		"1.5 GiB": 1.5 * (1 << 30), "3 MB": 3 << 20, "2T": 2 << 40,
	} {
		if got, ok := MetaNumber(v); !ok || got != want {
			t.Errorf("MetaNumber(%q) = %v, %v; want %v", v, got, ok, want)
		}
	}
	for _, v := range []string{"lantern", "2026-10-31 14:02", "5X", "GB", "Bike", "4 KQ"} {
		if got, ok := MetaNumber(v); ok {
			t.Errorf("MetaNumber(%q) = %v, want not a number", v, got)
		}
	}
}

// TestMetaNoValueSortsAsBlank: the no-value cell keeps a column numeric (so it stays high-first)
// and sorts last in either direction.
func TestMetaNoValueSortsAsBlank(t *testing.T) {
	values := map[string]string{"/a": "2", "/b": MetaNoValue, "/c": "10"}
	if !MetaValuesNumeric(values, "") {
		t.Fatal("no-value cell made the column text")
	}
	if MetaValuesNumeric(map[string]string{"/b": MetaNoValue}, "") {
		t.Fatal("column of only no-value cells reported numeric")
	}
	entries := []localfs.Entry{{Name: "a", Path: "/a"}, {Name: "b", Path: "/b"}, {Name: "c", Path: "/c"}}
	keys, numeric := buildMetaSortKeys(entries, "col", func(string) (map[string]string, string, bool) { return values, "", true })
	if !numeric || !keys["/b"].missing {
		t.Fatalf("keys = %+v, numeric = %v; want /b missing in a numeric column", keys, numeric)
	}
	for _, reverse := range []bool{false, true} {
		if compareMetaKeys(keys["/b"], keys["/a"], reverse) <= 0 {
			t.Fatalf("reverse=%v: no-value cell sorted before a number", reverse)
		}
	}
}
