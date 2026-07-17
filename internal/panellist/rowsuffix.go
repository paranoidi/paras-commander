package panellist

// NewRowSuffix assembles a RowSuffix from already-resolved per-entry inputs. This is the
// genuinely common core shared by internal/ui/panel_render.go drawPanelRow and
// internal/panelcarousel/draw.go drawCarouselColumn: both compute a job glyph, new-file tier,
// rename mark, and subtree mark and pack them into a RowSuffix, but how each caller derives
// those four values differs (name-width gating, active-column gating, closure vs direct
// lookup), so that part stays at the call site.
func NewRowSuffix(jobGlyph rune, tier NewFileMarkTier, renameMark, subtreeMark bool) RowSuffix {
	return RowSuffix{
		JobGlyph:         jobGlyph,
		NewFileTier:      tier,
		RenameMark:       renameMark,
		SubtreeSelection: subtreeMark,
	}
}
