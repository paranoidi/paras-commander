package dialog

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

// Widget IDs shared by the Filter and Select group dialogs (twins: see docs/agents/dialogs.md).
const (
	PatternShellRadio  = dlg.ID("mode:shell")
	PatternRegexRadio  = dlg.ID("mode:regex")
	PatternSimpleRadio = dlg.ID("mode:simple")
	PatternInput       = dlg.ID("pattern")
	PatternFilesOnly   = dlg.ID("opt:files-only")
	PatternDirsOnly    = dlg.ID("opt:dirs-only")
	PatternInvert      = dlg.ID("opt:invert")
	PatternCase        = dlg.ID("opt:case")
	PatternFullPath    = dlg.ID("opt:full-path")
	PatternIncludeMeta = dlg.ID("opt:include-meta")
	PatternOnlyMeta    = dlg.ID("opt:only-meta")

	patternTypePrefix = "type:"
)

// PatternTypeID is the ID of the file-type checkbox i (FilterTypeLabels order).
func PatternTypeID(i int) dlg.ID { return dlg.IndexedID(patternTypePrefix, i) }

// patternTypeIndex is the inverse of PatternTypeID.
func patternTypeIndex(id dlg.ID) (int, bool) {
	i, ok := dlg.IDIndex(id, patternTypePrefix)
	return i, ok && i >= 0 && i < FilterTypeCount
}

// PatternIsRadio reports whether id is one of the pattern mode radios (Enter selects a radio but
// confirms the dialog from any other widget).
func PatternIsRadio(id dlg.ID) bool {
	return id == PatternShellRadio || id == PatternRegexRadio || id == PatternSimpleRadio
}

// patternDialogFrameWidth is the frame width of the Filter and Select group dialogs on a screen
// layoutWidth wide: PatternDialogWidth, kept two cells off each side of the screen, and never
// narrower than the 30 cells the pattern input needs to be usable.
func patternDialogFrameWidth(layoutWidth int) int {
	return max(min(PatternDialogWidth, layoutWidth-4), patternDialogMinWidth)
}

const patternDialogMinWidth = 30

// patternView is what the Filter and Select group dialogs share: the same mode radios, file-type
// checkboxes, pattern input with its live preview and hint, and option rows. Each dialog fills in
// the fields of its own state and which optional checkboxes it has.
type patternView struct {
	Title       string
	LayoutWidth int

	Mode          panel.GroupPatternMode
	Types         [FilterTypeCount]bool
	FilesOnly     bool
	DirsOnly      bool
	CaseSensitive bool
	ShowFilesDirs bool
	ShowInvert    bool // Filter only
	Invert        bool
	ShowFullPath  bool // Select group in the find and output contexts only
	FullPath      bool
	MetaCount     int
	IncludeMeta   bool
	OnlyMeta      bool

	Text           string
	Cursor, Scroll int
	Invalid        bool
	Preview, Hint  string
}

// patternPreviewText formats the live match-count preview shown on the Pattern row:
// "<files> <file-icon> <folders> <folder-icon>", omitting either count when it's zero. Empty when
// the preview is hidden or both counts are zero.
func patternPreviewText(show bool, files, folders int, styles theme.Theme) string {
	if !show {
		return ""
	}
	var parts []string
	if files > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", files, styles.IconFile()))
	}
	if folders > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", folders, styles.IconFolder()))
	}
	return strings.Join(parts, " ")
}

// patternHintText is the compile-error hint shown under the pattern input: an explicit compile
// hint, else the error of compiling text in mode. Empty when the pattern is fine.
func patternHintText(text string, mode panel.GroupPatternMode, caseSensitive bool, compileHint string) string {
	if hint := strings.TrimSpace(compileHint); hint != "" {
		return hint
	}
	if mode != panel.GroupPatternRegex && mode != panel.GroupPatternShell {
		return ""
	}
	pat := strings.TrimSpace(text)
	if pat == "" {
		return ""
	}
	_, err := panel.NewGroupMatcher(pat, mode, caseSensitive)
	if err == nil {
		return ""
	}
	msg := err.Error()
	const regexPrefix = "invalid regexp: "
	const shellPrefix = "invalid shell pattern: "
	if strings.HasPrefix(msg, regexPrefix) {
		msg = strings.TrimPrefix(msg, regexPrefix)
	} else if strings.HasPrefix(msg, shellPrefix) {
		msg = strings.TrimPrefix(msg, shellPrefix)
	}
	if msg == "" {
		if mode == panel.GroupPatternRegex {
			return "invalid regexp"
		}
		return "invalid shell pattern"
	}
	return msg
}

// patternInvalid reports whether the pattern input renders in the dialog's error style: an
// uncompilable pattern, or a valid pattern whose live preview matches nothing.
func patternInvalid(text, hint string, previewShow bool, files, folders int) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}
	if hint != "" {
		return true
	}
	return previewShow && files == 0 && folders == 0
}

// patternSpec builds the widget spec of the Filter and Select group dialogs. Radios and type
// checkboxes share rows (the checkbox columns line up under the widest radio label), each option
// row puts its second checkbox under the first type column, and the pattern row carries its live
// preview at the right. The type checkboxes are disabled while Directories only is on.
func patternSpec(v patternView) dlg.Spec {
	// Column boxes derive from the widest labels so the checkboxes line up on every row.
	radioBox := utf8.RuneCountInString(draw.RadioText("Regular expression", false)) + 2
	typeBox := utf8.RuneCountInString(" "+draw.CheckboxText("Pictures", false)) + 2
	radio := func(id dlg.ID, label string, mnemonic rune, mode panel.GroupPatternMode) dlg.Item {
		return dlg.Item{ID: id, Kind: dlg.KindRadio, Label: label, Mnemonic: mnemonic, On: v.Mode == mode, Width: radioBox}
	}
	check := func(id dlg.ID, label string, mnemonic rune, on bool) dlg.Item {
		return dlg.Item{ID: id, Kind: dlg.KindCheckbox, Label: label, Mnemonic: mnemonic, On: on, Width: radioBox}
	}
	radios := []dlg.Item{
		radio(PatternShellRadio, "Shell patterns", PatternMnemonicShell, panel.GroupPatternShell),
		radio(PatternRegexRadio, "Regular expression", PatternMnemonicRegex, panel.GroupPatternRegex),
		radio(PatternSimpleRadio, "Simple", PatternMnemonicSimple, panel.GroupPatternSimple),
	}
	// Head: a radio per row with the type checkboxes in two columns to its right. Radios and
	// checkboxes are separate Tab stops (Seg).
	var head []dlg.Item
	for i, r := range radios {
		cells := []dlg.Item{r}
		for t := i; t < FilterTypeCount; t += 3 {
			cell := dlg.Item{
				ID: PatternTypeID(t), Kind: dlg.KindCheckbox, Label: FilterTypeLabels[t], Mnemonic: FilterTypeMnemonics[t],
				On: v.Types[t], Disabled: PatternTypesDisabled(v.DirsOnly), Seg: 1,
			}
			if t < 3 {
				cell.Width = typeBox // the first column; the second ends the row
			}
			cells = append(cells, cell)
		}
		head = append(head, dlg.Item{Kind: dlg.KindRow, Cells: cells})
	}
	head = append(head, dlg.Item{Kind: dlg.KindSeparator})

	// The live preview takes the rest of the Pattern row when it fits clear of the label.
	frameW := patternDialogFrameWidth(v.LayoutWidth)
	preview := v.Preview
	if utf8.RuneCountInString(preview) >= frameW-4-len("Pattern:") {
		preview = ""
	}
	pattern := []dlg.Item{
		{Kind: dlg.KindRow, Cells: []dlg.Item{
			{Kind: dlg.KindLabel, Label: "Pattern:"},
			{Kind: dlg.KindLabel, Label: preview, Grow: true, Right: true},
		}},
		{ID: PatternInput, Kind: dlg.KindInput, Value: v.Text, Cursor: v.Cursor, Scroll: v.Scroll, Invalid: v.Invalid},
		// The row after the input is reserved for the compile-error hint so the dialog never
		// grows when the hint appears.
		{Kind: dlg.KindLabel, Label: v.Hint},
	}
	if v.Hint != "" {
		pattern[len(pattern)-1].Style = dlg.TextError
	}

	var options []dlg.Item
	row := func(cells ...dlg.Item) { options = append(options, dlg.Item{Kind: dlg.KindRow, Cells: cells}) }
	if v.ShowFilesDirs {
		row(check(PatternFilesOnly, "Files only", PatternMnemonicFilesOnly, v.FilesOnly),
			dlg.Item{ID: PatternDirsOnly, Kind: dlg.KindCheckbox, Label: "Directories only", Mnemonic: PatternMnemonicDirsOnly, On: v.DirsOnly})
	}
	caseBox := dlg.Item{ID: PatternCase, Kind: dlg.KindCheckbox, Label: "Case sensitive", Mnemonic: PatternMnemonicCase, On: v.CaseSensitive}
	switch {
	case v.ShowInvert:
		row(check(PatternInvert, "Invert", PatternMnemonicInvert, v.Invert), caseBox)
	case v.ShowFullPath:
		row(check(PatternCase, "Case sensitive", PatternMnemonicCase, v.CaseSensitive),
			dlg.Item{ID: PatternFullPath, Kind: dlg.KindCheckbox, Label: "Match full path", Mnemonic: PatternMnemonicFullPath, On: v.FullPath})
		// Up from the buttons lands on Match full path, the last checkbox of the last row.
		options[len(options)-1].EntryLast = v.MetaCount == 0
	default:
		row(check(PatternCase, "Case sensitive", PatternMnemonicCase, v.CaseSensitive))
	}
	if v.MetaCount > 0 {
		row(check(PatternIncludeMeta, "Include meta columns", PatternMnemonicIncludeMeta, v.IncludeMeta),
			dlg.Item{ID: PatternOnlyMeta, Kind: dlg.KindCheckbox, Label: "Only meta columns", Mnemonic: PatternMnemonicOnlyMeta, On: v.OnlyMeta})
	}

	return dlg.Spec{
		Title: v.Title, Width: PatternDialogWidth, MinWidth: patternDialogMinWidth,
		Groups: []dlg.Group{
			{Columns: [][]dlg.Item{head}},
			{Columns: [][]dlg.Item{pattern}},
			{Columns: [][]dlg.Item{options}},
		},
		Buttons: []dlg.Button{
			dlg.OKButton,
			dlg.CancelButton,
		},
	}
}

// patternToggles are the fields the Filter and Select group dialogs share, as pointers into the
// dialog's state; Invert and FullPath are nil where the dialog has no such checkbox.
type patternToggles struct {
	Mode                                    *panel.GroupPatternMode
	Types                                   *[FilterTypeCount]bool
	FilesOnly, DirsOnly, Case               *bool
	Invert, FullPath, IncludeMeta, OnlyMeta *bool
}

// apply folds the activation of widget id into the state: a radio selects its mode, a checkbox
// toggles (Files only / Directories only exclude each other, Only meta implies Include meta, and
// dropping Include meta drops Only meta). It reports whether the state changed.
func (t patternToggles) apply(id dlg.ID) bool {
	flip := func(p *bool) bool {
		if p == nil {
			return false
		}
		*p = !*p
		return true
	}
	switch id {
	case PatternShellRadio:
		*t.Mode = panel.GroupPatternShell
	case PatternRegexRadio:
		*t.Mode = panel.GroupPatternRegex
	case PatternSimpleRadio:
		*t.Mode = panel.GroupPatternSimple
	case PatternFilesOnly:
		*t.FilesOnly = !*t.FilesOnly
		if *t.FilesOnly {
			*t.DirsOnly = false
		}
	case PatternDirsOnly:
		*t.DirsOnly = !*t.DirsOnly
		if *t.DirsOnly {
			*t.FilesOnly = false
		}
	case PatternInvert:
		return flip(t.Invert)
	case PatternCase:
		return flip(t.Case)
	case PatternFullPath:
		return flip(t.FullPath)
	case PatternIncludeMeta:
		*t.IncludeMeta = !*t.IncludeMeta
		if !*t.IncludeMeta {
			*t.OnlyMeta = false
		}
	case PatternOnlyMeta:
		*t.OnlyMeta = !*t.OnlyMeta
		if *t.OnlyMeta {
			*t.IncludeMeta = true
		}
	default:
		i, ok := patternTypeIndex(id)
		if !ok || PatternTypesDisabled(*t.DirsOnly) {
			return false
		}
		t.Types[i] = !t.Types[i]
	}
	return true
}
