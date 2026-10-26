package compare

import (
	"bytes"
	"context"
	"crypto/sha256"
	"math"

	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// SameContent reports whether a and b hold identical bytes, using the same chunked prefix hashing
// as the dedup scan and bailing out at the first differing chunk. It is the one pairwise content
// check: panel compare and conflict verification both use it. chunkBytes <= 0 hashes each whole
// file in one round. Files of different size are different without reading either. onRead, when
// set, receives the cumulative bytes read from both files.
func SameContent(ctx context.Context, a, b pathloc.Path, chunkBytes int64, buf []byte, onRead func(read int64)) (bool, error) {
	ea, err := fsbackend.Default().Stat(ctx, a)
	if err != nil {
		return false, err
	}
	eb, err := fsbackend.Default().Stat(ctx, b)
	if err != nil {
		return false, err
	}
	if ea.Size != eb.Size {
		return false, nil
	}
	if chunkBytes <= 0 {
		chunkBytes = math.MaxInt64
	}
	if len(buf) == 0 {
		buf = make([]byte, 32*1024)
	}
	var total int64
	report := func(base int64) func(int64) {
		return func(read int64) {
			if onRead != nil {
				onRead(base + read)
			}
		}
	}
	for offset := int64(0); offset < ea.Size; {
		n := min(chunkBytes, ea.Size-offset)
		ha, hb := sha256.New(), sha256.New()
		if err := hashChunk(ctx, a, ha, buf, offset, n, report(total)); err != nil {
			return false, err
		}
		if err := hashChunk(ctx, b, hb, buf, offset, n, report(total+n)); err != nil {
			return false, err
		}
		total += 2 * n
		if !bytes.Equal(ha.Sum(nil), hb.Sum(nil)) {
			return false, nil
		}
		offset += n
	}
	return true, nil
}
