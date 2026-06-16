package ops

import (
	"context"
	"fmt"
	"io/fs"
	"os"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

func mkdirModeForItem(item PlanItem, opts Options) fs.FileMode {
	if opts.PreservePermissions && item.Mode != 0 {
		return item.Mode
	}
	return 0o755
}

func applyItemMetadata(ctx context.Context, item PlanItem, opts Options) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !opts.PreservePermissions && !opts.PreserveTimestamps {
		return nil
	}
	if item.Dst.Scheme() != pathloc.SchemeFile {
		return nil
	}
	host, err := item.Dst.FilePath()
	if err != nil {
		return err
	}
	if opts.PreservePermissions && item.Mode != 0 {
		if err := os.Chmod(host, item.Mode); err != nil {
			return fmt.Errorf("preserve permissions on %q: %w", host, err)
		}
	}
	if opts.PreserveTimestamps && !item.ModTime.IsZero() {
		atime := item.AccessTime
		if atime.IsZero() {
			atime = item.ModTime
		}
		if err := os.Chtimes(host, atime, item.ModTime); err != nil {
			return fmt.Errorf("preserve timestamps on %q: %w", host, err)
		}
	}
	return nil
}
