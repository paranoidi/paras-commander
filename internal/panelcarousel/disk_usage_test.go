package panelcarousel

import "testing"

func TestDiskUsageFillColumns(t *testing.T) {
	if got, sl := diskUsageFillColumns(500, 1000, 10); got != 5 || sl {
		t.Fatalf("half ratio: got %d,%v want 5,false", got, sl)
	}
	if got, sl := diskUsageFillColumns(1000, 1000, 10); got != 10 || sl {
		t.Fatalf("full ratio: got %d,%v want 10,false", got, sl)
	}
	if got, sl := diskUsageFillColumns(0, 1000, 10); got != 0 || sl {
		t.Fatal("zero usage")
	}
	if got, sl := diskUsageFillColumns(1, 10000, 100); got != 0 || !sl {
		t.Fatalf("sub-cell ratio: got %d,%v want 0,true", got, sl)
	}
}
