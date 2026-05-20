// Package diskusage provides subtree size caching and scanning utilities.
//
// This file contains a concurrent directory tree walker that aggregates subtree sizes.
// Derived from github.com/viktomas/godu (MIT). Original license:
// https://github.com/viktomas/godu/blob/master/LICENSE.md
package diskusage

import (
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// File represents nodes in a memoized filesystem tree with accumulated directory sizes.
type File struct {
	Name   string
	Parent *File
	Size   int64
	IsDir  bool
	Files  []*File
}

// Path returns a path rooted like upstream godu's walker: Parent == nil denotes the root
// node whose Name is the cleaned root directory path.
func (f *File) Path() string {
	if f.Parent == nil {
		return f.Name
	}
	return filepath.Join(f.Parent.Path(), f.Name)
}

// UpdateSize aggregates directory sizes recursively.
func (f *File) UpdateSize() {
	if !f.IsDir {
		return
	}
	var sum int64
	for _, child := range f.Files {
		child.UpdateSize()
		sum += child.Size
	}
	f.Size = sum
}

// ReadDir resolves directory contents like os.ReadDir exposing FileInfo semantics.
type ReadDir func(dirname string) ([]fs.FileInfo, error)

// ShouldIgnoreFolder returns true when a directory must not be descended into.
type ShouldIgnoreFolder func(absolutePath string) bool

func ignoringReadDir(shouldIgnore ShouldIgnoreFolder, original ReadDir) ReadDir {
	return func(path string) ([]fs.FileInfo, error) {
		if shouldIgnore(path) {
			return []fs.FileInfo{}, nil
		}
		return original(path)
	}
}

// ReadDirInfos adapts os.ReadDir to satisfy ReadDir.
func ReadDirInfos(path string) ([]fs.FileInfo, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	out := make([]fs.FileInfo, 0, len(entries))
	for _, entry := range entries {
		fi, ierr := entry.Info()
		if ierr != nil {
			continue
		}
		out = append(out, fi)
	}
	return out, nil
}

// WalkFolder traverses path and aggregates sizes on the returned tree (UpdateSize applied before return).
// walkConcurrency limits concurrent subdirectory branches (minimum 1). Smaller values reduce HDD/NAS read storms.
// progress, when non-nil, receives folder-branch counts analogous to upstream godu; WalkFolder never closes progress.
func WalkFolder(
	rootPath string,
	readDir ReadDir,
	ignore ShouldIgnoreFolder,
	progress chan<- int,
	walkConcurrency int,
) *File {
	if readDir == nil {
		readDir = ReadDirInfos
	}
	if ignore == nil {
		ignore = func(string) bool { return false }
	}
	if walkConcurrency < 1 {
		walkConcurrency = 1
	}
	rootPath = filepath.Clean(rootPath)

	var wg sync.WaitGroup
	sem := make(chan bool, walkConcurrency)

	tree := walkSubFolderConcurrently(rootPath, nil, ignoringReadDir(ignore, readDir), sem, &wg, progress)
	wg.Wait()

	tree.UpdateSize()

	return tree
}

func walkSubFolderConcurrently(
	path string,
	parent *File,
	readDir ReadDir,
	sem chan bool,
	wg *sync.WaitGroup,
	progress chan<- int,
) *File {
	dirName, leafName := filepath.Split(path)
	result := &File{IsDir: true}
	if parent != nil {
		result.Name = leafName
		result.Parent = parent
	} else {
		result.Name = filepath.Join(dirName, leafName)
	}

	entries, err := readDir(path)
	if err != nil {
		return result
	}

	result.Files = make([]*File, 0, len(entries))

	numSubFolders := 0
	defer notifyProgress(progress, &numSubFolders)

	var mu sync.Mutex
	for _, entry := range entries {
		if entry.IsDir() {
			numSubFolders++
			subFolderPath := filepath.Join(path, entry.Name())
			wg.Add(1)

			go func(subPath string) {
				defer wg.Done()

				sem <- true
				subFolder := walkSubFolderConcurrently(subPath, result, readDir, sem, wg, progress)

				mu.Lock()
				result.Files = append(result.Files, subFolder)
				mu.Unlock()

				<-sem
			}(subFolderPath)
			continue
		}

		mu.Lock()
		result.Files = append(result.Files, &File{
			Name:   entry.Name(),
			Parent: result,
			Size:   entry.Size(),
			IsDir:  false,
			Files:  nil,
		})
		mu.Unlock()
	}

	return result
}

func notifyProgress(progress chan<- int, numSubfolders *int) {
	if progress == nil {
		return
	}
	n := *numSubfolders
	if n <= 0 {
		return
	}
	progress <- n
}

// FlattenSizes records every node's Size keyed by filepath.Clean of Path().
// Call after the root subtree has UpdateSize applied on the subtree root chain.
func FlattenSizes(root *File, dest map[string]int64) {
	if root == nil || dest == nil {
		return
	}
	dest[filepath.Clean(root.Path())] = root.Size

	for _, ch := range root.Files {
		FlattenSizes(ch, dest)
	}
}
