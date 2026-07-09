package compare

import (
	"strings"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// EffectiveDisplayRoot returns the directory the dedup results view is rooted at.
// When no trim applies, this equals Root (the scan path).
func (s DedupSnapshot) EffectiveDisplayRoot() pathloc.Path {
	if !s.DisplayRoot.IsZero() {
		return s.DisplayRoot
	}
	return s.Root
}

// DisplayRootTrimmed reports whether the results view was re-rooted under the scan path.
func (s DedupSnapshot) DisplayRootTrimmed() bool {
	return !s.DisplayRoot.IsZero() && !s.DisplayRoot.Equal(s.Root)
}

// WithTrimmedDisplayRoot returns a copy whose DisplayRoot and file Rel paths are
// adjusted when every duplicate lives under a single-child directory chain with
// no branching and no duplicate files in intermediate folders.
func (s DedupSnapshot) WithTrimmedDisplayRoot() DedupSnapshot {
	out := s
	if s.Phase != DedupDone || len(s.Groups) == 0 {
		out.DisplayRoot = s.Root
		return out
	}
	trimRel := dedupTrimDisplayRootRel(s.Groups)
	if trimRel == "" {
		out.DisplayRoot = s.Root
		return out
	}
	displayRoot, err := JoinRel(s.Root, trimRel)
	if err != nil {
		out.DisplayRoot = s.Root
		return out
	}
	out.DisplayRoot = displayRoot
	prefix := trimRel + "/"
	for gi := range out.Groups {
		for fi := range out.Groups[gi].Files {
			rel := out.Groups[gi].Files[fi].Rel
			if strings.HasPrefix(rel, prefix) {
				out.Groups[gi].Files[fi].Rel = rel[len(prefix):]
			}
		}
	}
	return out
}

// dedupTrimDisplayRootRel finds the deepest relative directory under the scan
// root that may serve as the dedup display root. It returns "" when the scan
// root itself should be used (files at scan root, branching siblings, etc.).
func dedupTrimDisplayRootRel(groups []DedupGroup) string {
	var rels []string
	for _, g := range groups {
		for _, f := range g.Files {
			rels = append(rels, f.Rel)
		}
	}
	if len(rels) == 0 {
		return ""
	}

	trimRel := ""
	cur := ""
	for {
		fileHere := false
		for _, rel := range rels {
			if RelDir(rel) == cur {
				fileHere = true
				break
			}
		}
		if fileHere {
			trimRel = cur
			break
		}

		subdirs := map[string]struct{}{}
		for _, rel := range rels {
			rest := rel
			if cur != "" {
				prefix := cur + "/"
				if !strings.HasPrefix(rest, prefix) {
					continue
				}
				rest = rest[len(prefix):]
			}
			if rest == "" {
				continue
			}
			if i := strings.IndexByte(rest, '/'); i >= 0 {
				subdirs[rest[:i]] = struct{}{}
			}
		}
		if len(subdirs) != 1 {
			// Stop at the branching point: trim through the single-child chain
			// we walked (e.g. scan root → test-cases → {diff-a, diff-b} → test-cases).
			trimRel = cur
			break
		}
		var seg string
		for name := range subdirs {
			seg = name
		}
		if cur == "" {
			cur = seg
		} else {
			cur = cur + "/" + seg
		}
	}
	return trimRel
}
