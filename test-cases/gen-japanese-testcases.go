//go:build ignore

// Temporary generator for Japanese filename encoding test cases.
// Run: ./gen-japanese-testcases.sh   or   go run gen-japanese-testcases.go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf8"

	"golang.org/x/text/encoding/japanese"
)

func main() {
	root := filepath.Join(".", "japanese")
	if err := os.RemoveAll(root); err != nil {
		fatal(err)
	}
	if err := os.Mkdir(root, 0o755); err != nil {
		fatal(err)
	}

	cases := []struct {
		kind   string
		utf8   string
		nameFn func(string) (string, error)
	}{
		{
			kind:   "utf8-control",
			utf8:   "ascii-only",
			nameFn: func(s string) (string, error) { return s, nil },
		},
		{
			kind:   "utf8-proper",
			utf8:   "正しい日本語フォルダ",
			nameFn: func(s string) (string, error) { return s, nil },
		},
		{
			kind:   "shift_jis-bytes",
			utf8:   "文書",
			nameFn: encodeShiftJIS,
		},
		{
			kind:   "shift_jis-bytes",
			utf8:   "画像フォルダ",
			nameFn: encodeShiftJIS,
		},
		{
			kind:   "euc_jp-bytes",
			utf8:   "音楽",
			nameFn: encodeEUCJP,
		},
		{
			kind:   "mojibake",
			utf8:   "テスト",
			nameFn: encodeMojibakeFromShiftJIS,
		},
	}

	var readme string
	readme += "Japanese encoding test cases (generated; safe to delete this tree)\n\n"
	readme += "Open japanese/ in paras-commander, rename a garbled entry, press F4 when offered.\n\n"

	for i, tc := range cases {
		onDisk, err := tc.nameFn(tc.utf8)
		if err != nil {
			fatal(fmt.Errorf("%s: %w", tc.kind, err))
		}
		dir := filepath.Join(root, onDisk)
		if err := os.Mkdir(dir, 0o755); err != nil {
			fatal(err)
		}
		marker := filepath.Join(dir, "README.txt")
		body := fmt.Sprintf("kind=%s\nutf8=%q\non_disk_bytes=%d\nvalid_utf8=%v\n",
			tc.kind, tc.utf8, len(onDisk), utf8.ValidString(onDisk))
		if err := os.WriteFile(marker, []byte(body), 0o644); err != nil {
			fatal(err)
		}

		readme += fmt.Sprintf("%d. [%s] %q\n", i+1, tc.kind, tc.utf8)
		readme += fmt.Sprintf("   dir: japanese/%q\n", onDisk)
		readme += fmt.Sprintf("   valid UTF-8 on disk: %v\n", utf8.ValidString(onDisk))
		if onDisk != tc.utf8 {
			if tc.kind == "mojibake" {
				readme += "   (UTF-8 mojibake — use Rename → F4 Encoding)\n"
			} else {
				readme += "   (legacy bytes — use Rename → F4 Encoding)\n"
			}
		} else if tc.kind == "utf8-proper" {
			readme += "   (proper UTF-8 — F4 should not appear)\n"
		} else {
			readme += "   (ASCII — F4 should not appear)\n"
		}
		readme += "\n"
	}

	readmePath := filepath.Join(root, "README.txt")
	if err := os.WriteFile(readmePath, []byte(readme), 0o644); err != nil {
		fatal(err)
	}

	fmt.Printf("Created %s with %d testcase directories.\n", root, len(cases))
	fmt.Printf("See %s for details.\n", readmePath)
}

func encodeShiftJIS(utf8 string) (string, error) {
	return japanese.ShiftJIS.NewEncoder().String(utf8)
}

func encodeEUCJP(utf8 string) (string, error) {
	return japanese.EUCJP.NewEncoder().String(utf8)
}

func encodeMojibakeFromShiftJIS(utf8 string) (string, error) {
	sjis, err := japanese.ShiftJIS.NewEncoder().String(utf8)
	if err != nil {
		return "", err
	}
	var out []rune
	for _, b := range []byte(sjis) {
		out = append(out, rune(b))
	}
	return string(out), nil
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "gen-japanese-testcases: %v\n", err)
	os.Exit(1)
}
