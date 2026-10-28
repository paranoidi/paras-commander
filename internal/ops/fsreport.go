package ops

import (
	"context"
	"os"

	"github.com/paranoidi/paras-commander/internal/fschange"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// The report* helpers tell ctx's fschange sink what a primitive just did to a local path. They are
// the one place remote paths are dropped; the subject is Lstat'ed (never followed) only when a
// sink listens, matching how disk-usage walks size entries.

// reportRenamed reports that from now lives at to.
func reportRenamed(ctx context.Context, from, to pathloc.Path) {
	reportRenamedHost(ctx, hostPathOf(from), hostPathOf(to))
}

func reportRenamedHost(ctx context.Context, from, to string) {
	if from == "" || to == "" || !fschange.Enabled(ctx) {
		return
	}
	if c, ok := statChange(fschange.Renamed, to); ok {
		c.From = from
		fschange.Report(ctx, c)
	}
}

// reportRemoved reports that loc and everything under it is gone.
func reportRemoved(ctx context.Context, loc pathloc.Path, isDir bool) {
	reportRemovedHost(ctx, hostPathOf(loc), isDir)
}

func reportRemovedHost(ctx context.Context, host string, isDir bool) {
	if host != "" {
		fschange.Report(ctx, fschange.Change{Kind: fschange.Removed, Path: host, IsDir: isDir})
	}
}

// reportDirty reports that loc changed by an unknown amount.
func reportDirty(ctx context.Context, loc pathloc.Path, isDir bool) {
	reportDirtyHost(ctx, hostPathOf(loc), isDir)
}

func reportDirtyHost(ctx context.Context, host string, isDir bool) {
	if host != "" {
		fschange.Report(ctx, fschange.Change{Kind: fschange.Dirty, Path: host, IsDir: isDir})
	}
}

// reportCreated reports that loc was just created (or a file there overwritten).
func reportCreated(ctx context.Context, loc pathloc.Path) {
	reportCreatedHost(ctx, hostPathOf(loc))
}

func reportCreatedHost(ctx context.Context, host string) {
	if host == "" || !fschange.Enabled(ctx) {
		return
	}
	if c, ok := statChange(fschange.Created, host); ok {
		fschange.Report(ctx, c)
	}
}

// statChange builds a change of kind for host from its Lstat; false when it is not there.
func statChange(kind fschange.Kind, host string) (fschange.Change, bool) {
	fi, err := os.Lstat(host)
	if err != nil {
		return fschange.Change{}, false
	}
	c := fschange.Change{Kind: kind, Path: host, IsDir: fi.IsDir()}
	if !c.IsDir {
		c.Bytes = fi.Size()
	}
	return c, true
}
