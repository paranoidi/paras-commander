package jobs

import (
	"path/filepath"
	"slices"

	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// ComputeVolumeDevs stats each local source and the destination once and stores the
// deduplicated device IDs in j.VolumeDevs. Jobs with a remote destination store none
// (they never count as local volume contention, matching the prior per-check rule).
// Called from AddJob so every enqueue path caches the IDs before the job can run.
// ponytail: one stat per source at enqueue; if huge selections on a contended mount
// ever hurt, dedupe by parent directory here.
func (j *Job) ComputeVolumeDevs() {
	j.VolumeDevs = nil
	if j.Destination.IsRemote() {
		return
	}
	add := func(loc pathloc.Path) {
		host, err := loc.FilePath()
		if err != nil {
			return
		}
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
	for _, src := range j.Sources {
		add(src)
	}
	add(j.Destination)
}

// HasVolumeDev reports whether dev is one of the job's cached volume device IDs.
func (j *Job) HasVolumeDev(dev uint64) bool {
	return slices.Contains(j.VolumeDevs, dev)
}
