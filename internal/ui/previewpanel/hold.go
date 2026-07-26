package previewpanel

import "strings"

// Holdable reports whether st has displayable body content worth keeping while loading.
func (st State) Holdable() bool {
	if !st.Open || st.Phase != PhaseDone {
		return false
	}
	return st.drawableBody()
}

func (st State) drawableBody() bool {
	if st.ImagePayload != "" {
		return true
	}
	if st.Source == SourceInternalHighlighted && len(st.HighlightedCells) > 0 {
		return true
	}
	if strings.TrimSpace(st.CombinedText) != "" {
		return true
	}
	if strings.TrimSpace(st.ErrorMsg) != "" {
		return true
	}
	return st.ExitCode != 0 && strings.TrimSpace(st.Path) != ""
}

// DrawWarmCandidate is true when wrapped lines should be precomputed for drawing.
func (st State) DrawWarmCandidate() bool {
	if !st.Open {
		return false
	}
	if st.BodyHeld {
		return st.drawableBody()
	}
	if st.Phase != PhaseDone || strings.TrimSpace(st.ErrorMsg) != "" {
		return false
	}
	if st.Source == SourceInternalHighlighted {
		return len(st.HighlightedCells) > 0
	}
	if strings.TrimSpace(st.CombinedText) == "" && st.ExitCode == 0 {
		return false
	}
	return true
}

// MergeDrawWithHold builds a draw snapshot that keeps the previous body while live is loading.
func MergeDrawWithHold(live, hold State) State {
	if !live.Open {
		return live
	}
	if live.Phase != PhasePending && live.Phase != PhaseRunning {
		return live
	}
	if !hold.Holdable() {
		return live
	}
	out := live
	out.Source = hold.Source
	out.CombinedText = hold.CombinedText
	out.HighlightedCells = hold.HighlightedCells
	out.highlightCacheKey = hold.highlightCacheKey
	out.GutterWidth = hold.GutterWidth
	out.ErrorMsg = hold.ErrorMsg
	out.ExitCode = hold.ExitCode
	out.ImagePayload = hold.ImagePayload
	out.ImagePxW = hold.ImagePxW
	out.ImagePxH = hold.ImagePxH
	out.ImageProtocol = hold.ImageProtocol
	out.ImageUnicodePlaceholder = hold.ImageUnicodePlaceholder
	out.WrapCacheSnapshot(hold)
	out.BodyHeld = true
	return out
}
