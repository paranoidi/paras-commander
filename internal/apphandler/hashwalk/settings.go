package hashwalk

import (
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/diskusage"
)

// Settings are shared walk/hash knobs for compare and dedup scans.
type Settings struct {
	HashWorkers int
	// ReadBufferBytes is the per-reader buffer size (config compare.read_buffer_kib).
	ReadBufferBytes int
	// ChunkBytes is the find-duplicates hashing chunk (config compare.chunk_bytes).
	ChunkBytes int64
	// FirstChunkBytes is the first find-duplicates round's read (config
	// compare.first_chunk_bytes); rounds double up to ChunkBytes.
	FirstChunkBytes int64
	ShouldSkip      diskusage.ShouldIgnoreFolder
}

// FromCompareConfig builds hash/walk settings from compare config and ignore hooks.
func FromCompareConfig(
	cfg config.CompareConfig,
	diskIgnore diskusage.ShouldIgnoreFolder,
	volGate diskusage.ListingVolumeGate,
) Settings {
	bufKiB := cfg.ReadBufferKiB
	if bufKiB <= 0 {
		bufKiB = config.DefaultCompareReadBufferKiB
	}
	workers := cfg.HashConcurrency
	if workers <= 0 {
		workers = config.DefaultCompareHashConcurrency
	}
	return Settings{
		HashWorkers:     workers,
		ReadBufferBytes: bufKiB * 1024,
		ChunkBytes:      cfg.ChunkBytes,
		FirstChunkBytes: cfg.FirstChunkBytes,
		ShouldSkip:      diskusage.ComposeListingVolumeIgnore(diskIgnore, volGate),
	}
}
