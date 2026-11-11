package ops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/paranoidi/paras-commander/internal/archive"
	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/fschange"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

var (
	errExtractSkipped    = errors.New("extract skipped")
	errExtractNoPassword = errors.New("extract skipped: no password")
)

type extractCancelError struct{ err error }

func (e *extractCancelError) Error() string { return e.err.Error() }
func (e *extractCancelError) Unwrap() error { return e.err }

// ExtractItem is one archive scheduled for extraction.
type ExtractItem struct {
	// Path is the archive's host path or sftp:// URI.
	Path   string
	Format archive.Format
	// Source is the index of the archive among the paths the plan was built from.
	Source int32
}

// ExtractPlan describes a validated extract operation.
type ExtractPlan struct {
	Items []ExtractItem
	// Destination is a directory: a host path or an sftp:// URI. A remote archive is downloaded to
	// a local temporary directory first, a remote destination is extracted into one and uploaded
	// (ExecuteExtract removes the directories again).
	Destination string
	Toolchain   archive.Toolchain
	// AsDirectory extracts each container archive into Destination/<archive name minus suffix>,
	// created by the worker; stream formats ignore it.
	AsDirectory bool
	// Conflict is consulted when a stream extract would replace an existing file.
	// A nil resolver fails that item without replacing the file. Returning false
	// skips the item. Returning an error cancels remaining archives.
	Conflict ConflictResolver
	// Password, when set, is asked for the password of an encrypted archive (retry is true when
	// the previous answer was rejected). ok=false skips that archive; nil fails it. The last
	// accepted password is tried first on the following archives of the same ExecuteExtract.
	Password ArchivePasswordFunc
	// Throttle paces OnTransfer; zero fields use the worker progress defaults.
	Throttle ProgressEmitThrottle
	// OnTransfer, when set, hears the bytes moved while a remote archive is downloaded or a remote
	// destination is uploaded: the user-facing path being transferred (the remote archive, the
	// remote destination file; never a temporary one) and the bytes moved so far in this
	// ExecuteExtract call. Local extracts never call it.
	OnTransfer func(path string, doneBytes int64)
	// OnItem, when set, reports each archive extracted (ItemDone), skipped or failed, with the
	// reason for the last two.
	OnItem ItemFunc
	// OnSourceStart, when set, reports each archive's ExtractItem.Source as it begins.
	OnSourceStart func(i int32)
}

// ArchivePasswordFunc asks the user for archivePath's password. It blocks until answered; a
// canceled ctx must return ok=false.
type ArchivePasswordFunc func(ctx context.Context, archivePath string, retry bool) (pw string, ok bool)

func (p ExtractPlan) report(item ExtractItem, res ItemResult, reason string) {
	if p.OnItem == nil {
		return
	}
	src, err := pathloc.File(item.Path)
	if err != nil {
		return
	}
	p.OnItem(PlanItem{Src: src, Source: item.Source}, res, reason)
}

// PlanExtract builds an extract plan from source paths and destination directory (host paths or
// sftp:// URIs; remote ones are checked through their backend). Skips non-files, unknown formats,
// existing stream outputs, and later stream items whose output basename collides with an earlier
// item. Returns error when no runnable archives remain.
func PlanExtract(paths []string, destDir string, tc archive.Toolchain) (ExtractPlan, []string, error) {
	return planExtract(context.Background(), paths, destDir, tc, false)
}

// PlanExtractForJob is PlanExtract for the job worker: a stream archive whose output already
// exists, or collides with an earlier one, stays in the plan so ExtractPlan.Conflict decides.
func PlanExtractForJob(ctx context.Context, paths []string, destDir string, tc archive.Toolchain) (ExtractPlan, []string, error) {
	return planExtract(ctx, paths, destDir, tc, true)
}

func planExtract(ctx context.Context, paths []string, destDir string, tc archive.Toolchain, keepStreams bool) (ExtractPlan, []string, error) {
	destLoc, err := pathloc.Parse(destDir)
	if err != nil {
		return ExtractPlan{}, nil, &Error{Op: "extract", Text: "destination is empty", Err: err}
	}
	destDir = destLoc.String()
	info, err := extractStat(ctx, destLoc)
	if err != nil {
		return ExtractPlan{}, nil, &Error{Op: "extract", Text: fmt.Sprintf("destination %q: %v", destDir, err), Err: err}
	}
	if !info.dir {
		return ExtractPlan{}, nil, &Error{Op: "extract", Text: "destination is not a directory"}
	}

	var skipped []string
	var items []ExtractItem
	var unavailable []string
	claimedStream := make(map[string]string)

	for i, p := range paths {
		loc, err := pathloc.Parse(p)
		if err != nil {
			skipped = append(skipped, fmt.Sprintf("%s: %v", p, err))
			continue
		}
		p = loc.String()
		name := loc.Base()
		fi, err := extractStat(ctx, loc)
		if err != nil {
			skipped = append(skipped, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		if !fi.regular {
			skipped = append(skipped, name+": not a regular file")
			continue
		}
		f, ok := archive.FormatForName(name)
		if !ok {
			skipped = append(skipped, name+": unsupported archive type")
			continue
		}
		if !f.Available(tc) {
			unavailable = append(unavailable, fmt.Sprintf("%s: %s not found", name, f.RequiredToolName()))
			continue
		}
		if !keepStreams {
			if reason, skip := streamOutputConflict(ctx, name, f, destLoc, claimedStream); skip {
				skipped = append(skipped, reason)
				continue
			}
		}
		items = append(items, ExtractItem{Path: p, Format: f, Source: int32(i)})
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
	run := &extractRun{meter: transferMeter{th: effectiveProgressThrottle(plan.Throttle), fn: plan.OnTransfer}}

	for _, item := range plan.Items {
		if err := itemBoundaryErr(ctx); err != nil {
			return done, err
		}
		if plan.OnSourceStart != nil {
			plan.OnSourceStart(item.Source)
		}
		if item.Format.NeedsStdoutSink() {
			name := archive.OutputBasename(item.Path, item.Format)
			if prev, ok := claimedStream[name]; ok {
				plan.report(item, ItemFailed, fmt.Sprintf("output %q collides with %s", name, filepath.Base(prev)))
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
		kept, err := extractStaged(ctx, item, plan, run)
		var skipReason string
		switch {
		case errors.Is(err, errExtractNoPassword):
			skipReason = ReasonNoPassword
		case errors.Is(err, errExtractSkipped):
			skipReason = ReasonExists
		}
		if skipReason != "" {
			plan.report(item, ItemSkipped, skipReason)
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
			plan.report(item, ItemFailed, err.Error())
			failCount++
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %w", filepath.Base(item.Path), err)
			}
		} else {
			plan.report(item, ItemDone, ReasonKeptExisting(kept))
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

// extractRun is the state ExecuteExtract keeps across its archives.
type extractRun struct {
	pw    string // last accepted archive password, in memory for this call only
	meter transferMeter
}

// transferMeter counts the bytes of staged transfers and reports them (through fn) at most as
// often as th allows.
type transferMeter struct {
	th        ProgressEmitThrottle
	fn        func(path string, doneBytes int64)
	total     int64
	sinceEmit int64
	lastEmit  time.Time
}

// add counts delta bytes moved for path and reports unless throttled.
func (m *transferMeter) add(path string, delta int64) {
	m.total += delta
	m.sinceEmit += delta
	if m.fn == nil {
		return
	}
	if m.sinceEmit >= m.th.MinBytes || (m.sinceEmit > 0 && !m.lastEmit.IsZero() && time.Since(m.lastEmit) >= m.th.MinInterval) {
		m.flush(path)
	}
}

// flush reports the current total for path.
func (m *transferMeter) flush(path string) {
	if m.fn == nil {
		return
	}
	m.fn(path, m.total)
	m.sinceEmit, m.lastEmit = 0, time.Now()
}

// extractStaged extracts one archive. When the archive or the destination is remote it works
// through local temporary directories that are removed again whatever the outcome: a remote
// archive is downloaded first, a remote destination receives the extracted top-level entries
// afterwards through the copy executor (so ExtractPlan.Conflict decides about existing files). The
// scratch work is not reported to ctx's fschange sink.
func extractStaged(ctx context.Context, item ExtractItem, plan ExtractPlan, run *extractRun) (kept int, err error) {
	srcLoc, err := pathloc.Parse(item.Path)
	if err != nil {
		return 0, err
	}
	destLoc, err := pathloc.Parse(plan.Destination)
	if err != nil {
		return 0, err
	}
	var ask func(ctx context.Context, retry bool) (string, bool)
	if plan.Password != nil { // the prompt names the archive as the user knows it, not its temporary copy
		ask = func(ctx context.Context, retry bool) (string, bool) {
			return plan.Password(ctx, item.Path, retry)
		}
	}
	local, destDir, extractCtx := item, plan.Destination, ctx
	if !srcLoc.IsLocal() {
		dir, err := os.MkdirTemp("", "pc-extract-src-")
		if err != nil {
			return 0, err
		}
		defer func() { _ = os.RemoveAll(dir) }()
		local.Path = filepath.Join(dir, srcLoc.Base())
		dstLoc, err := pathloc.File(local.Path)
		if err != nil {
			return 0, err
		}
		onWritten := func(n int64) { run.meter.add(item.Path, n) }
		res, err := copyFileTransfer(fschange.WithoutSink(ctx), srcLoc, dstLoc, DefaultOptions(), nil, nil, onWritten)
		if err != nil || res != copyDone {
			if err == nil {
				err = errExtractSkipped
			}
			return 0, fmt.Errorf("download: %w", err)
		}
		run.meter.flush(item.Path)
	}
	if !destLoc.IsLocal() {
		stage, err := os.MkdirTemp("", "pc-extract-dst-")
		if err != nil {
			return 0, err
		}
		defer func() { _ = os.RemoveAll(stage) }()
		destDir, extractCtx = stage, fschange.WithoutSink(ctx)
	}
	if err := extractOne(extractCtx, local, destDir, plan.AsDirectory, plan.Toolchain, plan.Conflict, ask, run); err != nil {
		return 0, err
	}
	if destLoc.IsLocal() {
		return 0, nil
	}
	entries, err := os.ReadDir(destDir)
	if err != nil {
		return 0, err
	}
	staged := make([]pathloc.Path, 0, len(entries))
	for _, e := range entries {
		p, err := pathloc.File(filepath.Join(destDir, e.Name()))
		if err != nil {
			return 0, err
		}
		staged = append(staged, p)
	}
	if len(staged) == 0 {
		return 0, nil
	}
	resolver := plan.Conflict
	if resolver != nil { // count what the user's answers kept in place
		resolver = func(ctx context.Context, src, dst string, facts FileConflictFacts) (ConflictResolution, error) {
			res, err := plan.Conflict(ctx, src, dst, facts)
			if err == nil && res.Action == ActionSkip {
				kept++
			}
			return res, err
		}
	}
	base := run.meter.total
	progress := func(_, dst string, _ int, doneBytes int64) {
		run.meter.total = base + doneBytes
		run.meter.flush(dst)
	}
	_, doneBytes, err := ExecuteCopy(ctx, staged, destLoc, DefaultOptions(), plan.Throttle, progress, resolver, nil)
	run.meter.total = base + doneBytes
	if err != nil {
		return kept, fmt.Errorf("upload: %w", err)
	}
	return kept, nil
}

func extractOne(ctx context.Context, item ExtractItem, destDir string, asDir bool, tc archive.Toolchain, resolver ConflictResolver, askPassword func(ctx context.Context, retry bool) (string, bool), run *extractRun) error {
	target := destDir
	if asDir && !item.Format.NeedsStdoutSink() {
		target = filepath.Join(destDir, archive.OutputBasename(item.Path, item.Format))
	}
	if item.Format.NeedsStdoutSink() {
		argv, err := archive.BuildArgv(item.Format, item.Path, target, tc)
		if err != nil {
			return err
		}
		return extractViaStdout(ctx, argv, item, destDir, resolver)
	}
	// The archive tool picks the names it writes: report what changed at destDir's top level
	// (snapshotted before target is created, so a new subdirectory counts).
	before := snapshotTopLevel(ctx, destDir)
	defer reportTopLevelChanges(ctx, destDir, before)
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	runTool := func(pw string) cmdrun.RunResult {
		argv, stdin, err := archive.PasswordArgv(item.Format, item.Path, target, tc, pw)
		if err != nil {
			return cmdrun.RunResult{LaunchErr: err, ExitCode: -1}
		}
		return cmdrun.RunWithStdin(ctx, argv, filepath.Dir(item.Path), cmdrun.MaxStreamBytes, stdin)
	}
	res := runTool(run.pw)
	for retry := false; askPassword != nil && res.LaunchErr == nil && passwordFailed(item.Format, res); retry = true {
		typed, ok := askPassword(ctx, retry)
		if !ok {
			return errExtractNoPassword
		}
		if res = runTool(typed); !passwordFailed(item.Format, res) {
			run.pw = typed
		}
	}
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

// passwordFailed reports whether a failed run shows a missing or rejected archive password.
func passwordFailed(f archive.Format, res cmdrun.RunResult) bool {
	return res.ExitCode != 0 && archive.PasswordFailure(f, string(res.Stdout)+string(res.Stderr))
}

// extractStatResult is what the planner needs to know about a path.
type extractStatResult struct{ dir, regular bool }

// extractStat stats loc, following symlinks: on disk with os.Stat, elsewhere through its backend.
func extractStat(ctx context.Context, loc pathloc.Path) (extractStatResult, error) {
	if loc.IsLocal() {
		fi, err := os.Stat(loc.String())
		if err != nil {
			return extractStatResult{}, err
		}
		return extractStatResult{dir: fi.IsDir(), regular: fi.Mode().IsRegular()}, nil
	}
	e, err := statEntryFollow(ctx, loc)
	if err != nil {
		return extractStatResult{}, err
	}
	return extractStatResult{dir: e.Type == fsbackend.EntryDirectory, regular: e.Type == fsbackend.EntryFile}, nil
}

func streamOutputConflict(ctx context.Context, archiveName string, f archive.Format, destLoc pathloc.Path, claimed map[string]string) (string, bool) {
	if !f.NeedsStdoutSink() {
		return "", false
	}
	name := archive.OutputBasename(archiveName, f)
	if prev, ok := claimed[name]; ok {
		return fmt.Sprintf("%s: output %q collides with %s", archiveName, name, prev), true
	}
	destPath, err := destLoc.Join(name)
	if err == nil {
		_, err = statEntry(ctx, destPath)
		if err == nil {
			return fmt.Sprintf("%s: output %q already exists", archiveName, name), true
		}
		if isNotExist(err) {
			err = nil
		}
	}
	if err != nil {
		return fmt.Sprintf("%s: %v", archiveName, err), true
	}
	claimed[name] = archiveName
	return "", false
}

func decideStreamOverwrite(ctx context.Context, src, dst string, resolver ConflictResolver) (ConflictAction, error) {
	if resolver == nil {
		return ActionSkip, fmt.Errorf("destination %q already exists", dst)
	}
	facts, err := StatFileConflictFacts(src, dst)
	if err != nil {
		return ActionSkip, err
	}
	res, err := resolver(ctx, src, dst, facts)
	if err != nil {
		return ActionSkip, &extractCancelError{err: err}
	}
	return res.Action, nil
}

func extractViaStdout(ctx context.Context, argv []string, item ExtractItem, destDir string, resolver ConflictResolver) error {
	outPath := filepath.Join(destDir, archive.OutputBasename(item.Path, item.Format))
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	flags := os.O_CREATE | os.O_WRONLY | os.O_EXCL
	_, statErr := os.Lstat(outPath)
	if statErr == nil {
		action, err := decideStreamOverwrite(ctx, item.Path, outPath, resolver)
		if err != nil {
			return err
		}
		switch action {
		case ActionOverwrite:
			flags = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
		case ActionRename:
			loc, err := pathloc.Parse(outPath)
			if err == nil {
				loc, err = uniqueSiblingName(ctx, loc)
			}
			if err != nil {
				return err
			}
			outPath = loc.String()
		default:
			return errExtractSkipped
		}
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
		if os.Remove(outPath) == nil {
			reportRemovedHost(ctx, outPath, false) // an overwrite already truncated the old file
		}
		if msg := strings.TrimSpace(stderrBuf.String()); msg != "" {
			return fmt.Errorf("%s", msg)
		}
		return err
	}
	reportCreatedHost(ctx, outPath)
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

// topLevelEntry is what snapshotTopLevel keeps of one entry of a directory.
type topLevelEntry struct {
	modTime time.Time
	size    int64
	isDir   bool
}

// snapshotTopLevel lists dir's entries when ctx has an fschange sink (nil otherwise, or when dir
// does not exist yet).
func snapshotTopLevel(ctx context.Context, dir string) map[string]topLevelEntry {
	if !fschange.Enabled(ctx) {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	snap := make(map[string]topLevelEntry, len(entries))
	for _, de := range entries {
		if fi, err := de.Info(); err == nil {
			snap[de.Name()] = topLevelEntry{modTime: fi.ModTime(), size: fi.Size(), isDir: fi.IsDir()}
		}
	}
	return snap
}

// reportTopLevelChanges reports each entry of dir that is new or whose mtime, size or kind
// differs from before as dirty: its size is unknown, so it is rescanned (a directory's mtime
// moves when an entry is added to it). Unchanged entries are not rescanned.
func reportTopLevelChanges(ctx context.Context, dir string, before map[string]topLevelEntry) {
	after := snapshotTopLevel(ctx, dir)
	for name, a := range after {
		if b, ok := before[name]; ok && b == a {
			continue
		}
		reportDirtyHost(ctx, filepath.Join(dir, name), a.isDir)
	}
}
