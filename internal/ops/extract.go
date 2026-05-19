package ops

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/archive"
	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/localfs"
)

// ExtractItem is one archive scheduled for extraction.
type ExtractItem struct {
	Path   string
	Format archive.Format
}

// ExtractPlan describes a validated extract operation.
type ExtractPlan struct {
	Items       []ExtractItem
	Destination string
	Toolchain   archive.Toolchain
}

// PlanExtract builds an extract plan from source paths and destination directory.
// Skips non-files and unknown formats; returns error when no runnable archives remain.
func PlanExtract(paths []string, destDir string, tc archive.Toolchain) (ExtractPlan, []string, error) {
	destDir = filepath.Clean(destDir)
	if destDir == "" {
		return ExtractPlan{}, nil, &Error{Op: "extract", Text: "destination is empty"}
	}
	info, err := os.Stat(destDir)
	if err != nil {
		return ExtractPlan{}, nil, &Error{Op: "extract", Text: fmt.Sprintf("destination %q: %v", destDir, err), Err: err}
	}
	if !info.IsDir() {
		return ExtractPlan{}, nil, &Error{Op: "extract", Text: "destination is not a directory"}
	}

	var skipped []string
	var items []ExtractItem
	var unavailable []string

	for _, p := range paths {
		p = filepath.Clean(p)
		fi, err := os.Stat(p)
		if err != nil {
			skipped = append(skipped, fmt.Sprintf("%s: %v", filepath.Base(p), err))
			continue
		}
		if !fi.Mode().IsRegular() {
			skipped = append(skipped, filepath.Base(p)+": not a regular file")
			continue
		}
		f, ok := archive.FormatForName(p)
		if !ok {
			skipped = append(skipped, filepath.Base(p)+": unsupported archive type")
			continue
		}
		if !f.Available(tc) {
			unavailable = append(unavailable, fmt.Sprintf("%s: %s not found", filepath.Base(p), f.RequiredToolName()))
			continue
		}
		items = append(items, ExtractItem{Path: p, Format: f})
	}

	if len(items) == 0 {
		msg := "no supported archives to extract"
		if len(unavailable) > 0 {
			msg = strings.Join(unavailable, "; ")
		} else if len(skipped) > 0 {
			msg = "no supported archives selected"
		}
		return ExtractPlan{}, append(skipped, unavailable...), &Error{Op: "extract", Text: msg}
	}

	return ExtractPlan{
		Items:       items,
		Destination: destDir,
		Toolchain:   tc,
	}, append(skipped, unavailable...), nil
}

// ExtractProgress is called after each archive completes (success or failure).
type ExtractProgress func(archivePath string, doneFiles int)

// ExecuteExtract runs extraction for each item in plan.
// progress is called after each archive attempt. Returns cumulative done count and
// an error if any archive failed (remaining archives still run).
func ExecuteExtract(ctx context.Context, plan ExtractPlan, progress ExtractProgress) (int, error) {
	var done int
	var firstErr error
	var failCount int

	for _, item := range plan.Items {
		if err := ctx.Err(); err != nil {
			return done, err
		}
		err := extractOne(ctx, item, plan.Destination, plan.Toolchain)
		if err != nil {
			failCount++
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %w", filepath.Base(item.Path), err)
			}
		} else {
			done++
		}
		if progress != nil {
			progress(item.Path, done)
		}
	}
	if failCount > 0 {
		if failCount > 1 && firstErr != nil {
			return done, fmt.Errorf("%w (%d archives failed)", firstErr, failCount)
		}
		return done, firstErr
	}
	return done, nil
}

func extractOne(ctx context.Context, item ExtractItem, destDir string, tc archive.Toolchain) error {
	argv, err := archive.BuildArgv(item.Format, item.Path, destDir, tc)
	if err != nil {
		return err
	}
	if item.Format.NeedsStdoutSink() {
		return extractViaStdout(ctx, argv, item, destDir)
	}
	res := cmdrun.Run(ctx, argv, filepath.Dir(item.Path), cmdrun.MaxStreamBytes)
	if res.LaunchErr != nil {
		return res.LaunchErr
	}
	if res.ExitCode != 0 {
		msg := strings.TrimSpace(string(res.Stderr))
		if msg == "" {
			msg = strings.TrimSpace(string(res.Stdout))
		}
		if msg == "" {
			msg = fmt.Sprintf("exit code %d", res.ExitCode)
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func extractViaStdout(ctx context.Context, argv []string, item ExtractItem, destDir string) error {
	outPath := filepath.Join(destDir, archive.OutputBasename(item.Path, item.Format))
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = filepath.Dir(item.Path)
	cmd.Stdout = f
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		_ = os.Remove(outPath)
		if msg := strings.TrimSpace(stderrBuf.String()); msg != "" {
			return fmt.Errorf("%s", msg)
		}
		return err
	}
	return nil
}

// ExtractItemPaths returns source paths from extract items.
func ExtractItemPaths(items []ExtractItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Path
	}
	return out
}

// FilterArchiveEntries returns paths of regular files with known archive formats.
func FilterArchiveEntries(entries []localfs.Entry) (archives []string, skipped int) {
	for _, e := range entries {
		if e.Type != localfs.EntryFile {
			skipped++
			continue
		}
		if _, ok := archive.FormatForName(e.Name); !ok {
			skipped++
			continue
		}
		archives = append(archives, e.Path)
	}
	return archives, skipped
}
