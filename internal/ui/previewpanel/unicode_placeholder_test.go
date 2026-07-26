package previewpanel

import "testing"

// TestUnicodePlaceholderCellMatchesKittyDocExample cross-checks against the worked example
// in kitty's own graphics-protocol.rst for image id 33554474 (0x02000042): row/col diacritics
// 0/1 and a third (image-id-high-byte) diacritic of index 2 (0x30e), since 33554474>>24 == 2.
func TestUnicodePlaceholderCellMatchesKittyDocExample(t *testing.T) {
	const imageID = 33554474
	mainc, combc := UnicodePlaceholderCell(0, 1, imageID)
	if mainc != unicodePlaceholderChar {
		t.Fatalf("mainc = %U, want %U", mainc, unicodePlaceholderChar)
	}
	want := []rune{0x305, 0x30d, 0x30e}
	if len(combc) != len(want) {
		t.Fatalf("combc = %v, want %v", combc, want)
	}
	for i := range want {
		if combc[i] != want[i] {
			t.Errorf("combc[%d] = %U, want %U", i, combc[i], want[i])
		}
	}
}

func TestUnicodePlaceholderCellRowColOrderAndRange(t *testing.T) {
	_, combc := UnicodePlaceholderCell(2, 5, 1)
	if combc[0] != numberToDiacritic[2] {
		t.Errorf("row diacritic = %U, want %U (index 2)", combc[0], numberToDiacritic[2])
	}
	if combc[1] != numberToDiacritic[5] {
		t.Errorf("col diacritic = %U, want %U (index 5)", combc[1], numberToDiacritic[5])
	}
	// id=1 has a zero high byte, so the third diacritic is index 0.
	if combc[2] != numberToDiacritic[0] {
		t.Errorf("id-high diacritic = %U, want %U (index 0)", combc[2], numberToDiacritic[0])
	}
}

func TestUnicodePlaceholderColorSplitsLower24Bits(t *testing.T) {
	// 33554474 = 2*2^24 + 42: same id as the kitty doc's worked example, where the low byte
	// (42) is shown as the color and the high byte (2) as the third diacritic (see
	// TestUnicodePlaceholderCellMatchesKittyDocExample).
	r, g, b := UnicodePlaceholderColor(33554474)
	if r != 0 || g != 0 || b != 42 {
		t.Fatalf("UnicodePlaceholderColor(33554474) = (%d,%d,%d), want (0,0,42)", r, g, b)
	}
	r, g, b = UnicodePlaceholderColor(0x0203ff)
	if r != 0x02 || g != 0x03 || b != 0xff {
		t.Fatalf("UnicodePlaceholderColor(0x0203ff) = (%#x,%#x,%#x), want (0x02,0x03,0xff)", r, g, b)
	}
}

func TestClampPlaceholderDim(t *testing.T) {
	cases := []struct{ n, max, want int }{
		{n: 0, max: 10, want: 1},
		{n: -5, max: 10, want: 1},
		{n: 5, max: 10, want: 5},
		{n: 20, max: 10, want: 10},
		{n: 1000, max: 10000, want: MaxUnicodePlaceholderGridSize},
	}
	for _, c := range cases {
		if got := clampPlaceholderDim(c.n, c.max); got != c.want {
			t.Errorf("clampPlaceholderDim(%d, %d) = %d, want %d", c.n, c.max, got, c.want)
		}
	}
}
