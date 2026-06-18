package filenameenc

import (
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/text/encoding/japanese"
)

func TestDetectCandidatesShiftJISLegacyBytes(t *testing.T) {
	t.Parallel()
	want := "日本語フォルダ"
	encoded, err := japanese.ShiftJIS.NewEncoder().String(want)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if utf8.ValidString(encoded) {
		t.Fatal("encoded name should be invalid UTF-8")
	}
	cands := DetectCandidates(encoded)
	if len(cands) == 0 {
		t.Fatal("want candidates")
	}
	if cands[0].UTF8 != want {
		t.Fatalf("first UTF8 = %q want %q", cands[0].UTF8, want)
	}
	if !strings.Contains(cands[0].Label, "Shift-JIS") {
		t.Fatalf("label = %q want Shift-JIS", cands[0].Label)
	}
}

func TestDetectCandidatesMojibake(t *testing.T) {
	t.Parallel()
	want := "テスト"
	sjis, err := japanese.ShiftJIS.NewEncoder().String(want)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	mojibake := Latin1MojibakeFromUTF8([]byte(sjis))
	if !utf8.ValidString(mojibake) {
		t.Fatal("mojibake should be valid UTF-8")
	}
	cands := DetectCandidates(mojibake)
	if len(cands) == 0 {
		t.Fatal("want candidates")
	}
	found := false
	for _, c := range cands {
		if c.UTF8 == want {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("candidates %v do not include %q", cands, want)
	}
}

func TestDetectCandidatesNoFalsePositiveASCII(t *testing.T) {
	t.Parallel()
	if got := DetectCandidates("hello_world.txt"); len(got) != 0 {
		t.Fatalf("got %v want none", got)
	}
}

func TestDetectCandidatesNoFalsePositiveUTF8Japanese(t *testing.T) {
	t.Parallel()
	if got := DetectCandidates("日本語"); len(got) != 0 {
		t.Fatalf("got %v want none", got)
	}
}

func TestJapaneseScore(t *testing.T) {
	t.Parallel()
	if japaneseScore("abc") < minJapaneseScore {
		// expected
	} else {
		t.Fatal("ascii should score low")
	}
	if japaneseScore("あいう") < minJapaneseScore {
		t.Fatal("hiragana should score high")
	}
}
