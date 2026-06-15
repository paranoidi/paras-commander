package ui

import "strings"

// FilePreviewHoldable reports whether st has displayable body content worth keeping on screen
// while the next preview subprocess runs.
func FilePreviewHoldable(st FilePreviewState) bool {
	if !st.Open || st.Phase != FilePreviewPhaseDone {
		return false
	}
	return filePreviewDrawableBody(st)
}

func filePreviewDrawableBody(st FilePreviewState) bool {
	if strings.TrimSpace(st.CombinedText) != "" {
		return true
	}
	if strings.TrimSpace(st.ErrorMsg) != "" {
		return true
	}
	return st.ExitCode != 0 && strings.TrimSpace(st.Path) != ""
}

// FilePreviewDrawWarmCandidate is true when wrapped lines should be precomputed for drawing.
func FilePreviewDrawWarmCandidate(st FilePreviewState) bool {
	if !st.Open {
		return false
	}
	if st.BodyHeld {
		return filePreviewDrawableBody(st)
	}
	if st.Phase != FilePreviewPhaseDone || strings.TrimSpace(st.ErrorMsg) != "" {
		return false
	}
	if strings.TrimSpace(st.CombinedText) == "" && st.ExitCode == 0 {
		return false
	}
	return true
}

// MergeFilePreviewDrawWithHold builds a draw snapshot that keeps the previous preview body visible
// while live is loading a new path. When hold is empty or live is not loading, live is returned.
func MergeFilePreviewDrawWithHold(live, hold FilePreviewState) FilePreviewState {
	if !live.Open {
		return live
	}
	if live.Phase != FilePreviewPhasePending && live.Phase != FilePreviewPhaseRunning {
		return live
	}
	if !FilePreviewHoldable(hold) {
		return live
	}
	out := live
	out.CombinedText = hold.CombinedText
	out.ErrorMsg = hold.ErrorMsg
	out.ExitCode = hold.ExitCode
	out.WrapCacheSnapshot(hold)
	out.BodyHeld = true
	return out
}
