package preview

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bep/imagemeta"
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

// exifWantedTags limits the EXIF walk to only the tags ImageCaption ever formats — the decoder's
// default ShouldHandleTag would otherwise happily walk (and allocate for) every tag in every IFD.
var exifWantedTags = map[string]bool{
	"Make": true, "Model": true, "LensModel": true, "DateTimeOriginal": true,
	"FNumber": true, "ExposureTime": true, "FocalLength": true, "ExposureCompensation": true,
	"ISO": true, "FocalLengthIn35mmFormat": true, "ExposureProgram": true, "Flash": true,
	"WhiteBalance": true, "Software": true,
	// Consulted by Tags.GetLatLong, not read directly here.
	"GPSLatitude": true, "GPSLongitude": true, "GPSLatitudeRef": true, "GPSLongitudeRef": true,
}

// exifImageFormat maps a file extension to the imagemeta.ImageFormat Decode needs — auto-detect
// isn't implemented by the library, so every caller must supply it explicitly.
func exifImageFormat(path string) (imagemeta.ImageFormat, bool) {
	switch strings.ToLower(strings.TrimPrefix(filepath.Ext(path), ".")) {
	case "jpg", "jpeg":
		return imagemeta.JPEG, true
	case "tif", "tiff":
		return imagemeta.TIFF, true
	case "png":
		return imagemeta.PNG, true
	case "webp":
		return imagemeta.WebP, true
	default:
		return 0, false
	}
}

// boostedMetadataLevel maps "off" (and the unvalidated zero value) up to "basic" — used by
// text-only fallback paths where no image will ever be shown (decode too large, images
// disabled, no protocol): showing nothing there would leave the pane pointlessly blank, unlike
// the successfully-rendered-image case where "off" legitimately means no caption at all.
func boostedMetadataLevel(level string) string {
	if level == "" || level == config.PreviewImageMetadataOff {
		return config.PreviewImageMetadataBasic
	}
	return level
}

// ImageCaption returns the metadata text drawn under a still image at the configured
// [preview].image_metadata level: the base "FORMAT image / W × H px / size" line, then EXIF
// lines. level == "off" (or "", the zero value of an unvalidated config.PreviewConfig literal)
// returns "" — a successfully rendered image shows no caption at all at that level, matching the
// image-only behavior before this feature existed. Decode errors or missing EXIF data never
// surface as an error — metadata is decoration, not something worth failing a preview over.
func ImageCaption(path, format string, w, h int, size int64, level string) string {
	if level == "" || level == config.PreviewImageMetadataOff {
		return ""
	}
	base := formatImageMeta(format, w, h, size)
	lines := exifLines(path, level)
	if len(lines) == 0 {
		return base
	}
	return base + "\n" + strings.Join(lines, "\n")
}

// exifLines returns the EXIF detail lines for level ("basic", "essentials", or "full"), or nil
// when the format has no EXIF support, the file can't be decoded, or no relevant tags are set.
func exifLines(path, level string) []string {
	format, ok := exifImageFormat(path)
	if !ok {
		return nil
	}
	tags, err := decodeExifTags(path, format, func(tag string) bool { return exifWantedTags[tag] })
	if err != nil {
		return nil
	}
	exif := tags.EXIF()
	if len(exif) == 0 {
		return nil
	}

	camera := cameraLabel(exifString(exif, "Make"), exifString(exif, "Model"))
	lens := exifString(exif, "LensModel")
	date := formatExifDateTime(exifString(exif, "DateTimeOriginal"))

	if level == config.PreviewImageMetadataBasic {
		line := joinNonEmpty(" / ", camera, date)
		if line == "" {
			return nil
		}
		return []string{line}
	}

	// essentials and full share the first three lines (camera/lens, exposure, GPS).
	var lines []string
	if l := joinNonEmpty(" / ", camera, lens); l != "" {
		lines = append(lines, l)
	}

	var expParts []string
	if date != "" {
		expParts = append(expParts, date)
	}
	if fn, ok := exifRatFloat(exif, "FNumber"); ok {
		expParts = append(expParts, formatFNumber(fn))
	}
	if shutter, ok := exifRatString(exif, "ExposureTime"); ok {
		expParts = append(expParts, shutter+" s")
	}
	if iso, ok := exifInt(exif, "ISO"); ok {
		expParts = append(expParts, fmt.Sprintf("ISO %d", iso))
	}
	if fl, ok := exifRatFloat(exif, "FocalLength"); ok {
		expParts = append(expParts, fmt.Sprintf("%s mm", trimFloat(fl)))
	}
	if len(expParts) > 0 {
		lines = append(lines, strings.Join(expParts, " / "))
	}

	if lat, long, err := tags.GetLatLong(); err == nil && (lat != 0 || long != 0) {
		lines = append(lines, fmt.Sprintf("GPS %.4f, %.4f", lat, long))
	}

	if level == config.PreviewImageMetadataFull {
		var fullParts []string
		if sw := exifString(exif, "Software"); sw != "" {
			fullParts = append(fullParts, sw)
		}
		if eq, ok := exifInt(exif, "FocalLengthIn35mmFormat"); ok && eq > 0 {
			fullParts = append(fullParts, fmt.Sprintf("%d mm eq.", eq))
		}
		if ev, ok := exifRatFloat(exif, "ExposureCompensation"); ok {
			fullParts = append(fullParts, formatEV(ev))
		}
		if fl, ok := exifInt(exif, "Flash"); ok {
			fullParts = append(fullParts, flashLabel(fl))
		}
		if prog, ok := exifInt(exif, "ExposureProgram"); ok {
			if lbl := exposureProgramLabel(prog); lbl != "" {
				fullParts = append(fullParts, lbl)
			}
		}
		if wb, ok := exifInt(exif, "WhiteBalance"); ok {
			fullParts = append(fullParts, "WB "+whiteBalanceLabel(wb))
		}
		if len(fullParts) > 0 {
			lines = append(lines, strings.Join(fullParts, " / "))
		}
	}

	return lines
}

// decodeExifTags opens path and decodes the EXIF tags for which want returns true, with a 2s
// timeout — good enough for a local file read; a hung decode should never be able to stall a
// preview indefinitely.
func decodeExifTags(path string, format imagemeta.ImageFormat, want func(tag string) bool) (imagemeta.Tags, error) {
	f, err := os.Open(path)
	if err != nil {
		return imagemeta.Tags{}, err
	}
	defer func() { _ = f.Close() }()

	var tags imagemeta.Tags
	_, err = imagemeta.Decode(imagemeta.Options{
		R:           f,
		ImageFormat: format,
		Sources:     imagemeta.EXIF,
		ShouldHandleTag: func(t imagemeta.TagInfo) bool {
			return want(t.Tag)
		},
		HandleTag: func(t imagemeta.TagInfo) error {
			tags.Add(t)
			return nil
		},
		Timeout: 2 * time.Second,
	})
	return tags, err
}

// exifOrientation returns the EXIF Orientation tag (1-8) for path, or 1 (normal) when the format
// has no EXIF, the file can't be decoded, or the tag is missing/out of range.
func exifOrientation(path string) int {
	const normal = 1
	format, ok := exifImageFormat(path)
	if !ok {
		return normal
	}
	tags, err := decodeExifTags(path, format, func(tag string) bool { return tag == "Orientation" })
	if err != nil {
		return normal
	}
	if v, ok := exifInt(tags.EXIF(), "Orientation"); ok && v >= 1 && v <= 8 {
		return v
	}
	return normal
}

// cameraLabel joins make/model, dropping a make prefix duplicated in model (e.g. make
// "Panasonic", model "Panasonic DC-G9" -> "Panasonic DC-G9", not "Panasonic Panasonic DC-G9").
func cameraLabel(makeStr, model string) string {
	makeStr = strings.TrimSpace(makeStr)
	model = strings.TrimSpace(model)
	switch {
	case makeStr == "":
		return model
	case model == "":
		return makeStr
	case strings.HasPrefix(model, makeStr):
		return model
	default:
		return makeStr + " " + model
	}
}

// formatExifDateTime reformats EXIF's "2024:09:29 10:37:52" into "2024-09-29 10:37:52". Returns
// the raw string unchanged if it doesn't parse.
func formatExifDateTime(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	t, err := time.Parse("2006:01:02 15:04:05", raw)
	if err != nil {
		return raw
	}
	return t.Format("2006-01-02 15:04:05")
}

// ratValue is satisfied by imagemeta.Rat[uint32] and imagemeta.Rat[int32] alike — Float64/String
// don't depend on the generic parameter, so this narrower interface lets exifRatFloat/
// exifRatString accept either without caring which one a given tag decoded to.
type ratValue interface {
	Float64() float64
	String() string
}

func exifString(exif map[string]imagemeta.TagInfo, name string) string {
	t, ok := exif[name]
	if !ok {
		return ""
	}
	s, _ := t.Value.(string)
	return strings.TrimSpace(s)
}

func exifRatFloat(exif map[string]imagemeta.TagInfo, name string) (float64, bool) {
	t, ok := exif[name]
	if !ok {
		return 0, false
	}
	r, ok := t.Value.(ratValue)
	if !ok {
		return 0, false
	}
	return r.Float64(), true
}

// exifRatString returns the tag's Rat.String() form ("1/125", or "2" when the denominator is 1)
// — used for shutter speed, where the raw fraction reads better than a decimal.
func exifRatString(exif map[string]imagemeta.TagInfo, name string) (string, bool) {
	t, ok := exif[name]
	if !ok {
		return "", false
	}
	r, ok := t.Value.(ratValue)
	if !ok {
		return "", false
	}
	return r.String(), true
}

// exifInt converts any of the integer types the decoder may have used for a SHORT/LONG-typed tag
// (observed: uint16 for most cameras, uint32 for some) into a plain int.
func exifInt(exif map[string]imagemeta.TagInfo, name string) (int, bool) {
	t, ok := exif[name]
	if !ok {
		return 0, false
	}
	switch v := t.Value.(type) {
	case uint8:
		return int(v), true
	case uint16:
		return int(v), true
	case uint32:
		return int(v), true
	case int32:
		return int(v), true
	case int:
		return v, true
	default:
		return 0, false
	}
}

func joinNonEmpty(sep string, parts ...string) string {
	var out []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, sep)
}

// trimFloat formats v with one decimal, dropping a trailing ".0" (4.0 -> "4", 4.9 -> "4.9").
func trimFloat(v float64) string {
	s := strconv.FormatFloat(v, 'f', 1, 64)
	return strings.TrimSuffix(strings.TrimSuffix(s, "0"), ".")
}

func formatFNumber(v float64) string {
	return "f/" + trimFloat(v)
}

// formatEV formats an exposure-compensation value with an explicit sign ("+0.3 EV", "-0.7 EV").
func formatEV(v float64) string {
	sign := "+"
	if v < 0 {
		sign = ""
	}
	return sign + trimFloat(v) + " EV"
}

func flashLabel(v int) string {
	if v&0x1 != 0 {
		return "flash"
	}
	return "no flash"
}

// exposureProgramLabel maps the EXIF ExposureProgram enum (tag 0x8822) to a short label.
func exposureProgramLabel(v int) string {
	switch v {
	case 1:
		return "Manual"
	case 2:
		return "Program AE"
	case 3:
		return "Aperture priority"
	case 4:
		return "Shutter priority"
	case 5:
		return "Creative program"
	case 6:
		return "Action program"
	case 7:
		return "Portrait mode"
	case 8:
		return "Landscape mode"
	default:
		return ""
	}
}

// whiteBalanceLabel maps the EXIF WhiteBalance enum (tag 0xA403): 0 = auto, anything else = manual.
func whiteBalanceLabel(v int) string {
	if v == 0 {
		return "auto"
	}
	return "manual"
}

// ImageRenderBudgetPxH is the pixel height left for the image once caption rows (wrapped at
// textWidth) plus one separator row are reserved; maxPxH is returned unchanged when caption is
// empty. Uses previewpanel.TotalLines with a zero tcell.Style — style doesn't affect wrap
// geometry — the same wrapper drawImageBody uses to lay out the caption, so this can't disagree
// with what actually gets drawn.
func ImageRenderBudgetPxH(maxPxH, cellPxH, textWidth int, caption string) int {
	if strings.TrimSpace(caption) == "" || cellPxH < 1 {
		return maxPxH
	}
	captionRows := previewpanel.TotalLines(
		previewpanel.State{Source: previewpanel.SourceExternalANSI, CombinedText: caption},
		textWidth, tcell.Style{})
	if captionRows < 1 {
		captionRows = 1
	}
	const captionSeparatorRows = 1
	contentRows := maxPxH / cellPxH
	availableRows := contentRows - captionRows - captionSeparatorRows
	if availableRows < 0 {
		availableRows = 0
	}
	budget := availableRows * cellPxH
	if budget < 0 {
		budget = 0
	}
	return budget
}

// JoinMediaText joins media metadata text with a trailing status line (e.g. "Generating
// thumbnails…"), skipping the blank-line separator when meta is empty so an off video_metadata
// setting doesn't leave leading blank rows above the status line.
func JoinMediaText(meta, status string) string {
	switch {
	case meta == "":
		return status
	case status == "":
		return meta
	default:
		return meta + "\n\n" + status
	}
}
