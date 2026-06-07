package gitignore

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type workTreeCacheEntry struct {
	workRoot      string
	metadataValid bool
	fingerprint   string
}

type workTreeResolver struct {
	mu      sync.Mutex
	entries map[string]workTreeCacheEntry
}

var sharedWorkTreeResolver = &workTreeResolver{
	entries: make(map[string]workTreeCacheEntry),
}

func (r *workTreeResolver) workTreeRoot(dir string) string {
	return r.resolve(dir, false)
}

func (r *workTreeResolver) validWorkTreeRoot(dir string) string {
	return r.resolve(dir, true)
}

func (r *workTreeResolver) resolve(dir string, requireValidMetadata bool) string {
	dir, err := filepath.Abs(dir)
	if err != nil || dir == "" {
		return ""
	}
	dir = filepath.Clean(dir)

	r.mu.Lock()
	defer r.mu.Unlock()

	if ent, ok := r.entries[dir]; ok && r.entryFresh(ent) {
		return r.resultFromEntry(ent, requireValidMetadata)
	}

	if root := r.cachedAncestorRoot(dir, requireValidMetadata); root != "" {
		r.remember(dir, root, gitMetadataValid(root))
		return root
	}

	return r.walkAndRemember(dir, requireValidMetadata)
}

func (r *workTreeResolver) cachedAncestorRoot(dir string, requireValidMetadata bool) string {
	for cur := dir; ; cur = filepath.Dir(cur) {
		ent, ok := r.entries[cur]
		if !ok || !r.entryFresh(ent) || ent.workRoot == "" {
			parent := filepath.Dir(cur)
			if parent == cur {
				return ""
			}
			continue
		}
		if requireValidMetadata && !ent.metadataValid {
			parent := filepath.Dir(cur)
			if parent == cur {
				return ""
			}
			continue
		}
		if pathUnderWorkRoot(dir, ent.workRoot) {
			return ent.workRoot
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return ""
		}
	}
}

func (r *workTreeResolver) walkAndRemember(dir string, requireValidMetadata bool) string {
	chain := []string{dir}
	cur := dir
	for {
		if _, err := os.Stat(filepath.Join(cur, ".git")); err == nil {
			valid := gitMetadataValid(cur)
			for _, p := range chain {
				r.remember(p, cur, valid)
			}
			if requireValidMetadata && !valid {
				return ""
			}
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return ""
		}
		cur = parent
		chain = append(chain, cur)
	}
}

func (r *workTreeResolver) remember(dir, workRoot string, metadataValid bool) {
	fp := ""
	if workRoot != "" {
		fp = workTreeFingerprint(workRoot)
	}
	r.entries[dir] = workTreeCacheEntry{
		workRoot:      workRoot,
		metadataValid: metadataValid,
		fingerprint:   fp,
	}
}

func (r *workTreeResolver) entryFresh(ent workTreeCacheEntry) bool {
	return ent.fingerprint == workTreeFingerprint(ent.workRoot)
}

func (r *workTreeResolver) resultFromEntry(ent workTreeCacheEntry, requireValidMetadata bool) string {
	if requireValidMetadata && !ent.metadataValid {
		return ""
	}
	return ent.workRoot
}

func pathUnderWorkRoot(dir, workRoot string) bool {
	dir = filepath.Clean(dir)
	workRoot = filepath.Clean(workRoot)
	if dir == workRoot {
		return true
	}
	rel, err := filepath.Rel(workRoot, dir)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func workTreeFingerprint(workRoot string) string {
	var b strings.Builder
	appendFileMtime(&b, filepath.Join(workRoot, ".git"))
	gitDir := resolveGitDir(filepath.Join(workRoot, ".git"), workRoot)
	if gitDir != "" {
		appendFileMtime(&b, filepath.Join(gitDir, "HEAD"))
	}
	return b.String()
}
