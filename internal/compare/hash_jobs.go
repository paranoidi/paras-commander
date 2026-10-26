package compare

import (
	"cmp"
	"slices"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// hashJobsNeeded returns the content work classification needs, ordered smallest-first so
// quick wins land early in the UI. A same-path pair of equal size is one pair job compared
// with SameContent (chunked, stops at the first differing chunk); unequal size is ContentDiff
// without reading. Unpaired files are hashed in full only when their size appears among
// unpaired files on the other side (potential Relocated matches).
func hashJobsNeeded(primary, secondary []FileRecord) []hashJob {
	pByRel := indexFiles(primary)
	sByRel := indexFiles(secondary)

	var jobs []hashJob
	pUnpaired := make([]FileRecord, 0)
	sUnpaired := make([]FileRecord, 0)

	sAbs := make(map[string]pathloc.Path, len(secondary))
	for _, f := range secondary {
		sAbs[f.Rel] = f.Abs
		if _, ok := pByRel[f.Rel]; !ok {
			sUnpaired = append(sUnpaired, f)
		}
	}
	for _, f := range primary {
		s, ok := sByRel[f.Rel]
		if !ok {
			pUnpaired = append(pUnpaired, f)
			continue
		}
		if f.Size == s.size {
			jobs = append(jobs, hashJob{pair: true, rel: f.Rel, loc: f.Abs, other: sAbs[f.Rel], size: f.Size})
		}
	}

	pSizes := unpairedSizeSet(pByRel, sByRel, nil)
	sSizes := unpairedSizeSet(sByRel, pByRel, nil)
	for _, f := range pUnpaired {
		if sSizes[f.Size] {
			jobs = append(jobs, hashJob{side: 0, rel: f.Rel, loc: f.Abs, size: f.Size})
		}
	}
	for _, f := range sUnpaired {
		if pSizes[f.Size] {
			jobs = append(jobs, hashJob{side: 1, rel: f.Rel, loc: f.Abs, size: f.Size})
		}
	}
	slices.SortFunc(jobs, func(a, b hashJob) int {
		if c := cmp.Compare(a.size, b.size); c != 0 {
			return c
		}
		if c := cmp.Compare(a.rel, b.rel); c != 0 {
			return c
		}
		return cmp.Compare(a.side, b.side)
	})
	return jobs
}

// unpairedSizeSet returns sizes of files in byRel that have no same-rel counterpart in
// otherByRel and are not marked consumed (consumed may be nil), shared by hashJobsNeeded
// and Classify so both agree on which unpaired files are relocation-hash candidates.
func unpairedSizeSet(byRel, otherByRel map[string]fileMeta, consumed map[string]bool) map[int64]bool {
	out := make(map[int64]bool)
	for rel, f := range byRel {
		if consumed != nil && consumed[rel] {
			continue
		}
		if _, onOther := otherByRel[rel]; onOther {
			continue
		}
		out[f.size] = true
	}
	return out
}
