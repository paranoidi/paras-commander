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
	return []bool{false, true, true}
}

// carouselSplitTokenValid reports whether tok is a valid carousel.split entry at column idx.
// This grammar mirrors internal/panelcarousel/split.go's parseSplitToken (config can't import
// panelcarousel — the duplication already exists today for the base grammar, extended here for
// fit-mode tokens too).
func carouselSplitTokenValid(tok string, idx int) bool {
	tok = strings.TrimSpace(tok)
	if tok == "*" {
		return true
	}
	if strings.HasPrefix(tok, "<") {
		if idx == 2 {
			return false
		}
		return carouselSplitCapValid(strings.TrimPrefix(tok, "<"))
	}
	return carouselSplitCapValid(tok)
}

// carouselSplitCapValid validates the shared "N" / "N%" grammar used by fixed/percent tokens
// and by fit-mode tokens after stripping their "<" prefix.
func carouselSplitCapValid(tok string) bool {
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
	for i, tok := range split {
		if !carouselSplitTokenValid(tok, i) {
			return false
		}
	}
	return true
}

func carouselShowSizeValid(showSize []bool) bool {
	return len(showSize) == 0 || len(showSize) == 3
}

func normalizeCarousel(c *CarouselConfig) {
	if !carouselSplitValid(c.Split) {
		c.Split = DefaultCarouselSplit()
	}
	if !carouselShowSizeValid(c.ShowSize) {
		c.ShowSize = DefaultCarouselShowSize()
	}
	if len(c.ShowSize) == 0 {
		c.ShowSize = DefaultCarouselShowSize()
	}
}
