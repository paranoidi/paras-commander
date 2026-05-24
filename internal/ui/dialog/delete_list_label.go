package dialog

import (
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/primitive"
)

// DeleteListEntryName returns the delete confirmation list label for one entry.
// Selections in the current panel directory use the basename only; selections
// elsewhere use a path from the deepest directory shared with the panel path
// (e.g. "Some Series/Season 01" when the panel has left that tree).
func DeleteListEntryName(panelPath, homeDir, entryPath, entryName string) string {
	panelPath = filepath.Clean(panelPath)
	entryPath = filepath.Clean(entryPath)
	if filepath.Dir(entryPath) == panelPath {
		return entryName
	}
	rel, ancBase, ok := deleteListPathFromCommonAncestor(panelPath, entryPath)
	if !ok || rel == "" || rel == "." {
		return entryName
	}
	return deleteListLabelWithHome(rel, homeDir, ancBase, entryPath)
}

func deleteListLabelWithHome(rel, homeDir, ancBase, entryPath string) string {
	if homeDir == "" {
		return rel
	}
	full := primitive.PathWithHomeTilde(entryPath, homeDir)
	if !strings.HasPrefix(full, "~/") {
		return rel
	}
	ancDisp := primitive.PathWithHomeTilde(ancBase, homeDir)
	switch ancDisp {
	case "~":
		ancDisp = "~/"
	case "~/":
		// keep
	}
	if ancDisp == "~/" || strings.HasPrefix(full, ancDisp) {
		suffix := strings.TrimPrefix(full, ancDisp)
		suffix = strings.TrimPrefix(suffix, "/")
		if suffix != "" {
			return suffix
		}
	}
	return rel
}

// DeleteListEntryNameFitsWidth shortens a delete list label for the available column width.
func DeleteListEntryNameFitsWidth(label, entryPath string, width int) string {
	if width <= 0 {
		return ""
	}
	if label == "" {
		return ""
	}
	if !deleteListLabelIsContextual(label, entryPath) {
		return label
	}
	return primitive.FitPathForWidth(label, width)
}

func deleteListLabelIsContextual(label, entryPath string) bool {
	entryPath = filepath.Clean(entryPath)
	if entryPath == "" {
		return strings.ContainsRune(label, '/')
	}
	return label != filepath.Base(entryPath)
}

func deleteListPathFromCommonAncestor(panelPath, entryPath string) (rel string, ancBase string, ok bool) {
	panel, err1 := pathloc.Parse(panelPath)
	entryLoc, err2 := pathloc.Parse(entryPath)
	if err1 != nil || err2 != nil || panel.Scheme() != entryLoc.Scheme() {
		return "", "", false
	}
	anc, ok := deepestSharedAncestor(panel, entryLoc)
	if !ok {
		return "", "", false
	}
	rel, ok = pathRelativeUnderAncestor(anc, entryPath)
	if !ok {
		return "", "", false
	}
	ancBase, ok = ancestorBasePath(anc)
	if !ok {
		return "", "", false
	}
	return rel, ancBase, true
}

func ancestorBasePath(anc pathloc.Path) (string, bool) {
	switch anc.Scheme() {
	case pathloc.SchemeFile:
		base, err := anc.FilePath()
		return base, err == nil
	case pathloc.SchemeSFTP:
		return anc.String(), true
	default:
		return "", false
	}
}

func deepestSharedAncestor(panelDir, entryLoc pathloc.Path) (pathloc.Path, bool) {
	for anc := panelDir; !anc.IsZero(); anc = anc.Parent() {
		if entryLoc.HasPrefix(anc) && panelDir.HasPrefix(anc) {
			return anc, true
		}
	}
	return pathloc.Path{}, false
}

func pathRelativeUnderAncestor(anc pathloc.Path, entryPath string) (string, bool) {
	switch anc.Scheme() {
	case pathloc.SchemeFile:
		base, err := anc.FilePath()
		if err != nil {
			return "", false
		}
		rel, err := filepath.Rel(base, filepath.Clean(entryPath))
		if err != nil || strings.HasPrefix(rel, "..") {
			return "", false
		}
		return rel, true
	case pathloc.SchemeSFTP:
		entryLoc, err := pathloc.Parse(entryPath)
		if err != nil {
			return "", false
		}
		return sftpRelUnderAncestor(anc, entryLoc)
	default:
		return "", false
	}
}

func sftpRelUnderAncestor(anc, entryLoc pathloc.Path) (string, bool) {
	if !entryLoc.HasPrefix(anc) {
		return "", false
	}
	ancRemote, err := pathloc.SFTPRemotePath(anc)
	if err != nil {
		return "", false
	}
	entryRemote, err := pathloc.SFTPRemotePath(entryLoc)
	if err != nil {
		return "", false
	}
	return sftpRemoteRel(ancRemote, entryRemote)
}

func sftpRemoteRel(ancRemote, entryRemote string) (string, bool) {
	if entryRemote == ancRemote {
		return "", false
	}
	switch ancRemote {
	case "/":
		rel := strings.TrimPrefix(entryRemote, "/")
		if rel == "" || strings.HasPrefix(rel, "..") {
			return "", false
		}
		return rel, true
	case "~":
		if !strings.HasPrefix(entryRemote, "~/") && entryRemote != "~" {
			return "", false
		}
		rel := strings.TrimPrefix(entryRemote, "~/")
		if rel == "" || strings.HasPrefix(rel, "..") {
			return "", false
		}
		return rel, true
	}
	if !strings.HasPrefix(entryRemote, ancRemote) {
		return "", false
	}
	rel := strings.TrimPrefix(entryRemote, ancRemote)
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" || strings.HasPrefix(rel, "..") {
		return "", false
	}
	return rel, true
}
