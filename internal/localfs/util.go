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
func CopyFile(ctx context.Context, src, dst string, bufSize int, preservePerms, preserveTimes, dir, syncAfterWrite, tryKernelFastCopy bool, extra CopyFileOpts, onWritten func(int64)) error {
	srcInfo, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("stat source %q: %w", src, err)
	}
	if IsSymlink(srcInfo) {
		return copySymlink(src, dst, dir)
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

	fastDone := false
	if tryKernelFastCopy {
		ok, ferr := tryKernelReflinkCopy(ctx, srcFile, dstFile, srcInfo.Size(), onWritten)
		if ferr != nil {
			_ = dstFile.Close()
			_ = os.Remove(target)
			return fmt.Errorf("copy content %q -> %q: %w", src, target, ferr)
		}
		fastDone = ok
	}
	if !fastDone && extra.CopyFileRange {
		ok, ferr := tryKernelFileRangeCopy(ctx, srcFile, dstFile, srcInfo.Size(), onWritten)
		if ferr != nil {
			_ = dstFile.Close()
			_ = os.Remove(target)
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
		dstWrapped := io.Writer(dstFile)
		if onWritten != nil {
			dstWrapped = &countingWriter{w: dstFile, fn: onWritten}
		}
		_, err = io.CopyBuffer(dstWrapped, srcWrapped, buf)
		if err != nil {
			_ = dstFile.Close()
			_ = os.Remove(target)
			return fmt.Errorf("copy content %q -> %q: %w", src, target, err)
		}
	}
	if syncAfterWrite {
		if err := dstFile.Sync(); err != nil {
			_ = dstFile.Close()
			_ = os.Remove(target)
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
			if err := CopyFile(context.Background(), childSrc, destDir, bufSize, preservePerms, preserveTimes, true, syncAfterWrite, false, CopyFileOpts{}, nil); err != nil {
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
	return walkDirRecursive(root, fn)
}

func walkDirRecursive(dir string, fn func(string, fs.FileInfo) error) error {
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if err := fn(dir, info); err != nil {
		return err
	}
	if !info.IsDir() {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.IsDir() {
			// walkDirRecursive calls fn on the directory, so no need to call it here.
			if err := walkDirRecursive(path, fn); err != nil {
				return err
			}
		} else {
			if err := fn(path, info); err != nil {
				return err
			}
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
