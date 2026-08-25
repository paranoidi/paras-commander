package compare

import (
	"cmp"
	"slices"
)

// hashJobsNeeded returns the subset of files that must be content-hashed for
// classification, ordered smallest-first so quick wins land early in the UI.
// Same-path pairs with unequal size are ContentDiff without hashing. Unpaired
// files are hashed only when their size appears among unpaired files on the
// other side (potential Relocated matches).
func hashJobsNeeded(primary, secondary []FileRecord) []hashJob {
	pByRel := indexFiles(primary)
	sByRel := indexFiles(secondary)

	var jobs []hashJob
	pUnpaired := make([]FileRecord, 0)
	sUnpaired := make([]FileRecord, 0)

	for _, f := range primary {
		s, ok := sByRel[f.Rel]
		if !ok {
			pUnpaired = append(pUnpaired, f)
			continue
		}
		if f.Size == s.size {
			jobs = append(jobs, hashJob{side: 0, rel: f.Rel, loc: f.Abs, size: f.Size})
		}
	}
	for _, f := range secondary {
		p, ok := pByRel[f.Rel]
		if !ok {
			sUnpaired = append(sUnpaired, f)
			continue
		}
		if f.Size == p.size {
			jobs = append(jobs, hashJob{side: 1, rel: f.Rel, loc: f.Abs, size: f.Size})
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
		// Same size: both sides of one path before the next path, so same-path
		// pairs leave Pending as soon as both hashes land.
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
