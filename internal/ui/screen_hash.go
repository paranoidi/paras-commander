package ui

import (
	"github.com/gdamore/tcell/v2"
)

// FNV-1a 64-bit (same constants as hash/fnv).
const (
	fnvOffset64 = 14695981039346656037
	fnvPrime64  = 1099511628211
)

// HashScreenLogical fingerprints the logical cell buffer (tcell's back buffer) before Show/Sync.
// It is used to avoid redundant terminal updates when nothing visible changed.
func HashScreenLogical(s tcell.Screen) uint64 {
	w, h := s.Size()
	if w <= 0 || h <= 0 {
		return 0
	}
	hsh := uint64(fnvOffset64)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			str, sty, width := s.Get(x, y)
			hsh ^= uint64(width)
			hsh *= fnvPrime64
			for i := 0; i < len(str); i++ {
				hsh ^= uint64(str[i])
				hsh *= fnvPrime64
			}
			fg, bg, attr := sty.Decompose()
			hsh ^= uint64(fg)
			hsh *= fnvPrime64
			hsh ^= uint64(bg)
			hsh *= fnvPrime64
			hsh ^= uint64(attr)
			hsh *= fnvPrime64
		}
	}
	return hsh
}
