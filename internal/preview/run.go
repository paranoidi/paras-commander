package preview

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/gitstatus"
	"github.com/paranoidi/paras-commander/internal/preview/chromaformat"
	"github.com/paranoidi/paras-commander/internal/preview/mdformat"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

// markdownExtensions are the file extensions eligible for rendered (not raw
// Chroma-highlighted) markdown in internal preview mode. Single source of
// truth for the extension list.
var markdownExtensions = []string{".md", ".markdown"}

// IsMarkdownPath reports whether path is eligible for rendered markdown (vs. raw
// Chroma-highlighted source) in internal preview mode. Exported so callers outside
// this package (e.g. the app layer's raw/rendered toggle) share the same extension
// check instead of re-deriving it.
func IsMarkdownPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, e := range markdownExtensions {
		if ext == e {
			return true
		}
	}
	return false
}

// WillRenderMarkdown reports whether Run(req) would produce content via the rendered-markdown
// formatter (mdformat) rather than raw/diff/chroma output. Mirrors Run's dispatch, including the
// git-diff fallback to plain content for untracked/ignored/unmodified files (see runGitDiff), so
// callers can size layout (e.g. the fullscreen preview's margin) before the async run completes.
func WillRenderMarkdown(req Request) bool {
	if strings.ToLower(strings.TrimSpace(req.Preview.Mode)) == config.PreviewModeExternal {
		return false
	}
	if !IsMarkdownPath(req.Path) || req.RawMarkdown {
		return false
	}
	if req.GitDiff {
		var eff gitstatus.Status
		if req.GitStatus != nil {
			eff = req.GitStatus.Effective()
		}
		if eff != gitstatus.NotModified && eff != gitstatus.New && eff != gitstatus.Ignored {
			return false
		}
	}
	return true
}

// Request is input for a single preview run.
type Request struct {
	Path      string
	TextWidth int
	WorkDir   string
	Preview   config.PreviewConfig
	BaseStyle tcell.Style
	// GitDiff, when true, shows git diff output instead of file content.
	GitDiff   bool
	GitStatus *gitstatus.Cell // nil when not in a git repo or file is unmodified
	// RawMarkdown, when true, skips rendered markdown and shows raw Chroma-highlighted
	// source for a markdown path instead. Ignored for non-markdown paths.
	RawMarkdown bool
	// Image, when true, runs the in-process image preview path instead of text/diff/markdown.
	Image bool
	// ImageMaxPxW / ImageMaxPxH are the pixel budget for fit-scaling (cells × cell pixel size).
	ImageMaxPxW int
	ImageMaxPxH int
	// ImageProtocol selects sixel vs Kitty encoding (resolved by the caller).
	ImageProtocol previewpanel.ImageProtocol
}

// Result is the unified preview output for internal and external backends.
type Result struct {
	Source           previewpanel.Source
	CombinedText     string
	HighlightedCells []previewpanel.AnsiCell
	GutterWidth      int
	ExitCode         int
	ErrorMsg         string
	Truncated        bool
	// IsDiff is true when this result came from a git diff run.
	IsDiff bool
	// IsMarkdown is true when content was produced by the rendered-markdown formatter
	// (mdformat) rather than raw/diff/chroma text. Drives the fullscreen preview's
	// left/right margin (see previewpanel.State.IsMarkdown).
	IsMarkdown bool
	// DiffHunkLines holds 0-based source line numbers where each contiguous
	// +/- change run begins (first added/removed line of the chunk). Used by
	// Ctrl+Alt+J/K. Not @@ headers — with large -U context there is usually
	// only one @@ for the whole file.
	DiffHunkLines []int
	// StatusText is a short git-status label to show in the preview title (e.g. "no changes", "ignored").
	// Set when a diff was requested but there was nothing to diff, so plain content is shown instead.
	StatusText string
	// StatusThemeKey is the panel.git.* theme key used to color StatusText.
	StatusThemeKey string
	// ImagePayload is a raw sixel DCS or Kitty APC sequence for image previews (empty for text).
	ImagePayload string
	// ImagePxW / ImagePxH are the encoded image dimensions in pixels.
	ImagePxW int
	ImagePxH int
	// ImageProtocol identifies which graphics protocol ImagePayload uses.
	ImageProtocol previewpanel.ImageProtocol
}

// Run executes internal Chroma highlighting or an external preview command.
func Run(ctx context.Context, req Request) Result {
	if req.Image {
		return runImage(req)
	}
	if req.GitDiff {
		return runGitDiff(ctx, req)
	}
	mode := strings.ToLower(strings.TrimSpace(req.Preview.Mode))
	switch mode {
	case config.PreviewModeExternal:
		return runExternal(ctx, req)
	default:
		if IsMarkdownPath(req.Path) && !req.RawMarkdown {
			return runMarkdown(req)
		}
		return runInternal(req)
	}
}

func runInternal(req Request) Result {
	data, truncated, err := readFileLimited(req.Path, config.DefaultMaxPreviewBytes)
	if err != nil {
		return Result{ErrorMsg: err.Error()}
	}
	source := string(data)
	textW := req.TextWidth
	if textW < 1 {
		textW = 1
	}
	lineCount := strings.Count(source, "\n") + 1
	if source == "" {
		lineCount = 1
	}
	gutterReserve := 0
	if req.Preview.LineNumbers {
		digits := len(fmt.Sprintf("%d", lineCount))
		gutterReserve = digits + 1
	}
	contentW := textW - gutterReserve
	if contentW < 1 {
		contentW = 1
	}
	hl := chromaformat.Highlight(source, chromaformat.Options{
		Path:         req.Path,
		StyleName:    req.Preview.Style,
		BaseStyle:    req.BaseStyle,
		LineNumbers:  req.Preview.LineNumbers,
		TabWidth:     config.DefaultPreviewTabWidth,
		ContentWidth: contentW,
	})
	cells := hl.Cells
	if truncated {
		cells = append(cells, previewpanel.AnsiCell{R: '\n', St: req.BaseStyle})
		for _, r := range "\n[output truncated]\n" {
			cells = append(cells, previewpanel.AnsiCell{R: r, St: req.BaseStyle})
		}
	}
	return Result{
		Source:           previewpanel.SourceInternalHighlighted,
		HighlightedCells: cells,
		GutterWidth:      hl.GutterWidth,
		Truncated:        truncated,
	}
}

// runMarkdown renders block-level markdown (headings, emphasis, lists, fenced
// code, ...) instead of raw Chroma token coloring. Never reached for a git-dirty
// file: Run only falls through to this default branch once req.GitDiff is false.
func runMarkdown(req Request) Result {
	data, truncated, err := readFileLimited(req.Path, config.DefaultMaxPreviewBytes)
	if err != nil {
		return Result{ErrorMsg: err.Error()}
	}
	contentW := max(1, req.TextWidth)
	md := mdformat.Render(string(data), mdformat.Options{
		Path:         req.Path,
		StyleName:    req.Preview.Style,
		BaseStyle:    req.BaseStyle,
		ContentWidth: contentW,
	})
	cells := md.Cells
	if truncated {
		cells = append(cells, previewpanel.AnsiCell{R: '\n', St: req.BaseStyle})
		for _, r := range "\n[output truncated]\n" {
			cells = append(cells, previewpanel.AnsiCell{R: r, St: req.BaseStyle})
		}
	}
	return Result{
		Source:           previewpanel.SourceInternalHighlighted,
		HighlightedCells: cells,
		Truncated:        truncated,
		IsMarkdown:       true,
	}
}

func runExternal(ctx context.Context, req Request) Result {
	argv, err := cmdrun.BuildFilePreviewArgv(req.Preview.Command, req.Path, req.TextWidth)
	if err != nil {
		return Result{ErrorMsg: err.Error()}
	}
	res := cmdrun.Run(ctx, argv, req.WorkDir, cmdrun.MaxStreamBytes)
	if res.LaunchErr != nil {
		return Result{ErrorMsg: res.LaunchErr.Error(), ExitCode: -1}
	}
	var b strings.Builder
	b.WriteString(string(res.Stdout))
	if len(res.Stderr) > 0 {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("--- stderr ---\n")
		b.WriteString(string(res.Stderr))
	}
	combined := b.String()
	truncated := res.StdoutTrim || res.StderrTrim
	if truncated {
		combined += "\n\n[output truncated]\n"
	}
	return Result{
		Source:       previewpanel.SourceExternalANSI,
		CombinedText: combined,
		ExitCode:     res.ExitCode,
		Truncated:    truncated,
	}
}

// gitDiffFallback shows plain file content with a git-status label when there is
// nothing meaningful to diff (untracked, ignored, unmodified, or an empty diff).
func gitDiffFallback(ctx context.Context, req Request, status gitstatus.Status) Result {
	req.GitDiff = false
	res := Run(ctx, req)
	res.StatusText = status.Label()
	res.StatusThemeKey = status.ThemeKey()
	return res
}

func runGitDiff(ctx context.Context, req Request) Result {
	var eff gitstatus.Status
	if req.GitStatus != nil {
		eff = req.GitStatus.Effective()
	}
	// Nothing to diff: unmodified, untracked, or ignored — show plain content instead.
	if eff == gitstatus.NotModified || eff == gitstatus.New || eff == gitstatus.Ignored {
		return gitDiffFallback(ctx, req, eff)
	}

	// Choose diff args: staged-only changes use --cached; otherwise compare to HEAD.
	// Large -U keeps unchanged lines so the preview is the whole file with +/- markers
	// (git's default ~3-line context would show only local hunks).
	diffArgs := []string{fmt.Sprintf("-U%d", config.DefaultPreviewGitDiffContextLines)}
	if req.GitStatus != nil &&
		req.GitStatus.Staged != gitstatus.NotModified &&
		req.GitStatus.Unstaged == gitstatus.NotModified {
		diffArgs = append(diffArgs, "--cached")
	} else {
		diffArgs = append(diffArgs, "HEAD")
	}

	mode := strings.ToLower(strings.TrimSpace(req.Preview.Mode))
	if mode == config.PreviewModeExternal {
		argv := append([]string{"git", "diff", "--color=always"}, append(diffArgs, "--", req.Path)...)
		res := cmdrun.Run(ctx, argv, req.WorkDir, cmdrun.MaxStreamBytes)
		if res.LaunchErr != nil {
			return Result{IsDiff: true, ErrorMsg: res.LaunchErr.Error(), ExitCode: -1}
		}
		combined := strings.TrimRight(string(res.Stdout), "\n")
		if len(res.Stderr) > 0 {
			if combined != "" {
				combined += "\n"
			}
			combined += "--- stderr ---\n" + string(res.Stderr)
		}
		if strings.TrimSpace(combined) == "" {
			return gitDiffFallback(ctx, req, eff)
		}
		return Result{
			IsDiff:        true,
			Source:        previewpanel.SourceExternalANSI,
			CombinedText:  combined,
			ExitCode:      res.ExitCode,
			DiffHunkLines: parseDiffChangeChunkLines(stripANSI(string(res.Stdout))),
		}
	}

	// Internal mode: run without color and highlight with Chroma diff lexer.
	argv := append([]string{"git", "diff", "--no-color"}, append(diffArgs, "--", req.Path)...)
	res := cmdrun.Run(ctx, argv, req.WorkDir, cmdrun.MaxStreamBytes)
	if res.LaunchErr != nil {
		return Result{IsDiff: true, ErrorMsg: res.LaunchErr.Error(), ExitCode: -1}
	}
	rawDiff := strings.TrimRight(string(res.Stdout), "\n")
	if strings.TrimSpace(rawDiff) == "" {
		return gitDiffFallback(ctx, req, eff)
	}

	hunkLines := parseDiffChangeChunkLines(rawDiff)

	textW := max(1, req.TextWidth)
	lineCount := strings.Count(rawDiff, "\n") + 1
	gutterReserve := 0
	if req.Preview.LineNumbers {
		digits := len(fmt.Sprintf("%d", lineCount))
		gutterReserve = digits + 1
	}
	contentW := textW - gutterReserve
	if contentW < 1 {
		contentW = 1
	}
	hl := chromaformat.Highlight(rawDiff, chromaformat.Options{
		Path:         "file.diff",
		StyleName:    req.Preview.Style,
		BaseStyle:    req.BaseStyle,
		LineNumbers:  req.Preview.LineNumbers,
		TabWidth:     config.DefaultPreviewTabWidth,
		ContentWidth: contentW,
	})
	return Result{
		IsDiff:           true,
		Source:           previewpanel.SourceInternalHighlighted,
		HighlightedCells: hl.Cells,
		GutterWidth:      hl.GutterWidth,
		DiffHunkLines:    hunkLines,
	}
}

// parseDiffChangeChunkLines returns 0-based source line numbers where each
// contiguous run of unified-diff change lines (+/-) begins. File headers
// (---/+++) and @@ markers are ignored. Groups of consecutive +/- lines
// (e.g. a deletion followed immediately by an addition) count as one chunk.
func parseDiffChangeChunkLines(diff string) []int {
	var lines []int
	inChunk := false
	for i, line := range strings.Split(diff, "\n") {
		if isUnifiedDiffChangeLine(line) {
			if !inChunk {
				lines = append(lines, i)
				inChunk = true
			}
			continue
		}
		inChunk = false
	}
	return lines
}

// isUnifiedDiffChangeLine reports a body line that adds or removes content.
// Excludes ---/+++ file headers (git writes a space after the marker) so a
// deleted/added line whose content starts with -- / ++ is still counted.
func isUnifiedDiffChangeLine(line string) bool {
	if line == "" {
		return false
	}
	switch line[0] {
	case '+':
		return !strings.HasPrefix(line, "+++ ")
	case '-':
		return !strings.HasPrefix(line, "--- ")
	default:
		return false
	}
}

// stripANSI removes CSI SGR sequences (ESC [ … m) so external colored diff
// text can be scanned for +/- line prefixes.
func stripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && ((s[j] >= '0' && s[j] <= '9') || s[j] == ';') {
				j++
			}
			if j < len(s) && s[j] == 'm' {
				i = j + 1
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func readFileLimited(path string, maxBytes int) ([]byte, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = f.Close() }()
	lim := maxBytes
	if lim < 1 {
		lim = config.DefaultMaxPreviewBytes
	}
	buf := make([]byte, lim+1)
	n, err := io.ReadFull(f, buf)
	if err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return buf[:n], false, nil
		}
		return nil, false, err
	}
	extra := make([]byte, 1)
	more, readErr := f.Read(extra)
	truncated := more > 0
	if readErr != nil && readErr != io.EOF {
		return nil, truncated, readErr
	}
	return buf[:n], truncated, nil
}
