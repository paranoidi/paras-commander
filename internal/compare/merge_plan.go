package compare

import (
	"fmt"
	"path/filepath"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// MergeDirection selects which panel is the copy source root.
type MergeDirection int

const (
	MergeTowardSecondary MergeDirection = iota
	MergeTowardPrimary
)

// MergeOptions configures merge plan generation.
type MergeOptions struct {
	Direction    MergeDirection
	CopyMissing  bool
	CopyModified bool
	MoveMode     bool
}

// MergeCopyItem is one file copy in a merge plan.
type MergeCopyItem struct {
	Src string
	Dst string
}

// MergePlan is the resolved copy/move operations for a merge.
type MergePlan struct {
	Copies     []MergeCopyItem
	TotalBytes int64
}

// MergeInput collects selected paths and optional filter fallback scope.
type MergeInput struct {
	PrimarySelected   map[string]bool
	SecondarySelected map[string]bool
	Filter            Filter
}

// BuildMergePlan resolves copy/delete operations from snapshot rows in scope.
func BuildMergePlan(snap Snapshot, rows []Row, in MergeInput, opts MergeOptions) (MergePlan, error) {
	scope := mergeScopePaths(snap, rows, in)
	if len(scope) == 0 {
		return MergePlan{}, fmt.Errorf("no files in scope")
	}
	var plan MergePlan
	for _, row := range rows {
		if !rowInScope(row, snap, scope) {
			continue
		}
		if err := appendMergeCopies(&plan, snap, row, opts); err != nil {
			return MergePlan{}, err
		}
	}
	if len(plan.Copies) == 0 {
		return MergePlan{}, fmt.Errorf("nothing to do with current options")
	}
	return plan, nil
}

func mergeScopePaths(snap Snapshot, rows []Row, in MergeInput) map[string]bool {
	scope := make(map[string]bool)
	for p, on := range in.PrimarySelected {
		if on && p != "" {
			scope[filepath.Clean(p)] = true
		}
	}
	for p, on := range in.SecondarySelected {
		if on && p != "" {
			scope[filepath.Clean(p)] = true
		}
	}
	if len(scope) > 0 {
		return scope
	}
	for _, row := range rows {
		if p, err := rowPrimaryAbs(snap, row); err == nil && p != "" {
			scope[p] = true
		}
		if p, err := rowSecondaryAbs(snap, row); err == nil && p != "" {
			scope[p] = true
		}
	}
	return scope
}

func rowInScope(row Row, snap Snapshot, scope map[string]bool) bool {
	if p, err := rowPrimaryAbs(snap, row); err == nil && scope[p] {
		return true
	}
	if p, err := rowSecondaryAbs(snap, row); err == nil && scope[p] {
		return true
	}
	return false
}

func appendMergeCopies(plan *MergePlan, snap Snapshot, row Row, opts MergeOptions) error {
	if !opts.CopyMissing && !opts.CopyModified {
		return nil
	}
	switch row.Kind {
	case KindPrimaryOnly:
		if !opts.CopyMissing || opts.Direction != MergeTowardSecondary {
			return nil
		}
		src, err := rowPrimaryAbs(snap, row)
		if err != nil {
			return err
		}
		dst, err := JoinRel(snap.SecondaryRoot, row.PrimaryRel)
		if err != nil {
			return err
		}
		plan.addCopy(src, dst.String(), row.Size)
	case KindSecondaryOnly:
		if !opts.CopyMissing || opts.Direction != MergeTowardPrimary {
			return nil
		}
		src, err := rowSecondaryAbs(snap, row)
		if err != nil {
			return err
		}
		dst, err := JoinRel(snap.PrimaryRoot, row.SecondaryRel)
		if err != nil {
			return err
		}
		plan.addCopy(src, dst.String(), row.Size)
	case KindContentDiff:
		if !opts.CopyModified {
			return nil
		}
		if opts.Direction == MergeTowardSecondary {
			src, err := rowPrimaryAbs(snap, row)
			if err != nil {
				return err
			}
			dst, err := JoinRel(snap.SecondaryRoot, row.PrimaryRel)
			if err != nil {
				return err
			}
			plan.addCopy(src, dst.String(), row.Size)
			return nil
		}
		src, err := rowSecondaryAbs(snap, row)
		if err != nil {
			return err
		}
		dst, err := JoinRel(snap.PrimaryRoot, row.SecondaryRel)
		if err != nil {
			return err
		}
		plan.addCopy(src, dst.String(), row.Size)
	case KindRelocated:
		if !opts.CopyMissing {
			return nil
		}
		if opts.Direction == MergeTowardSecondary {
			src, err := rowPrimaryAbs(snap, row)
			if err != nil {
				return err
			}
			dst, err := JoinRel(snap.SecondaryRoot, row.SecondaryRel)
			if err != nil {
				return err
			}
			plan.addCopy(src, dst.String(), row.Size)
			return nil
		}
		src, err := rowSecondaryAbs(snap, row)
		if err != nil {
			return err
		}
		dst, err := JoinRel(snap.PrimaryRoot, row.PrimaryRel)
		if err != nil {
			return err
		}
		plan.addCopy(src, dst.String(), row.Size)
	}
	return nil
}

func (p *MergePlan) addCopy(src, dst string, size int64) {
	p.Copies = append(p.Copies, MergeCopyItem{Src: src, Dst: dst})
	p.TotalBytes += size
}

func rowPrimaryAbs(snap Snapshot, row Row) (string, error) {
	if row.PrimaryRel == "" {
		return "", nil
	}
	loc, err := JoinRel(snap.PrimaryRoot, row.PrimaryRel)
	if err != nil {
		return "", err
	}
	return filepath.Clean(loc.String()), nil
}

func rowSecondaryAbs(snap Snapshot, row Row) (string, error) {
	if row.SecondaryRel == "" {
		return "", nil
	}
	loc, err := JoinRel(snap.SecondaryRoot, row.SecondaryRel)
	if err != nil {
		return "", err
	}
	return filepath.Clean(loc.String()), nil
}

// PreviewMergePlan returns counts for dialog preview without error on empty plan.
func PreviewMergePlan(snap Snapshot, rows []Row, in MergeInput, opts MergeOptions) (copies int, bytes int64) {
	plan, err := BuildMergePlan(snap, rows, in, opts)
	if err != nil {
		return 0, 0
	}
	return len(plan.Copies), plan.TotalBytes
}

// CopyDestinationDir returns the parent directory to pass as a copy job destination.
func CopyDestinationDir(dstFile string) (pathloc.Path, error) {
	return pathloc.Parse(filepath.Dir(dstFile))
}
