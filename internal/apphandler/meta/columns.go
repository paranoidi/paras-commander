package meta

import (
	"fmt"
	"slices"
	"strings"

	"github.com/paranoidi/paras-commander/internal/columnprofile"
	"github.com/paranoidi/paras-commander/internal/metacmds"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// columnSnapshot is what a panel showed before [fileindex].default_profile applied.
type columnSnapshot struct {
	profile string
	columns panel.ColumnSet
}

// ApplyProfile switches the panel to a profile's file and meta columns. It is the path of the
// profile cycle, the Columns dialog and startup; every change of a panel's columns ends in
// applyColumns.
func (h *Handler) ApplyProfile(panelID int, profile columnprofile.Profile) {
	h.applyColumns(panelID, h.profileColumns(profile), profile.Name, profile.Meta)
}

// profileColumns is the profile's file columns, warning about names no column has.
func (h *Handler) profileColumns(profile columnprofile.Profile) panel.ColumnSet {
	set, unknown := profile.Set()
	if len(unknown) > 0 {
		h.host.SetTransientMessage(fmt.Sprintf("Profile %s: unknown columns %s", profile.Name, strings.Join(unknown, ", ")), ui.MessageUrgencyWarn)
	}
	return set
}

// dropMetaSort falls back to Name at once, like an explicit Sort dialog apply, rather than leaving
// the panel sorted by a column that just vanished from the header.
func (h *Handler) dropMetaSort(panelID int, p *panel.State) {
	p.Sort.Mode = panel.SortName
	p.Sort.MetaColumn = ""
	h.host.ResortPanel(panelID)
}

// applyColumns sets the panel's file columns and profile name and switches its meta columns,
// re-running nothing when they stay the same. A user change of columns: the panel no longer
// follows the index default.
func (h *Handler) applyColumns(panelID int, set panel.ColumnSet, profile string, metaNames []string) {
	p := h.host.PanelByID(panelID)
	if p == nil {
		return
	}
	h.autoColumns[panelID] = nil
	h.setFileColumns(panelID, p, set, profile)
	h.switchMeta(panelID, p, metaNames)
}

// setFileColumns stores the file columns; turning on Created reloads the listing, which only
// fetches birth times while that column is shown.
func (h *Handler) setFileColumns(panelID int, p *panel.State, set panel.ColumnSet, profile string) {
	needsReload := set.Has(panel.ColumnCreated) && !p.Columns.Has(panel.ColumnCreated)
	p.Columns = set
	p.ColumnProfile = profile
	if needsReload && !p.Path.IsZero() {
		h.host.ReloadPanelListing(panelID)
	}
}

// switchMeta makes names the panel's meta columns. A meta sort on a column that goes away falls
// back to Name at once.
func (h *Handler) switchMeta(panelID int, p *panel.State, names []string) {
	prev := h.activeEntries[panelID]
	if len(names) == 0 {
		if len(prev) > 0 {
			h.deactivate(panelID)
		}
		return
	}
	names = h.limitActiveNames(names)
	if slices.Equal(prev, names) {
		return
	}
	if p.Sort.Mode == panel.SortMeta && !slices.Contains(names, p.Sort.MetaColumn) {
		h.dropMetaSort(panelID, p)
	}
	wasActive := make(map[string]bool, len(prev))
	for _, n := range prev {
		wasActive[n] = true
	}
	h.sortCheck[panelID] = wasActive
	h.activate(panelID, names)
}

// sortOnActivation applies the first newly active column's sort_on_activation, once per column
// change, now that the load has told which entries ask for it.
func (h *Handler) sortOnActivation(p *panel.State, panelID int, sorted []metacmds.MetaEntry) {
	wasActive := h.sortCheck[panelID]
	if wasActive == nil {
		return
	}
	h.sortCheck[panelID] = nil
	for _, name := range h.activeEntries[panelID] {
		if wasActive[name] {
			continue
		}
		for _, e := range sorted {
			if e.Name == name && e.SortOnActivation {
				p.Sort.Mode = panel.SortMeta
				p.Sort.MetaColumn = name
				p.Sort.Reverse = e.SortReverse
				h.host.ResortPanel(panelID)
				return
			}
		}
	}
}

// CycleProfile moves the panel to the next profile of the cycle.
func (h *Handler) CycleProfile(panelID int) {
	p := h.host.PanelByID(panelID)
	if p == nil {
		return
	}
	next := columnprofile.Next(h.profiles, p.ColumnProfile)
	h.ApplyProfile(panelID, next)
	h.host.SetTransientMessage("Columns: "+next.Name, ui.MessageUrgencyInfo)
}

// ActivateStartupMeta switches on the startup profile's meta columns on both panels; the panels
// were constructed with its file columns.
func (h *Handler) ActivateStartupMeta(names []string) {
	if len(names) == 0 {
		return
	}
	for panelID := range h.activeEntries {
		if p := h.host.PanelByID(panelID); p != nil {
			h.switchMeta(panelID, p, names)
		}
	}
}

// applyIndexProfile applies [fileindex].default_profile on entering an index view, remembering
// what to restore; false when there is none to apply.
func (h *Handler) applyIndexProfile(panelID int, p *panel.State) bool {
	name := h.config.FileIndex.DefaultProfile
	if name == "" {
		return false
	}
	profile, ok := columnprofile.Find(h.profiles, name)
	if !ok {
		h.host.SetTransientMessage(fmt.Sprintf("Unknown column profile %q", name), ui.MessageUrgencyWarn)
		return false
	}
	snapshot := &columnSnapshot{profile: p.ColumnProfile, columns: p.Columns}
	h.ApplyProfile(panelID, profile)
	h.autoColumns[panelID] = snapshot
	return true
}

// restoreAutoColumns undoes applyIndexProfile on leaving the index views.
func (h *Handler) restoreAutoColumns(panelID int, p *panel.State) {
	snapshot := h.autoColumns[panelID]
	h.autoColumns[panelID] = nil
	h.setFileColumns(panelID, p, snapshot.columns, snapshot.profile)
	h.deactivate(panelID)
}

// matchingProfile names the first profile showing exactly these columns, "" when none does.
func (h *Handler) matchingProfile(set panel.ColumnSet, metaNames []string) string {
	want := slices.Sorted(slices.Values(metaNames))
	for _, profile := range columnprofile.All(h.profiles) {
		if s, _ := profile.Set(); s == set && slices.Equal(slices.Sorted(slices.Values(profile.Meta)), want) {
			return profile.Name
		}
	}
	return ""
}
