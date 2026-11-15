package dialog

import (
	"slices"
	"strings"
	"unicode"

	"github.com/paranoidi/paras-commander/internal/filterpattern"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// PatternPhase is the screen the Filter or Select group dialog shows.
type PatternPhase int

const (
	// PatternPhaseMain is the pattern and its options.
	PatternPhaseMain PatternPhase = iota
	// PatternPhaseLoadPicker lists the saved patterns; picking one fills the dialog.
	PatternPhaseLoadPicker
	// PatternPhaseHistoryPicker lists the patterns applied this session; picking one fills the dialog.
	PatternPhaseHistoryPicker
	// PatternPhaseOverwritePicker lists the saved patterns; picking one overwrites it.
	PatternPhaseOverwritePicker
	// PatternPhaseSavePrompt asks for a new saved pattern's name and description.
	PatternPhaseSavePrompt
)

// PatternPickerPhase reports whether phase is one of the fuzzy picker screens.
func PatternPickerPhase(phase PatternPhase) bool {
	switch phase {
	case PatternPhaseLoadPicker, PatternPhaseHistoryPicker, PatternPhaseOverwritePicker:
		return true
	default:
		return false
	}
}

// SavedPatternPicker is the fuzzy-filtered pattern list of the picker phases.
type SavedPatternPicker struct {
	Items       []filterpattern.Pattern
	Query       string
	QueryCursor int
	QueryScroll int
	Ranked      []int
	MatchRanges [][]search.Range
	Selected    int
	ListScroll  int
}

// SavedPatternScreens is the saved-pattern sub-screens of the Filter and Select group dialogs
// (embedded in both states): which screen is on top, the focus of a picker or the save prompt,
// and their contents.
type SavedPatternScreens struct {
	// Phase is the screen on top; ScreenFocus is the focused widget of a picker or the save
	// prompt, empty for its first. The dialog's own Focus is kept while a sub-screen is on top,
	// so returning lands where the user was.
	Phase       PatternPhase
	ScreenFocus dlg.ID
	Picker      SavedPatternPicker
	SaveName    PromptField
	SaveDesc    PromptField
}

// Widgets of the saved-pattern save prompt.
const (
	PatternSaveNameInput dlg.ID = "pattern-save-name"
	PatternSaveDescInput dlg.ID = "pattern-save-desc"
)

// screen is the sub-screen on top, or the main spec with mainFocus. noun names the saved thing in
// the titles ("filter", "pattern").
func (s *SavedPatternScreens) screen(noun string, main dlg.Spec, mainFocus *dlg.ID, screenH int, searchIcon string) DialogScreen {
	switch {
	case PatternPickerPhase(s.Phase):
		title := "Load " + noun
		switch s.Phase {
		case PatternPhaseHistoryPicker:
			title = capitalize(noun) + " history"
		case PatternPhaseOverwritePicker:
			title = "Overwrite " + noun
		}
		pk := s.Picker
		spec := pickerSpec(title, savedPickerWidth, screenH, searchIcon, len(pk.Items),
			func(i int) string { return SavedPatternLine(pk.Items[i]) },
			pk.Ranked, pk.MatchRanges, pk.Query, pk.QueryCursor, pk.QueryScroll, pk.Selected, pk.ListScroll)
		return DialogScreen{spec, &s.ScreenFocus}
	case s.Phase == PatternPhaseSavePrompt:
		return DialogScreen{savePromptSpec("Save "+noun, PatternSaveNameInput, s.SaveName, PatternSaveDescInput, s.SaveDesc), &s.ScreenFocus}
	}
	return DialogScreen{main, mainFocus}
}

func capitalize(s string) string {
	r := []rune(s)
	if len(r) > 0 {
		r[0] = unicode.ToUpper(r[0])
	}
	return string(r)
}

// SavedPatternLine is a pattern's picker row, also its fuzzy-search corpus: "Name — Description"
// for a saved pattern, a summary of the pattern and its options for an unnamed history entry.
func SavedPatternLine(p filterpattern.Pattern) string {
	if name := strings.TrimSpace(p.Name); name != "" {
		if p.Description != "" {
			return name + " — " + p.Description
		}
		return name
	}
	var parts []string
	if p.Pattern != "" {
		parts = append(parts, p.Mode+": "+p.Pattern)
	}
	if label := filterTypeLabels(p.DirsOnly, FilterTypesFromNames(p.Types)); label != "" {
		parts = append(parts, label)
	}
	var flags []string
	for _, f := range []struct {
		on    bool
		label string
	}{
		{p.FilesOnly, "files only"}, {p.DirsOnly, "directories only"}, {p.Invert, "invert"},
		{p.CaseSensitive, "case sensitive"}, {p.FullPath, "full path"}, {p.OnlyMeta, "only meta"},
	} {
		if f.on {
			flags = append(flags, f.label)
		}
	}
	if len(flags) > 0 {
		parts = append(parts, strings.Join(flags, ", "))
	}
	return strings.Join(parts, " · ")
}

// filterTypeLabels returns the comma-joined labels of the checked types that apply.
func filterTypeLabels(dirsOnly bool, types [FilterTypeCount]bool) string {
	if PatternTypesDisabled(dirsOnly) {
		return ""
	}
	var labels []string
	for i, on := range types {
		if on {
			labels = append(labels, FilterTypeLabels[i])
		}
	}
	return strings.Join(labels, ", ")
}

// filterTypeName is the filters.toml name of type i (its label in lowercase).
func filterTypeName(i int) string { return strings.ToLower(FilterTypeLabels[i]) }

// FilterTypesToNames returns the filters.toml names of the checked types, in checkbox order.
func FilterTypesToNames(types [FilterTypeCount]bool) []string {
	var names []string
	for i, on := range types {
		if on {
			names = append(names, filterTypeName(i))
		}
	}
	return names
}

// FilterTypesFromNames checks the types named in names; unknown names are ignored.
func FilterTypesFromNames(names []string) [FilterTypeCount]bool {
	var types [FilterTypeCount]bool
	for i := range types {
		types[i] = slices.ContainsFunc(names, func(n string) bool { return strings.EqualFold(strings.TrimSpace(n), filterTypeName(i)) })
	}
	return types
}

func patternModeName(m panel.GroupPatternMode) string {
	switch m {
	case panel.GroupPatternShell:
		return filterpattern.ModeShell
	case panel.GroupPatternRegex:
		return filterpattern.ModeRegex
	default:
		return filterpattern.ModeSimple
	}
}

func patternModeFromName(s string) panel.GroupPatternMode {
	switch s {
	case filterpattern.ModeShell:
		return panel.GroupPatternShell
	case filterpattern.ModeRegex:
		return panel.GroupPatternRegex
	default:
		return panel.GroupPatternSimple
	}
}

// patternFields points at one twin dialog's pattern and options, with what that dialog shows: the
// single conversion between a dialog state and a saved pattern. A nil Invert / FullPath toggle
// means the dialog has no such option.
type patternFields struct {
	Text           *string
	Cursor, Scroll *int
	PrefillPending *bool
	Toggles        patternToggles
	MetaCount      int
	ShowFilesDirs  bool
	ShowFullPath   bool
}

// saved returns the pattern and options as a saved pattern named name. Options the dialog does
// not show are stored as the dialog opens with them: Include meta on (it is on whenever there are
// meta columns), the rest off.
func (f patternFields) saved(name, description string) filterpattern.Pattern {
	t := f.Toggles
	p := filterpattern.Pattern{
		Name: name, Description: description, Pattern: *f.Text, Mode: patternModeName(*t.Mode),
		Types: FilterTypesToNames(*t.Types), CaseSensitive: *t.Case,
		IncludeMeta: *t.IncludeMeta, OnlyMeta: *t.OnlyMeta,
	}
	if f.ShowFilesDirs {
		p.FilesOnly, p.DirsOnly = *t.FilesOnly, *t.DirsOnly
	}
	if t.Invert != nil {
		p.Invert = *t.Invert
	}
	if t.FullPath != nil && f.ShowFullPath {
		p.FullPath = *t.FullPath
	}
	if f.MetaCount == 0 {
		p.IncludeMeta, p.OnlyMeta = true, false
	}
	return p
}

// load fills the pattern and options from p, keeping only what the dialog shows.
func (f patternFields) load(p filterpattern.Pattern) {
	t := f.Toggles
	*f.Text = p.Pattern
	*f.Cursor = len([]rune(p.Pattern))
	*f.Scroll = 0
	*f.PrefillPending = false
	*t.Mode = patternModeFromName(p.Mode)
	*t.Types = FilterTypesFromNames(p.Types)
	*t.Case = p.CaseSensitive
	*t.FilesOnly, *t.DirsOnly = false, false
	if f.ShowFilesDirs {
		*t.FilesOnly = p.FilesOnly
		*t.DirsOnly = p.DirsOnly && !p.FilesOnly
	}
	if t.Invert != nil {
		*t.Invert = p.Invert
	}
	if t.FullPath != nil {
		*t.FullPath = f.ShowFullPath && p.FullPath
	}
	*t.IncludeMeta, *t.OnlyMeta = false, false
	if f.MetaCount > 0 {
		*t.IncludeMeta = p.IncludeMeta || p.OnlyMeta
		*t.OnlyMeta = p.OnlyMeta
	}
}

func (s *FilterDialogState) patternFields() patternFields {
	return patternFields{
		Text: &s.Text, Cursor: &s.TextCursor, Scroll: &s.TextScroll, PrefillPending: &s.PrefillPending,
		Toggles: s.toggles(), MetaCount: s.MetaColumnCount, ShowFilesDirs: FilterShowsFilesDirs(*s),
	}
}

func (s *GroupSelectState) patternFields() patternFields {
	return patternFields{
		Text: &s.Text, Cursor: &s.TextCursor, Scroll: &s.TextScroll, PrefillPending: &s.PrefillPending,
		Toggles: s.toggles(), MetaCount: s.MetaColumnCount, ShowFilesDirs: true, ShowFullPath: GroupSelectShowsFullPath(*s),
	}
}

// SavedPattern returns the Filter dialog's pattern and options as a saved pattern named name.
func (s FilterDialogState) SavedPattern(name, description string) filterpattern.Pattern {
	return s.patternFields().saved(name, description)
}

// LoadPattern fills the Filter dialog from p: no Files only / Directories only outside the panel,
// no meta matching without meta columns, no Match full path.
func (s *FilterDialogState) LoadPattern(p filterpattern.Pattern) { s.patternFields().load(p) }

// SavedPattern returns the Select group dialog's pattern and options as a saved pattern named name.
func (s GroupSelectState) SavedPattern(name, description string) filterpattern.Pattern {
	return s.patternFields().saved(name, description)
}

// LoadPattern fills the Select group dialog from p: no Invert, Match full path only where it is
// shown, no meta matching without meta columns.
func (s *GroupSelectState) LoadPattern(p filterpattern.Pattern) { s.patternFields().load(p) }

// Screen is the Filter dialog's screen on top: a saved-pattern sub-screen, else the main screen.
func (s *FilterDialogState) Screen(layout Layout, styles theme.Theme) DialogScreen {
	return s.screen("filter", FilterDialogView(*s, layout, styles), &s.Focus, layout.Height, styles.IconSearchIcon())
}

// Screen is the Select group dialog's screen on top: a saved-pattern sub-screen, else the main
// screen.
func (s *GroupSelectState) Screen(layout Layout, styles theme.Theme) DialogScreen {
	return s.screen("pattern", GroupSelectView(*s, layout, styles), &s.Focus, layout.Height, styles.IconSearchIcon())
}
