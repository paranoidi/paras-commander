package archive

import (
	"fmt"
	"path/filepath"
)

// BuildArgv returns argv for extracting archivePath into destDir using tc.
// Stream formats (.gz, .bz2, .xz, .Z) use NeedsStdoutSink; BuildArgv still returns
// the decompress argv (caller writes stdout to OutputBasename).
func BuildArgv(f Format, archivePath, destDir string, tc Toolchain) ([]string, error) {
	if !f.Available(tc) {
		return nil, fmt.Errorf("%s not found", f.RequiredToolName())
	}
	archivePath = filepath.Clean(archivePath)
	destDir = filepath.Clean(destDir)
	tool := f.ToolPath(tc)
	switch f {
	case FormatTarBz2, FormatTbz2:
		return []string{tool, "-C", destDir, "-xvjf", archivePath}, nil
	case FormatTarGz, FormatTgz:
		return []string{tool, "-C", destDir, "-xvzf", archivePath}, nil
	case FormatTarXz:
		return []string{tool, "-C", destDir, "-xvf", archivePath}, nil
	case FormatTarZst:
		return []string{tool, "-C", destDir, "--zstd", "-xvf", archivePath}, nil
	case FormatTar:
		return []string{tool, "-C", destDir, "-xvf", archivePath}, nil
	case FormatZip, FormatJar:
		return []string{tool, "-o", archivePath, "-d", destDir}, nil
	case FormatSevenZ:
		return []string{tool, "x", archivePath, "-o" + destDir, "-y"}, nil
	case FormatRar:
		return []string{tool, "x", "-o+", archivePath, destDir + string(filepath.Separator)}, nil
	case FormatGz:
		return []string{tool, "-dc", archivePath}, nil
	case FormatBz2:
		return []string{tool, "-dc", archivePath}, nil
	case FormatXz:
		return []string{tool, "-dc", archivePath}, nil
	case FormatZ:
		return []string{tool, "-c", archivePath}, nil
	default:
		return nil, fmt.Errorf("unsupported archive format")
	}
}
