package archive

import (
	"fmt"
	"path/filepath"
	"strings"
)

// BuildArgv returns argv for extracting archivePath into destDir using tc.
// Container tools use noninteractive keep-existing flags (unzip -n, 7z -aos,
// unrar -o-, tar -k) so members already in dest are not replaced.
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
		return []string{tool, "-C", destDir, "-k", "-xvjf", archivePath}, nil
	case FormatTarGz, FormatTgz:
		return []string{tool, "-C", destDir, "-k", "-xvzf", archivePath}, nil
	case FormatTarXz:
		return []string{tool, "-C", destDir, "-k", "-xvf", archivePath}, nil
	case FormatTarZst:
		return []string{tool, "-C", destDir, "--zstd", "-k", "-xvf", archivePath}, nil
	case FormatTar:
		return []string{tool, "-C", destDir, "-k", "-xvf", archivePath}, nil
	case FormatZip, FormatJar:
		return []string{tool, "-n", archivePath, "-d", destDir}, nil
	case FormatSevenZ:
		return []string{tool, "x", archivePath, "-o" + destDir, "-aos"}, nil
	case FormatRar:
		return []string{tool, "x", "-o-", archivePath, destDir + string(filepath.Separator)}, nil
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

// passwordMarkers are the lowercase fragments the extract tools print when an archive is
// encrypted and no (or the wrong) password was given: unzip "unable to get password" /
// "incorrect password", 7z "Enter password" / "Wrong password", unrar "Enter password" /
// "password is incorrect" / "Incorrect password".
var passwordMarkers = []string{
	"unable to get password", "incorrect password", "enter password", "wrong password", "password is incorrect",
}

// PasswordFailure reports whether a failed extract's tool output shows that the archive needs a
// password or that the one given was rejected. Only formats that can be encrypted qualify.
func PasswordFailure(f Format, output string) bool {
	switch f {
	case FormatZip, FormatJar, FormatSevenZ, FormatRar:
	default:
		return false
	}
	output = strings.ToLower(output)
	for _, m := range passwordMarkers {
		if strings.Contains(output, m) {
			return true
		}
	}
	return false
}

// PasswordArgv is BuildArgv with pw applied, plus the stdin the tool reads it from (nil when the
// password is in argv). 7z and unrar read it from stdin when they have no terminal; unzip only
// takes -P.
func PasswordArgv(f Format, archivePath, destDir string, tc Toolchain, pw string) (argv []string, stdin []byte, err error) {
	argv, err = BuildArgv(f, archivePath, destDir, tc)
	if err != nil || pw == "" {
		return argv, nil, err
	}
	switch f {
	case FormatZip, FormatJar:
		// ponytail: unzip has no stdin or env password option, so pw is visible in the process
		// list while it runs.
		argv = append([]string{argv[0], "-P", pw}, argv[1:]...)
	case FormatSevenZ, FormatRar:
		stdin = []byte(pw + "\n")
	}
	return argv, stdin, nil
}
