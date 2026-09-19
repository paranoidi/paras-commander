package preview

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/paranoidi/paras-commander/internal/config"
)

// tiffEntry is one IFD entry being assembled for the test fixture's EXIF blob.
type tiffEntry struct {
	tag          uint16
	typ          uint16
	count        uint32
	inline       [4]byte
	externalData []byte // non-nil: written to the shared external-data area, offset patched into inline
}

func tiffStringEntry(tag uint16, s string) tiffEntry {
	b := append([]byte(s), 0)
	return tiffEntry{tag: tag, typ: 2, count: uint32(len(b)), externalData: b}
}

func tiffShortEntry(tag uint16, v uint16) tiffEntry {
	e := tiffEntry{tag: tag, typ: 3, count: 1}
	binary.LittleEndian.PutUint16(e.inline[:2], v)
	return e
}

func tiffLongEntry(tag uint16, v uint32) tiffEntry {
	e := tiffEntry{tag: tag, typ: 4, count: 1}
	binary.LittleEndian.PutUint32(e.inline[:], v)
	return e
}

func tiffRationalEntry(tag uint16, num, den uint32) tiffEntry {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint32(b[0:4], num)
	binary.LittleEndian.PutUint32(b[4:8], den)
	return tiffEntry{tag: tag, typ: 5, count: 1, externalData: b}
}

func tiffSignedRationalEntry(tag uint16, num, den int32) tiffEntry {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint32(b[0:4], uint32(num))
	binary.LittleEndian.PutUint32(b[4:8], uint32(den))
	return tiffEntry{tag: tag, typ: 10, count: 1, externalData: b}
}

// buildTestTIFFExif assembles a little-endian TIFF/EXIF blob: header, IFD0 (Make/Model/Software/
// ExifIFD pointer), an Exif sub-IFD (the rest), and a trailing external-data area holding every
// ASCII string and RATIONAL value (TIFF only stores values >4 bytes by offset).
func buildTestTIFFExif() []byte {
	ifd0 := []tiffEntry{
		tiffStringEntry(0x010F, "Panasonic"),       // Make
		tiffStringEntry(0x0110, "Panasonic DC-G9"), // Model (prefixed by Make)
		tiffStringEntry(0x0131, "TestSoft 1.0"),    // Software
		tiffLongEntry(0x8769, 0),                   // ExifIFD pointer, patched below
	}
	exifIFD := []tiffEntry{
		tiffStringEntry(0x9003, "2024:09:29 10:37:52"), // DateTimeOriginal
		tiffRationalEntry(0x829D, 49, 10),              // FNumber 4.9
		tiffRationalEntry(0x829A, 1, 125),              // ExposureTime 1/125s
		tiffShortEntry(0x8827, 800),                    // ISO
		tiffRationalEntry(0x920A, 460, 10),             // FocalLength 46mm
		tiffStringEntry(0xA434, "LUMIX 12-60mm"),       // LensModel
		tiffShortEntry(0xA405, 92),                     // FocalLengthIn35mmFormat
		tiffSignedRationalEntry(0x9204, 3, 10),         // ExposureCompensation +0.3 EV
		tiffShortEntry(0x9209, 1),                      // Flash (bit0 fired)
		tiffShortEntry(0x8822, 3),                      // ExposureProgram (Aperture priority)
		tiffShortEntry(0xA403, 1),                      // WhiteBalance (manual)
	}
	sort.Slice(ifd0, func(i, j int) bool { return ifd0[i].tag < ifd0[j].tag })
	sort.Slice(exifIFD, func(i, j int) bool { return exifIFD[i].tag < exifIFD[j].tag })

	ifd0Offset := 8
	ifd0Size := 2 + len(ifd0)*12 + 4
	exifIFDOffset := ifd0Offset + ifd0Size
	exifIFDSize := 2 + len(exifIFD)*12 + 4
	externBase := exifIFDOffset + exifIFDSize

	for i := range ifd0 {
		if ifd0[i].tag == 0x8769 {
			binary.LittleEndian.PutUint32(ifd0[i].inline[:], uint32(exifIFDOffset))
		}
	}

	var extern bytes.Buffer
	assignExternal := func(entries []tiffEntry) {
		for i := range entries {
			if entries[i].externalData == nil {
				continue
			}
			off := externBase + extern.Len()
			binary.LittleEndian.PutUint32(entries[i].inline[:], uint32(off))
			extern.Write(entries[i].externalData)
			if extern.Len()%2 != 0 {
				extern.WriteByte(0)
			}
		}
	}
	assignExternal(ifd0)
	assignExternal(exifIFD)

	writeIFD := func(buf *bytes.Buffer, entries []tiffEntry) {
		_ = binary.Write(buf, binary.LittleEndian, uint16(len(entries)))
		for _, e := range entries {
			_ = binary.Write(buf, binary.LittleEndian, e.tag)
			_ = binary.Write(buf, binary.LittleEndian, e.typ)
			_ = binary.Write(buf, binary.LittleEndian, e.count)
			buf.Write(e.inline[:])
		}
		_ = binary.Write(buf, binary.LittleEndian, uint32(0)) // next IFD offset: none
	}

	var buf bytes.Buffer
	buf.WriteString("II")
	_ = binary.Write(&buf, binary.LittleEndian, uint16(42))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(ifd0Offset))
	writeIFD(&buf, ifd0)
	writeIFD(&buf, exifIFD)
	buf.Write(extern.Bytes())
	return buf.Bytes()
}

// writeTestJPEGWithEXIF encodes a small JPEG and splices an APP1 Exif segment (built by
// buildTestTIFFExif) right after the SOI marker, then writes it to path.
func writeTestJPEGWithEXIF(t *testing.T, path string, w, h int) {
	t.Helper()
	writeTestJPEGWithExifBytes(t, path, w, h, buildTestTIFFExif())
}

// buildTestTIFFExifOrientation assembles a minimal little-endian TIFF/EXIF blob containing only
// an Orientation tag in IFD0 — a SHORT value fits inline, so no external-data area is needed.
func buildTestTIFFExifOrientation(orientation uint16) []byte {
	ifd0 := []tiffEntry{tiffShortEntry(0x0112, orientation)}
	var buf bytes.Buffer
	buf.WriteString("II")
	_ = binary.Write(&buf, binary.LittleEndian, uint16(42))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(8))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(len(ifd0)))
	for _, e := range ifd0 {
		_ = binary.Write(&buf, binary.LittleEndian, e.tag)
		_ = binary.Write(&buf, binary.LittleEndian, e.typ)
		_ = binary.Write(&buf, binary.LittleEndian, e.count)
		buf.Write(e.inline[:])
	}
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0)) // next IFD offset: none
	return buf.Bytes()
}

// writeTestJPEGWithOrientation encodes a small JPEG and splices an APP1 Exif segment carrying
// only the given Orientation tag value.
func writeTestJPEGWithOrientation(t *testing.T, path string, w, h int, orientation uint16) {
	t.Helper()
	writeTestJPEGWithExifBytes(t, path, w, h, buildTestTIFFExifOrientation(orientation))
}

// writeTestJPEGWithExifBytes encodes a small JPEG (distinct-colored pixels per coordinate) and
// splices an APP1 Exif segment wrapping tiff right after the SOI marker, then writes it to path.
func writeTestJPEGWithExifBytes(t *testing.T, path string, w, h int, tiff []byte) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 80, A: 255})
		}
	}
	var jpegBuf bytes.Buffer
	if err := jpeg.Encode(&jpegBuf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("jpeg.Encode: %v", err)
	}
	raw := jpegBuf.Bytes()
	if len(raw) < 2 || raw[0] != 0xFF || raw[1] != 0xD8 {
		t.Fatalf("encoded image missing JPEG SOI marker")
	}

	payload := append([]byte("Exif\x00\x00"), tiff...)
	length := len(payload) + 2
	app1 := []byte{0xFF, 0xE1, byte(length >> 8), byte(length)}
	app1 = append(app1, payload...)

	out := make([]byte, 0, len(raw)+len(app1))
	out = append(out, raw[:2]...)
	out = append(out, app1...)
	out = append(out, raw[2:]...)

	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatalf("write jpeg: %v", err)
	}
}

func TestImageCaptionOffReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shot.jpg")
	writeTestJPEGWithEXIF(t, path, 8, 6)

	if got := ImageCaption(path, "JPEG", 8, 6, 12345, config.PreviewImageMetadataOff); got != "" {
		t.Fatalf("ImageCaption(off) = %q, want empty (image only)", got)
	}

	// Zero-value semantics: an empty level (unvalidated config literal) behaves like "off".
	if got := ImageCaption(path, "JPEG", 8, 6, 12345, ""); got != "" {
		t.Fatalf("ImageCaption(\"\") = %q, want empty", got)
	}
}

func TestImageCaptionBasic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shot.jpg")
	writeTestJPEGWithEXIF(t, path, 8, 6)

	got := ImageCaption(path, "JPEG", 8, 6, 12345, config.PreviewImageMetadataBasic)
	base := formatImageMeta("JPEG", 8, 6, 12345)
	want := base + "\nPanasonic DC-G9 / 2024-09-29 10:37:52"
	if got != want {
		t.Fatalf("ImageCaption(basic) = %q, want %q", got, want)
	}
}

func TestImageCaptionEssentials(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shot.jpg")
	writeTestJPEGWithEXIF(t, path, 8, 6)

	got := ImageCaption(path, "JPEG", 8, 6, 12345, config.PreviewImageMetadataEssentials)
	base := formatImageMeta("JPEG", 8, 6, 12345)
	want := base + "\n" +
		"Panasonic DC-G9 / LUMIX 12-60mm\n" +
		"2024-09-29 10:37:52 / f/4.9 / 1/125 s / ISO 800 / 46 mm"
	if got != want {
		t.Fatalf("ImageCaption(essentials) =\n%q\nwant\n%q", got, want)
	}
}

func TestImageCaptionFull(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shot.jpg")
	writeTestJPEGWithEXIF(t, path, 8, 6)

	got := ImageCaption(path, "JPEG", 8, 6, 12345, config.PreviewImageMetadataFull)
	base := formatImageMeta("JPEG", 8, 6, 12345)
	want := base + "\n" +
		"Panasonic DC-G9 / LUMIX 12-60mm\n" +
		"2024-09-29 10:37:52 / f/4.9 / 1/125 s / ISO 800 / 46 mm\n" +
		"TestSoft 1.0 / 92 mm eq. / +0.3 EV / flash / Aperture priority / WB manual"
	if got != want {
		t.Fatalf("ImageCaption(full) =\n%q\nwant\n%q", got, want)
	}
}

func TestImageCaptionNoEXIFDataFallsBackToBaseLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plain.png")
	writeTestPNG(t, path, 8, 6)

	got := ImageCaption(path, "PNG", 8, 6, 999, config.PreviewImageMetadataFull)
	want := formatImageMeta("PNG", 8, 6, 999)
	if got != want {
		t.Fatalf("ImageCaption(no exif) = %q, want %q", got, want)
	}
}

func TestImageRenderBudgetPxH(t *testing.T) {
	// Empty caption: budget is untouched.
	if got := ImageRenderBudgetPxH(400, 20, 40, ""); got != 400 {
		t.Fatalf("ImageRenderBudgetPxH(empty caption) = %d, want 400", got)
	}
	// One line of caption + one separator row reserves 2 cells' worth of pixels.
	got := ImageRenderBudgetPxH(400, 20, 40, "JPEG image / 8 × 6 px / 1.0 KB")
	want := 400 - 2*20
	if got != want {
		t.Fatalf("ImageRenderBudgetPxH(one line) = %d, want %d", got, want)
	}
	// Never negative even when the caption alone exceeds the budget.
	if got := ImageRenderBudgetPxH(10, 20, 40, "line one\nline two\nline three"); got < 0 {
		t.Fatalf("ImageRenderBudgetPxH must not go negative, got %d", got)
	}
}

func TestJoinMediaText(t *testing.T) {
	if got := JoinMediaText("", "Generating thumbnails…"); got != "Generating thumbnails…" {
		t.Fatalf("JoinMediaText(empty meta) = %q", got)
	}
	if got := JoinMediaText("Media / 1.0 MB", ""); got != "Media / 1.0 MB" {
		t.Fatalf("JoinMediaText(empty status) = %q", got)
	}
	want := "Media / 1.0 MB\n\nGenerating thumbnails…"
	if got := JoinMediaText("Media / 1.0 MB", "Generating thumbnails…"); got != want {
		t.Fatalf("JoinMediaText = %q, want %q", got, want)
	}
}
