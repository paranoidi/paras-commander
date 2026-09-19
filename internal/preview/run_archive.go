package preview

import (
	"cmp"
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	devicons "github.com/epilande/go-devicons"
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/paras-commander/internal/archive"
	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/panellist"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/treeflat"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

var archiveToolchain = sync.OnceValue(archive.ProbeToolchain)

type archiveEntry struct {
	Name  string
	IsDir bool
}

// archiveFileInfo adapts archiveEntry to fs.FileInfo for go-devicons.
type archiveFileInfo struct {
	entry archiveEntry
}

func (e archiveFileInfo) Name() string       { return e.entry.Name }
func (e archiveFileInfo) Size() int64        { return 0 }
func (e archiveFileInfo) Mode() fs.FileMode  { return 0 }
func (e archiveFileInfo) ModTime() time.Time { return time.Time{} }
func (e archiveFileInfo) IsDir() bool        { return e.entry.IsDir }
func (e archiveFileInfo) Sys() interface{}   { return nil }

// archiveTreeIconCells is the fixed terminal cell width reserved for the icon column, mirroring
// panelIconStripCells (internal/ui/panel_icon_strip.go).
const archiveTreeIconCells = 2

// runArchiveList lists a listable archive's member paths as a fully expanded path tree, using
// the same external toolchain the extract dialog uses (internal/archive).
func runArchiveList(ctx context.Context, req Request, f archive.Format) Result {
	tc := archiveToolchain()
	if !f.Available(tc) {
		return Result{ErrorMsg: fmt.Sprintf("%s not found in PATH (needed for archive listing)", f.RequiredToolName())}
	}
	tool := f.ToolPath(tc)
	argv := archive.ListArgv(f, tool, req.Path)
	if len(argv) == 0 {
		return Result{ErrorMsg: "archive listing not supported"}
	}
	res := cmdrun.Run(ctx, argv, filepath.Dir(req.Path), cmdrun.MaxStreamBytes)
	if res.LaunchErr != nil {
		return Result{ErrorMsg: res.LaunchErr.Error(), ExitCode: -1}
	}
	if res.ExitCode != 0 {
		msg := strings.TrimSpace(string(res.Stderr))
		if msg == "" {
			msg = fmt.Sprintf("%s exit %d", f.RequiredToolName(), res.ExitCode)
		}
		return Result{ErrorMsg: msg, ExitCode: res.ExitCode}
	}
	paths := archive.ParseListing(f, res.Stdout)
	cells := renderArchiveTree(paths, req.Theme, req.BaseStyle)
	if res.StdoutTrim {
		cells = append(cells, previewpanel.AnsiCell{R: '\n', St: req.BaseStyle})
		cells = appendRunes(cells, "[listing truncated]", req.BaseStyle)
		cells = append(cells, previewpanel.AnsiCell{R: '\n', St: req.BaseStyle})
	}
	return Result{
		Source:           previewpanel.SourceInternalHighlighted,
		HighlightedCells: cells,
		Truncated:        res.StdoutTrim,
		IsArchive:        true,
	}
}

func renderArchiveTree(paths []string, th theme.Theme, base tcell.Style) []previewpanel.AnsiCell {
	roots := archiveRoots(paths)
	rows := treeflat.Flatten(roots, nil)
	bold := base.Bold(true)
	connStyle := th.PanelRowTreeConnector
	if connStyle == (tcell.Style{}) {
		connStyle = base
	}
	var cells []previewpanel.AnsiCell
	for _, row := range rows {
		prefix := panellist.TreeConnectorPrefix(row.Depth, row.LastChild, row.AncestorHasNext, th)
		cells = appendRunes(cells, prefix, connStyle)
		if th.UseNerdfontIcons {
			cells = appendArchiveTreeIcon(cells, row.Value, th, base)
		}
		nameStyle := base
		if row.Value.IsDir {
			nameStyle = bold
		}
		cells = appendRunes(cells, row.Value.Name, nameStyle)
		cells = append(cells, previewpanel.AnsiCell{R: '\n', St: base})
	}
	return cells
}

// appendArchiveTreeIcon renders one entry's devicon/folder-icon into a fixed
// archiveTreeIconCells-wide column, matching paintPanelIconStrip's budget
// (internal/ui/panel_icon_strip.go).
func appendArchiveTreeIcon(cells []previewpanel.AnsiCell, entry archiveEntry, th theme.Theme, base tcell.Style) []previewpanel.AnsiCell {
	var icon string
	var fg tcell.Color
	if entry.IsDir {
		icon = th.FolderIcon(theme.FolderIconDefault)
		fg = th.FolderIconForeground(theme.FolderIconDefault, "", base)
	} else {
		st := devicons.IconForInfo(archiveFileInfo{entry: entry})
		icon = st.Icon
		if icon == "" {
			icon = " "
		}
		var err error
		fg, err = theme.ParseHexColor(st.Color)
		if err != nil {
			fg, _, _ = base.Decompose()
		}
	}
	iconStyle := base.Foreground(fg)

	col := 0
	for _, r := range icon {
		w := runewidth.RuneWidth(r)
		if w < 1 {
			w = 1
		}
		if col+w > archiveTreeIconCells {
			break
		}
		cells = append(cells, previewpanel.AnsiCell{R: r, St: iconStyle})
		col += w
	}
	for col < archiveTreeIconCells {
		cells = append(cells, previewpanel.AnsiCell{R: ' ', St: base})
		col++
	}
	return cells
}

func appendRunes(cells []previewpanel.AnsiCell, s string, st tcell.Style) []previewpanel.AnsiCell {
	for _, r := range s {
		cells = append(cells, previewpanel.AnsiCell{R: r, St: st})
	}
	return cells
}

// archiveBuild is a path trie. Algorithm mirrors dedupDirRoots/dedupDirNodes
// (internal/ui/dedup_view_types.go) — that code is welded to dedup types.
type archiveBuild struct {
	sub   map[string]*archiveBuild
	files []treeflat.Node[archiveEntry]
}

func archiveRoots(paths []string) []treeflat.Node[archiveEntry] {
	root := &archiveBuild{}
	for _, p := range paths {
		insertArchivePath(root, p)
	}
	return archiveNodes(root, "")
}

func insertArchivePath(root *archiveBuild, path string) {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "./")
	isDir := strings.HasSuffix(path, "/")
	path = strings.Trim(path, "/")
	if path == "" {
		return
	}
	parts := strings.Split(path, "/")
	cur := root
	for i, part := range parts {
		last := i == len(parts)-1
		if last && !isDir {
			rel := strings.Join(parts, "/")
			cur.files = append(cur.files, treeflat.Node[archiveEntry]{
				ID:    "f:" + rel,
				Value: archiveEntry{Name: part, IsDir: false},
			})
			return
		}
		if cur.sub == nil {
			cur.sub = map[string]*archiveBuild{}
		}
		next := cur.sub[part]
		if next == nil {
			next = &archiveBuild{}
			cur.sub[part] = next
		}
		cur = next
	}
}

func archiveNodes(b *archiveBuild, rel string) []treeflat.Node[archiveEntry] {
	names := make([]string, 0, len(b.sub))
	for name := range b.sub {
		names = append(names, name)
	}
	slices.Sort(names)
	out := make([]treeflat.Node[archiveEntry], 0, len(names)+len(b.files))
	for _, name := range names {
		childRel := name
		if rel != "" {
			childRel = rel + "/" + name
		}
		children := archiveNodes(b.sub[name], childRel)
		out = append(out, treeflat.Node[archiveEntry]{
			ID:       "d:" + childRel,
			Value:    archiveEntry{Name: name, IsDir: true},
			Children: children,
		})
	}
	files := slices.Clone(b.files)
	slices.SortFunc(files, func(a, b treeflat.Node[archiveEntry]) int {
		return cmp.Compare(a.Value.Name, b.Value.Name)
	})
	return append(out, files...)
}
