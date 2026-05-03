package fsvol

// VolumeBytes reports available and total capacity for the volume (mount point)
// that contains path. ok is false when the host OS cannot determine values.
func VolumeBytes(path string) (avail uint64, total uint64, ok bool) {
	return volumeBytes(path)
}
