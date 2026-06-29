package preview

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/gitstatus"
	"github.com/paranoidi/paras-commander/internal/preview/chromaformat"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

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
	// DiffHunkLines holds 0-based source line numbers of @@ hunk markers.
	DiffHunkLines []int
}

// Run executes internal Chroma highlighting or an external preview command.
func Run(ctx context.Context, req Request) Result {
	if req.GitDiff {
		return runGitDiff(ctx, req)
	}
	mode := strings.ToLower(strings.TrimSpace(req.Preview.Mode))
	switch mode {
	case config.PreviewModeExternal:
		return runExternal(ctx, req)
	default:
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

func runGitDiff(ctx context.Context, req Request) Result {
	// Untracked new file: no diff available, fall back to content preview.
	if req.GitStatus != nil &&
		req.GitStatus.Staged == gitstatus.NotModified &&
		req.GitStatus.Unstaged == gitstatus.New {
		req.GitDiff = false
		return Run(ctx, req)
	}

	// Choose diff args: staged-only changes use --cached; otherwise compare to HEAD.
	var diffArgs []string
	if req.GitStatus != nil &&
		req.GitStatus.Staged != gitstatus.NotModified &&
		req.GitStatus.Unstaged == gitstatus.NotModified {
		diffArgs = []string{"--cached"}
	} else {
		diffArgs = []string{"HEAD"}
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
			return Result{IsDiff: true, ErrorMsg: "No changes"}
		}
		return Result{
			IsDiff:       true,
			Source:       previewpanel.SourceExternalANSI,
			CombinedText: combined,
			ExitCode:     res.ExitCode,
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
		return Result{IsDiff: true, ErrorMsg: "No changes"}
	}

	hunkLines := parseDiffHunkLines(rawDiff)

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

func parseDiffHunkLines(diff string) []int {
	var lines []int
	for i, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "@@") {
			lines = append(lines, i)
		}
	}
	return lines
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
