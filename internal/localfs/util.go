package localfs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// pathErrorReason returns pathErr.Err when err is *os.PathError so wrappers that
// already include the path (via %q) do not repeat it in the message chain.
func pathErrorReason(err error) error {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) && pathErr.Err != nil {
		return pathErr.Err
	}
	return err
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

// Lstat reads file info without following symlinks.
func Lstat(path string) (fs.FileInfo, error) {
	return os.Lstat(path)
}

// IsSymlink reports whether the given FileInfo is a symlink.
func IsSymlink(info fs.FileInfo) bool {
	return info.Mode()&fs.ModeSymlink != 0
}

// ReadSymlink reads the target of a symbolic link.
func ReadSymlink(path string) (string, error) {
	return os.Readlink(path)
}

// MakeSymlink creates a symbolic link at newPath pointing to target.
func MakeSymlink(target, newPath string) error {
	return os.Symlink(target, newPath)
}

// IsCrossDeviceRenameError returns true if err is the result of trying to
// rename across filesystem boundaries.
func IsCrossDeviceRenameError(err error) bool {
	if err == nil {
		return false
	}
	linkErr, ok := err.(*os.LinkError)
	if !ok {
		return false
	}
	return linkErr.Err != nil && linkErr.Err.Error() == "invalid cross-device link"
}

// CopyFile copies a regular file from src to dst.
// If preservePerms is true, the source file permissions are replicated.
// If preserveTimes is true, the source file access/mod times are replicated.
// If dir is true, dst is a directory and the source's basename is used.
// If syncAfterWrite is true, the destination file is fsync'd before close (slow for many small files).
// If tryKernelFastCopy is true (Linux), ioctl(FICLONE) is attempted before read/write (CoW when supported).
// onWritten is called with the byte length of each successful destination write (optional).
// ctx cancellation is checked before each read from src.
func CopyFile(ctx context.Context, src, dst string, bufSize int, preservePerms, preserveTimes, dir, tryKernelFastCopy bool, extra CopyFileOpts, onWritten func(int64)) error {
	srcInfo, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("stat source %q: %w", src, err)
	}
	if IsSymlink(srcInfo) {
		if !extra.FollowSymlinks {
			return copySymlink(src, dst, dir)
		}
		resolved, err := os.Stat(src)
		if err != nil {
			return fmt.Errorf("stat source %q: %w", src, err)
		}
		srcInfo = resolved
	}

	target := dst
	if dir {
		target = filepath.Join(dst, filepath.Base(src))
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source %q: %w", src, err)
	}
	defer func() { _ = srcFile.Close() }()

	dstFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o666)
	if err != nil {
		return fmt.Errorf("create destination %q: %w", target, err)
	}

	if extra.shouldPreallocate(srcInfo.Size()) {
		if err := preallocateDestination(dstFile, srcInfo.Size()); err != nil {
			abortPartialLocalCopy(dstFile, target)
			return fmt.Errorf("preallocate destination %q: %w", target, err)
		}
	}

	// Bounded write-behind: when this file is going to be fsync'd anyway, also fsync every
	// SyncWriteBehindBytes during the copy instead of only at the end. Without this, a
	// filesystem with a large dirty-data buffer (e.g. ZFS) accepts writes at memory speed and
	// reports every byte through onWritten long before it reaches disk; the single end-of-file
	// fsync then blocks for as long as the whole flush takes with zero further progress
	// callbacks, so job speed/ETA decay toward zero mid-file. Reporting progress after each
	// bounded flush keeps DoneBytes from running more than N bytes ahead of the disk and keeps
	// the final fsync short.
	var syncErr error
	if extra.syncNow(srcInfo.Size()) && extra.SyncWriteBehindBytes > 0 {
		inner := onWritten
		var sinceSync int64
		onWritten = func(n int64) {
			sinceSync += n
			if sinceSync >= extra.SyncWriteBehindBytes {
				if err := dstFile.Sync(); err != nil && syncErr == nil {
					syncErr = err
				}
				sinceSync = 0
			}
			if inner != nil {
				inner(n)
			}
		}
	}

	fastDone := false
	if tryKernelFastCopy {
		ok, ferr := tryKernelReflinkCopy(ctx, srcFile, dstFile, srcInfo.Size(), onWritten)
		if ferr != nil {
			abortPartialLocalCopy(dstFile, target)
			return fmt.Errorf("copy content %q -> %q: %w", src, target, ferr)
		}
		fastDone = ok
	}
	if !fastDone && extra.CopyFileRange {
		ok, ferr := tryKernelFileRangeCopy(ctx, srcFile, dstFile, srcInfo.Size(), extra.CopyFileRangeChunkBytes, onWritten)
		if ferr != nil {
			abortPartialLocalCopy(dstFile, target)
			return fmt.Errorf("copy content %q -> %q: %w", src, target, ferr)
		}
		fastDone = ok
	}
	if !fastDone {
		buf := extra.Buf
		if len(buf) < bufSize {
			buf = make([]byte, bufSize)
		} else {
			buf = buf[:bufSize]
		}
		srcWrapped := io.Reader(srcFile)
		if ctx != nil {
			srcWrapped = &ctxReader{ctx: ctx, r: srcFile}
		}
		if extra.SparseCopy {
			err = copySparseUserspace(ctx, srcFile, dstFile, srcInfo.Size(), buf, onWritten)
		} else {
			dstWrapped := io.Writer(dstFile)
			if onWritten != nil {
				dstWrapped = &countingWriter{w: dstFile, fn: onWritten}
			}
			_, err = io.CopyBuffer(dstWrapped, srcWrapped, buf)
		}
		if err != nil {
			abortPartialLocalCopy(dstFile, target)
			return fmt.Errorf("copy content %q -> %q: %w", src, target, err)
		}
	}
	if syncErr != nil {
		abortPartialLocalCopy(dstFile, target)
		return fmt.Errorf("sync destination %q: %w", target, syncErr)
	}
	if extra.syncNow(srcInfo.Size()) {
		if err := dstFile.Sync(); err != nil {
			abortPartialLocalCopy(dstFile, target)
			return fmt.Errorf("sync destination %q: %w", target, err)
		}
	}
	if err := dstFile.Close(); err != nil {
		_ = os.Remove(target)
		return fmt.Errorf("close destination %q: %w", target, err)
	}

	if preservePerms {
		if err := os.Chmod(target, srcInfo.Mode().Perm()); err != nil {
			return fmt.Errorf("preserve permissions on %q: %w", target, err)
		}
	}
	if preserveTimes {
		atime, mtime := FileTimes(srcInfo)
		if err := os.Chtimes(target, atime, mtime); err != nil {
			return fmt.Errorf("preserve timestamps on %q: %w", target, err)
		}
	}
	return nil
}

// CopyDir recursively copies a directory tree from src to dst.
func CopyDir(src, dst string, bufSize int, preservePerms, preserveTimes, syncAfterWrite bool) error {
	srcInfo, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("stat source dir %q: %w", src, err)
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("source %q is not a directory", src)
	}

	destDir := filepath.Join(dst, filepath.Base(src))
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create destination dir %q: %w", destDir, err)
	}

	if preservePerms {
		if err := os.Chmod(destDir, srcInfo.Mode().Perm()); err != nil {
			return fmt.Errorf("preserve permissions on %q: %w", destDir, err)
		}
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("read source dir %q: %w", src, err)
	}

	for _, entry := range entries {
		childSrc := filepath.Join(src, entry.Name())
		childInfo, err := os.Lstat(childSrc)
		if err != nil {
			return fmt.Errorf("stat child %q: %w", childSrc, err)
		}

		if childInfo.IsDir() {
			if err := CopyDir(childSrc, destDir, bufSize, preservePerms, preserveTimes, syncAfterWrite); err != nil {
				return err
			}
		} else if IsSymlink(childInfo) {
			if err := copySymlink(childSrc, destDir, true); err != nil {
				return err
			}
		} else if childInfo.Mode().IsRegular() {
			if err := CopyFile(context.Background(), childSrc, destDir, bufSize, preservePerms, preserveTimes, true, false, CopyFileOpts{SyncPerFile: syncAfterWrite}, nil); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("unsupported file type for %q (mode %v)", childSrc, childInfo.Mode())
		}
	}

	if preserveTimes {
		atime, mtime := FileTimes(srcInfo)
		if err := os.Chtimes(destDir, atime, mtime); err != nil {
			return fmt.Errorf("preserve timestamps on %q: %w", destDir, err)
		}
	}
	return nil
}

func copySymlink(src, dst string, dir bool) error {
	target, err := os.Readlink(src)
	if err != nil {
		return fmt.Errorf("read symlink target %q: %w", src, err)
	}
	destPath := dst
	if dir {
		destPath = filepath.Join(dst, filepath.Base(src))
	}
	if err := os.Symlink(target, destPath); err != nil {
		return fmt.Errorf("create symlink %q -> %q: %w", destPath, target, err)
	}
	return nil
}

// WalkDirRecursive walks a directory recursively, calling fn for every file,
// directory, and symlink including the root. It returns entries in deterministic order.
func WalkDirRecursive(root string, fn func(path string, info fs.FileInfo) error) error {
	info, err := os.Lstat(root)
	if err != nil {
		return err
	}
	return walkDirRecursive(root, info, fn)
}

// walkDirRecursive walks path (already Lstat'd as info) and its descendants, calling fn once per
// node. Recursion passes each child's already-known os.ReadDir info down instead of re-Lstat-ing
// it at the top of the call, since os.ReadDir's DirEntry.Info() is itself Lstat-based.
func walkDirRecursive(path string, info fs.FileInfo, fn func(string, fs.FileInfo) error) error {
	if err := fn(path, info); err != nil {
		return err
	}
	if !info.IsDir() {
		return nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		childPath := filepath.Join(path, entry.Name())
		childInfo, err := entry.Info()
		if err != nil {
			return err
		}
		if err := walkDirRecursive(childPath, childInfo, fn); err != nil {
			return err
		}
	}
	return nil
}

// maxSymlinkDerefDepth caps how deep WalkDirRecursiveDeref will chase a chain of
// directory symlinks, guarding against pathological long non-repeating chains the same
// way maxTreeExpandDepth (internal/panel/tree_state.go) guards tree-expand recursion.
const maxSymlinkDerefDepth = 32

// WalkDirRecursiveDeref walks like WalkDirRecursive, except every symlink encountered
// (including the root) is dereferenced: fn receives the resolved FileInfo (no ModeSymlink bit),
// and a symlink resolving to a directory is walked as if its target's children lived at the
// symlink's own path — path is always the logical/symlink-named path, never the resolved real
// path (os.Stat/os.ReadDir/os.Open on that path string already follow the symlink).
//
// A symlink that cannot be safely dereferenced falls back to the plain WalkDirRecursive
// behavior for that node (fn receives the Lstat info, no recursion): onFallback(path, reason) is
// called first, with reason describing why — a broken target or permission error (the os.Stat
// error text), "cycle detected" (the resolved directory matches one already open on the current
// recursion path, via os.SameFile), or "max depth exceeded" (maxSymlinkDerefDepth directory
// symlinks deep). Cycle detection is scoped to the current recursion path (an ancestor stack
// pushed on recursion-enter, implicit via the Go call stack, and popped on return) rather than a
// whole-tree visited set, so two sibling symlinks pointing at the same shared, non-cyclic target
// are not mistaken for a cycle.
func WalkDirRecursiveDeref(root string, onFallback func(path string, reason string), fn func(path string, info fs.FileInfo) error) error {
	info, err := os.Lstat(root)
	if err != nil {
		return err
	}
	return walkDirRecursiveDeref(root, info, nil, onFallback, fn)
}

func walkDirRecursiveDeref(path string, lstatInfo fs.FileInfo, ancestors []fs.FileInfo, onFallback func(string, string), fn func(string, fs.FileInfo) error) error {
	nodeInfo := lstatInfo
	if IsSymlink(lstatInfo) {
		resolved, err := os.Stat(path)
		if err != nil {
			onFallback(path, pathErrorReason(err).Error())
			return fn(path, lstatInfo)
		}
		if !resolved.IsDir() {
			return fn(path, resolved)
		}
		if len(ancestors) >= maxSymlinkDerefDepth {
			onFallback(path, "max depth exceeded")
			return fn(path, lstatInfo)
		}
		for _, a := range ancestors {
			if os.SameFile(a, resolved) {
				onFallback(path, "cycle detected")
				return fn(path, lstatInfo)
			}
		}
		nodeInfo = resolved
	}
	if err := fn(path, nodeInfo); err != nil {
		return err
	}
	if !nodeInfo.IsDir() {
		return nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	childAncestors := append(ancestors, nodeInfo)
	for _, entry := range entries {
		childPath := filepath.Join(path, entry.Name())
		childInfo, err := entry.Info()
		if err != nil {
			return err
		}
		if err := walkDirRecursiveDeref(childPath, childInfo, childAncestors, onFallback, fn); err != nil {
			return err
		}
	}
	return nil
}

// CopyMetadata preserves file permissions and optionally timestamps on dest.
func CopyMetadata(srcInfo fs.FileInfo, dest string, preservePerms, preserveTimes bool) error {
	if preservePerms {
		if err := os.Chmod(dest, srcInfo.Mode().Perm()); err != nil {
			return fmt.Errorf("preserve permissions on %q: %w", dest, err)
		}
	}
	if preserveTimes {
		atime, mtime := FileTimes(srcInfo)
		if err := os.Chtimes(dest, atime, mtime); err != nil {
			return fmt.Errorf("preserve timestamps on %q: %w", dest, err)
		}
	}
	return nil
}

// GetFileSize returns the file size from a FileInfo, or 0 for directories/symlinks.
func GetFileSize(info fs.FileInfo) int64 {
	if info.Mode().IsRegular() {
		return info.Size()
	}
	return 0
}
