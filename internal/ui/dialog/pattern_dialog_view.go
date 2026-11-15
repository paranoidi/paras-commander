package dialog

import (
	"fmt"
	"path/filepath"
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
	PatternScopeFiles  = dlg.ID("scope:files")
	PatternScopeDirs   = dlg.ID("scope:dirs")
	PatternScopeBoth   = dlg.ID("scope:both")
	PatternInvert      = dlg.ID("opt:invert")
	PatternCase        = dlg.ID("opt:case")
	PatternFullPath    = dlg.ID("opt:full-path")
	PatternIncludeMeta = dlg.ID("opt:include-meta")
	PatternOnlyMeta    = dlg.ID("opt:only-meta")
	// PatternClearButton is the Filter dialog's Clear button (removes the active filter).
	PatternClearButton = dlg.ID("clear")

	patternTypePrefix = "type:"
)

// PatternTypeID is the ID of the file-type checkbox i (FilterTypeLabels order).
func PatternTypeID(i int) dlg.ID { return dlg.IndexedID(patternTypePrefix, i) }

// patternTypeIndex is the inverse of PatternTypeID.
func patternTypeIndex(id dlg.ID) (int, bool) {
	i, ok := dlg.IDIndex(id, patternTypePrefix)
	return i, ok && i >= 0 && i < FilterTypeCount
}

// PatternIsRadio reports whether id is one of the pattern mode or scope radios (Enter selects a
// radio but confirms the dialog from any other widget).
func PatternIsRadio(id dlg.ID) bool {
	switch id {
	case PatternShellRadio, PatternRegexRadio, PatternSimpleRadio, PatternScopeFiles, PatternScopeDirs, PatternScopeBoth:
		return true
	}
	return false
}

// patternScopeRadios are the scope radios (Files / Dirs / Both) in display order.
var patternScopeRadios = [...]struct {
	id       dlg.ID
	label    string
	mnemonic rune
	scope    panel.PatternScope
}{
	{PatternScopeFiles, "Files", PatternMnemonicScopeFiles, panel.ScopeFiles},
	{PatternScopeDirs, "Dirs", PatternMnemonicScopeDirs, panel.ScopeDirs},
	{PatternScopeBoth, "Both", PatternMnemonicScopeBoth, panel.ScopeBoth},
}

// patternDialogWidth is the frame width of the Filter and Select group dialogs for a marker
// markerW cells wide (dlg.MarkerWidth; 0 = the ASCII marker): the grid's first two columns, the
// Regular expression radio and the frame (border, padding and a one-cell right margin). It is 65
// with the default theme's icons.
func patternDialogWidth(markerW int) int {
	if markerW <= 0 {
		markerW = draw.DialogOptionWidth("")
	}
	cols := patternColumns(markerW)
	const frame = 3
	return cols[0] + cols[1] + markerW + utf8.RuneCountInString(patternModeLabels[2]) + frame
}

// patternDialogFrameWidth is the frame width of the Filter and Select group dialogs on a screen
// layoutWidth wide: patternDialogWidth for the marker markerW cells wide, kept two cells off
// each side of the screen, and never narrower than the 30 cells the pattern input needs.
func patternDialogFrameWidth(layoutWidth, markerW int) int {
	return max(min(patternDialogWidth(markerW), layoutWidth-4), patternDialogMinWidth)
}

const patternDialogMinWidth = 30

// patternView is what the Filter and Select group dialogs share: the same mode radios, file-type
// checkboxes, pattern input with its live preview and hint, and option rows. Each dialog fills in
// the fields of its own state and which optional checkboxes it has.
type patternView struct {
	Title       string
	LayoutWidth int
	// MarkerWidth is the theme's radio / checkbox marker width (dlg.MarkerWidth) the layout is
	// measured with; 0 measures the ASCII marker.
	MarkerWidth int

	Mode          panel.GroupPatternMode
	Types         [FilterTypeCount]bool
	Scope         panel.PatternScope
	CaseSensitive bool
	ShowScope     bool
	ShowInvert    bool // Filter only
	ShowClear     bool // Filter only, while the target has a filter: a "Clear filter" button after OK / Cancel
	Invert        bool
	ShowFullPath  bool // Select group in the find and output contexts only
	FullPath      bool
	MetaCount     int
	IncludeMeta   bool
	OnlyMeta      bool

	Text           string
	Cursor, Scroll int
	Placeholder    bool // Text is a suggested default (Select group's ghost "*.ext")
	Invalid        bool
	Preview, Hint  string
}

// GhostExtPattern is the suggested pattern a pattern dialog (Filter, Select group) opens with:
// "*.<ext>" of the cursor entry, or "" for a directory or a name without an extension.
func GhostExtPattern(name string, isDir bool) string {
	ext := filepath.Ext(name)
	if isDir || ext == "" || ext == name {
		return ""
	}
	return "*" + ext
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

// patternModeLabels are the mode radio labels in display order (Simple, Shell, Regex).
var patternModeLabels = [...]string{"Simple / auto shell", "Shell patterns", "Regular expression"}

// patternSpec builds the widget spec of the Filter and Select group dialogs on one three-column
// grid (patternColumns): the mode radios on one row, a separator, the type checkboxes in three
// columns and a blank row, the Pattern row with its live preview at the right, the input and its
// hint row, then the scope radios and the option rows. The type checkboxes are disabled while the
// scope is Dirs.
func patternSpec(v patternView) dlg.Spec {
	cols := patternColumns(v.MarkerWidth)
	radio := func(id dlg.ID, label string, mnemonic rune, mode panel.GroupPatternMode, col int) dlg.Item {
		return dlg.Item{ID: id, Kind: dlg.KindRadio, Label: label, Mnemonic: mnemonic, On: v.Mode == mode, Width: cols[col]}
	}
	// check is a checkbox in the grid's first column (the second ends its row).
	check := func(id dlg.ID, label string, mnemonic rune, on bool) dlg.Item {
		return dlg.Item{ID: id, Kind: dlg.KindCheckbox, Label: label, Mnemonic: mnemonic, On: on, Width: cols[0]}
	}
	// Radios and type checkboxes are separate Tab stops (Seg).
	head := []dlg.Item{
		{Kind: dlg.KindRow, Cells: []dlg.Item{
			radio(PatternSimpleRadio, patternModeLabels[0], PatternMnemonicSimple, panel.GroupPatternSimple, 0),
			radio(PatternShellRadio, patternModeLabels[1], PatternMnemonicShell, panel.GroupPatternShell, 1),
			radio(PatternRegexRadio, patternModeLabels[2], PatternMnemonicRegex, panel.GroupPatternRegex, 2),
		}},
		{Kind: dlg.KindSeparator},
	}
	const typeCols = 3
	for start := 0; start < FilterTypeCount; start += typeCols {
		var cells []dlg.Item
		for t := start; t < min(start+typeCols, FilterTypeCount); t++ {
			cells = append(cells, dlg.Item{
				ID: PatternTypeID(t), Kind: dlg.KindCheckbox, Label: FilterTypeLabels[t], Mnemonic: FilterTypeMnemonics[t],
				On: v.Types[t], Disabled: PatternTypesDisabled(v.Scope), Seg: 1, Width: cols[t-start],
			})
		}
		head = append(head, dlg.Item{Kind: dlg.KindRow, Cells: cells})
	}
	head = append(head, dlg.Item{Kind: dlg.KindLabel}) // a blank row before the Pattern label

	// The live preview takes the rest of the Pattern row when it fits clear of the label.
	frameW := patternDialogFrameWidth(v.LayoutWidth, v.MarkerWidth)
	preview := v.Preview
	if utf8.RuneCountInString(preview) >= frameW-4-len("Pattern:") {
		preview = ""
	}
	pattern := []dlg.Item{
		{Kind: dlg.KindRow, Cells: []dlg.Item{
			{Kind: dlg.KindLabel, Label: "Pattern:"},
			{Kind: dlg.KindLabel, Label: preview, Grow: true, Right: true},
		}},
		{ID: PatternInput, Kind: dlg.KindInput, Value: v.Text, Cursor: v.Cursor, Scroll: v.Scroll, Invalid: v.Invalid, Placeholder: v.Placeholder},
		// The row after the input is reserved for the compile-error hint so the dialog never
		// grows when the hint appears.
		{Kind: dlg.KindLabel, Label: v.Hint},
	}
	if v.Hint != "" {
		pattern[len(pattern)-1].Style = dlg.TextError
	}

	var options []dlg.Item
	row := func(cells ...dlg.Item) { options = append(options, dlg.Item{Kind: dlg.KindRow, Cells: cells}) }
	if v.ShowScope {
		var cells []dlg.Item
		for i, r := range patternScopeRadios {
			cells = append(cells, dlg.Item{
				ID: r.id, Kind: dlg.KindRadio, Label: r.label, Mnemonic: r.mnemonic, On: v.Scope == r.scope, Width: cols[i],
			})
		}
		row(cells...)
	}
	caseBox := dlg.Item{ID: PatternCase, Kind: dlg.KindCheckbox, Label: "Case sensitive", Mnemonic: PatternMnemonicCase, On: v.CaseSensitive}
	switch {
	case v.ShowInvert:
		row(check(PatternInvert, "Exclude matches", PatternMnemonicInvert, v.Invert), caseBox)
	case v.ShowFullPath:
		row(check(PatternCase, "Case sensitive", PatternMnemonicCase, v.CaseSensitive),
			dlg.Item{ID: PatternFullPath, Kind: dlg.KindCheckbox, Label: "Match whole path", Mnemonic: PatternMnemonicFullPath, On: v.FullPath})
	default:
		row(check(PatternCase, "Case sensitive", PatternMnemonicCase, v.CaseSensitive))
	}
	if v.MetaCount > 0 {
		row(check(PatternIncludeMeta, "Include meta columns", PatternMnemonicIncludeMeta, v.IncludeMeta),
			dlg.Item{ID: PatternOnlyMeta, Kind: dlg.KindCheckbox, Label: "Only meta columns", Mnemonic: PatternMnemonicOnlyMeta, On: v.OnlyMeta})
	}

	return dlg.Spec{
		Title: v.Title, Width: patternDialogWidth(v.MarkerWidth), MarkerWidth: v.MarkerWidth, MinWidth: patternDialogMinWidth,
		Groups: []dlg.Group{
			{Columns: [][]dlg.Item{head}},
			{Columns: [][]dlg.Item{pattern}},
			{Columns: [][]dlg.Item{options}},
		},
		Buttons: patternButtons(v.ShowClear),
	}
}

// patternColumns are the pattern dialogs' grid column boxes for a marker markerW cells wide
// (0 = the ASCII marker): each of the first two fits its widest boxed cell plus a one-cell gap,
// the third takes the rest of the row. Only rows with a third cell size the second column (the
// option rows' second cells end their row).
func patternColumns(markerW int) [3]int {
	if markerW <= 0 {
		markerW = draw.DialogOptionWidth("")
	}
	columns := [2][]string{
		// "Include meta columns" may fill its box: the next cell's marker still opens with a space.
		{patternModeLabels[0], FilterTypeLabels[FilterTypeVideos], FilterTypeLabels[FilterTypeAudio], "Files",
			"Exclude matches", "Case sensitive"},
		{patternModeLabels[1], FilterTypeLabels[FilterTypePictures], FilterTypeLabels[FilterTypeDocuments], "Dirs"},
	}
	var widths [3]int
	for i, labels := range columns {
		for _, l := range labels {
			widths[i] = max(widths[i], markerW+utf8.RuneCountInString(l)+1)
		}
	}
	return widths
}

// patternButtons is the button strip: OK / Cancel, plus "Clear filter" when the Filter dialog's target
// has a filter to clear.
func patternButtons(clear bool) []dlg.Button {
	buttons := dlg.OKCancelButtons()
	if clear {
		buttons = append(buttons, dlg.Button{ID: PatternClearButton, Label: "Clear filter", Mnemonic: PatternMnemonicClear})
	}
	return buttons
}

// patternToggles are the fields the Filter and Select group dialogs share, as pointers into the
// dialog's state; Invert and FullPath are nil where the dialog has no such checkbox.
type patternToggles struct {
	Mode                                    *panel.GroupPatternMode
	Types                                   *[FilterTypeCount]bool
	Scope                                   *panel.PatternScope
	Case                                    *bool
	Invert, FullPath, IncludeMeta, OnlyMeta *bool
}

// apply folds the activation of widget id into the state: a radio selects its mode, a checkbox
// toggles (Only meta implies Include meta, and
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
	case PatternScopeFiles:
		*t.Scope = panel.ScopeFiles
	case PatternScopeDirs:
		*t.Scope = panel.ScopeDirs
	case PatternScopeBoth:
		*t.Scope = panel.ScopeBoth
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
		if !ok || PatternTypesDisabled(*t.Scope) {
			return false
		}
		t.Types[i] = !t.Types[i]
	}
	return true
}
