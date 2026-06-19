package panelcarousel

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/ui/geom"
)

func TestParseLayoutDefaults(t *testing.T) {
	t.Parallel()
	got, err := ParseLayout([]string{"*", "*", "*"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := DefaultLayout()
	if got != want {
		t.Fatalf("ParseLayout default = %+v, want %+v", got, want)
	}
}

func TestResolveSplitExamples(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		split     []string
		innerW    int
		showChild bool
		want      [3]int
	}{
		{
			name:   "percent_flex",
			split:  []string{"20%", "30%", "*"},
			innerW: 90, showChild: true,
			want: [3]int{18, 27, 45},
		},
		{
			name:   "equal_thirds",
			split:  []string{"33%", "33%", "*"},
			innerW: 90, showChild: true,
			want: [3]int{30, 30, 30},
		},
		{
			name:   "mixed_fixed_percent_flex",
			split:  []string{"16", "20%", "*"},
			innerW: 90, showChild: true,
			want: [3]int{16, 15, 59},
		},
		{
			name:   "two_column_folds_child_into_center",
			split:  []string{"20%", "30%", "*"},
			innerW: 90, showChild: false,
			want: [3]int{18, 72, 0},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			layout, err := ParseLayout(tc.split, []bool{true, true, true})
			if err != nil {
				t.Fatal(err)
			}
			got := layout.Resolve(tc.innerW, tc.showChild)
			if got != tc.want {
				t.Fatalf("Resolve(%d, %v) = %v, want %v", tc.innerW, tc.showChild, got, tc.want)
			}
			sum := got[0] + got[1] + got[2]
			if sum != tc.innerW {
				t.Fatalf("widths sum %d, want %d", sum, tc.innerW)
			}
		})
	}
}

func TestResolveEqualFlexThirds(t *testing.T) {
	t.Parallel()
	layout := DefaultLayout()
	for _, innerW := range []int{72, 90, 91, 100} {
		got := layout.Resolve(innerW, true)
		if got[0]+got[1]+got[2] != innerW {
			t.Fatalf("innerW=%d sum=%d", innerW, got[0]+got[1]+got[2])
		}
		// Equal flex should be within 1 cell per column.
		for i := 0; i < 3; i++ {
			avg := innerW / 3
			if got[i] < avg-1 || got[i] > avg+1 {
				t.Fatalf("innerW=%d col[%d]=%d, want ~%d", innerW, i, got[i], avg)
			}
		}
	}
}

func TestParseLayoutInvalid(t *testing.T) {
	t.Parallel()
	if _, err := ParseLayout([]string{"20%"}, nil); err == nil {
		t.Fatal("expected error for short split")
	}
	if _, err := ParseLayout([]string{"*", "*", "bad"}, nil); err == nil {
		t.Fatal("expected error for invalid token")
	}
	if _, err := ParseLayout([]string{"*", "*", "*"}, []bool{true}); err == nil {
		t.Fatal("expected error for wrong show_size length")
	}
}

func TestMinInnerWidthPercentSplitFitsAtClassicCarouselWidth(t *testing.T) {
	t.Parallel()
	layout, err := ParseLayout([]string{"20%", "30%", "*"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	const classicInner = 72
	if layout.MinInnerWidth(true) > classicInner {
		t.Fatalf("MinInnerWidth(true) = %d, want <= %d", layout.MinInnerWidth(true), classicInner)
	}
	rect := geom.Rect{X: 0, Y: 0, Width: classicInner + 2, Height: 20}
	if !LayoutFits(rect, layout, true) {
		t.Fatal("20/30/* split should fit at classic carousel inner width 72")
	}
}
