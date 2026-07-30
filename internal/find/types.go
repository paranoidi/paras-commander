package find

import (
	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/gitignore"
	"github.com/paranoidi/paras-commander/internal/scan"
)

// Entry is one indexed file or directory under the search root.
type Entry = scan.Entry

// Options configures a find session walk.
type Options struct {
	IncludeHidden bool
	Gitignore     *gitignore.Cache
	ShouldSkipDir diskusage.ShouldIgnoreFolder
}
