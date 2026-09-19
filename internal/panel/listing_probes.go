package panel

import (
	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/fsvol"
	"github.com/paranoidi/paras-commander/internal/gitignore"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// PathProbes carries the per-directory filesystem probes ApplyListing needs besides the entries
// themselves: the volume's free/total bytes (statfs), its device id (stat) and the enclosing git
// work-tree root (a .git stat per ancestor on first sight). Each is a round trip on a network
// mount, so callers that fetch a listing on a background goroutine compute these there too
// (ProbeListingPath) and hand them to ApplyListingWithProbes, keeping the UI goroutine free of
// filesystem calls when the listing lands.
type PathProbes struct {
	VolumeAvail uint64
	VolumeTotal uint64
	VolumeOK    bool
	Device      uint64
	DeviceOK    bool
	WorkRoot    string
}

// ProbeListingPath runs the filesystem probes for loc. includeVolume=false skips the statfs and
// device stat (the "heavy" probes SuppressHeavyPathProbes gates on a job-saturated volume); the
// git work-tree lookup always runs. Remote locations probe nothing.
func ProbeListingPath(loc pathloc.Path, includeVolume bool) PathProbes {
	var pr PathProbes
	if loc.IsRemote() {
		return pr
	}
	host, err := loc.FilePath()
	if err != nil {
		return pr
	}
	if includeVolume {
		pr.VolumeAvail, pr.VolumeTotal, pr.VolumeOK = fsvol.VolumeBytes(host)
		pr.Device, pr.DeviceOK = diskusage.PathDevice(host)
	}
	pr.WorkRoot = gitignore.ValidWorkTreeRoot(host)
	return pr
}
