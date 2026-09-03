package preview

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

func writeRuleTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunRulesFirstDeclinesSecondMatches(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lantern.txt")
	writeRuleTestFile(t, path)

	req := Request{
		Path:      path,
		TextWidth: 80,
		WorkDir:   dir,
		Preview: config.PreviewConfig{
			ShellPatterns: true,
			Commands: []config.PreviewCommandRule{
				{When: []string{"f *.txt"}, Command: "sh -c 'exit 7'"},
				{When: []string{"f *.txt"}, Command: `sh -c 'echo lantern; exit 0'`},
			},
		},
	}
	res, matched := RunRules(context.Background(), req)
	if !matched {
		t.Fatal("matched = false, want true (second rule should have taken responsibility)")
	}
	if res.CombinedText != "lantern\n" {
		t.Fatalf("CombinedText = %q, want %q", res.CombinedText, "lantern\n")
	}
	if res.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", res.ExitCode)
	}
}

func TestRunRulesAllMatchingRulesDecline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "meadow.txt")
	writeRuleTestFile(t, path)

	req := Request{
		Path:    path,
		WorkDir: dir,
		Preview: config.PreviewConfig{
			ShellPatterns: true,
			Commands: []config.PreviewCommandRule{
				{When: []string{"f *.txt"}, Command: "sh -c 'exit 1'"},
				{When: []string{"f *.txt"}, Command: "sh -c 'exit 2'"},
			},
		},
	}
	if _, matched := RunRules(context.Background(), req); matched {
		t.Fatal("matched = true, want false when every matching rule declines")
	}
}

func TestRunRulesNoCommandsSpawnsNoSubprocess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "orchard.txt")
	writeRuleTestFile(t, path)

	req := Request{Path: path, WorkDir: dir, Preview: config.PreviewConfig{}}
	if _, matched := RunRules(context.Background(), req); matched {
		t.Fatal("matched = true, want false when Commands is empty")
	}
}

func TestRunRulesNonMatchingRuleSkipped(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cobalt.txt")
	writeRuleTestFile(t, path)

	req := Request{
		Path:    path,
		WorkDir: dir,
		Preview: config.PreviewConfig{
			ShellPatterns: true,
			Commands: []config.PreviewCommandRule{
				{When: []string{"f *.pdf"}, Command: "sh -c 'echo should-not-run; exit 0'"},
			},
		},
	}
	if _, matched := RunRules(context.Background(), req); matched {
		t.Fatal("matched = true, want false: rule's when (*.pdf) does not match a .txt path")
	}
}

func TestRunRulesDirectoryRule(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "harbor")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	req := Request{
		Path:    sub,
		WorkDir: sub,
		IsDir:   true,
		Preview: config.PreviewConfig{
			Commands: []config.PreviewCommandRule{
				{When: []string{"t r"}, Command: "sh -c 'echo should-not-run; exit 0'"},
				{When: []string{"t d"}, Command: "sh -c 'echo tree-output; exit 0'"},
			},
		},
	}
	res, matched := RunRules(context.Background(), req)
	if !matched {
		t.Fatal("matched = false, want true: second rule's \"t d\" should match a directory")
	}
	if res.CombinedText != "tree-output\n" {
		t.Fatalf("CombinedText = %q, want %q", res.CombinedText, "tree-output\n")
	}
}

func TestSniffGraphicsProtocol(t *testing.T) {
	tests := []struct {
		name   string
		stdout []byte
		want   previewpanel.ImageProtocol
		wantOK bool
	}{
		{"sixel", []byte("\x1bPq...sixel-data\x1b\\"), previewpanel.ImageProtocolSixel, true},
		{"kitty", []byte("\x1b_Gf=100,a=T;base64data\x1b\\"), previewpanel.ImageProtocolKitty, true},
		{"plain text", []byte("just some text output\n"), 0, false},
		{"leading whitespace before sixel", []byte("\n\t \x1bPq...\x1b\\"), previewpanel.ImageProtocolSixel, true},
		// Real tools (chafa, img2sixel) commonly wrap the DCS in cursor-visibility CSI
		// sequences rather than emitting it as the very first bytes.
		{"cursor-toggle CSI before sixel (chafa-style)",
			[]byte("\x1b[?25l\x1b[?80l\x1b[?8452l\x1bPq...sixel-data...\x1b\\\x1b[?25h"),
			previewpanel.ImageProtocolSixel, true},
		{"empty", nil, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proto, ok := sniffGraphicsProtocol(tt.stdout)
			if ok != tt.wantOK || proto != tt.want {
				t.Fatalf("sniffGraphicsProtocol(%q) = (%v, %v), want (%v, %v)", tt.stdout, proto, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestRunRulesGraphicsOutputSetsImagePayload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "picture.png")
	writeRuleTestFile(t, path)

	req := Request{
		Path:        path,
		WorkDir:     dir,
		ImageMaxPxW: 800,
		ImageMaxPxH: 600,
		Preview: config.PreviewConfig{
			ShellPatterns: true,
			Commands: []config.PreviewCommandRule{
				{When: []string{"f *.png"}, Command: `printf '\033Pq...sixel-data...\033\\'`},
			},
		},
	}
	res, matched := RunRules(context.Background(), req)
	if !matched {
		t.Fatal("matched = false, want true")
	}
	if res.ImagePayload == "" {
		t.Fatal("ImagePayload empty, want the sniffed Sixel payload")
	}
	if res.ImageProtocol != previewpanel.ImageProtocolSixel {
		t.Fatalf("ImageProtocol = %v, want Sixel", res.ImageProtocol)
	}
	if res.ImagePxW != 800 || res.ImagePxH != 600 {
		t.Fatalf("ImagePxW/PxH = %d/%d, want the pane's full pixel budget 800/600", res.ImagePxW, res.ImagePxH)
	}
}

func TestMatchAnyCommandRuleTypeAndPattern(t *testing.T) {
	cfg := config.PreviewConfig{
		ShellPatterns: true,
		Commands: []config.PreviewCommandRule{
			{When: []string{"f *.epub"}},
			{When: []string{"t d"}},
		},
	}
	if !MatchAnyCommandRule(cfg, "/somewhere/book.epub", false, "/somewhere") {
		t.Fatal("expected a file rule match for *.epub")
	}
	if MatchAnyCommandRule(cfg, "/somewhere/notes.txt", false, "/somewhere") {
		t.Fatal("expected no match for a .txt file against *.epub/t d rules")
	}
	if !MatchAnyCommandRule(cfg, "/somewhere/nested", true, "/somewhere") {
		t.Fatal("expected the \"t d\" rule to match a directory")
	}
}

func TestMatchAnyCommandRuleGlobVsRegex(t *testing.T) {
	globCfg := config.PreviewConfig{
		ShellPatterns: true,
		Commands:      []config.PreviewCommandRule{{When: []string{"f *.md"}}},
	}
	if !MatchAnyCommandRule(globCfg, "/x/readme.md", false, "/x") {
		t.Fatal("expected shell-glob match for *.md")
	}

	regexCfg := config.PreviewConfig{
		ShellPatterns: false,
		Commands:      []config.PreviewCommandRule{{When: []string{`f \.md$`}}},
	}
	if !MatchAnyCommandRule(regexCfg, "/x/readme.md", false, "/x") {
		t.Fatal("expected regex match for \\.md$")
	}
	if MatchAnyCommandRule(regexCfg, "/x/readme.mdx", false, "/x") {
		t.Fatal("regex \\.md$ must not match readme.mdx")
	}
}

func TestRunRulesInvalidRegexRuleSkippedNotFatal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gravel.txt")
	writeRuleTestFile(t, path)

	req := Request{
		Path:    path,
		WorkDir: dir,
		Preview: config.PreviewConfig{
			// ShellPatterns false (default) → f's pattern is a regexp; "[" is invalid regexp
			// syntax and must be treated as a non-match, not a fatal error.
			Commands: []config.PreviewCommandRule{
				{When: []string{"f ["}, Command: "sh -c 'echo should-not-run; exit 0'"},
				{When: []string{"f .*"}, Command: "sh -c 'echo fallback; exit 0'"},
			},
		},
	}
	res, matched := RunRules(context.Background(), req)
	if !matched {
		t.Fatal("matched = false, want true: the invalid-regex rule should be skipped, not abort the loop")
	}
	if res.CombinedText != "fallback\n" {
		t.Fatalf("CombinedText = %q, want %q", res.CombinedText, "fallback\n")
	}
}

func TestRunRulesGraphicsOutputWithTrailingTextSplitsCaption(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "picture.png")
	writeRuleTestFile(t, path)

	req := Request{
		Path:         path,
		WorkDir:      dir,
		TextWidth:    80,
		ImageMaxPxW:  800,
		ImageMaxPxH:  600,
		ImageCellPxH: 20,
		Preview: config.PreviewConfig{
			ShellPatterns: true,
			Commands: []config.PreviewCommandRule{
				{When: []string{"f *.png"}, Command: `sh -c 'printf "\033Pq...sixel-data...\033\\\\\nRuntime: 136 min\nScore: 8.3/10\n"'`},
			},
		},
	}
	res, matched := RunRules(context.Background(), req)
	if !matched {
		t.Fatal("matched = false, want true")
	}
	if res.ImagePayload != "\x1bPq...sixel-data...\x1b\\" {
		t.Fatalf("ImagePayload = %q, want only the sixel chunk (no trailing text)", res.ImagePayload)
	}
	if res.CombinedText != "Runtime: 136 min\nScore: 8.3/10" {
		t.Fatalf("CombinedText = %q, want the trailing metadata lines (surrounding newlines trimmed)", res.CombinedText)
	}
	// contentRows = 600/20 = 30; captionRows = 2 lines + 1 separator = 3 reserved rows.
	wantPxH := (30 - 2 - 1) * 20
	if res.ImagePxH != wantPxH {
		t.Fatalf("ImagePxH = %d, want %d (shrunk to reserve caption rows)", res.ImagePxH, wantPxH)
	}
	if !res.ImageFirst {
		t.Fatal("ImageFirst = false, want true: the command's stdout had the image before its text")
	}
}

func TestRunRulesGraphicsOutputNoTrailingTextKeepsFullHeight(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "picture.png")
	writeRuleTestFile(t, path)

	req := Request{
		Path:         path,
		WorkDir:      dir,
		ImageMaxPxW:  800,
		ImageMaxPxH:  600,
		ImageCellPxH: 20,
		Preview: config.PreviewConfig{
			ShellPatterns: true,
			Commands: []config.PreviewCommandRule{
				// chafa-style: leading + trailing cursor-visibility CSI noise, no real text.
				{When: []string{"f *.png"}, Command: `sh -c 'printf "\033[?25l\033Pq...sixel-data...\033\\\\\033[?25h"'`},
			},
		},
	}
	res, matched := RunRules(context.Background(), req)
	if !matched {
		t.Fatal("matched = false, want true")
	}
	if res.CombinedText != "" {
		t.Fatalf("CombinedText = %q, want empty (no real trailing text)", res.CombinedText)
	}
	if res.ImagePxH != 600 {
		t.Fatalf("ImagePxH = %d, want unchanged 600 (no caption to reserve rows for)", res.ImagePxH)
	}
	wantPayload := "\x1b[?25l\x1bPq...sixel-data...\x1b\\\x1b[?25h"
	if res.ImagePayload != wantPayload {
		t.Fatalf("ImagePayload = %q, want %q (CSI noise bundled with image, not split off)", res.ImagePayload, wantPayload)
	}
}

func TestStripLeadingCSISequences(t *testing.T) {
	tests := []struct {
		name         string
		in           string
		wantConsumed string
		wantRest     string
	}{
		{"chafa trailing show-cursor", "\x1b[?25h", "\x1b[?25h", ""},
		{"real text immediately", "Runtime: 136 min\n", "", "Runtime: 136 min\n"},
		{"multiple CSI then text", "\x1b[?25h\x1b[0m" + "Runtime: 136 min\n", "\x1b[?25h\x1b[0m", "Runtime: 136 min\n"},
		{"empty", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			consumed, rest := stripLeadingCSISequences([]byte(tt.in))
			if string(consumed) != tt.wantConsumed || string(rest) != tt.wantRest {
				t.Fatalf("stripLeadingCSISequences(%q) = (%q, %q), want (%q, %q)",
					tt.in, consumed, rest, tt.wantConsumed, tt.wantRest)
			}
		})
	}
}

func TestSplitGraphicsPayload(t *testing.T) {
	tests := []struct {
		name          string
		in            string
		wantImagePart string
		wantTextPart  string
		wantProto     previewpanel.ImageProtocol
		wantOK        bool
	}{
		{"chafa-style trailing CSI, no text",
			"\x1b[?25l\x1bPq...\x1b\\\x1b[?25h",
			"\x1b[?25l\x1bPq...\x1b\\\x1b[?25h", "",
			previewpanel.ImageProtocolSixel, true},
		{"movie-info-style immediate real text",
			"\x1bPq...\x1b\\\nRuntime: 136 min\n",
			"\x1bPq...\x1b\\", "Runtime: 136 min",
			previewpanel.ImageProtocolSixel, true},
		{"no terminator found (truncated) falls back to whole-image",
			"\x1bPq...sixel-data-with-no-terminator",
			"\x1bPq...sixel-data-with-no-terminator", "",
			previewpanel.ImageProtocolSixel, true},
		{"kitty introducer",
			"\x1b_Gf=100,a=T;base64data\x1b\\\nsome caption\n",
			"\x1b_Gf=100,a=T;base64data\x1b\\", "some caption",
			previewpanel.ImageProtocolKitty, true},
		{"kitty multi-chunk transmission (m=1...m=1...m=0) all belongs to the image",
			"\x1b_Ga=T,f=100,m=1;AAAA\x1b\\\x1b_Gm=1;BBBB\x1b\\\x1b_Gm=0;CCCC\x1b\\\nsome caption\n",
			"\x1b_Ga=T,f=100,m=1;AAAA\x1b\\\x1b_Gm=1;BBBB\x1b\\\x1b_Gm=0;CCCC\x1b\\", "some caption",
			previewpanel.ImageProtocolKitty, true},
		{"kitty multi-chunk with no explicit m=0 on the last chunk (m key just absent)",
			"\x1b_Ga=T,m=1;AAAA\x1b\\\x1b_Gi=1;BBBB\x1b\\text-after",
			"\x1b_Ga=T,m=1;AAAA\x1b\\\x1b_Gi=1;BBBB\x1b\\", "text-after",
			previewpanel.ImageProtocolKitty, true},
		{"plain text, no graphics", "just text\n", "", "", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			imagePart, textPart, proto, ok := splitGraphicsPayload([]byte(tt.in))
			if ok != tt.wantOK || proto != tt.wantProto ||
				string(imagePart) != tt.wantImagePart || string(textPart) != tt.wantTextPart {
				t.Fatalf("splitGraphicsPayload(%q) = (%q, %q, %v, %v), want (%q, %q, %v, %v)",
					tt.in, imagePart, textPart, proto, ok,
					tt.wantImagePart, tt.wantTextPart, tt.wantProto, tt.wantOK)
			}
		})
	}
}

func TestRewriteKittyForPlaceholder(t *testing.T) {
	in := "\x1b_Ga=T,f=100,i=99,t=d,m=1;AAAA\x1b\\\x1b_Gi=99,m=0;BBBB\x1b\\"
	got := string(rewriteKittyForPlaceholder([]byte(in)))
	want := "\x1b_Ga=T,f=100,t=d,m=1,i=1,U=1,q=2;AAAA\x1b\\\x1b_Gm=0,i=1;BBBB\x1b\\"
	if got != want {
		t.Fatalf("rewriteKittyForPlaceholder(%q) = %q, want %q", in, got, want)
	}

	// A chunk with no pre-existing i= (movie-info's own commands never send one) still ends up
	// on the fixed placeholder id.
	in2 := "\x1b_Ga=T,f=100,t=d,m=0;AAAA\x1b\\"
	got2 := string(rewriteKittyForPlaceholder([]byte(in2)))
	want2 := "\x1b_Ga=T,f=100,t=d,m=0,i=1,U=1,q=2;AAAA\x1b\\"
	if got2 != want2 {
		t.Fatalf("rewriteKittyForPlaceholder(%q) = %q, want %q", in2, got2, want2)
	}
}

func TestImagePixelDims(t *testing.T) {
	w, h, ok := sixelPixelDims([]byte(`0;1q"1;1;500;750#0;2;0;0;0#...`))
	if !ok || w != 500 || h != 750 {
		t.Fatalf("sixelPixelDims() = (%d, %d, %v), want (500, 750, true)", w, h, ok)
	}
	if _, _, ok := sixelPixelDims([]byte("q...no-raster-attrs...")); ok {
		t.Fatal("sixelPixelDims() ok = true for a payload with no raster attributes")
	}

	w, h, ok = kittyPixelDims([]byte("\x1b_Ga=T,f=100,s=64,v=32;base64data\x1b\\"))
	if !ok || w != 64 || h != 32 {
		t.Fatalf("kittyPixelDims() = (%d, %d, %v), want (64, 32, true)", w, h, ok)
	}
	if _, _, ok := kittyPixelDims([]byte("\x1b_Ga=T,f=100;base64data\x1b\\")); ok {
		t.Fatal("kittyPixelDims() ok = true for a payload with no s=/v= params")
	}
}

// TestKittyPNGPixelDims covers the actual reported bug: movie-info's real Kitty command never
// declares s=/v= (f=100 PNG is self-describing, so a well-behaved sender doesn't need to), which
// meant imagePixelDims always fell back to the "fill whatever space is left" reservation — a
// poster then rendered far too large under placeholder mode (which, unlike cursor-relative mode,
// faithfully renders at whatever size is declared), pushing the caption down with it. Recovering
// the real size from the PNG's own IHDR fixes that.
func TestKittyPNGPixelDims(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 40, 60))
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		t.Fatal(err)
	}
	encoded := base64.StdEncoding.EncodeToString(pngBuf.Bytes())

	// Chunk at a size that is NOT a multiple of 4 (37), unlike pc's own encoder's 4096 — a
	// third-party sender's chunking scheme must not be assumed to align to base64 groups.
	const chunkSize = 37
	var payload bytes.Buffer
	first := true
	for len(encoded) > 0 {
		n := min(chunkSize, len(encoded))
		chunk := encoded[:n]
		encoded = encoded[n:]
		more := 1
		if len(encoded) == 0 {
			more = 0
		}
		if first {
			fmt.Fprintf(&payload, "\x1b_Ga=T,f=100,t=d,m=%d;%s\x1b\\", more, chunk)
			first = false
		} else {
			fmt.Fprintf(&payload, "\x1b_Gm=%d;%s\x1b\\", more, chunk)
		}
	}

	w, h, ok := kittyPNGPixelDims(payload.Bytes())
	if !ok || w != 40 || h != 60 {
		t.Fatalf("kittyPNGPixelDims() = (%d, %d, %v), want (40, 60, true)", w, h, ok)
	}

	// imagePixelDims must reach the same fallback when kittyPixelDims alone finds nothing.
	w, h, ok = imagePixelDims(payload.Bytes(), previewpanel.ImageProtocolKitty)
	if !ok || w != 40 || h != 60 {
		t.Fatalf("imagePixelDims() = (%d, %d, %v), want (40, 60, true)", w, h, ok)
	}
}

// TestRunRulesGraphicsOutputUsesRealDimsWhenTheyFit covers the actual bug that motivated
// imagePixelDims: pc cannot resize a rule command's own pre-encoded Sixel bytes, so declaring an
// arbitrary "fill the space left after the caption" height (as opposed to the image's own real,
// baked-in size) leaves a gap between where the image actually renders and where the caption
// gets positioned. When the real declared size fits the available budget, it must be used as-is.
func TestRunRulesGraphicsOutputUsesRealDimsWhenTheyFit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "picture.png")
	writeRuleTestFile(t, path)

	req := Request{
		Path:         path,
		WorkDir:      dir,
		TextWidth:    80,
		ImageMaxPxW:  800,
		ImageMaxPxH:  600,
		ImageCellPxH: 20,
		Preview: config.PreviewConfig{
			ShellPatterns: true,
			Commands: []config.PreviewCommandRule{
				{When: []string{"f *.png"}, Command: `sh -c 'printf "\033P0;1q\"1;1;300;100#0;2;0;0;0#0~~~~$-\033\\\\\nCaption line\n"'`},
			},
		},
	}
	res, matched := RunRules(context.Background(), req)
	if !matched {
		t.Fatal("matched = false, want true")
	}
	if res.ImagePxW != 300 || res.ImagePxH != 100 {
		t.Fatalf("ImagePxW/PxH = %d/%d, want the payload's own declared 300/100 (not a reservation-based guess)",
			res.ImagePxW, res.ImagePxH)
	}
	if res.CombinedText != "Caption line" {
		t.Fatalf("CombinedText = %q, want %q", res.CombinedText, "Caption line")
	}
}

// TestRunRulesGraphicsOutputUsesRealHeightEvenWhenWidthOverflows covers the actual reported bug:
// a poster image (e.g. 500px wide) commonly exceeds a narrow quick-view side column's pixel
// budget even when its height fits comfortably. Rejecting the real size entirely because width
// didn't fit meant falling back to a reservation with no relation to the real height, leaving a
// gap between where the image actually renders and where the caption gets positioned — height
// alone must decide whether to trust the real size; width is only clamped, never a disqualifier.
func TestRunRulesGraphicsOutputUsesRealHeightEvenWhenWidthOverflows(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "picture.png")
	writeRuleTestFile(t, path)

	req := Request{
		Path:         path,
		WorkDir:      dir,
		TextWidth:    80,
		ImageMaxPxW:  300, // narrower than the payload's declared 500px width
		ImageMaxPxH:  2000,
		ImageCellPxH: 20,
		Preview: config.PreviewConfig{
			ShellPatterns: true,
			Commands: []config.PreviewCommandRule{
				{When: []string{"f *.png"}, Command: `sh -c 'printf "\033P0;1q\"1;1;500;100#0;2;0;0;0#0~~~~$-\033\\\\\nCaption line\n"'`},
			},
		},
	}
	res, matched := RunRules(context.Background(), req)
	if !matched {
		t.Fatal("matched = false, want true")
	}
	if res.ImagePxH != 100 {
		t.Fatalf("ImagePxH = %d, want the payload's own declared 100 (real height fits even though width doesn't)", res.ImagePxH)
	}
	if res.ImagePxW != 300 {
		t.Fatalf("ImagePxW = %d, want clamped to the pane's budget 300, not dropped/ignored", res.ImagePxW)
	}
}
