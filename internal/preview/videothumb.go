package preview

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"sync"

	xdraw "golang.org/x/image/draw"

	"github.com/paranoidi/paras-commander/internal/workpool"
)

// Thumbnail timing/grid logic adapted from github.com/kmou424/go-video-thumb (MIT).
// Header banner, fonts, and per-tile timestamps are omitted; metadata is text below the grid.

// calculateTimeMarks returns n evenly spaced seek points in (0, durationSec),
// using duration/(n+1) spacing (same idea as go-video-thumb). Marks are clamped
// below duration so short videos still yield frames.
func calculateTimeMarks(durationSec float64, n int) []float64 {
	if n < 1 || durationSec <= 0 {
		return nil
	}
	marks := make([]float64, n)
	interval := durationSec / float64(n+1)
	maxT := durationSec * 0.99
	if maxT < 0 {
		maxT = 0
	}
	for i := 0; i < n; i++ {
		t := interval * float64(i+1)
		if t > maxT {
			t = maxT
		}
		marks[i] = t
	}
	return marks
}

func extractFramePNG(videoPath string, timeSec float64) (image.Image, error) {
	pngBytes, err := ffmpegFramePNG(videoPath, timeSec)
	if err != nil {
		return nil, err
	}
	if len(pngBytes) == 0 {
		return nil, fmt.Errorf("ffmpeg produced no frame at %.2fs", timeSec)
	}
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		return nil, err
	}
	return img, nil
}

// extractThumbFrames extracts cols*rows frames, running up to workers ffmpeg processes
// concurrently. onFrame, if non-nil, is called once per completed frame with a monotonically
// increasing done count, serialized so calls always land in strict 1..n order — even though
// frames land in the returned slice at their own timestamp index regardless of completion order.
func extractThumbFrames(ctx context.Context, videoPath string, durationSec float64, cols, rows, workers int, onFrame func(done int)) ([]image.Image, error) {
	n := cols * rows
	marks := calculateTimeMarks(durationSec, n)
	if len(marks) == 0 {
		return nil, fmt.Errorf("no frame timestamps")
	}
	runCtx := ctx
	if runCtx == nil {
		runCtx = context.Background()
	}
	runCtx, cancel := context.WithCancel(runCtx)
	defer cancel()

	pool := workpool.New(workers)
	frames := make([]image.Image, len(marks))
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		done     int
		firstErr error
	)
	for i, t := range marks {
		if err := pool.Acquire(runCtx); err != nil {
			break
		}
		wg.Add(1)
		go func(i int, t float64) {
			defer wg.Done()
			img, err := extractFramePNG(videoPath, t)
			pool.Release() // free the slot immediately — bookkeeping/callback below don't need it
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
					cancel()
				}
				mu.Unlock()
				return
			}
			frames[i] = img // disjoint index per goroutine, safe without mu
			mu.Lock()
			done++
			if onFrame != nil {
				onFrame(done)
			}
			mu.Unlock()
		}(i, t)
	}
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}
	if ctx != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return frames, nil
}

// composeThumbGrid tiles frames into a cols×rows grid fitting maxW×maxH.
// The grid is sized to the frames' aspect ratio (no black letterbox margins);
// tiles are scaled to fill each cell exactly.
func composeThumbGrid(frames []image.Image, cols, rows, maxW, maxH int) image.Image {
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	n := cols * rows
	if len(frames) > n {
		frames = frames[:n]
	}
	if len(frames) == 0 || maxW < 1 || maxH < 1 {
		return image.NewRGBA(image.Rect(0, 0, 1, 1))
	}
	fw := frames[0].Bounds().Dx()
	fh := frames[0].Bounds().Dy()
	if fw < 1 {
		fw = 1
	}
	if fh < 1 {
		fh = 1
	}
	// Pack cols×rows of fw×fh into maxW×maxH, preserving frame aspect.
	scale := float64(maxW) / float64(fw*cols)
	if s := float64(maxH) / float64(fh*rows); s < scale {
		scale = s
	}
	cellW := int(float64(fw) * scale)
	cellH := int(float64(fh) * scale)
	if cellW < 1 {
		cellW = 1
	}
	if cellH < 1 {
		cellH = 1
	}
	gridW := cellW * cols
	gridH := cellH * rows
	dst := image.NewRGBA(image.Rect(0, 0, gridW, gridH))

	for i, fr := range frames {
		row := i / cols
		col := i % cols
		scaled := scaleImageExact(fr, cellW, cellH)
		ox := col * cellW
		oy := row * cellH
		r := image.Rect(ox, oy, ox+cellW, oy+cellH)
		xdraw.Draw(dst, r, scaled, scaled.Bounds().Min, xdraw.Src)
	}
	return dst
}

// scaleImageExact scales src to exactly w×h (may upscale).
func scaleImageExact(src image.Image, w, h int) image.Image {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	b := src.Bounds()
	if b.Dx() == w && b.Dy() == h {
		return src
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, b, xdraw.Src, nil)
	return dst
}

// buildVideoThumbGrid extracts and composites a thumbnail grid for path. The reported total
// (via onFrame) is cols*rows+1: one step per extracted frame plus one final step for compositing
// them into the grid, which is itself slow enough (scaling every tile) to warrant its own step
// rather than leaving progress looking stalled at n/n while it runs.
func buildVideoThumbGrid(ctx context.Context, path string, durationSec float64, cols, rows, maxW, maxH, workers int, onFrame func(done, total int)) (image.Image, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	totalSteps := cols*rows + 1
	frameProgress := func(done int) {
		if onFrame != nil {
			onFrame(done, totalSteps)
		}
	}
	frames, err := extractThumbFrames(ctx, path, durationSec, cols, rows, workers, frameProgress)
	if err != nil {
		return nil, err
	}
	grid := composeThumbGrid(frames, cols, rows, maxW, maxH)
	if onFrame != nil {
		onFrame(totalSteps, totalSteps)
	}
	return grid, nil
}
