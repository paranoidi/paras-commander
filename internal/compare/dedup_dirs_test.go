package compare

import (
	"crypto/sha256"
	"testing"
)

// dirMembers runs groupDirsByDigest over synthetic files (rel -> content) and
// returns member rel -> strongest kind (exact wins) the way the UI marks rows.
func dirMembers(files map[string]string) map[string]DedupDirKind {
	var recs []FileRecord
	hashOf := map[string][32]byte{}
	for rel, body := range files {
		recs = append(recs, FileRecord{Rel: rel, Size: int64(len(body))})
		hashOf[rel] = sha256.Sum256([]byte(body))
	}
	out := map[string]DedupDirKind{}
	for _, g := range groupDirsByDigest(recs, hashOf, nil, nil, nil) {
		for _, r := range g.Rels {
			if k, ok := out[r]; !ok || (k == DirContent && g.Kind == DirExact) {
				out[r] = g.Kind
			}
		}
	}
	return out
}

func TestDedupDirGroupsEveryCopyMarked(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  map[string]DedupDirKind
	}{
		{"nested wrapper copy", map[string]string{
			"maple/sub/lantern": "one", "walnut/lantern": "one",
			// walnut is an exact twin of maple/sub, maple itself is a content (wrapper) copy.
		}, map[string]DedupDirKind{"maple": DirContent, "walnut": DirExact, "maple/sub": DirExact}},
		{"nested wrapper three copies", map[string]string{
			"maple/sub/lantern": "one", "walnut/lantern": "one", "birch/sub/lantern": "one",
		}, map[string]DedupDirKind{"maple": DirExact, "walnut": DirExact, "birch": DirExact}},
		{"parents match partially", map[string]string{
			"maple/ribbon/lantern": "one", "maple/ribbon/anchor": "two",
			"walnut/ribbon/lantern": "one", "walnut/ribbon/anchor": "two",
			"birch/ribbon/lantern": "one", "birch/ribbon/anchor": "two", "birch/extra": "three-extra",
			"cedar/extra": "three-extra",
		}, map[string]DedupDirKind{
			"maple": DirExact, "walnut": DirExact,
			"maple/ribbon": DirExact, "walnut/ribbon": DirExact, "birch/ribbon": DirExact,
		}},
		{"exact and content overlap", map[string]string{
			"maple/lantern": "one", "maple/anchor": "two",
			"walnut/lantern": "one", "walnut/anchor": "two",
			"birch/lantern": "one", "birch/ribbon": "two",
		}, map[string]DedupDirKind{"maple": DirExact, "walnut": DirExact, "birch": DirContent}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := dirMembers(tc.files)
			for rel, kind := range tc.want {
				if g, ok := got[rel]; !ok || g != kind {
					t.Errorf("%s: kind %v present=%v, want %v (all: %v)", rel, g, ok, kind, got)
				}
			}
		})
	}
}

func TestDedupDirGroupsTrimKeepsEveryCopy(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "outer/maple/sub/lantern", "one")
	writeFile(t, dir, "outer/walnut/lantern", "one")
	writeFile(t, dir, "outer/birch/sub/lantern", "one")
	snap := scanDirGroups(t, dir).WithTrimmedDisplayRoot()
	got := map[string]bool{}
	for _, g := range snap.DirGroups {
		for _, r := range g.Rels {
			got[r] = true
		}
	}
	for _, want := range []string{"maple", "walnut", "birch"} {
		if !got[want] {
			t.Errorf("trimmed members %v missing %q", got, want)
		}
	}
}
