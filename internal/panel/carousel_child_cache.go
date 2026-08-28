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

// CarouselParentPreviewTarget returns the center directory whose parent listing the carousel
// parent column should show — also the cache key an async parent-snapshot fetch is tagged with
// (see internal/app's carousel snapshot dispatch). pathloc.Path is already canonical, so the
// string needs no further cleaning (and must not get any: filepath.Clean would collapse the "//"
// of an sftp:// location).
func (s *State) CarouselParentPreviewTarget() (string, bool) {
	if s.Path.IsZero() {
		return "", false
	}
	return s.Path.String(), true
}

// CarouselParentCacheValid reports whether the cached parent listing matches the current center
// directory.
func (s *State) CarouselParentCacheValid() bool {
	target, ok := s.CarouselParentPreviewTarget()
	return ok && s.CarouselSideCache.ParentOK && s.CarouselSideCache.ParentSourceDir == target
}

// CarouselChildPreviewTarget returns the directory path for the child preview column when the
// center cursor is on a directory entry — also the cache key an async child-snapshot fetch is
// tagged with (see internal/app's carousel snapshot dispatch).
func (s *State) CarouselChildPreviewTarget() (string, bool) {
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
	target, ok := s.CarouselChildPreviewTarget()
	return ok && s.CarouselChildCacheValidFor(target)
}

// CarouselChildCacheValidFor reports whether the cached child listing was fetched for target. It
// takes the target rather than re-deriving it so a caller that already resolved it — the async
// dispatch reconcile, which runs on every event — does not pay for CarouselChildPreviewTarget
// (a CurrentEntry lookup plus filepath.Clean) twice.
func (s *State) CarouselChildCacheValidFor(target string) bool {
	return s.CarouselSideCache.ChildOK && s.CarouselSideCache.ChildCursorDir == target
}

// CarouselChildCachePaintDuringCoalesce reports whether the child column should be repainted
// from the in-memory cache while file-list nav coalesce is active. The cursor may have moved
// (including onto a file); the cache is kept visible until the debounce flush reloads it.
func (s *State) CarouselChildCachePaintDuringCoalesce() bool {
	return s.CarouselChildPreviewCoalesce &&
		s.CarouselSideCache.ChildOK &&
		s.CarouselCenterHasSubdirectories()
}

func (s *State) invalidateCarouselChildCache() {
	s.CarouselSideCache.ChildOK = false
	s.CarouselSideCache.ChildCursorDir = ""
}
