package ops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/archive"
	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/localfs"
)

var errExtractSkipped = errors.New("extract skipped")

type extractCancelError struct{ err error }

func (e *extractCancelError) Error() string { return e.err.Error() }
func (e *extractCancelError) Unwrap() error { return e.err }

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
	// Conflict is consulted when a stream extract would replace an existing file.
	// A nil resolver fails that item without replacing the file. Returning false
	// skips the item. Returning an error cancels remaining archives.
	Conflict ConflictResolver
}

// PlanExtract builds an extract plan from source paths and destination directory.
// Skips non-files, unknown formats, existing stream outputs, and later stream items
// whose output basename collides with an earlier item. Returns error when no
// runnable archives remain.
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
	claimedStream := make(map[string]string)

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
		if reason, skip := streamOutputConflict(p, f, destDir, claimedStream); skip {
			skipped = append(skipped, reason)
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
// an error if any archive failed (remaining archives still run unless Conflict
// cancels). Existing stream outputs are not replaced unless Conflict returns true.
func ExecuteExtract(ctx context.Context, plan ExtractPlan, progress ExtractProgress) (int, error) {
	var done int
	var firstErr error
	var failCount int
	claimedStream := make(map[string]string)

	for _, item := range plan.Items {
		if err := ctx.Err(); err != nil {
			return done, err
		}
		if item.Format.NeedsStdoutSink() {
			name := archive.OutputBasename(item.Path, item.Format)
			if prev, ok := claimedStream[name]; ok {
				failCount++
				if firstErr == nil {
					firstErr = fmt.Errorf("%s: output %q collides with %s", filepath.Base(item.Path), name, filepath.Base(prev))
				}
				if progress != nil {
					progress(item.Path, done)
				}
				continue
			}
			claimedStream[name] = item.Path
		}
		err := extractOne(ctx, item, plan.Destination, plan.Toolchain, plan.Conflict)
		if errors.Is(err, errExtractSkipped) {
			if progress != nil {
				progress(item.Path, done)
			}
			continue
		}
		var canceled *extractCancelError
		if errors.As(err, &canceled) {
			if progress != nil {
				progress(item.Path, done)
			}
			return done, fmt.Errorf("%s: %w", filepath.Base(item.Path), err)
		}
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

func extractOne(ctx context.Context, item ExtractItem, destDir string, tc archive.Toolchain, resolver ConflictResolver) error {
	argv, err := archive.BuildArgv(item.Format, item.Path, destDir, tc)
	if err != nil {
		return err
	}
	if item.Format.NeedsStdoutSink() {
		return extractViaStdout(ctx, argv, item, destDir, resolver)
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

func streamOutputConflict(archivePath string, f archive.Format, destDir string, claimed map[string]string) (string, bool) {
	if !f.NeedsStdoutSink() {
		return "", false
	}
	name := archive.OutputBasename(archivePath, f)
	if prev, ok := claimed[name]; ok {
		return fmt.Sprintf("%s: output %q collides with %s", filepath.Base(archivePath), name, prev), true
	}
	destPath := filepath.Join(destDir, name)
	_, err := os.Lstat(destPath)
	if err == nil {
		return fmt.Sprintf("%s: output %q already exists", filepath.Base(archivePath), name), true
	}
	if !os.IsNotExist(err) {
		return fmt.Sprintf("%s: %v", filepath.Base(archivePath), err), true
	}
	claimed[name] = filepath.Base(archivePath)
	return "", false
}

func decideStreamOverwrite(src, dst string, resolver ConflictResolver) (bool, error) {
	if resolver == nil {
		return false, fmt.Errorf("destination %q already exists", dst)
	}
	facts, err := StatFileConflictFacts(src, dst)
	if err != nil {
		return false, err
	}
	overwrite, err := resolver(src, dst, facts)
	if err != nil {
		return false, &extractCancelError{err: err}
	}
	return overwrite, nil
}

func extractViaStdout(ctx context.Context, argv []string, item ExtractItem, destDir string, resolver ConflictResolver) error {
	outPath := filepath.Join(destDir, archive.OutputBasename(item.Path, item.Format))
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	flags := os.O_CREATE | os.O_WRONLY | os.O_EXCL
	_, statErr := os.Lstat(outPath)
	if statErr == nil {
		overwrite, err := decideStreamOverwrite(item.Path, outPath, resolver)
		if err != nil {
			return err
		}
		if !overwrite {
			return errExtractSkipped
		}
		flags = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	} else if !os.IsNotExist(statErr) {
		return statErr
	}
	f, err := os.OpenFile(outPath, flags, 0o644)
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
