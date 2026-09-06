package jobs

import (
	"path/filepath"
	"slices"

	"github.com/paranoidi/paras-commander/internal/diskusage"
)

// ComputeVolumeDevs stats one directory per distinct source parent plus the destination
// and stores the deduplicated device IDs in j.VolumeDevs. Jobs with a remote destination
// store none (they never count as local volume contention, matching the prior per-check
// rule). Called from AddJob so every enqueue path caches the IDs before the job can run.
//
// Sources are grouped by parent directory rather than stat'd individually: AddJob runs on
// the UI thread, and one stat per source would freeze the app for len(Sources) x round-trip
// latency when a large selection is queued from a network mount. A source's own device
// differs from its parent's only when the source is itself a mount point, which this
// contention heuristic can afford to miss.
func (j *Job) ComputeVolumeDevs() {
	j.VolumeDevs = nil
	if j.Destination.IsRemote() {
		return
	}
	add := func(host string) {
		host = filepath.Clean(host)
		if host == "" || host == "." {
			return
		}
		dev, ok := diskusage.PathDevice(host)
		if !ok {
			return
		}
		if slices.Contains(j.VolumeDevs, dev) {
			return
		}
		j.VolumeDevs = append(j.VolumeDevs, dev)
	}
	seenDirs := make(map[string]struct{})
	for _, src := range j.Sources {
		host, err := src.FilePath()
		if err != nil {
			continue
		}
		dir := filepath.Dir(filepath.Clean(host))
		if _, ok := seenDirs[dir]; ok {
			continue
		}
		seenDirs[dir] = struct{}{}
		add(dir)
	}
	if host, err := j.Destination.FilePath(); err == nil {
		add(host)
	}
}

// HasVolumeDev reports whether dev is one of the job's cached volume device IDs.
func (j *Job) HasVolumeDev(dev uint64) bool {
	return slices.Contains(j.VolumeDevs, dev)
}
