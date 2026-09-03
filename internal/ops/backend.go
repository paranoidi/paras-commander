package ops

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func defaultRegistry() *fsbackend.Registry {
	return fsbackend.Default()
}

func backendFor(loc pathloc.Path) (fsbackend.Backend, error) {
	return defaultRegistry().Backend(loc)
}

func useLocalFastPath(src, dst pathloc.Path) bool {
	return src.Scheme() == pathloc.SchemeFile && dst.Scheme() == pathloc.SchemeFile
}

func sameSFTPHost(a, b pathloc.Path) bool {
	if a.Scheme() != pathloc.SchemeSFTP || b.Scheme() != pathloc.SchemeSFTP {
		return false
	}
	ha, err := pathloc.SFTPHostPart(a)
	if err != nil {
		return false
	}
	hb, err := pathloc.SFTPHostPart(b)
	if err != nil {
		return false
	}
	return ha == hb
}

func statEntry(ctx context.Context, loc pathloc.Path) (fsbackend.Entry, error) {
	be, err := backendFor(loc)
	if err != nil {
		return fsbackend.Entry{}, err
	}
	return be.Stat(ctx, loc)
}

func destinationIsDir(ctx context.Context, dest pathloc.Path) (bool, error) {
	ent, err := statEntry(ctx, dest)
	if err != nil {
		if isNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return ent.Type == fsbackend.EntryDirectory, nil
}

func isNotExist(err error) bool {
	if err == nil {
		return false
	}
	if os.IsNotExist(err) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such file") || strings.Contains(msg, "not found")
}

func resolveChild(parent pathloc.Path, name string) (pathloc.Path, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return pathloc.Path{}, errors.New("empty name")
	}
	if strings.HasPrefix(name, "sftp://") {
		return pathloc.Parse(name)
	}
	if parent.IsRemote() {
		if strings.Contains(name, "/") || strings.Contains(name, string(filepath.Separator)) {
			return pathloc.Path{}, fmt.Errorf("name must be a single path element")
		}
		return parent.Join(name)
	}
	if filepath.IsAbs(name) {
		return pathloc.File(name)
	}
	host, err := parent.FilePath()
	if err != nil {
		return pathloc.Path{}, err
	}
	return pathloc.File(filepath.Join(host, name))
}

func ensureParentDirs(ctx context.Context, loc pathloc.Path) error {
	parent := loc.Parent()
	if parent.IsZero() || parent.Equal(loc) {
		return nil
	}
	if _, err := statEntry(ctx, parent); err == nil {
		return nil
	} else if !isNotExist(err) {
		return err
	}
	if err := ensureParentDirs(ctx, parent); err != nil {
		return err
	}
	be, err := backendFor(parent)
	if err != nil {
		return err
	}
	return be.Mkdir(ctx, parent, 0o755)
}

func removePathRecursive(ctx context.Context, loc pathloc.Path) error {
	ent, err := statEntry(ctx, loc)
	if err != nil {
		if isNotExist(err) {
			return nil
		}
		return err
	}
	be, err := backendFor(loc)
	if err != nil {
		return err
	}
	if ent.Type != fsbackend.EntryDirectory {
		return be.Remove(ctx, loc)
	}
	if loc.Scheme() == pathloc.SchemeFile {
		host, err := loc.FilePath()
		if err != nil {
			return err
		}
		return localfs.RemoveAll(host)
	}
	children, err := be.List(ctx, loc)
	if err != nil {
		return err
	}
	for _, child := range children {
		if child.Name == "." || child.Name == ".." {
			continue
		}
		if err := removePathRecursive(ctx, child.Loc); err != nil {
			return err
		}
	}
	return be.Remove(ctx, loc)
}

func removePartialTransferDest(ctx context.Context, dst pathloc.Path) {
	be, err := backendFor(dst)
	if err != nil {
		return
	}
	_ = be.Remove(ctx, dst)
}

func countTransferNodes(ctx context.Context, loc pathloc.Path) (int, error) {
	ent, err := statEntry(ctx, loc)
	if err != nil {
		return 0, err
	}
	if ent.Type != fsbackend.EntryDirectory {
		return 1, nil
	}
	n := 0
	var countDir func(pathloc.Path) error
	countDir = func(dir pathloc.Path) error {
		n++
		be, err := backendFor(dir)
		if err != nil {
			return err
		}
		children, err := be.List(ctx, dir)
		if err != nil {
			return err
		}
		for _, c := range children {
			if c.Name == "." || c.Name == ".." {
				continue
			}
			if c.Type == fsbackend.EntryDirectory {
				if err := countDir(c.Loc); err != nil {
					return err
				}
			} else {
				n++
			}
		}
		return nil
	}
	if err := countDir(loc); err != nil {
		return 0, err
	}
	return n, nil
}

func statConflictFacts(ctx context.Context, src, dst pathloc.Path) (FileConflictFacts, error) {
	if useLocalFastPath(src, dst) {
		sh, err := src.FilePath()
		if err != nil {
			return FileConflictFacts{}, err
		}
		dh, err := dst.FilePath()
		if err != nil {
			return FileConflictFacts{}, err
		}
		return StatFileConflictFacts(sh, dh)
	}
	se, err := statEntry(ctx, src)
	if err != nil {
		return FileConflictFacts{}, err
	}
	de, err := statEntry(ctx, dst)
	if err != nil {
		return FileConflictFacts{}, err
	}
	kind := "file"
	if se.Type == fsbackend.EntrySymlink {
		kind = "symlink"
	}
	return FileConflictFacts{
		Kind:       kind,
		SourceSize: se.Size,
		SourceMod:  se.ModifiedAt,
		DestSize:   de.Size,
		DestMod:    de.ModifiedAt,
	}, nil
}

// resolveDestConflict returns proceed=false when the resolver declined the overwrite.
func resolveDestConflict(ctx context.Context, src, dst pathloc.Path, resolver ConflictResolver) (proceed bool, err error) {
	if err := ensureParentDirs(ctx, dst); err != nil {
		return false, fmt.Errorf("create parent for %q: %w", dst, err)
	}
	if _, err := statEntry(ctx, dst); err == nil {
		facts, ferr := statConflictFacts(ctx, src, dst)
		if ferr != nil {
			return false, fmt.Errorf("conflict stat %q %q: %w", src, dst, ferr)
		}
		proceed, perr := resolveOverwriteDecision(src.String(), dst.String(), resolver, facts)
		if perr != nil {
			return false, perr
		}
		if !proceed {
			return false, nil
		}
		if err := removePathRecursive(ctx, dst); err != nil {
			return false, fmt.Errorf("remove existing %q: %w", dst, err)
		}
	} else if !isNotExist(err) {
		return false, fmt.Errorf("stat destination %q: %w", dst, err)
	}
	return true, nil
}

// applyTransferMetadata applies permission/timestamp preservation and the
// after-each-file sync gate to a freshly copied local destination file.
func applyTransferMetadata(ctx context.Context, src, dst pathloc.Path, srcEnt fsbackend.Entry, opts Options) error {
	if opts.PreservePermissions && dst.Scheme() == pathloc.SchemeFile {
		if host, err := dst.FilePath(); err == nil {
			_ = os.Chmod(host, srcEnt.Mode.Perm())
		}
	}
	if opts.PreserveTimestamps && dst.Scheme() == pathloc.SchemeFile {
		if host, err := dst.FilePath(); err == nil {
			atime, mtime := transferSourceTimes(src, srcEnt)
			_ = os.Chtimes(host, atime, mtime)
		}
	}
	if opts.SyncAfterEachFile && dst.Scheme() == pathloc.SchemeFile {
		if host, err := dst.FilePath(); err == nil {
			if opts.SyncFileNow(srcEnt.Size) {
				if err := syncLocalPath(host); err != nil {
					removePartialTransferDest(ctx, dst)
					return err
				}
			}
		}
	}
	return nil
}

func copyFileTransfer(ctx context.Context, src, dst pathloc.Path, opts Options, resolver ConflictResolver, buf []byte, onWritten func(int64)) (copied bool, err error) {
	if proceed, err := resolveDestConflict(ctx, src, dst, resolver); err != nil {
		return false, err
	} else if !proceed {
		return false, nil
	}

	srcBE, err := backendFor(src)
	if err != nil {
		return false, err
	}
	dstBE, err := backendFor(dst)
	if err != nil {
		return false, err
	}
	srcEnt, err := statEntry(ctx, src)
	if err != nil {
		return false, err
	}
	rc, err := srcBE.OpenRead(ctx, src)
	if err != nil {
		return false, err
	}
	defer func() { _ = rc.Close() }()

	wc, err := dstBE.OpenWrite(ctx, dst, srcEnt.Size, fsbackend.CreateOpts{Truncate: true})
	if err != nil {
		return false, err
	}

	bufSize := BufferSize(opts.CopyBufferKiB)
	if len(buf) < bufSize {
		buf = make([]byte, bufSize)
	} else {
		buf = buf[:bufSize]
	}
	cw := &countingWriter{w: wc, fn: onWritten}
	_, err = io.CopyBuffer(cw, &ctxReader{ctx: ctx, r: rc}, buf)
	if closeErr := wc.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		removePartialTransferDest(ctx, dst)
		return false, err
	}

	if err := applyTransferMetadata(ctx, src, dst, srcEnt, opts); err != nil {
		return false, err
	}
	return true, nil
}

func copySymlinkTransfer(ctx context.Context, src, dst pathloc.Path, resolver ConflictResolver) (copied bool, err error) {
	if proceed, err := resolveDestConflict(ctx, src, dst, resolver); err != nil {
		return false, err
	} else if !proceed {
		return false, nil
	}

	srcBE, err := backendFor(src)
	if err != nil {
		return false, err
	}
	target, err := srcBE.ReadSymlink(ctx, src)
	if err != nil {
		return false, fmt.Errorf("read symlink %q: %w", src, err)
	}
	dstBE, err := backendFor(dst)
	if err != nil {
		return false, err
	}
	if err := dstBE.Symlink(ctx, dst, target); err != nil {
		return false, fmt.Errorf("create symlink %q -> %q: %w", dst, target, err)
	}
	return true, nil
}

func syncLocalPath(path string) error {
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return f.Sync()
}

func transferSourceTimes(src pathloc.Path, srcEnt fsbackend.Entry) (atime, mtime time.Time) {
	mtime = srcEnt.ModifiedAt
	atime = mtime
	if src.Scheme() != pathloc.SchemeFile {
		return atime, mtime
	}
	host, err := src.FilePath()
	if err != nil {
		return atime, mtime
	}
	info, err := os.Lstat(host)
	if err != nil {
		return atime, mtime
	}
	return localfs.FileTimes(info)
}

type countingWriter struct {
	w  io.Writer
	fn func(int64)
}

func (cw *countingWriter) Write(p []byte) (int, error) {
	n, err := cw.w.Write(p)
	if n > 0 && cw.fn != nil {
		cw.fn(int64(n))
	}
	return n, err
}

type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (cr *ctxReader) Read(p []byte) (int, error) {
	if err := cr.ctx.Err(); err != nil {
		return 0, err
	}
	return cr.r.Read(p)
}

func entryFromPathString(path string) (localfs.Entry, error) {
	loc, err := pathloc.Parse(path)
	if err != nil {
		return localfs.Entry{}, err
	}
	if loc.IsRemote() {
		ent, err := statEntry(context.Background(), loc)
		if err != nil {
			return localfs.Entry{}, err
		}
		return fsbackend.ToPanelEntry(ent), nil
	}
	return localfs.EntryFromPath(path)
}

func planItemFromEntry(srcLoc, dstLoc pathloc.Path, ent fsbackend.Entry) (PlanItem, error) {
	mod := ent.ModifiedAt
	meta := PlanItem{
		Src:        srcLoc,
		Dst:        dstLoc,
		Mode:       ent.Mode.Perm(),
		AccessTime: mod,
		ModTime:    mod,
	}
	switch ent.Type {
	case fsbackend.EntryDirectory:
		meta.IsDir = true
		meta.IsSymlink = ent.Mode&fs.ModeSymlink != 0
		return meta, nil
	case fsbackend.EntrySymlink:
		meta.IsSymlink = true
		return meta, nil
	case fsbackend.EntryFile:
		meta.FileSize = ent.Size
		return meta, nil
	default:
		return PlanItem{}, fmt.Errorf("unsupported file type for %q", srcLoc)
	}
}

func walkBackendTree(ctx context.Context, rootSrc, rootDst pathloc.Path, sink func(PlanItem) error, afterVisit func(string) error) error {
	rootEnt, err := statEntry(ctx, rootSrc)
	if err != nil {
		return err
	}
	if rootEnt.Type != fsbackend.EntryDirectory {
		return fmt.Errorf("%q is not a directory", rootSrc)
	}
	if err := appendBackendWalk(ctx, rootSrc, rootDst, sink, afterVisit); err != nil {
		return err
	}
	return nil
}

func appendBackendWalk(ctx context.Context, dirSrc, dirDst pathloc.Path, sink func(PlanItem) error, afterVisit func(string) error) error {
	be, err := backendFor(dirSrc)
	if err != nil {
		return err
	}
	entries, err := be.List(ctx, dirSrc)
	if err != nil {
		return fmt.Errorf("list %q: %w", dirSrc, err)
	}
	dirEnt, err := statEntry(ctx, dirSrc)
	if err != nil {
		return err
	}
	item, err := planItemFromEntry(dirSrc, dirDst, dirEnt)
	if err != nil {
		return err
	}
	if err := sink(item); err != nil {
		return err
	}
	if afterVisit != nil {
		if err := afterVisit(dirSrc.String()); err != nil {
			return err
		}
	}
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if e.Name == "." || e.Name == ".." {
			continue
		}
		childDst, err := dirDst.Join(e.Name)
		if err != nil {
			return err
		}
		switch e.Type {
		case fsbackend.EntryDirectory:
			if err := appendBackendWalk(ctx, e.Loc, childDst, sink, afterVisit); err != nil {
				return err
			}
		case fsbackend.EntrySymlink:
			mod := e.ModifiedAt
			if err := sink(PlanItem{
				Src:        e.Loc,
				Dst:        childDst,
				IsSymlink:  true,
				Mode:       e.Mode.Perm(),
				AccessTime: mod,
				ModTime:    mod,
			}); err != nil {
				return err
			}
			if afterVisit != nil {
				if err := afterVisit(e.Loc.String()); err != nil {
					return err
				}
			}
		case fsbackend.EntryFile:
			mod := e.ModifiedAt
			if err := sink(PlanItem{
				Src:        e.Loc,
				Dst:        childDst,
				FileSize:   e.Size,
				Mode:       e.Mode.Perm(),
				AccessTime: mod,
				ModTime:    mod,
			}); err != nil {
				return err
			}
			if afterVisit != nil {
				if err := afterVisit(e.Loc.String()); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("unsupported file type for %q", e.Loc)
		}
	}
	return nil
}
