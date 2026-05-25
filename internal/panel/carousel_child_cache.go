package panel

import (
	"path/filepath"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

// CarouselCenterHasSubdirectories reports whether the center listing has any child
// directory entries (the carousel child preview column is only useful then).
func (s *State) CarouselCenterHasSubdirectories() bool {
	for _, e := range s.Entries {
		if e.Type != localfs.EntryDirectory {
			continue
		}
		if e.Name == "." || e.Name == ".." {
			continue
		}
		return true
	}
	return false
}

// carouselChildPreviewTarget returns the directory path for the child preview column when the
// center cursor is on a directory entry.
func (s *State) carouselChildPreviewTarget() (string, bool) {
	entry, ok := s.CurrentEntry()
	if !ok || entry.Type != localfs.EntryDirectory {
		return "", false
	}
	path := filepath.Clean(entry.Path)
	if path == "" || path == "." {
		return "", false
	}
	return path, true
}

// CarouselChildCacheValid reports whether the cached child listing matches the current cursor.
func (s *State) CarouselChildCacheValid() bool {
	if !s.CarouselSideCache.ChildOK {
		return false
	}
	target, ok := s.carouselChildPreviewTarget()
	if !ok {
		return false
	}
	return s.CarouselSideCache.ChildCursorDir == target
}

func (s *State) invalidateCarouselChildCache() {
	s.CarouselSideCache.ChildOK = false
	s.CarouselSideCache.ChildCursorDir = ""
}
