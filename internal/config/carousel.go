package config

import (
	"strconv"
	"strings"
)

// DefaultCarouselSplit returns the built-in carousel column width tokens.
func DefaultCarouselSplit() []string {
	return []string{
		DefaultCarouselSplit0,
		DefaultCarouselSplit1,
		DefaultCarouselSplit2,
	}
}

// DefaultCarouselShowSize returns the built-in per-column size visibility flags.
func DefaultCarouselShowSize() []bool {
	return []bool{true, true, true}
}

// carouselSplitTokenValid reports whether tok is a valid carousel_split entry.
func carouselSplitTokenValid(tok string) bool {
	tok = strings.TrimSpace(tok)
	if tok == "*" {
		return true
	}
	if strings.HasSuffix(tok, "%") {
		pctStr := strings.TrimSpace(strings.TrimSuffix(tok, "%"))
		if pctStr == "" {
			return false
		}
		pct, err := strconv.Atoi(pctStr)
		return err == nil && pct >= 0 && pct <= 100
	}
	n, err := strconv.Atoi(tok)
	return err == nil && n >= 1
}

func carouselSplitValid(split []string) bool {
	if len(split) != 3 {
		return false
	}
	for _, tok := range split {
		if !carouselSplitTokenValid(tok) {
			return false
		}
	}
	return true
}

func carouselShowSizeValid(showSize []bool) bool {
	return len(showSize) == 0 || len(showSize) == 3
}

func normalizeCarouselUI(ui *UIConfig) {
	if !carouselSplitValid(ui.CarouselSplit) {
		ui.CarouselSplit = DefaultCarouselSplit()
	}
	if !carouselShowSizeValid(ui.CarouselShowSize) {
		ui.CarouselShowSize = DefaultCarouselShowSize()
	}
	if len(ui.CarouselShowSize) == 0 {
		ui.CarouselShowSize = DefaultCarouselShowSize()
	}
}
