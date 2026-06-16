//go:build linux

package localfs

import (
	"context"
	"errors"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

func copySparseUserspace(ctx context.Context, srcFile, dstFile *os.File, size int64, buf []byte, onWritten func(int64)) error {
	if size == 0 {
		return nil
	}
	offset := int64(0)
	for offset < size {
		if err := ctx.Err(); err != nil {
			return err
		}
		dataStart, err := unix.Seek(int(srcFile.Fd()), offset, unix.SEEK_DATA)
		if err != nil {
			if errors.Is(err, unix.ENXIO) {
				break
			}
			return err
		}
		if dataStart >= size {
			break
		}
		holeStart, err := unix.Seek(int(srcFile.Fd()), dataStart, unix.SEEK_HOLE)
		if err != nil {
			if errors.Is(err, unix.ENXIO) {
				holeStart = size
			} else {
				return err
			}
		}
		if holeStart > size {
			holeStart = size
		}
		if dataStart > offset {
			if err := punchHole(dstFile, offset, dataStart-offset); err != nil {
				return err
			}
		}
		regionLen := holeStart - dataStart
		if regionLen > 0 {
			if _, err := srcFile.Seek(dataStart, io.SeekStart); err != nil {
				return err
			}
			if _, err := dstFile.Seek(dataStart, io.SeekStart); err != nil {
				return err
			}
			remaining := regionLen
			for remaining > 0 {
				if err := ctx.Err(); err != nil {
					return err
				}
				chunk := int64(len(buf))
				if chunk > remaining {
					chunk = remaining
				}
				n, err := io.ReadFull(srcFile, buf[:chunk])
				if err != nil {
					return err
				}
				written, err := dstFile.Write(buf[:n])
				if err != nil {
					return err
				}
				if onWritten != nil {
					onWritten(int64(written))
				}
				remaining -= int64(written)
			}
		}
		offset = holeStart
	}
	if offset < size {
		if err := punchHole(dstFile, offset, size-offset); err != nil {
			return err
		}
	}
	return dstFile.Truncate(size)
}

func punchHole(dstFile *os.File, offset, length int64) error {
	if length <= 0 {
		return nil
	}
	err := unix.Fallocate(int(dstFile.Fd()), unix.FALLOC_FL_PUNCH_HOLE|unix.FALLOC_FL_KEEP_SIZE, offset, length)
	if err == nil {
		return nil
	}
	_, err = dstFile.Seek(offset+length, io.SeekStart)
	return err
}
