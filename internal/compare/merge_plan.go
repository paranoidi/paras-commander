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

// NameSide picks the compare column whose relative path a Relocated row's file takes on merge.
type NameSide int

const (
	NameDefault   NameSide = iota // follow MergeOptions.RelocatedUseSource
	NamePrimary                   // the primary (left) column's path
	NameSecondary                 // the secondary (right) column's path
)

// MergeOptions configures merge plan generation.
type MergeOptions struct {
	Direction    MergeDirection
	CopyMissing  bool
	CopyModified bool
	MoveMode     bool
	// RelocatedUseSource renames the destination's copy of each Relocated row to the source
	// side's path; false keeps the destination's name (nothing to do).
	RelocatedUseSource bool
	// RelocatedNames overrides RelocatedUseSource per row (absent or NameDefault: follow it).
	RelocatedNames map[RowKey]NameSide
}

// MergeCopyItem is one file copy in a merge plan.
type MergeCopyItem struct {
	Src string
	Dst string
}

// MergePlan is the resolved copy/move operations for a merge. Renames move a Relocated row's
// destination file to the source's relative path inside the destination tree; MoveMode does
// not apply to them.
type MergePlan struct {
	Copies     []MergeCopyItem
	Renames    []MergeCopyItem
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
		if err := appendMergeRename(&plan, snap, row, opts); err != nil {
			return MergePlan{}, err
		}
	}
	if len(plan.Copies) == 0 && len(plan.Renames) == 0 {
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
	}
	return nil
}

// RelocatedNameSide resolves which side's path a Relocated row's file takes when merging in
// direction: the row's override, else the source side when useSource, else the destination.
func RelocatedNameSide(row Row, direction MergeDirection, useSource bool, names map[RowKey]NameSide) NameSide {
	if side := names[row.Key()]; side != NameDefault {
		return side
	}
	sourceIsPrimary := direction == MergeTowardSecondary
	if useSource == sourceIsPrimary {
		return NamePrimary
	}
	return NameSecondary
}

// appendMergeRename adds the in-destination rename for a Relocated row whose resolved name
// side is the source; a row keeping the destination's name needs nothing.
func appendMergeRename(plan *MergePlan, snap Snapshot, row Row, opts MergeOptions) error {
	if row.Kind != KindRelocated {
		return nil
	}
	side := RelocatedNameSide(row, opts.Direction, opts.RelocatedUseSource, opts.RelocatedNames)
	root, from, to := snap.SecondaryRoot, row.SecondaryRel, row.PrimaryRel
	if opts.Direction == MergeTowardPrimary {
		root, from, to = snap.PrimaryRoot, row.PrimaryRel, row.SecondaryRel
	}
	if (side == NamePrimary) == (opts.Direction == MergeTowardPrimary) {
		return nil // the destination's own name wins
	}
	src, err := JoinRel(root, from)
	if err != nil {
		return err
	}
	dst, err := JoinRel(root, to)
	if err != nil {
		return err
	}
	plan.Renames = append(plan.Renames, MergeCopyItem{Src: src.String(), Dst: dst.String()})
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

// PreviewMergePlan returns the plan for the dialog preview, empty instead of an error.
func PreviewMergePlan(snap Snapshot, rows []Row, in MergeInput, opts MergeOptions) MergePlan {
	plan, err := BuildMergePlan(snap, rows, in, opts)
	if err != nil {
		return MergePlan{}
	}
	return plan
}

// CopyDestinationDir returns the parent directory to pass as a copy job destination.
func CopyDestinationDir(dstFile string) (pathloc.Path, error) {
	return pathloc.Parse(filepath.Dir(dstFile))
}
