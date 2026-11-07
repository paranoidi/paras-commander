package geom

import "testing"

func TestInactivePaneWidth(t *testing.T) {
	zoom := PanelPaneSplit{Zoom: true, ActivePercent: 70, InactivePercent: 30}
	tests := []struct {
		name  string
		total int
		split func() PanelPaneSplit
		want  int
	}{
		{"zoomed primary active", 100, func() PanelPaneSplit { return zoom }, 30},
		{"zoomed secondary active", 100, func() PanelPaneSplit { s := zoom; s.ActivePanel = 1; return s }, 30},
		{"even odd width", 101, func() PanelPaneSplit { return PanelPaneSplit{} }, 51},
		{"hidden still measured", 100, func() PanelPaneSplit { s := zoom; s.HideInactivePanel = true; return s }, 30},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := InactivePaneWidth(tt.total, tt.split()); got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}
