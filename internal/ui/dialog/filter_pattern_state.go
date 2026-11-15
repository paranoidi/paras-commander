package dialog

import (
	"slices"
	"strings"

	"github.com/paranoidi/paras-commander/internal/filterpattern"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// FilterPhase is the screen the Filter dialog shows.
type FilterPhase int

const (
	// FilterPhaseMain is the pattern and its options.
	FilterPhaseMain FilterPhase = iota
	// FilterPhaseLoadPicker lists the saved patterns; picking one fills the dialog.
	FilterPhaseLoadPicker
	// FilterPhaseHistoryPicker lists the patterns applied this session; picking one fills the dialog.
	FilterPhaseHistoryPicker
	// FilterPhaseOverwritePicker lists the saved patterns; picking one overwrites it.
	FilterPhaseOverwritePicker
	// FilterPhaseSavePrompt asks for a new saved pattern's name and description.
	FilterPhaseSavePrompt
)

// FilterPickerPhase reports whether phase is one of the fuzzy picker screens.
func FilterPickerPhase(phase FilterPhase) bool {
	switch phase {
	case FilterPhaseLoadPicker, FilterPhaseHistoryPicker, FilterPhaseOverwritePicker:
		return true
	default:
		return false
	}
}

// FilterPatternPicker is the fuzzy-filtered pattern list of the picker phases.
type FilterPatternPicker struct {
	Items       []filterpattern.Pattern
	Query       string
	QueryCursor int
	QueryScroll int
	Ranked      []int
	MatchRanges [][]search.Range
	Selected    int
	ListScroll  int
}

// Widgets of the Filter dialog's save prompt.
const (
	FilterSaveNameInput dlg.ID = "filter-save-name"
	FilterSaveDescInput dlg.ID = "filter-save-desc"
)

// Screen is the Filter dialog's screen on top: a picker or the save prompt with ScreenFocus, else
// the main screen with Focus.
func (s *FilterDialogState) Screen(layout Layout, styles theme.Theme) DialogScreen {
	switch {
	case FilterPickerPhase(s.Phase):
		return DialogScreen{filterPickerView(*s, layout.Height, styles.IconSearchIcon()), &s.ScreenFocus}
	case s.Phase == FilterPhaseSavePrompt:
		return DialogScreen{savePromptSpec("Save filter", FilterSaveNameInput, s.SaveName, FilterSaveDescInput, s.SaveDesc), &s.ScreenFocus}
	}
	return DialogScreen{FilterDialogView(*s, layout, styles), &s.Focus}
}

func filterPickerView(s FilterDialogState, screenH int, searchIcon string) dlg.Spec {
	title := "Load filter"
	switch s.Phase {
	case FilterPhaseHistoryPicker:
		title = "Filter history"
	case FilterPhaseOverwritePicker:
		title = "Overwrite filter"
	}
	pk := s.Picker
	return pickerSpec(title, savedPickerWidth, screenH, searchIcon, len(pk.Items),
		func(i int) string { return FilterPatternLine(pk.Items[i]) },
		pk.Ranked, pk.MatchRanges, pk.Query, pk.QueryCursor, pk.QueryScroll, pk.Selected, pk.ListScroll)
}

// FilterPatternLine is a pattern's picker row, also its fuzzy-search corpus: "Name — Description"
// for a saved pattern, a summary of the pattern and its options for an unnamed history entry.
func FilterPatternLine(p filterpattern.Pattern) string {
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
	types := FilterTypesFromNames(p.Types)
	if _, label := filterTypeLabels(p.DirsOnly, types); label != "" {
		parts = append(parts, label)
	}
	var flags []string
	for _, f := range []struct {
		on    bool
		label string
	}{
		{p.FilesOnly, "files only"}, {p.DirsOnly, "directories only"}, {p.Invert, "invert"},
		{p.CaseSensitive, "case sensitive"}, {p.OnlyMeta, "only meta"},
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

// filterTypeLabels returns whether any type applies and the comma-joined checked type labels.
func filterTypeLabels(dirsOnly bool, types [FilterTypeCount]bool) (bool, string) {
	if PatternTypesDisabled(dirsOnly) {
		return false, ""
	}
	var labels []string
	for i, on := range types {
		if on {
			labels = append(labels, FilterTypeLabels[i])
		}
	}
	return len(labels) > 0, strings.Join(labels, ", ")
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

func filterModeName(m panel.GroupPatternMode) string {
	switch m {
	case panel.GroupPatternShell:
		return filterpattern.ModeShell
	case panel.GroupPatternRegex:
		return filterpattern.ModeRegex
	default:
		return filterpattern.ModeSimple
	}
}

func filterModeFromName(s string) panel.GroupPatternMode {
	switch s {
	case filterpattern.ModeShell:
		return panel.GroupPatternShell
	case filterpattern.ModeRegex:
		return panel.GroupPatternRegex
	default:
		return panel.GroupPatternSimple
	}
}

// SavedPattern returns the dialog's pattern and options as a saved pattern named name. Options the
// dialog does not show are stored as the dialog opens with them: Include meta on (it is on
// whenever there are meta columns), Files only and Directories only off.
func (s FilterDialogState) SavedPattern(name, description string) filterpattern.Pattern {
	p := filterpattern.Pattern{
		Name: name, Description: description, Pattern: s.Text, Mode: filterModeName(s.PatternMode),
		Types: FilterTypesToNames(s.Types), FilesOnly: s.FilesOnly, DirsOnly: s.DirsOnly,
		CaseSensitive: s.CaseSensitive, Invert: s.Invert,
		IncludeMeta: s.IncludeMetaColumns, OnlyMeta: s.OnlyMetaColumns,
	}
	if s.MetaColumnCount == 0 {
		p.IncludeMeta, p.OnlyMeta = true, false
	}
	return p
}

// LoadPattern fills the pattern and options from p, keeping only what this dialog shows: no Files
// only / Directories only outside the panel, no meta matching without meta columns.
func (s *FilterDialogState) LoadPattern(p filterpattern.Pattern) {
	s.Text = p.Pattern
	s.TextCursor = len([]rune(p.Pattern))
	s.TextScroll = 0
	s.PrefillPending = false
	s.PatternMode = filterModeFromName(p.Mode)
	s.Types = FilterTypesFromNames(p.Types)
	s.CaseSensitive = p.CaseSensitive
	s.Invert = p.Invert
	s.FilesOnly, s.DirsOnly = false, false
	if FilterShowsFilesDirs(*s) {
		s.FilesOnly = p.FilesOnly
		s.DirsOnly = p.DirsOnly && !p.FilesOnly
	}
	s.IncludeMetaColumns, s.OnlyMetaColumns = false, false
	if s.MetaColumnCount > 0 {
		s.IncludeMetaColumns = p.IncludeMeta || p.OnlyMeta
		s.OnlyMetaColumns = p.OnlyMeta
	}
}
