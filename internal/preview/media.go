package preview

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/language"

	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

// GeneratingThumbnailsLabel is appended under media metadata while ffmpeg extracts frames.
const GeneratingThumbnailsLabel = "Generating thumbnails"

// ffprobeDoc is the JSON shape returned by ffprobe -of json.
type ffprobeDoc struct {
	Streams []ffprobeStream `json:"streams"`
	Format  ffprobeFormat   `json:"format"`
}

type ffprobeStream struct {
	CodecType          string `json:"codec_type"`
	CodecName          string `json:"codec_name"`
	CodecLongName      string `json:"codec_long_name"`
	Width              int    `json:"width"`
	Height             int    `json:"height"`
	DisplayAspectRatio string `json:"display_aspect_ratio"`
	BitRate            string `json:"bit_rate"`
	Duration           string `json:"duration"`
	RFrameRate         string `json:"r_frame_rate"`
	PixFmt             string `json:"pix_fmt"`
	SampleRate         string `json:"sample_rate"`
	Channels           int    `json:"channels"`
	Disposition        *struct {
		AttachedPic int `json:"attached_pic"`
	} `json:"disposition"`
	Tags struct {
		Language string `json:"language"`
		Title    string `json:"title"`
	} `json:"tags"`
}

type ffprobeFormat struct {
	Filename string `json:"filename"`
	Duration string `json:"duration"`
	Size     string `json:"size"`
	BitRate  string `json:"bit_rate"`
}

// MediaThumbWork holds probe state for a follow-up thumbnail encode after metadata is shown.
type MediaThumbWork struct {
	meta     string
	duration float64
}

// MediaThumbDuration returns the probed duration from MediaThumbWork, or 0.
func MediaThumbDuration(work *MediaThumbWork) float64 {
	if work == nil {
		return 0
	}
	return work.duration
}

// RunMediaMeta probes the file and returns text metadata. When work is non-nil,
// the caller should show GeneratingThumbnailsLabel under the meta, then call RunMediaThumbs.
// A nil ctx runs the ffprobe subprocess unbounded (context.Background()).
func RunMediaMeta(ctx context.Context, req Request) (res Result, work *MediaThumbWork) {
	raw, err := ffprobeJSON(ctx, req.Path)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "executable file not found") || strings.Contains(msg, "ffprobe") {
			return Result{ErrorMsg: "ffprobe not found (install ffmpeg)"}, nil
		}
		return Result{ErrorMsg: msg}, nil
	}
	var doc ffprobeDoc
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return Result{ErrorMsg: "ffprobe: " + err.Error()}, nil
	}

	meta := formatMediaMeta(doc)
	res = Result{
		Source:       previewpanel.SourceExternalANSI,
		CombinedText: meta,
	}

	vStream := primaryVideoStream(doc)
	if vStream == nil {
		return res, nil
	}
	if req.ImageProtocol == previewpanel.ImageProtocolNone ||
		req.ImageMaxPxW < 1 || req.ImageMaxPxH < 1 {
		return res, nil
	}

	duration := parseDurationSec(doc.Format.Duration)
	if duration <= 0 {
		duration = parseDurationSec(vStream.Duration)
	}
	if duration <= 0 {
		return res, nil
	}

	cellH := req.ImageCellPxH
	if cellH < 1 {
		cellH = 20
	}
	metaRows := strings.Count(meta, "\n") + 1 + 2 // lines + blank + generating/status line
	thumbMaxH := req.ImageMaxPxH - metaRows*cellH
	if thumbMaxH < cellH {
		return res, nil
	}

	if !req.Preview.VideoMetadata {
		// There will be thumbnail work: nothing to show above the grid while it generates, so
		// blank both the result text and the work's stashed meta — RunMediaThumbs' pending/
		// success text then stays empty (grid only) too.
		meta = ""
		res.CombinedText = ""
	}
	return res, &MediaThumbWork{meta: meta, duration: duration}
}

// RunMediaThumbs extracts and encodes the thumbnail grid after RunMediaMeta reported work.
// onProgress, if non-nil, is called after each thumbnail frame is extracted.
func RunMediaThumbs(ctx context.Context, req Request, work *MediaThumbWork, onProgress func(done, total int)) Result {
	metaText := ""
	duration := 0.0
	if work != nil {
		metaText = work.meta
		duration = work.duration
	}
	metaResult := Result{
		Source:       previewpanel.SourceExternalANSI,
		CombinedText: metaText,
	}
	if work == nil || duration <= 0 {
		return metaResult
	}

	cols := req.Preview.VideoThumbCols
	rows := req.Preview.VideoThumbRows
	if cols < 1 {
		cols = 2
	}
	if rows < 1 {
		rows = 2
	}
	workers := req.Preview.VideoThumbWorkers
	if workers < 1 {
		workers = 2
	}
	cellH := req.ImageCellPxH
	if cellH < 1 {
		cellH = 20
	}
	metaRows := 0
	if metaText != "" {
		metaRows = strings.Count(metaText, "\n") + 1 + 1 // lines + blank separator above image
	}
	thumbMaxH := req.ImageMaxPxH - metaRows*cellH
	if thumbMaxH < cellH {
		return metaResult
	}

	maxEdge := EffectiveVideoThumbMaxEdge(req.Preview, req.ImageProtocol, req.ImageInTmux)
	fi, err := os.Stat(req.Path)
	if err != nil {
		metaResult.CombinedText = JoinMediaText(metaText, "(thumbnails failed)")
		return metaResult
	}
	load := func(c context.Context, notify func(done, total int)) ([]byte, error) {
		return BuildVideoThumbMaxEdgePNG(c, req.Path, duration, cols, rows, maxEdge, workers, notify)
	}
	var pngBytes []byte
	if req.Cache != nil {
		pngBytes, err = req.Cache.LoadVideo(ctx, req.Path, fi.ModTime().UnixNano(), fi.Size(), maxEdge, cols, rows, onProgress, load)
	} else {
		pngBytes, err = load(ctx, onProgress)
	}
	if err != nil {
		if ctx != nil && ctx.Err() != nil {
			return Result{ErrorMsg: "Canceled"}
		}
		if strings.Contains(err.Error(), "executable file not found") {
			metaResult.CombinedText = JoinMediaText(metaText, "(ffmpeg not found; thumbnails skipped)")
		} else {
			metaResult.CombinedText = JoinMediaText(metaText, "(thumbnails failed)")
		}
		return metaResult
	}

	payload, pxW, pxH, err := EncodeRenderPayload(pngBytes, req.ImageMaxPxW, thumbMaxH, req.ImageProtocol, req.ImageUnicodePlaceholder, req.ImageInTmux)
	if err != nil {
		if err == errRenderTmuxTooLarge {
			metaResult.CombinedText = JoinMediaText(metaText, "too large for tmux")
		} else {
			metaResult.CombinedText = JoinMediaText(metaText, "(thumbnails failed)")
		}
		return metaResult
	}

	return Result{
		Source:                   previewpanel.SourceExternalANSI,
		CombinedText:             metaText,
		ImagePayload:             string(payload),
		ImagePxW:                 pxW,
		ImagePxH:                 pxH,
		ImageProtocol:            req.ImageProtocol,
		ImageUnicodePlaceholder:  req.ImageUnicodePlaceholder,
		ImageInTmux:              req.ImageInTmux,
		ImageCapabilityUncertain: req.ImageCapabilityUncertain,
		ImageFirst:               true,
	}
}

func runMedia(ctx context.Context, req Request) Result {
	meta, work := RunMediaMeta(ctx, req)
	if meta.ErrorMsg != "" || work == nil {
		return meta
	}
	return RunMediaThumbs(ctx, req, work, nil)
}

func primaryVideoStream(doc ffprobeDoc) *ffprobeStream {
	var fallback *ffprobeStream
	for i := range doc.Streams {
		s := &doc.Streams[i]
		if s.CodecType != "video" || s.Width <= 0 {
			continue
		}
		if s.Disposition != nil && s.Disposition.AttachedPic != 0 {
			if fallback == nil {
				fallback = s
			}
			continue
		}
		return s
	}
	return fallback
}

func formatMediaMeta(doc ffprobeDoc) string {
	var b strings.Builder
	fiSize := parseInt64(doc.Format.Size)
	if fiSize == 0 {
		if st, err := os.Stat(doc.Format.Filename); err == nil {
			fiSize = st.Size()
		}
	}
	dur := parseDurationSec(doc.Format.Duration)
	fmt.Fprintf(&b, "Media / %s", formatImageBytes(fiSize))
	if dur > 0 {
		fmt.Fprintf(&b, " / %s", formatClockDuration(dur))
	}
	if br := parseInt64(doc.Format.BitRate); br > 0 {
		fmt.Fprintf(&b, " / %s", formatBitRate(br))
	}
	b.WriteByte('\n')

	for _, s := range doc.Streams {
		if s.CodecType != "video" || s.Width <= 0 {
			continue
		}
		if s.Disposition != nil && s.Disposition.AttachedPic != 0 {
			continue
		}
		fmt.Fprintf(&b, "Video: %s", nonEmpty(s.CodecName, "unknown"))
		fmt.Fprintf(&b, " / %d×%d", s.Width, s.Height)
		if fps := parseFrameRate(s.RFrameRate); fps > 0 {
			fmt.Fprintf(&b, " / %.2f fps", fps)
		}
		if s.PixFmt != "" {
			fmt.Fprintf(&b, " / %s", s.PixFmt)
		}
		if s.DisplayAspectRatio != "" && s.DisplayAspectRatio != "0:1" {
			fmt.Fprintf(&b, " / DAR %s", s.DisplayAspectRatio)
		}
		b.WriteByte('\n')
	}

	if subs := formatSubtitleLangs(doc); subs != "" {
		fmt.Fprintf(&b, "Subtitles: %s\n", subs)
	}
	if ext := externalSubs(doc.Format.Filename); ext != "" {
		fmt.Fprintf(&b, "Subtitles (ext): %s\n", ext)
	}

	audioHeader := false
	for _, s := range doc.Streams {
		if s.CodecType != "audio" {
			continue
		}
		if !audioHeader {
			b.WriteString("Audio:\n")
			audioHeader = true
		}
		fmt.Fprintf(&b, "- %s / %s", langCode(s.Tags.Language), nonEmpty(s.CodecName, "unknown"))
		if s.Channels > 0 {
			fmt.Fprintf(&b, " / %d ch", s.Channels)
		}
		if br := parseInt64(s.BitRate); br > 0 {
			fmt.Fprintf(&b, " / %s", formatBitRate(br))
		}
		if title := strings.TrimSpace(s.Tags.Title); title != "" {
			fmt.Fprintf(&b, " — %s", title)
		}
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

// formatSubtitleLangs lists subtitle track languages in first-seen order, collapsing
// repeats into "EN×2". Empty when the file carries no subtitle tracks.
func formatSubtitleLangs(doc ffprobeDoc) string {
	var order []string
	counts := map[string]int{}
	for _, s := range doc.Streams {
		if s.CodecType != "subtitle" {
			continue
		}
		code := langCode(s.Tags.Language)
		if counts[code] == 0 {
			order = append(order, code)
		}
		counts[code]++
	}
	parts := make([]string, 0, len(order))
	for _, code := range order {
		if n := counts[code]; n > 1 {
			code = fmt.Sprintf("%s×%d", code, n)
		}
		parts = append(parts, code)
	}
	return strings.Join(parts, ", ")
}

// langCode maps an ffprobe language tag to an uppercase two-letter code ("dut" \u2192 "NL",
// "en-US" \u2192 "EN"). Untagged or undetermined tracks report "UND"; anything x/text cannot
// resolve is uppercased as-is.
func langCode(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	// language.Parse("und") resolves to base "en" at low confidence, so guard it first.
	if raw == "" || raw == "und" {
		return "UND"
	}
	tag, err := language.Parse(raw)
	if err != nil {
		return strings.ToUpper(raw)
	}
	base, conf := tag.Base()
	if conf == language.No {
		return strings.ToUpper(raw)
	}
	return strings.ToUpper(base.String())
}

// subtitleExts are the sidecar subtitle formats reported next to a video file.
var subtitleExts = map[string]bool{
	".srt": true, ".ass": true, ".ssa": true, ".vtt": true, ".sub": true,
	".sup": true, ".smi": true, ".ttml": true, ".dfxp": true,
}

// subtitleJunkTokens are filename tokens that look like a language code but never are
// (".forced", ".sdh", ".sub"). "und" is here too: language.Parse resolves it to English.
var subtitleJunkTokens = map[string]bool{
	"forced": true, "sdh": true, "cc": true, "sub": true, "subs": true,
	"subtitle": true, "subtitles": true, "default": true, "full": true, "und": true,
}

// subKey is one external-subtitle bucket: a language code and a file format.
type subKey struct{ lang, ext string }

// externalSubs lists sidecar subtitle files for videoPath — beside it, and in a
// Subs/ or Subtitles/ subdirectory — as `EN (srt), FI×2 (ass)`. Empty when none.
func externalSubs(videoPath string) string {
	if videoPath == "" {
		return ""
	}
	dir := filepath.Dir(videoPath)
	stem := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))

	var order []subKey
	counts := map[subKey]int{}
	scan := func(d string, descend bool) []string {
		var subDirs []string
		entries, err := os.ReadDir(d)
		if err != nil {
			return nil
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() {
				if lower := strings.ToLower(name); descend && (lower == "subs" || lower == "subtitles") {
					subDirs = append(subDirs, filepath.Join(d, name))
				}
				continue
			}
			ext := strings.ToLower(filepath.Ext(name))
			if !subtitleExts[ext] {
				continue
			}
			k := subKey{lang: subtitleFileLang(name, stem), ext: strings.TrimPrefix(ext, ".")}
			if counts[k] == 0 {
				order = append(order, k)
			}
			counts[k]++
		}
		return subDirs
	}
	for _, d := range scan(dir, true) {
		scan(d, false)
	}

	parts := make([]string, 0, len(order))
	for _, k := range order {
		lang := k.lang
		if n := counts[k]; n > 1 {
			lang = fmt.Sprintf("%s×%d", lang, n)
		}
		parts = append(parts, fmt.Sprintf("%s (%s)", lang, k.ext))
	}
	return strings.Join(parts, ", ")
}

// subtitleFileLang derives a language code from a subtitle filename, e.g.
// "Movie.2015.en.forced.srt" (video stem "Movie.2015") → "EN", "Subs/2_English.srt" →
// "EN". Tokens are read right to left; "UND" when none of them names a language.
func subtitleFileLang(name, videoStem string) string {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	if videoStem != "" && len(base) > len(videoStem) && strings.EqualFold(base[:len(videoStem)], videoStem) {
		base = base[len(videoStem):]
	}
	tokens := strings.FieldsFunc(base, func(r rune) bool {
		return strings.ContainsRune("._- []()", r)
	})
	for i := len(tokens) - 1; i >= 0; i-- {
		if code, ok := subtitleLangToken(strings.ToLower(tokens[i])); ok {
			return code
		}
	}
	// Multi-word display names ("brazilian portuguese") survive tokenization only as a whole.
	if code, ok := subtitleLangNames[strings.ToLower(strings.Join(tokens, " "))]; ok {
		return code
	}
	return "UND"
}

// subtitleLangToken resolves one filename token to an uppercase language code,
// accepting both codes ("en", "eng", "dut") and English names ("english").
func subtitleLangToken(tok string) (string, bool) {
	if subtitleJunkTokens[tok] {
		return "", false
	}
	if len(tok) == 2 || len(tok) == 3 {
		if tag, err := language.Parse(tok); err == nil {
			if base, conf := tag.Base(); conf != language.No {
				return strings.ToUpper(base.String()), true
			}
		}
	}
	code, ok := subtitleLangNames[tok]
	return code, ok
}

// subtitleLangNames maps English language names as they appear in subtitle
// filenames ("Subs/2_English.srt") to their code. Codes themselves are resolved by
// x/text; this covers only the names common in subtitle releases.
var subtitleLangNames = map[string]string{
	"english": "EN", "spanish": "ES", "french": "FR", "german": "DE",
	"italian": "IT", "portuguese": "PT", "brazilian portuguese": "PT",
	"dutch": "NL", "danish": "DA", "swedish": "SV", "norwegian": "NO",
	"finnish": "FI", "icelandic": "IS", "polish": "PL", "czech": "CS",
	"slovak": "SK", "hungarian": "HU", "romanian": "RO", "bulgarian": "BG",
	"greek": "EL", "russian": "RU", "ukrainian": "UK", "turkish": "TR",
	"arabic": "AR", "hebrew": "HE", "hindi": "HI", "chinese": "ZH",
	"simplified chinese": "ZH", "traditional chinese": "ZH", "japanese": "JA",
	"korean": "KO", "thai": "TH", "vietnamese": "VI", "indonesian": "ID",
	"malay": "MS", "croatian": "HR", "serbian": "SR", "slovenian": "SL",
	"estonian": "ET", "latvian": "LV", "lithuanian": "LT", "persian": "FA",
	"farsi": "FA",
}

func nonEmpty(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

func parseDurationSec(s string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || f < 0 {
		return 0
	}
	return f
}

func parseFrameRate(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "0/0" {
		return 0
	}
	if a, b, ok := strings.Cut(s, "/"); ok {
		num, err1 := strconv.ParseFloat(a, 64)
		den, err2 := strconv.ParseFloat(b, 64)
		if err1 == nil && err2 == nil && den != 0 {
			return num / den
		}
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

func parseInt64(s string) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func formatClockDuration(sec float64) string {
	d := time.Duration(sec * float64(time.Second))
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func formatBitRate(bps int64) string {
	if bps < 1000 {
		return fmt.Sprintf("%d bps", bps)
	}
	if bps < 1_000_000 {
		return fmt.Sprintf("%.0f kbps", float64(bps)/1000)
	}
	return fmt.Sprintf("%.2f Mbps", float64(bps)/1_000_000)
}
