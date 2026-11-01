package panel

import "testing"

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
