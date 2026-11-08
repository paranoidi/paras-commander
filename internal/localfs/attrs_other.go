//go:build !unix

package localfs

import "io/fs"

// entryAttrsFrom is unsupported off Unix; the file columns stay blank.
func entryAttrsFrom(_ fs.FileInfo) entryAttrs { return entryAttrs{} }
