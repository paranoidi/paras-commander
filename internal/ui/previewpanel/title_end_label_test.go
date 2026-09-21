package previewpanel

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/tcelltest"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestTitleEndLabelLayout(t *testing.T) {
	const titleX, gap, margin = 2, 2, 1
	tests := []struct {
		name         string
		innerRight   int
		endRunes     int
		wantShow     bool
		wantPathSlot int
	}{
		{
			name:         "absent",
			innerRight:   40,
			endRunes:     0,
			wantShow:     false,
			wantPathSlot: 39,
		},
		{
			name:         "narrow",
			innerRight:   21,
			endRunes:     16,
			wantShow:     false,
			wantPathSlot: 20,
		},
		{
			name:         "exact-fit",
			innerRight:   21,
			endRunes:     14,
			wantShow:     true,
			wantPathSlot: 3,
		},
		{
			name:         "clipped",
			innerRight:   40,
			endRunes:     10,
			wantShow:     true,
			wantPathSlot: 26,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contentCols := tt.innerRight - titleX + 1
			got := titleEndLabelLayout(titleX, tt.innerRight, contentCols, tt.endRunes, margin, gap)
			if got.ShowEnd != tt.wantShow {
				t.Fatalf("ShowEnd = %v, want %v", got.ShowEnd, tt.wantShow)
			}
			if got.PathSlotCols != tt.wantPathSlot {
				t.Fatalf("PathSlotCols = %d, want %d", got.PathSlotCols, tt.wantPathSlot)
			}
			if tt.wantShow {
				wantStart := tt.innerRight - tt.endRunes + 1 - margin
				if got.EndStartX != wantStart {
					t.Fatalf("EndStartX = %d, want %d", got.EndStartX, wantStart)
				}
			}
		})
	}
}

func TestPaintQuickViewTitleRowEndLabelCases(t *testing.T) {
	styles := theme.Default()
	chrome := styles.PanelChrome(false, false)
	home := "/home/user"
	path := "/home/user/projects/harborlantern"

	tests := []struct {
		name      string
		width     int
		endLabel  string
		wantEnd   bool
		wantTitle string
	}{
		{
			name:      "absent",
			width:     40,
			endLabel:  "",
			wantEnd:   false,
			wantTitle: "projects",
		},
		{
			name:     "narrow",
			width:    16,
			endLabel: " README.md ",
			wantEnd:  false,
		},
		{
			name:     "exact-fit",
			width:    28,
			endLabel: " README.md ",
			wantEnd:  true,
		},
		{
			name:     "clipped",
			width:    40,
			endLabel: " README.md ",
			wantEnd:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			screen := tcell.NewSimulationScreen("UTF-8")
			if err := screen.Init(); err != nil {
				t.Fatalf("Init() error = %v", err)
			}
			defer screen.Fini()
			screen.SetSize(tt.width, 4)
			titleX, innerRight, contentCols, y := quickViewTitleRowGeom(tt.width)
			paintQuickViewTitleRow(screen, titleX, innerRight, contentCols, y,
				path, home, chrome.Title, tt.endLabel, chrome.Title, chrome.Frame)
			got := tcelltest.TextAt(screen, titleX, y, contentCols)
			if tt.wantEnd {
				if !strings.Contains(got, strings.TrimSpace(tt.endLabel)) {
					t.Fatalf("row = %q, want end label %q", got, tt.endLabel)
				}
			} else if tt.endLabel != "" && strings.Contains(got, strings.TrimSpace(tt.endLabel)) {
				t.Fatalf("row = %q, want end label hidden", got)
			}
			if tt.wantTitle != "" && !strings.Contains(got, tt.wantTitle) {
				t.Fatalf("row = %q, want title %q", got, tt.wantTitle)
			}
		})
	}
}
