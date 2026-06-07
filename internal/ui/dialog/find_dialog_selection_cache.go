package dialog

import (
	"fmt"
	"math"
	"strings"

	"github.com/paranoidi/paras-commander/internal/panel"
)

type findMarkedSelCache struct {
	pruneBuilt bool
	gen        uint64
	count      int
	hasDirs    bool
	pruned     []string

	labelBuilt   bool
	label        string
	labelPending bool
}

// MarkedSelectionSizePainter supplies directory subtree sizes for the find-dialog selection indicator.
type MarkedSelectionSizePainter interface {
	ByteSize(absPath string) (int64, bool)
	DiskScanExcluded(absPath string, descendIntoMountPoints bool, listingDev uint64, listingDevValid bool, goduIgnore func(string) bool) bool
}

// MarkedSelGen returns the marked-selection derived-cache generation.
func (s FindDialogState) MarkedSelGen() uint64 {
	return s.markedSelGen
}

// InvalidateMarkedSelectionDerived bumps the generation and clears cached prune/label state.
func (s *FindDialogState) InvalidateMarkedSelectionDerived() {
	s.markedSelGen++
	s.markedSelCache = findMarkedSelCache{}
}

// InvalidateMarkedSelectionSizeLabel drops only the cached label so the next read rebuilds
// byte totals from cached pruned roots (e.g. after disk-usage cache updates).
func (s *FindDialogState) InvalidateMarkedSelectionSizeLabel() {
	s.markedSelCache.labelBuilt = false
	s.markedSelCache.label = ""
	s.markedSelCache.labelPending = false
}

// PrunedMarkedRoots returns conflict-pruned marked paths for disk scans and size totals.
func (s *FindDialogState) PrunedMarkedRoots() []string {
	s.ensureMarkedPruned()
	if !s.markedSelCache.pruneBuilt {
		return nil
	}
	return s.markedSelCache.pruned
}

// MarkedSelectionSizeLabel returns the selection count/size indicator text for marked find rows.
func (s *FindDialogState) MarkedSelectionSizeLabel(
	remote bool,
	painter MarkedSelectionSizePainter,
	descendIntoMountPoints bool,
	goduIgnore func(string) bool,
	workingSym string,
) (label string, ok bool) {
	if len(s.MarkedPaths) == 0 {
		return "", false
	}
	s.ensureMarkedPruned()
	if s.markedSelCache.labelBuilt && s.markedSelCache.gen == s.markedSelGen {
		if s.markedSelCache.label == "" {
			return "", false
		}
		return s.markedSelCache.label, true
	}
	s.rebuildMarkedLabel(remote, painter, descendIntoMountPoints, goduIgnore, workingSym)
	if !s.markedSelCache.labelBuilt || s.markedSelCache.label == "" {
		return "", false
	}
	return s.markedSelCache.label, true
}

func (s *FindDialogState) ensureMarkedPruned() {
	if s.markedSelCache.pruneBuilt && s.markedSelCache.gen == s.markedSelGen {
		return
	}
	s.rebuildMarkedPruned()
}

func (s *FindDialogState) rebuildMarkedPruned() {
	cache := findMarkedSelCache{gen: s.markedSelGen}
	if len(s.MarkedPaths) == 0 {
		s.markedSelCache = cache
		return
	}
	paths := make([]string, 0, len(s.MarkedPaths))
	for p, on := range s.MarkedPaths {
		if on {
			paths = append(paths, p)
		}
	}
	if len(paths) == 0 {
		s.markedSelCache = cache
		return
	}
	isDir := s.markedPathIsDir()
	hasDirs := markedSelectionHasDirs(paths, isDir)
	pruned := panel.PruneNestedPathsForSelection(paths, !hasDirs, isDir)
	cache.pruneBuilt = true
	cache.count = len(paths)
	cache.hasDirs = hasDirs
	cache.pruned = pruned
	s.markedSelCache = cache
}

func (s *FindDialogState) rebuildMarkedLabel(
	remote bool,
	painter MarkedSelectionSizePainter,
	descendIntoMountPoints bool,
	goduIgnore func(string) bool,
	workingSym string,
) {
	s.ensureMarkedPruned()
	if !s.markedSelCache.pruneBuilt || s.markedSelCache.count == 0 {
		return
	}
	var total int64
	pending := false
	for _, p := range s.markedSelCache.pruned {
		_, b, pend := findMarkedPathImpact(
			p, s.PathIsDir, s.PathSize, remote,
			s.ListingDevice, s.ListingDeviceValid,
			painter, descendIntoMountPoints, goduIgnore,
		)
		total += b
		if pend {
			pending = true
		}
	}
	word := "items"
	if s.markedSelCache.count == 1 {
		word = "item"
	}
	label := fmt.Sprintf("%d %s (%s)", s.markedSelCache.count, word, findFormatSelectionByteSize(total))
	if pending && workingSym != "" {
		label += " " + workingSym
	}
	s.markedSelCache.labelBuilt = true
	s.markedSelCache.label = label
	s.markedSelCache.labelPending = pending
}

func (s *FindDialogState) markedPathIsDir() func(string) bool {
	return func(path string) bool {
		if s.PathIsDir == nil {
			return false
		}
		return s.PathIsDir[path]
	}
}

func markedSelectionHasDirs(paths []string, isDir func(string) bool) bool {
	if isDir == nil {
		return false
	}
	for _, p := range paths {
		if isDir(p) {
			return true
		}
	}
	return false
}

func findMarkedPathImpact(
	path string,
	pathIsDir map[string]bool,
	pathSize map[string]int64,
	remote bool,
	listingDevice uint64,
	listingDeviceValid bool,
	painter MarkedSelectionSizePainter,
	descendIntoMountPoints bool,
	goduIgnore func(string) bool,
) (files, bytes int64, pending bool) {
	isDir := pathIsDir != nil && pathIsDir[path]
	if !isDir {
		if pathSize != nil {
			if sz, ok := pathSize[path]; ok {
				return 1, sz, false
			}
		}
		return 1, 0, false
	}
	if remote {
		return 0, 0, false
	}
	if painter == nil {
		return 0, 0, true
	}
	if sz, ok := painter.ByteSize(path); ok {
		return 0, sz, false
	}
	if painter.DiskScanExcluded(path, descendIntoMountPoints, listingDevice, listingDeviceValid, goduIgnore) {
		return 0, 0, false
	}
	return 0, 0, true
}

func findFormatSelectionByteSize(n int64) string {
	if n < 0 {
		n = 0
	}
	const (
		KiB = int64(1024)
		MiB = KiB * 1024
		GiB = MiB * 1024
		TiB = GiB * 1024
	)
	switch {
	case n < KiB:
		return fmt.Sprintf("%d B", n)
	case n < MiB:
		return findFormatSelectionUnit(float64(n)/float64(KiB), "KiB")
	case n < GiB:
		return findFormatSelectionUnit(float64(n)/float64(MiB), "MiB")
	case n < TiB:
		return findFormatSelectionUnit(float64(n)/float64(GiB), "GiB")
	default:
		return findFormatSelectionUnit(float64(n)/float64(TiB), "TiB")
	}
}

func findFormatSelectionUnit(v float64, unit string) string {
	if v >= 100 {
		return fmt.Sprintf("%.0f %s", v, unit)
	}
	if v >= 10 && math.Abs(v-math.Round(v)) < 1e-6 {
		return fmt.Sprintf("%.0f %s", v, unit)
	}
	s := fmt.Sprintf("%.2f", v)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	return s + " " + unit
}
