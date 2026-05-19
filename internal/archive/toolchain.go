package archive

import (
	"os/exec"
)

// Toolchain holds resolved binary paths from PATH (empty when missing).
type Toolchain struct {
	Tar        string
	Unzip      string
	SevenZ     string
	Unrar      string
	Gzip       string
	Bzip2      string
	Xz         string
	Uncompress string
}

// ProbeToolchain resolves external tools via exec.LookPath.
func ProbeToolchain() Toolchain {
	return Toolchain{
		Tar:        firstLookPath("tar", "gtar", "bsdtar"),
		Unzip:      look("unzip"),
		SevenZ:     firstLookPath("7z", "7za", "7zz"),
		Unrar:      look("unrar"),
		Gzip:       firstLookPath("gzip", "gunzip"),
		Bzip2:      firstLookPath("bzip2", "bunzip2"),
		Xz:         firstLookPath("xz", "unxz"),
		Uncompress: look("uncompress"),
	}
}

func look(name string) string {
	p, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return p
}

func firstLookPath(names ...string) string {
	for _, n := range names {
		if p := look(n); p != "" {
			return p
		}
	}
	return ""
}

// Available reports whether the toolchain has the binary needed for f.
func (f Format) Available(tc Toolchain) bool {
	switch f {
	case FormatTarBz2, FormatTarGz, FormatTarXz, FormatTarZst, FormatTar, FormatTbz2, FormatTgz:
		return tc.Tar != ""
	case FormatZip, FormatJar:
		return tc.Unzip != ""
	case FormatSevenZ:
		return tc.SevenZ != ""
	case FormatRar:
		return tc.Unrar != ""
	case FormatGz:
		return tc.Gzip != ""
	case FormatBz2:
		return tc.Bzip2 != ""
	case FormatXz:
		return tc.Xz != ""
	case FormatZ:
		return tc.Uncompress != ""
	default:
		return false
	}
}

// ToolPath returns the resolved binary path for f.
func (f Format) ToolPath(tc Toolchain) string {
	switch f {
	case FormatTarBz2, FormatTarGz, FormatTarXz, FormatTarZst, FormatTar, FormatTbz2, FormatTgz:
		return tc.Tar
	case FormatZip, FormatJar:
		return tc.Unzip
	case FormatSevenZ:
		return tc.SevenZ
	case FormatRar:
		return tc.Unrar
	case FormatGz:
		return tc.Gzip
	case FormatBz2:
		return tc.Bzip2
	case FormatXz:
		return tc.Xz
	case FormatZ:
		return tc.Uncompress
	default:
		return ""
	}
}
