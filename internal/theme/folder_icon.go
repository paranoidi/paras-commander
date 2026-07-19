package theme

import "github.com/gdamore/tcell/v2"

// FolderIconKind identifies which directory icon-strip glyph and color apply to a listing row.
type FolderIconKind int

const (
	FolderIconDefault FolderIconKind = iota
	FolderIconOpen
	FolderIconMount
	FolderIconScanning
	FolderIconExcluded
)

// FolderIconGlyph returns the icon-strip glyph for the given folder icon kind.
func (t Theme) FolderIconGlyph(kind FolderIconKind) string {
	switch kind {
	case FolderIconExcluded:
		return t.foldersSymbol(SymbolKeyFoldersExcluded, "\uf114")
	case FolderIconScanning:
		return t.foldersSymbol(SymbolKeyFoldersScanning, "\U000F0D0B")
	case FolderIconOpen:
		return t.foldersSymbol(SymbolKeyFoldersOpen, "\U000F0770")
	case FolderIconMount:
		return t.foldersSymbol(SymbolKeyFoldersMount, "\U000F0256")
	default:
		return t.foldersSymbol(SymbolKeyFoldersFolder, "\U000F024B")
	}
}

// FolderIconForeground picks the directory icon-strip color: cursor PanelFileIconFG override,
// then kind-specific style, then row foreground (for Default and as final fallback).
func (t Theme) FolderIconForeground(kind FolderIconKind, cursorStyleKey string, rowStyle tcell.Style) tcell.Color {
	rowFG, _, _ := rowStyle.Decompose()
	if cursorStyleKey != "" && t.PanelFileIconFG != nil {
		if c, ok := t.PanelFileIconFG[cursorStyleKey]; ok {
			return c
		}
	}
	switch kind {
	case FolderIconScanning:
		dfg, _, _ := t.PanelIconFolderScanning.Decompose()
		if dfg != tcell.ColorDefault {
			return dfg
		}
	case FolderIconOpen:
		return t.PanelRowIconForeground(cursorStyleKey, t.PanelIconFolderOpen)
	case FolderIconMount:
		return t.PanelRowIconForeground(cursorStyleKey, t.PanelIconFolderMount)
	case FolderIconExcluded:
		efg, _, _ := t.PanelIconFolderExcluded.Decompose()
		if efg != tcell.ColorDefault {
			return efg
		}
	case FolderIconDefault:
		return rowFG
	}
	return rowFG
}
