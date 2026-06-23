package menu

// TopID identifies a top-level pulldown for routing; Label stays display-only.
type TopID string

const (
	TopPanelLeft  TopID = "top.panel-left"
	TopPanelRight TopID = "top.panel-right"
	TopFile       TopID = "top.file"
	TopCommand    TopID = "top.command"
	TopDisplay    TopID = "top.display"
	TopOptions    TopID = "top.options"
	TopJobs       TopID = "top.jobs"
	TopCommands   TopID = "top.commands-screen"
	TopMessages   TopID = "top.messages-screen"
	TopDev        TopID = "top.dev"
)

// PanelScope selects primary/secondary panel for scoped pulldowns; PanelScopeNone means not panel-bound.
// Numeric values match ui.PrimaryPanel / ui.SecondaryPanel (0 and 1).
const (
	PanelScopeNone      = -1
	PanelScopePrimary   = 0
	PanelScopeSecondary = 1
)
