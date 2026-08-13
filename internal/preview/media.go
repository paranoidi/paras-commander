package preview

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

// GeneratingThumbnailsLine is appended under media metadata while ffmpeg extracts frames.
const GeneratingThumbnailsLine = "Generating thumbnails…"

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
// the caller should show GeneratingThumbnailsLine under the meta, then call RunMediaThumbs.
func RunMediaMeta(req Request) (res Result, work *MediaThumbWork) {
	raw, err := ffprobeJSON(req.Path)
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
	return res, &MediaThumbWork{meta: meta, duration: duration}
}

// RunMediaThumbs extracts and encodes the thumbnail grid after RunMediaMeta reported work.
func RunMediaThumbs(ctx context.Context, req Request, work *MediaThumbWork) Result {
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
	cellH := req.ImageCellPxH
	if cellH < 1 {
		cellH = 20
	}
	metaRows := strings.Count(metaText, "\n") + 1 + 1 // lines + blank separator above image
	thumbMaxH := req.ImageMaxPxH - metaRows*cellH
	if thumbMaxH < cellH {
		return metaResult
	}

	maxEdge := ImageMaxEdge(req.Preview)
	fi, err := os.Stat(req.Path)
	if err != nil {
		metaResult.CombinedText = metaText + "\n(thumbnails failed)"
		return metaResult
	}
	load := func(c context.Context) ([]byte, error) {
		return BuildVideoThumbMaxEdgePNG(c, req.Path, duration, cols, rows, maxEdge)
	}
	var pngBytes []byte
	if req.Cache != nil {
		pngBytes, err = req.Cache.LoadVideo(ctx, req.Path, fi.ModTime().UnixNano(), fi.Size(), maxEdge, cols, rows, load)
	} else {
		pngBytes, err = load(ctx)
	}
	if err != nil {
		if ctx != nil && ctx.Err() != nil {
			return Result{ErrorMsg: "Canceled"}
		}
		if strings.Contains(err.Error(), "executable file not found") {
			metaResult.CombinedText = metaText + "\n(ffmpeg not found; thumbnails skipped)"
		} else {
			metaResult.CombinedText = metaText + "\n(thumbnails failed)"
		}
		return metaResult
	}

	grid, err := DecodePNGBytes(pngBytes)
	if err != nil {
		metaResult.CombinedText = metaText + "\n(thumbnails failed)"
		return metaResult
	}
	grid = fitImage(grid, req.ImageMaxPxW, thumbMaxH)

	bounds := grid.Bounds()
	payload, err := encodeImagePayload(grid, req.ImageProtocol, req.ImageUnicodePlaceholder, req.ImageInTmux)
	if err != nil || payload == "" {
		return metaResult
	}
	if req.ImageProtocol == previewpanel.ImageProtocolSixel && req.ImageInTmux &&
		len(payload) >= config.DefaultPreviewTmuxSixelMaxBytes {
		return metaResult
	}

	return Result{
		Source:                  previewpanel.SourceExternalANSI,
		CombinedText:            metaText,
		ImagePayload:            payload,
		ImagePxW:                bounds.Dx(),
		ImagePxH:                bounds.Dy(),
		ImageProtocol:           req.ImageProtocol,
		ImageUnicodePlaceholder: req.ImageUnicodePlaceholder,
		ImageInTmux:             req.ImageInTmux,
	}
}

func runMedia(ctx context.Context, req Request) Result {
	meta, work := RunMediaMeta(req)
	if meta.ErrorMsg != "" || work == nil {
		return meta
	}
	return RunMediaThumbs(ctx, req, work)
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
		switch s.CodecType {
		case "video":
			if s.Width <= 0 {
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
		case "audio":
			fmt.Fprintf(&b, "Audio: %s", nonEmpty(s.CodecName, "unknown"))
			if s.SampleRate != "" {
				fmt.Fprintf(&b, " / %s Hz", s.SampleRate)
			}
			if s.Channels > 0 {
				fmt.Fprintf(&b, " / %d ch", s.Channels)
			}
			if br := parseInt64(s.BitRate); br > 0 {
				fmt.Fprintf(&b, " / %s", formatBitRate(br))
			}
			b.WriteByte('\n')
		}
	}
	return strings.TrimRight(b.String(), "\n")
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
