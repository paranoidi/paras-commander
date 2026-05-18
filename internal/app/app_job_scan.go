package app

import (
	"context"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ops"
)

func jobScanFunc() jobs.ScanFunc {
	return func(ctx context.Context, sources []string, destination string, hooks jobs.ScanWalkHooks) (jobs.ScanResult, error) {
		opts := ops.PlanBuildOptions{
			YieldEveryN: hooks.YieldEveryN,
			Yield:       hooks.Yield,
		}
		if hooks.OnPath != nil {
			opts.OnPath = hooks.OnPath
		}
		plan, totalItems, totalDirs, totalBytes, err := ops.BuildCopyPlanWithTotalsCtx(ctx, sources, destination, opts)
		if err != nil {
			return jobs.ScanResult{}, err
		}
		return jobs.ScanResult{
			Plan:       planItemsFromOps(plan),
			TotalFiles: totalItems,
			TotalDirs:  totalDirs,
			TotalBytes: totalBytes,
		}, nil
	}
}

func planItemsFromOps(items []ops.PlanItem) []jobs.PlanItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]jobs.PlanItem, len(items))
	for i, p := range items {
		out[i] = jobs.PlanItem{
			Src:       p.Src,
			Dst:       p.Dst,
			IsDir:     p.IsDir,
			IsSymlink: p.IsSymlink,
			FileSize:  p.FileSize,
		}
	}
	return out
}

func planItemsToOps(items []jobs.PlanItem) []ops.PlanItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]ops.PlanItem, len(items))
	for i, p := range items {
		out[i] = ops.PlanItem{
			Src:       p.Src,
			Dst:       p.Dst,
			IsDir:     p.IsDir,
			IsSymlink: p.IsSymlink,
			FileSize:  p.FileSize,
		}
	}
	return out
}
