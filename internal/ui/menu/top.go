package menu

// TopID identifies a top-level pulldown for routing; Label stays display-only.
type TopID string

const (
	TopPanelLeft  TopID = "top.panel-left"
	TopPanelRight TopID = "top.panel-right"
	TopFile       TopID = "top.file"
	TopCommand    TopID = "top.command"
	TopOptions    TopID = "top.options"
	TopJobs       TopID = "top.jobs"
	TopCommands   TopID = "top.commands-screen"
)

// PanelScope selects left/right panel for scoped pulldowns; PanelScopeNone means not panel-bound.
// Numeric values match ui.LeftPanel / ui.RightPanel (0 and 1).
const (
	PanelScopeNone  = -1
	PanelScopeLeft  = 0
	PanelScopeRight = 1
)
