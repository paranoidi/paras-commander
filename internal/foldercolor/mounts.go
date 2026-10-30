package foldercolor

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// Test hooks: where the mount table and the udev symlinks live, and how a device node's number
// is read (a regular-file fixture has none).
var (
	mountInfoPath = "/proc/self/mountinfo"
	diskDir       = "/dev/disk"
	devNumOf      = realDevNum
)

// realDevNum returns the "major:minor" of the block device behind a symlink.
func realDevNum(p string) (string, bool) {
	var st unix.Stat_t
	if unix.Stat(p, &st) != nil || st.Mode&unix.S_IFMT != unix.S_IFBLK {
		return "", false
	}
	return fmt.Sprintf("%d:%d", unix.Major(uint64(st.Rdev)), unix.Minor(uint64(st.Rdev))), true
}

// devMount is one mounted filesystem that has a GVfs device tree ("uuid-*" or "label-*").
type devMount struct {
	tree  string // metadata database name
	point string // absolute mount point
	root  string // path inside the filesystem that is mounted (mountinfo "root", "/" unless bind or subvolume)
}

// mountCache keeps the last deviceMounts result keyed on the raw mount table, so an unchanged
// table (the usual poll) costs one procfs read and no udev scan.
type mountCache struct {
	raw    []byte
	mounts []devMount
}

// deviceMounts lists the non-"/" mounts whose block device carries a filesystem UUID or label,
// named the way gvfsd-metadata names them: udev's by-uuid / by-label links use the same escaped
// strings, and the UUID wins over the label. Anonymous devices (tmpfs, btrfs subvolumes) have no
// link; gvfs files them under "root". Links are stat'ed only until every mounted device is named.
// ponytail: relabeling a still-mounted drive is not seen until the mount table changes.
func (c *mountCache) deviceMounts() []devMount {
	data, err := os.ReadFile(mountInfoPath)
	if err != nil {
		return nil
	}
	if c.raw != nil && bytes.Equal(data, c.raw) {
		return c.mounts
	}
	type mnt struct{ dev, root, point string }
	var mnts []mnt
	trees := map[string]string{} // "major:minor" -> tree ("" = still unnamed)
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Fields(line)
		if len(f) < 5 || unescapeMount(f[4]) == "/" {
			continue
		}
		mnts = append(mnts, mnt{dev: f[2], root: unescapeMount(f[3]), point: unescapeMount(f[4])})
		trees[f[2]] = ""
	}
	unnamed := len(trees)
	for _, kind := range []string{"uuid", "label"} {
		if unnamed == 0 {
			break
		}
		ents, _ := os.ReadDir(filepath.Join(diskDir, "by-"+kind))
		for _, e := range ents {
			dev, ok := devNumOf(filepath.Join(diskDir, "by-"+kind, e.Name()))
			if name, mounted := trees[dev]; ok && mounted && name == "" {
				trees[dev] = kind + "-" + e.Name()
				if unnamed--; unnamed == 0 {
					break
				}
			}
		}
	}
	var out []devMount
	for _, m := range mnts {
		if trees[m.dev] != "" {
			out = append(out, devMount{tree: trees[m.dev], point: m.point, root: m.root})
		}
	}
	c.raw, c.mounts = data, out
	return out
}

// unescapeMount decodes the octal escapes (\040 for a space) of a mountinfo path field.
func unescapeMount(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+4 <= len(s) {
			var v int
			if _, err := fmt.Sscanf(s[i+1:i+4], "%03o", &v); err == nil {
				b.WriteByte(byte(v))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// abs maps a database path ("/..." inside the filesystem) to the absolute path under the mount
// point; false when the entry lies outside the mounted root.
func (m devMount) abs(p string) (string, bool) {
	if m.root != "/" {
		if p != m.root && !strings.HasPrefix(p, m.root+"/") {
			return "", false
		}
		p = strings.TrimPrefix(p, m.root)
	}
	return filepath.Join(m.point, p), true
}
