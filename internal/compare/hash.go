package compare

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"

	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// HashFile reads loc and returns SHA-256 of its contents.
func HashFile(ctx context.Context, loc pathloc.Path, buf []byte, maxBytes int64) ([32]byte, error) {
	if maxBytes > 0 {
		ent, err := fsbackend.Default().Stat(ctx, loc)
		if err != nil {
			return [32]byte{}, err
		}
		if ent.Size > maxBytes {
			return [32]byte{}, fmt.Errorf("file exceeds compare max_hash_bytes (%d > %d)", ent.Size, maxBytes)
		}
	}
	be, err := fsbackend.Default().Backend(loc)
	if err != nil {
		return [32]byte{}, err
	}
	rc, err := be.OpenRead(ctx, loc)
	if err != nil {
		return [32]byte{}, err
	}
	defer func() { _ = rc.Close() }()

	h := sha256.New()
	if len(buf) == 0 {
		buf = make([]byte, 32*1024)
	}
	for {
		if ctx.Err() != nil {
			return [32]byte{}, ctx.Err()
		}
		n, readErr := rc.Read(buf)
		if n > 0 {
			if _, werr := h.Write(buf[:n]); werr != nil {
				return [32]byte{}, werr
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return [32]byte{}, readErr
		}
	}
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum, nil
}
