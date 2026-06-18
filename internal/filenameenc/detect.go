package filenameenc

import (
	"bytes"
	"sort"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
)

// Candidate is one decoded UTF-8 filename produced from a legacy encoding guess.
type Candidate struct {
	Label string
	UTF8  string
}

const (
	minJapaneseScore = 2
	maxCandidates    = 3
)

type scoredCandidate struct {
	Candidate
	score int
}

// DetectCandidates returns up to three plausible UTF-8 decodings for legacy Japanese
// filenames (invalid UTF-8 byte sequences or UTF-8 mojibake).
func DetectCandidates(name string) []Candidate {
	if name == "" {
		return nil
	}
	var raw []scoredCandidate
	if !utf8.ValidString(name) {
		raw = append(raw, decodeLegacyBytes([]byte(name))...)
	} else if looksLikeMojibake(name) {
		raw = append(raw, decodeMojibake(name)...)
	}
	return dedupeAndCap(raw)
}

func decodeLegacyBytes(b []byte) []scoredCandidate {
	return tryDecoders(b, []decoderSpec{
		{label: "Shift-JIS", dec: japanese.ShiftJIS.NewDecoder()},
		{label: "EUC-JP", dec: japanese.EUCJP.NewDecoder()},
	})
}

func decodeMojibake(s string) []scoredCandidate {
	var out []scoredCandidate
	latin1, ok := latin1BytesFromString(s)
	if ok {
		out = append(out, tryDecoders(latin1, []decoderSpec{
			{label: "Shift-JIS (mojibake)", dec: japanese.ShiftJIS.NewDecoder()},
			{label: "EUC-JP (mojibake)", dec: japanese.EUCJP.NewDecoder()},
		})...)
	}
	if cp1252, err := charmap.Windows1252.NewEncoder().String(s); err == nil && cp1252 != s {
		out = append(out, tryDecoders([]byte(cp1252), []decoderSpec{
			{label: "Shift-JIS (CP1252 mojibake)", dec: japanese.ShiftJIS.NewDecoder()},
			{label: "EUC-JP (CP1252 mojibake)", dec: japanese.EUCJP.NewDecoder()},
		})...)
	}
	return out
}

type decoderSpec struct {
	label string
	dec   *encoding.Decoder
}

func tryDecoders(raw []byte, specs []decoderSpec) []scoredCandidate {
	var out []scoredCandidate
	for _, spec := range specs {
		decoded, err := spec.dec.Bytes(raw)
		if err != nil {
			continue
		}
		text := string(decoded)
		if !utf8.ValidString(text) {
			continue
		}
		score := japaneseScore(text)
		if score < minJapaneseScore {
			continue
		}
		out = append(out, scoredCandidate{
			Candidate: Candidate{Label: spec.label, UTF8: text},
			score:     score,
		})
	}
	return out
}

func dedupeAndCap(raw []scoredCandidate) []Candidate {
	if len(raw) == 0 {
		return nil
	}
	sort.Slice(raw, func(i, j int) bool {
		if raw[i].score != raw[j].score {
			return raw[i].score > raw[j].score
		}
		return raw[i].Label < raw[j].Label
	})
	seen := make(map[string]struct{})
	out := make([]Candidate, 0, maxCandidates)
	for _, c := range raw {
		if _, ok := seen[c.UTF8]; ok {
			continue
		}
		seen[c.UTF8] = struct{}{}
		out = append(out, c.Candidate)
		if len(out) >= maxCandidates {
			break
		}
	}
	return out
}

func japaneseScore(s string) int {
	score := 0
	for _, r := range s {
		switch {
		case unicode.In(r, unicode.Hiragana, unicode.Katakana):
			score += 3
		case unicode.In(r, unicode.Han):
			score += 2
		case r == '・' || r == '々' || r == '〆' || r == 'ヶ':
			score += 1
		case r < 0x20 || r == utf8.RuneError || r == '\uFFFD':
			score -= 10
		case unicode.IsControl(r):
			score -= 10
		}
	}
	return score
}

func looksLikeMojibake(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	// Already readable Japanese UTF-8 — not mojibake.
	if japaneseScore(s) >= minJapaneseScore {
		return false
	}
	highLatin := 0
	for _, r := range s {
		if r >= 0x80 && r <= 0xFF {
			highLatin++
		}
	}
	return highLatin >= 2
}

func latin1BytesFromString(s string) ([]byte, bool) {
	buf := make([]byte, 0, len(s))
	for _, r := range s {
		if r > 0xFF {
			return nil, false
		}
		buf = append(buf, byte(r))
	}
	return buf, true
}

// Latin1MojibakeFromUTF8 builds a mojibake string by decoding UTF-8 bytes as Latin-1 runes.
// Used by tests to synthesize mojibake samples.
func Latin1MojibakeFromUTF8(b []byte) string {
	var buf bytes.Buffer
	for _, c := range b {
		buf.WriteRune(rune(c))
	}
	return buf.String()
}
