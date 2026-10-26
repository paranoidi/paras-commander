package compare

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// SameContent reports whether a and b hold identical bytes. Each file is opened once and both
// are read in step, half of buf each, stopping at the first differing block; a file that ends
// before the other differs. It is the one pairwise content check: panel compare and conflict
// verification both use it, after ruling out different sizes from metadata. An empty buf uses
// config.DefaultCompareReadBufferKiB. onRead, when set, receives the cumulative bytes read from
// both files.
func SameContent(ctx context.Context, a, b pathloc.Path, buf []byte, onRead func(read int64)) (bool, error) {
	ra, err := openContent(ctx, a)
	if err != nil {
		return false, err
	}
	defer func() { _ = ra.Close() }()
	rb, err := openContent(ctx, b)
	if err != nil {
		return false, err
	}
	defer func() { _ = rb.Close() }()
	if len(buf) < 2 {
		buf = make([]byte, config.DefaultCompareReadBufferKiB*1024)
	}
	half := len(buf) / 2
	ba, bb := buf[:half], buf[half:2*half]
	var total int64
	for {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		na, errA := readBlock(ra, ba)
		if errA != nil {
			return false, errA
		}
		nb, errB := readBlock(rb, bb)
		if errB != nil {
			return false, errB
		}
		total += int64(na + nb)
		if onRead != nil {
			onRead(total)
		}
		if na != nb || !bytes.Equal(ba[:na], bb[:nb]) {
			return false, nil
		}
		if na < half {
			return true, nil // both ended at the same offset
		}
	}
}

func openContent(ctx context.Context, loc pathloc.Path) (io.ReadCloser, error) {
	be, err := fsbackend.Default().Backend(loc)
	if err != nil {
		return nil, err
	}
	return be.OpenRead(ctx, loc)
}

// readBlock fills buf from r; a short count without error means r ended.
func readBlock(r io.Reader, buf []byte) (int, error) {
	n, err := io.ReadFull(r, buf)
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		err = nil
	}
	return n, err
}
