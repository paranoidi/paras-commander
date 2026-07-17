package hashwalk

import (
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/diskusage"
)

// Settings are shared walk/hash knobs for compare and dedup scans.
type Settings struct {
	HashWorkers  int
	ReadBuffer   []byte
	MaxHashBytes int64
	ShouldSkip   diskusage.ShouldIgnoreFolder
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
		HashWorkers:  workers,
		ReadBuffer:   make([]byte, bufKiB*1024),
		MaxHashBytes: cfg.MaxHashBytes,
		ShouldSkip:   diskusage.ComposeListingVolumeIgnore(diskIgnore, volGate),
	}
}
