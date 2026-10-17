package compare

import "strings"

// PruneRows drops removed files from rows. Removed rels are slash-separated and relative to
// each root; a removed directory covers everything under it. A two-sided row that loses one
// side becomes a one-sided row for the survivor. The input slice is not modified.
func PruneRows(rows []Row, primaryRemoved, secondaryRemoved []string) []Row {
	if len(primaryRemoved) == 0 && len(secondaryRemoved) == 0 {
		return rows
	}
	out := make([]Row, 0, len(rows))
	for _, r := range rows {
		pGone := r.PrimaryRel != "" && relRemoved(r.PrimaryRel, primaryRemoved)
		sGone := r.SecondaryRel != "" && relRemoved(r.SecondaryRel, secondaryRemoved)
		switch {
		case !pGone && !sGone:
		case (pGone || r.PrimaryRel == "") && (sGone || r.SecondaryRel == ""):
			continue
		case pGone:
			r.Kind, r.PrimaryRel = KindSecondaryOnly, ""
		default:
			r.Kind, r.SecondaryRel = KindPrimaryOnly, ""
		}
		if pGone != sGone {
			// ponytail: Size stays the primary-side size; differs only for a content-diff row
			// whose primary side was removed. Manual refresh corrects it.
			r.Err, r.Hashing, r.HashDone = "", false, true
		}
		out = append(out, r)
	}
	return out
}

func relRemoved(rel string, removed []string) bool {
	for _, r := range removed {
		if rel == r || strings.HasPrefix(rel, r+"/") {
			return true
		}
	}
	return false
}
