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

func TestParseSplitTokenFitModeValid(t *testing.T) {
	t.Parallel()
	spec, err := parseSplitToken("<16", 0)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Kind != SplitFitChars || spec.Value != 16 {
		t.Fatalf("parseSplitToken(^16, 0) = %+v, want Kind=SplitFitChars Value=16", spec)
	}

	spec, err = parseSplitToken("<33%", 1)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Kind != SplitFitPercent || spec.Value != 33 {
		t.Fatalf("parseSplitToken(^33%%, 1) = %+v, want Kind=SplitFitPercent Value=33", spec)
	}
}

func TestParseSplitTokenFitModeRejectedAtChildColumn(t *testing.T) {
	t.Parallel()
	if _, err := parseSplitToken("<16", 2); err == nil {
		t.Fatal("expected error for ^16 at index 2 (child column)")
	}
	if _, err := parseSplitToken("<33%", 2); err == nil {
		t.Fatal("expected error for ^33% at index 2 (child column)")
	}
}

func TestParseSplitTokenFitModeMalformed(t *testing.T) {
	t.Parallel()
	for _, tok := range []string{"<", "<abc", "<0", "<-1", "<150%", "<%"} {
		if _, err := parseSplitToken(tok, 0); err == nil {
			t.Fatalf("expected error for malformed fit token %q", tok)
		}
	}
}

func TestParseLayoutRejectsFitModeOnChildColumn(t *testing.T) {
	t.Parallel()
	if _, err := ParseLayout([]string{"*", "*", "<16"}, nil); err == nil {
		t.Fatal("expected ParseLayout error for ^16 at split[2]")
	}
}

func TestResolveMeasuredFitCharsUnderAndOverCap(t *testing.T) {
	t.Parallel()
	layout, err := ParseLayout([]string{"<16", "*", "*"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Measured content narrower than the 16-cell cap: column shrinks to the measured width.
	got := layout.ResolveMeasured(90, true, [3]int{8, 0, 0})
	if got[0] != 8 {
		t.Fatalf("col[0] = %d, want 8 (measured under cap)", got[0])
	}
	// Measured content wider than the cap: column clamps to the 16-cell cap.
	got = layout.ResolveMeasured(90, true, [3]int{40, 0, 0})
	if got[0] != 16 {
		t.Fatalf("col[0] = %d, want 16 (cap when measured exceeds it)", got[0])
	}
	if sum := got[0] + got[1] + got[2]; sum != 90 {
		t.Fatalf("widths sum %d, want 90", sum)
	}
}

func TestResolveMeasuredFitPercentUnderAndOverCap(t *testing.T) {
	t.Parallel()
	layout, err := ParseLayout([]string{"<33%", "*", "*"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	const innerW = 90
	cap33 := (innerW*33 + 50) / 100 // 30
	got := layout.ResolveMeasured(innerW, true, [3]int{10, 0, 0})
	if got[0] != 10 {
		t.Fatalf("col[0] = %d, want 10 (measured under cap)", got[0])
	}
	got = layout.ResolveMeasured(innerW, true, [3]int{60, 0, 0})
	if got[0] != cap33 {
		t.Fatalf("col[0] = %d, want %d (cap when measured exceeds it)", got[0], cap33)
	}
	if sum := got[0] + got[1] + got[2]; sum != innerW {
		t.Fatalf("widths sum %d, want %d", sum, innerW)
	}
}

func TestResolveUnmeasuredFitFallsBackToCap(t *testing.T) {
	t.Parallel()
	layout, err := ParseLayout([]string{"<16", "<33%", "*"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	const innerW = 90
	got := layout.Resolve(innerW, true)
	if got[0] != 16 {
		t.Fatalf("col[0] = %d, want 16 (unmeasured falls back to cap)", got[0])
	}
	want1 := (innerW*33 + 50) / 100
	if got[1] != want1 {
		t.Fatalf("col[1] = %d, want %d (unmeasured falls back to cap)", got[1], want1)
	}
	if sum := got[0] + got[1] + got[2]; sum != innerW {
		t.Fatalf("widths sum %d, want %d", sum, innerW)
	}
}

func TestResolveMeasuredFitMixedSumInvariant(t *testing.T) {
	t.Parallel()
	layout, err := ParseLayout([]string{"<16", "20%", "*"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, innerW := range []int{20, 40, 72, 90, 91, 100, 137} {
		for _, measured := range [][3]int{{0, 0, 0}, {5, 0, 0}, {12, 0, 0}, {40, 0, 0}} {
			got := layout.ResolveMeasured(innerW, true, measured)
			if sum := got[0] + got[1] + got[2]; sum != innerW {
				t.Fatalf("innerW=%d measured=%v sum=%d, want %d (got=%v)", innerW, measured, sum, innerW, got)
			}
		}
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
