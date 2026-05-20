package gitignore

import (
	"os"
	"path/filepath"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
)

// Matcher applies Git ignore rules for a listing directory inside a work tree.
type Matcher struct {
	workRoot string
	exclude  *ignore.GitIgnore
	byDir    map[string]*ignore.GitIgnore
}

func newMatcher(workRoot, listDir string) (*Matcher, error) {
	workRoot = filepath.Clean(workRoot)
	listDir = filepath.Clean(listDir)
	m := &Matcher{
		workRoot: workRoot,
		byDir:    make(map[string]*ignore.GitIgnore),
	}

	excludePath := filepath.Join(workRoot, ".git", "info", "exclude")
	if st, err := os.Stat(excludePath); err == nil && !st.IsDir() {
		gi, err := ignore.CompileIgnoreFile(excludePath)
		if err != nil {
			return nil, err
		}
		m.exclude = gi
	}

	for _, dir := range dirsFromRootTo(workRoot, listDir) {
		gitignorePath := filepath.Join(dir, ".gitignore")
		st, err := os.Stat(gitignorePath)
		if err != nil || st.IsDir() {
			continue
		}
		gi, err := ignore.CompileIgnoreFile(gitignorePath)
		if err != nil {
			return nil, err
		}
		m.byDir[dir] = gi
	}
	return m, nil
}

// Ignored reports whether absPath is ignored by Git rules loaded for this matcher.
func (m *Matcher) Ignored(absPath string, isDir bool) bool {
	if m == nil {
		return false
	}
	absPath = filepath.Clean(absPath)
	if m.workRoot == "" {
		return false
	}
	if !isUnderRoot(m.workRoot, absPath) {
		return false
	}

	parent := filepath.Dir(absPath)
	matched := false

	if m.exclude != nil {
		rel, err := filepath.Rel(m.workRoot, absPath)
		if err == nil {
			relSlash := filepath.ToSlash(rel)
			matched = matchLayer(m.exclude, relSlash, matched)
			if isDir {
				matched = matchLayer(m.exclude, relSlash+"/", matched)
			}
		}
	}

	for _, dir := range dirsFromRootTo(m.workRoot, parent) {
		gi := m.byDir[dir]
		if gi == nil {
			continue
		}
		rel, err := filepath.Rel(dir, absPath)
		if err != nil {
			continue
		}
		relSlash := filepath.ToSlash(rel)
		matched = matchLayer(gi, relSlash, matched)
		if isDir {
			matched = matchLayer(gi, relSlash+"/", matched)
		}
	}
	return matched
}

func matchLayer(gi *ignore.GitIgnore, rel string, matched bool) bool {
	ok, ip := gi.MatchesPathHow(rel)
	if !ok || ip == nil {
		return matched
	}
	if ip.Negate {
		return false
	}
	return true
}

func isUnderRoot(root, path string) bool {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	if root == path {
		return true
	}
	sep := string(filepath.Separator)
	return strings.HasPrefix(path, root+sep)
}
