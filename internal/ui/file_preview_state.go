package ui

// FilePreviewPhase is the async preview subprocess lifecycle.
type FilePreviewPhase int

const (
	FilePreviewPhaseIdle FilePreviewPhase = iota
	FilePreviewPhasePending
	FilePreviewPhaseRunning
	FilePreviewPhaseDone
)

// FilePreviewState holds inactive-panel file preview (drawn from FilePreviewDraw snapshot).
type FilePreviewState struct {
	Open bool
	// Phase is used while Open; Draw uses latest snapshot each frame.
	Phase FilePreviewPhase
	// Path is the absolute file path being previewed.
	Path string
	// TitleBase is shown in the panel title (usually filepath.Base(Path)).
	TitleBase string
	// CombinedText is stdout plus optional stderr section (UTF-8), may contain ANSI escapes.
	CombinedText string
	Scroll       int
	// ExitCode is the subprocess exit code when Phase == FilePreviewPhaseDone and ErrorMsg == "".
	ExitCode int
	// ErrorMsg is set for launch failures or when PhaseDone with non-zero exit (optional message).
	ErrorMsg string

	// Wrapped-line cache (invalid when CombinedText, wrapWidth, or wrapStyleKey mismatch).
	wrappedLines     [][]AnsiCell
	wrapWidth        int
	wrapStyleKey     uint64
	wrapCombinedText string
}

// WrapCacheSnapshot copies the wrapped-line cache from src when it matches this state's CombinedText.
func (st *FilePreviewState) WrapCacheSnapshot(src FilePreviewState) {
	if src.wrapCombinedText != st.CombinedText {
		return
	}
	st.wrappedLines = src.wrappedLines
	st.wrapWidth = src.wrapWidth
	st.wrapStyleKey = src.wrapStyleKey
	st.wrapCombinedText = src.wrapCombinedText
}

// CachedWrappedLineCount returns len(wrappedLines) when the layout cache matches textWidth.
func (st *FilePreviewState) CachedWrappedLineCount(textWidth int) (count int, ok bool) {
	if st.wrappedLines != nil && st.wrapWidth == textWidth && st.wrapCombinedText == st.CombinedText {
		return len(st.wrappedLines), true
	}
	return 0, false
}
