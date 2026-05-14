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
	Scroll int
	// ExitCode is the subprocess exit code when Phase == FilePreviewPhaseDone and ErrorMsg == "".
	ExitCode int
	// ErrorMsg is set for launch failures or when PhaseDone with non-zero exit (optional message).
	ErrorMsg string
}
