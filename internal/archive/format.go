// Package archive maps archive filenames to external extract commands.
package archive

import (
	"path/filepath"
	"strings"
)

// Format identifies an archive type by suffix.
type Format int

const (
	FormatUnknown Format = iota
	FormatTarBz2
	FormatTarGz
	FormatTarXz
	FormatTarZst
	FormatTar
	FormatTbz2
	FormatTgz
	FormatZip
	FormatJar
	FormatRar
	FormatSevenZ
	FormatGz
	FormatBz2
	FormatXz
	FormatZ
)

// suffixEntry pairs a lowercase suffix with its format (longest matches first).
type suffixEntry struct {
	suffix string
	format Format
}

// suffixTable is ordered longest-first so .tar.gz beats .gz.
var suffixTable = []suffixEntry{
	{".tar.bz2", FormatTarBz2},
	{".tar.gz", FormatTarGz},
	{".tar.xz", FormatTarXz},
	{".tar.zst", FormatTarZst},
	{".tbz2", FormatTbz2},
	{".tgz", FormatTgz},
	{".tar", FormatTar},
	{".zip", FormatZip},
	{".jar", FormatJar},
	{".rar", FormatRar},
	{".7z", FormatSevenZ},
	{".bz2", FormatBz2},
	{".gz", FormatGz},
	{".xz", FormatXz},
	{".z", FormatZ},
}

// FormatForName returns the archive format for a file basename or path.
func FormatForName(name string) (Format, bool) {
	base := strings.ToLower(filepath.Base(name))
	for _, e := range suffixTable {
		if strings.HasSuffix(base, e.suffix) {
			return e.format, true
		}
	}
	return FormatUnknown, false
}

// Suffix returns the matched suffix for f (lowercase, including dot).
func (f Format) Suffix() string {
	switch f {
	case FormatTarBz2:
		return ".tar.bz2"
	case FormatTarGz:
		return ".tar.gz"
	case FormatTarXz:
		return ".tar.xz"
	case FormatTarZst:
		return ".tar.zst"
	case FormatTar:
		return ".tar"
	case FormatTbz2:
		return ".tbz2"
	case FormatTgz:
		return ".tgz"
	case FormatZip:
		return ".zip"
	case FormatJar:
		return ".jar"
	case FormatRar:
		return ".rar"
	case FormatSevenZ:
		return ".7z"
	case FormatGz:
		return ".gz"
	case FormatBz2:
		return ".bz2"
	case FormatXz:
		return ".xz"
	case FormatZ:
		return ".z"
	default:
		return ""
	}
}

// NeedsStdoutSink reports formats that decompress to stdout (single output file).
func (f Format) NeedsStdoutSink() bool {
	switch f {
	case FormatGz, FormatBz2, FormatXz, FormatZ:
		return true
	default:
		return false
	}
}

// OutputBasename returns the decompressed filename inside dest for stream formats.
func OutputBasename(archivePath string, f Format) string {
	base := filepath.Base(archivePath)
	suf := f.Suffix()
	if suf == "" {
		return base
	}
	if strings.HasSuffix(strings.ToLower(base), suf) {
		return base[:len(base)-len(suf)]
	}
	return base
}

// RequiredToolName returns the human tool name for error messages.
func (f Format) RequiredToolName() string {
	switch f {
	case FormatTarBz2, FormatTarGz, FormatTarXz, FormatTarZst, FormatTar, FormatTbz2, FormatTgz:
		return "tar"
	case FormatZip, FormatJar:
		return "unzip"
	case FormatSevenZ:
		return "7z"
	case FormatRar:
		return "unrar"
	case FormatGz:
		return "gzip"
	case FormatBz2:
		return "bzip2"
	case FormatXz:
		return "xz"
	case FormatZ:
		return "uncompress"
	default:
		return ""
	}
}
