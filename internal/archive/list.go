package archive

import (
	"strings"
)

// Listable reports whether f can show a path listing (multi-member archives).
// Single-stream compressors (.gz/.bz2/.xz/.z) are not listable.
func (f Format) Listable() bool {
	switch f {
	case FormatTarBz2, FormatTarGz, FormatTarXz, FormatTarZst, FormatTar, FormatTbz2, FormatTgz,
		FormatZip, FormatJar, FormatRar, FormatSevenZ:
		return true
	default:
		return false
	}
}

// ListableName reports whether name/path is a listable archive by suffix.
func ListableName(name string) bool {
	f, ok := FormatForName(name)
	return ok && f.Listable()
}

// ListArgv returns argv to list member paths of archivePath with toolPath.
func ListArgv(f Format, toolPath, archivePath string) []string {
	switch f {
	case FormatTarBz2, FormatTarGz, FormatTarXz, FormatTarZst, FormatTar, FormatTbz2, FormatTgz:
		return []string{toolPath, "-tf", archivePath}
	case FormatZip, FormatJar:
		return []string{toolPath, "-Z1", archivePath}
	case FormatRar:
		return []string{toolPath, "lb", "--", archivePath}
	case FormatSevenZ:
		return []string{toolPath, "l", "-ba", "-slt", archivePath}
	default:
		return nil
	}
}

// ParseListing extracts member paths from tool stdout for f.
// Trailing "/" marks directories; other formats yield non-empty lines as paths.
func ParseListing(f Format, stdout []byte) []string {
	if f == FormatSevenZ {
		return parseSevenZListing(stdout)
	}
	var out []string
	for _, line := range strings.Split(string(stdout), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

func parseSevenZListing(stdout []byte) []string {
	var out []string
	for _, line := range strings.Split(string(stdout), "\n") {
		line = strings.TrimRight(line, "\r")
		const prefix = "Path = "
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		p := strings.TrimSpace(strings.TrimPrefix(line, prefix))
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}
