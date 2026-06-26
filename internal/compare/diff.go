package compare

import (
	"cmp"
	"slices"
)

type fileMeta struct {
	rel  string
	size int64
}

type hashBucket struct {
	primary   []fileMeta
	secondary []fileMeta
}

// Classify builds compare rows from walked files and per-side content hashes (keyed by rel path).
func Classify(primary, secondary []FileRecord, pHash, sHash map[string][32]byte, pErr, sErr map[string]string) []Row {
	pByRel := indexFiles(primary)
	sByRel := indexFiles(secondary)
	allRels := unionRels(pByRel, sByRel)

	var rows []Row
	pConsumed := make(map[string]bool)
	sConsumed := make(map[string]bool)

	// Same relative path on both sides.
	for _, rel := range allRels {
		p, pOK := pByRel[rel]
		s, sOK := sByRel[rel]
		if !pOK || !sOK {
			continue
		}
		ph, pHashed := pHash[rel]
		sh, sHashed := sHash[rel]
		row := classifySamePath(p, s, ph, sh, pHashed, sHashed, pErr[rel], sErr[rel])
		rows = append(rows, row)
		pConsumed[rel] = true
		sConsumed[rel] = true
	}

	// Hash buckets for cross-path matches and side-only files.
	buckets := map[[32]byte]*hashBucket{}
	for rel, f := range pByRel {
		if pConsumed[rel] {
			continue
		}
		if err := pErr[rel]; err != "" {
			rows = append(rows, Row{Kind: KindSkipped, PrimaryRel: f.rel, Size: f.size, Err: err, HashDone: true})
			pConsumed[rel] = true
			continue
		}
		h, ok := pHash[rel]
		if !ok {
			continue
		}
		addBucket(buckets, 0, f, h)
	}
	for rel, f := range sByRel {
		if sConsumed[rel] {
			continue
		}
		if err := sErr[rel]; err != "" {
			rows = append(rows, Row{Kind: KindSkipped, SecondaryRel: f.rel, Size: f.size, Err: err, HashDone: true})
			sConsumed[rel] = true
			continue
		}
		h, ok := sHash[rel]
		if !ok {
			continue
		}
		addBucket(buckets, 1, f, h)
	}

	for _, h := range sortedHashes(buckets) {
		b := buckets[h]
		slices.SortFunc(b.primary, func(a, b fileMeta) int { return cmp.Compare(a.rel, b.rel) })
		slices.SortFunc(b.secondary, func(a, b fileMeta) int { return cmp.Compare(a.rel, b.rel) })
		n := min(len(b.primary), len(b.secondary))
		for i := 0; i < n; i++ {
			p := b.primary[i]
			s := b.secondary[i]
			rows = append(rows, Row{
				Kind:         KindRelocated,
				PrimaryRel:   p.rel,
				SecondaryRel: s.rel,
				Size:         p.size,
				Hash:         h,
				HashDone:     true,
			})
			pConsumed[p.rel] = true
			sConsumed[s.rel] = true
		}
		for i := n; i < len(b.primary); i++ {
			p := b.primary[i]
			rows = append(rows, Row{
				Kind:       KindPrimaryOnly,
				PrimaryRel: p.rel,
				Size:       p.size,
				Hash:       h,
				HashDone:   true,
			})
			pConsumed[p.rel] = true
		}
		for i := n; i < len(b.secondary); i++ {
			s := b.secondary[i]
			rows = append(rows, Row{
				Kind:         KindSecondaryOnly,
				SecondaryRel: s.rel,
				Size:         s.size,
				Hash:         h,
				HashDone:     true,
			})
			sConsumed[s.rel] = true
		}
	}

	// Pending hashes (not yet computed).
	for rel, f := range pByRel {
		if pConsumed[rel] {
			continue
		}
		if pErr[rel] != "" {
			continue
		}
		rows = append(rows, Row{Kind: KindPrimaryOnly, PrimaryRel: f.rel, Size: f.size})
	}
	for rel, f := range sByRel {
		if sConsumed[rel] {
			continue
		}
		if sErr[rel] != "" {
			continue
		}
		rows = append(rows, Row{Kind: KindSecondaryOnly, SecondaryRel: f.rel, Size: f.size})
	}

	sortRows(rows)
	return rows
}

func classifySamePath(p, s fileMeta, ph, sh [32]byte, pHashed, sHashed bool, errP, errS string) Row {
	if errP != "" || errS != "" {
		err := errP
		if err == "" {
			err = errS
		}
		return Row{Kind: KindSkipped, PrimaryRel: p.rel, SecondaryRel: s.rel, Size: p.size, Err: err, HashDone: true}
	}
	if p.size != s.size {
		return Row{Kind: KindContentDiff, PrimaryRel: p.rel, SecondaryRel: s.rel, Size: p.size, HashDone: true}
	}
	if !pHashed || !sHashed {
		return Row{Kind: KindEqual, PrimaryRel: p.rel, SecondaryRel: s.rel, Size: p.size}
	}
	if ph == sh {
		return Row{Kind: KindEqual, PrimaryRel: p.rel, SecondaryRel: s.rel, Size: p.size, Hash: ph, HashDone: true}
	}
	return Row{Kind: KindContentDiff, PrimaryRel: p.rel, SecondaryRel: s.rel, Size: p.size, Hash: ph, HashDone: true}
}

func addBucket(buckets map[[32]byte]*hashBucket, side int, f fileMeta, h [32]byte) {
	b := buckets[h]
	if b == nil {
		b = &hashBucket{}
		buckets[h] = b
	}
	if side == 0 {
		b.primary = append(b.primary, f)
	} else {
		b.secondary = append(b.secondary, f)
	}
}

func indexFiles(files []FileRecord) map[string]fileMeta {
	out := make(map[string]fileMeta, len(files))
	for _, f := range files {
		out[f.Rel] = fileMeta{rel: f.Rel, size: f.Size}
	}
	return out
}

func unionRels(a, b map[string]fileMeta) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	for rel := range a {
		seen[rel] = struct{}{}
	}
	for rel := range b {
		seen[rel] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for rel := range seen {
		out = append(out, rel)
	}
	slices.Sort(out)
	return out
}

func sortedHashes(buckets map[[32]byte]*hashBucket) [][32]byte {
	out := make([][32]byte, 0, len(buckets))
	for h := range buckets {
		out = append(out, h)
	}
	slices.SortFunc(out, func(a, b [32]byte) int {
		return cmp.Compare(a[0], b[0])
	})
	return out
}

func sortRows(rows []Row) {
	slices.SortFunc(rows, func(a, b Row) int {
		if c := cmp.Compare(int(a.Kind), int(b.Kind)); c != 0 {
			return c
		}
		if c := cmp.Compare(a.PrimaryRel, b.PrimaryRel); c != 0 {
			return c
		}
		return cmp.Compare(a.SecondaryRel, b.SecondaryRel)
	})
}
