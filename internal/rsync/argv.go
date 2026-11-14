// Package rsync builds and runs rsync command lines for the Rsync dialog: the one place that
// turns the dialog's options into an argv (BuildArgv, shared by the dialog's command preview and
// the job), reads rsync's progress output and runs the process as a jobs.Task. It has no UI
// dependencies.
package rsync

import (
	"strconv"
	"strings"

	"github.com/paranoidi/paras-commander/internal/cmdmacro"
)

// Mode is what an rsync run does about files that exist on only one side.
type Mode int

const (
	// ModeUpdate copies new and changed files and leaves everything else alone.
	ModeUpdate Mode = iota
	// ModeMirror also deletes destination files that are not in the source (--delete).
	ModeMirror
	// ModeMove deletes each source file once it is transferred (--remove-source-files). rsync
	// leaves the emptied source directories behind.
	ModeMove
)

// Options is everything BuildArgv needs.
type Options struct {
	// Sources are the paths to send, as rsync takes them: host paths or [user@]host:path.
	Sources []string
	// Dest is the destination directory, host path or [user@]host:path.
	Dest string
	Mode Mode

	// Archive is -a; without it the run still recurses (-r) but preserves nothing.
	Archive bool
	// Checksum is -c: compare files by checksum instead of size and time.
	Checksum bool
	// OnlyNewer is -u: skip files that are newer at the destination.
	OnlyNewer bool
	// SkipExisting is --ignore-existing.
	SkipExisting bool
	// ContentsOnly puts a trailing slash on every source, so a directory's contents land in Dest
	// instead of the directory itself.
	ContentsOnly bool
	// Compress is -z.
	Compress bool
	// DryRun is -n: report what would change and change nothing.
	DryRun bool

	// Exclude are --exclude patterns, one flag each.
	Exclude []string
	// BwLimit is --bwlimit in KiB/s (0 = unlimited).
	BwLimit int
	// Extra are the user's own arguments, placed before the paths.
	Extra []string
	// SSH is the remote shell command for -e ("" = rsync's default), set for a remote side.
	SSH string

	// Report adds the flags the job needs to read rsync's output: one line per file, and with
	// Progress2 the overall progress line. The dialog's preview leaves both off.
	Report bool
	// Progress2 is true for rsync 3.1 and newer, which has --info=progress2.
	Progress2 bool
}

// itemizeFormat is the --out-format of a dry run: itemized changes, one per line.
const itemizeFormat = "%i %n%L"

// BuildArgv returns the rsync command line for o, argv[0] being "rsync".
func BuildArgv(o Options) []string {
	argv := []string{"rsync"}
	if o.Archive {
		argv = append(argv, "-a")
	} else {
		argv = append(argv, "-r")
	}
	for _, f := range []struct {
		on   bool
		flag string
	}{
		{o.Checksum, "-c"},
		{o.OnlyNewer, "-u"},
		{o.SkipExisting, "--ignore-existing"},
		{o.Compress, "-z"},
		{o.DryRun, "-n"},
	} {
		if f.on {
			argv = append(argv, f.flag)
		}
	}
	switch o.Mode {
	case ModeMirror:
		argv = append(argv, "--delete")
	case ModeMove:
		argv = append(argv, "--remove-source-files")
	}
	if o.BwLimit > 0 {
		argv = append(argv, "--bwlimit="+strconv.Itoa(o.BwLimit))
	}
	for _, x := range o.Exclude {
		argv = append(argv, "--exclude="+x)
	}
	if o.SSH != "" {
		argv = append(argv, "-e", o.SSH)
	}
	if o.Report {
		switch {
		case o.DryRun:
			argv = append(argv, "--out-format="+itemizeFormat)
		case o.Progress2:
			argv = append(argv, "--info=progress2", "--no-inc-recursive", "--out-format=%n")
		default:
			argv = append(argv, "--out-format=%n")
		}
	}
	argv = append(argv, o.Extra...)
	for _, s := range o.Sources {
		if o.ContentsOnly {
			s = withTrailingSlash(s)
		}
		argv = append(argv, s)
	}
	// A trailing slash makes the destination a directory even when it does not exist yet, so a
	// single file is never renamed to the destination name.
	return append(argv, withTrailingSlash(o.Dest))
}

func withTrailingSlash(s string) string {
	if strings.HasSuffix(s, "/") {
		return s
	}
	return s + "/"
}

// DisplayArgs is argv as the tokens of the command preview: arguments the shell would split or
// expand are single-quoted, so a token is one argument.
func DisplayArgs(argv []string) []string {
	parts := make([]string, len(argv))
	for i, a := range argv {
		parts[i] = a
		if a == "" || strings.ContainsAny(a, " \t\n'\"\\$`!*?[]{}()<>|&;#~%") {
			parts[i] = cmdmacro.QuoteShellArg(a)
		}
	}
	return parts
}

// Display is argv as one line for the command preview (DisplayArgs joined by spaces).
func Display(argv []string) string {
	return strings.Join(DisplayArgs(argv), " ")
}
