package theme

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/themes"
)

const defaultName = "default"

// QuickAction border style values for dialog.quickaction.border.
const (
	QuickActionBorderRounded = "rounded"
	QuickActionBorderSharp   = "sharp"
)

// Registry maps stable theme names to parsed render styles.
type Registry map[string]Theme

// NamedTheme describes a theme choice suitable for UI display.
type NamedTheme struct {
	Name  string
	Label string
	Theme Theme
}

// Theme contains semantic styles used by the UI renderer.
type Theme struct {
	Name string

	MenuBarActive        tcell.Style
	MenuBarInactive      tcell.Style
	MenuBarSelected      tcell.Style
	MenuDropdown         tcell.Style
	MenuDropdownSelected tcell.Style
	MenuDropdownFrame    tcell.Style
	MenuBarAccent        tcell.Style
	MenuBarAlert         tcell.Style // jobs need input (before permission tail)
	MenuDropdownAccent   tcell.Style
	MenuDetail           tcell.Style

	PanelActiveFrame   tcell.Style
	PanelInactiveFrame tcell.Style
	// PanelTargetFrame overrides the border for a panel whose current directory is the
	// resolved Copy/Move/Flatten destination (see App.updateDestinationTargetPanels).
	PanelTargetFrame     tcell.Style
	PanelActiveSurface   tcell.Style
	PanelInactiveSurface tcell.Style
	PanelActiveTitle     tcell.Style
	PanelInactiveTitle   tcell.Style
	// PanelActiveDiskUsageOverview / PanelInactiveDiskUsageOverview style the panel top-row volume summary (free / total + percent).
	PanelActiveDiskUsageOverview   tcell.Style
	PanelInactiveDiskUsageOverview tcell.Style
	PanelActiveHeader              tcell.Style
	PanelInactiveHeader            tcell.Style
	// Panel*HeaderCarousel styles parent/child column headers in carousel view.
	PanelActiveHeaderCarousel   tcell.Style
	PanelInactiveHeaderCarousel tcell.Style
	PanelRowFile                tcell.Style
	PanelRowDirectory           tcell.Style
	PanelRowSymlink             tcell.Style
	PanelRowSelected            tcell.Style
	// PanelRowInfo styles trailing listing columns (size, mtime, permissions, meta).
	PanelRowInfo tcell.Style
	// PanelDedupRowAllMarked styles dedup view rows when every file in the duplicate group is marked.
	PanelDedupRowAllMarked tcell.Style
	// PanelDedupRowCursorAllMarked styles the cursor row in a fully-marked duplicate group.
	PanelDedupRowCursorAllMarked tcell.Style
	// PanelDedupRowKeep styles dedup view file rows designated as survivors.
	PanelDedupRowKeep tcell.Style
	// PanelDedupRowCursorKeep styles the cursor row on a kept duplicate file.
	PanelDedupRowCursorKeep tcell.Style
	// PanelDedupRowMarkKeepSubtree styles the dir-row suffix when a subtree contains kept files.
	PanelDedupRowMarkKeepSubtree tcell.Style
	// PanelRowMarkSelectionSubtree styles the file-list suffix on directories with nested selections.
	PanelRowMarkSelectionSubtree tcell.Style
	// PanelRowMarkNew styles the file-list suffix for the latest transferred batch.
	PanelRowMarkNew tcell.Style
	// PanelRowMarkNewPrevious styles the file-list suffix for earlier new-file marks in the same directory.
	PanelRowMarkNewPrevious tcell.Style
	// PanelRowMarkRenamed styles the file-list suffix for recently renamed entries.
	PanelRowMarkRenamed tcell.Style
	// PanelRowMarkNoPermission styles the file-list suffix for entries the current user cannot access.
	PanelRowMarkNoPermission tcell.Style
	// PanelRowMarkJob styles the file-list job mark when the row is under a job's
	// write (destination) tree; PanelRowMarkJobRead styles the read (source) tree;
	// PanelRowMarkJobDecision styles either while the matched job waits on a user
	// decision. See Theme.PanelJobMarkStyle.
	PanelRowMarkJob         tcell.Style
	PanelRowMarkJobRead     tcell.Style
	PanelRowMarkJobDecision tcell.Style
	// PanelRowTreeConnector styles tree branch/continuation glyphs in tree-style lists.
	PanelRowTreeConnector tcell.Style
	// PanelIconFolderOpen styles the open-folder icon strip when the other panel is in that directory.
	PanelIconFolderOpen tcell.Style
	// PanelIconFolderMount styles the other-mount directory icon strip.
	PanelIconFolderMount tcell.Style
	// PanelText styles non-listing body copy on panel interiors (stdout, jobs detail, preview, etc.).
	PanelText tcell.Style
	// PanelHint highlights rows related to the cursor row (e.g. duplicate siblings in the dedupe treeview).
	PanelHint                   tcell.Style
	PanelCursorActive           tcell.Style
	PanelCursorActiveUnique     tcell.Style
	PanelCursorInactive         tcell.Style
	PanelActiveCursorSelected   tcell.Style
	PanelInactiveCursorSelected tcell.Style
	// PanelCarouselInactiveCursor* styles preview-pane cursors in carousel mode.
	PanelCarouselInactiveCursor         tcell.Style
	PanelCarouselInactiveCursorSelected tcell.Style
	// PanelStatusSync styles sync overlays on the border toward the inactive pane
	// ("Sync →" / "← Sync" side-by-side; "Sync ↓" / "↑ Sync" stacked).
	PanelStatusSync tcell.Style
	// PanelStatusQuickView styles quick-view overlays on the driver pane border toward the inactive pane.
	PanelStatusQuickView tcell.Style
	// PanelStatus* style Start/PhysicalLeft bottom-border segments (see ui panel_bottom_indicators).
	PanelStatusSelections     tcell.Style
	PanelStatusDotfilesHidden tcell.Style
	PanelStatusGitignore      tcell.Style
	PanelStatusStash          tcell.Style
	PanelStatusSelectionSize  tcell.Style
	PanelStatusOtherPanel     tcell.Style
	PanelScrollbarTrack       tcell.Style
	PanelScrollbarThumb       tcell.Style
	// PanelFileIconFG maps cursor-row style keys (e.g. panel.active.row.cursor) to file-devicon FG
	// when panel file icons are enabled. Absent keys use devicons' suggested hex.
	PanelFileIconFG map[string]tcell.Color

	// PanelBlocked* is used for both file panels when a modal or pulldown menu
	// has focus so the file browser reads visually behind the overlay.
	PanelBlockedFrame             tcell.Style
	PanelBlockedSurface           tcell.Style
	PanelBlockedTitle             tcell.Style
	PanelBlockedDiskUsageOverview tcell.Style
	PanelBlockedHeader            tcell.Style
	PanelBlockedHeaderCarousel    tcell.Style
	PanelBlockedRowFile           tcell.Style
	PanelBlockedRowDirectory      tcell.Style
	PanelBlockedRowSymlink        tcell.Style
	PanelBlockedRowSelected       tcell.Style
	PanelBlockedText              tcell.Style
	PanelBlockedCursor            tcell.Style
	PanelBlockedCursorSelected    tcell.Style

	// PanelIconFolderScanning styles disk-scan-pending directory icon strip.
	PanelIconFolderScanning tcell.Style
	// PanelIconFolderExcluded styles directory rows skipped by disk-usage traversal (devicons).
	PanelIconFolderExcluded tcell.Style
	// MenuSpinner styles the menu-bar activity spinner (braille dot spinner).
	MenuSpinner tcell.Style
	// MenuProgress* styles segmented progress in the menu-bar jobs strip.
	MenuProgressDone      tcell.Style
	MenuProgressRemaining tcell.Style
	// MenuJob* styles one-cell queue glyph per live job status in the menu bar.
	MenuJobScanning                  tcell.Style
	MenuJobQueued                    tcell.Style
	MenuJobRunning                   tcell.Style
	MenuJobPaused                    tcell.Style
	MenuJobCanceled                  tcell.Style
	MenuJobFailed                    tcell.Style
	MenuJobDecision                  tcell.Style
	MenuJobCompleted                 tcell.Style
	PanelUsageNormal                 tcell.Style
	PanelUsageSelected               tcell.Style
	PanelUsageCursorActive           tcell.Style
	PanelUsageCursorInactive         tcell.Style
	PanelActiveUsageCursorSelected   tcell.Style
	PanelInactiveUsageCursorSelected tcell.Style

	PanelGitNotModified tcell.Style
	PanelGitNew         tcell.Style
	PanelGitModified    tcell.Style
	PanelGitDeleted     tcell.Style
	PanelGitRenamed     tcell.Style
	PanelGitTypechange  tcell.Style
	PanelGitIgnored     tcell.Style
	PanelGitConflicted  tcell.Style

	FuzzyInput           tcell.Style
	FuzzyInputNomatch    tcell.Style
	FuzzyHighlight       tcell.Style
	FuzzyHighlightCursor tcell.Style

	DialogFrame                    tcell.Style
	DialogTitle                    tcell.Style
	DialogText                     tcell.Style
	DialogSurface                  tcell.Style
	DialogAccent                   tcell.Style
	DialogInputActive              tcell.Style
	DialogInputActivePlaceholder   tcell.Style
	DialogInputActiveError         tcell.Style
	DialogInputInactive            tcell.Style
	DialogInputInactivePlaceholder tcell.Style
	DialogInputInactiveError       tcell.Style
	DialogButtonInactive           tcell.Style
	DialogButtonActive             tcell.Style
	DialogButtonActiveDestructive  tcell.Style
	DialogOptionInactive           tcell.Style
	DialogOptionActive             tcell.Style
	DialogOptionActiveSelected     tcell.Style
	DialogOptionSelected           tcell.Style
	DialogOptionInvalid            tcell.Style
	DialogStatusSelectionSize      tcell.Style
	DialogProgressTrack            tcell.Style
	DialogProgressFill             tcell.Style
	DialogProgressLabelOnFill      tcell.Style
	DialogProgressLabelOnTrack     tcell.Style
	DialogMassRenameBefore         tcell.Style
	DialogMassRenameBeforeRemoved  tcell.Style
	DialogMassRenameBeforeReplaced tcell.Style
	DialogMassRenameAfter          tcell.Style
	DialogMassRenameAfterAdded     tcell.Style
	DialogMassRenameAfterError     tcell.Style

	// DialogQuickAction* is the minimal, separate theme subset for the F2 quick-action
	// list (buttonless letter-activated menu) — it does not reuse the generic Dialog*
	// fields above. DialogQuickActionBorderStyle selects the frame glyph set (see
	// QuickActionBorderGlyphs) and is parsed from dialog.quickaction.border, not a
	// tcell.Style like the rest of this block.
	DialogQuickActionFrame        tcell.Style
	DialogQuickActionSurface      tcell.Style
	DialogQuickActionText         tcell.Style
	DialogQuickActionTitle        tcell.Style
	DialogQuickActionAccent       tcell.Style
	DialogQuickActionListSelected tcell.Style
	DialogQuickActionBorderStyle  string

	MessageInfo  tcell.Style
	MessageWarn  tcell.Style
	MessageError tcell.Style

	JobsRow     tcell.Style
	JobsRunning tcell.Style
	JobsDone    tcell.Style
	JobsFailed  tcell.Style
	// Jobs list progress column (horizontal bar with centered percentage).
	JobsProgressTrack        tcell.Style
	JobsProgressFill         tcell.Style
	JobsProgressLabelOnFill  tcell.Style
	JobsProgressLabelOnTrack tcell.Style

	// JobsIcon* styles for the leading icon column in the jobs list (keys match job status strings).
	JobsIconsScanning  tcell.Style
	JobsIconsQueued    tcell.Style
	JobsIconsRunning   tcell.Style
	JobsIconsPaused    tcell.Style
	JobsIconsCanceled  tcell.Style
	JobsIconsFailed    tcell.Style
	JobsIconsDecision  tcell.Style
	JobsIconsCompleted tcell.Style

	// Symbols holds global glyphs (e.g. Nerd Font job status icons) referenced by the UI
	// from the [symbols] section of the theme file.
	Symbols map[string]string

	FooterKey        tcell.Style
	FooterLabel      tcell.Style
	FooterLabelShift tcell.Style

	// TerminalText styles the embedded terminal panel strip ([terminal] in TOML). Optional
	// (zero value falls back — see TerminalTextStyle) so imported/user themes without a
	// [terminal] table keep rendering correctly.
	TerminalText tcell.Style
}

// TerminalTextStyle returns the terminal panel body text style, falling back to the panel
// default text style when the theme omits terminal.text.
func (t Theme) TerminalTextStyle() tcell.Style {
	if t.TerminalText != (tcell.Style{}) {
		return t.TerminalText
	}
	return t.PanelText
}

// QuickActionBorderGlyphs returns the border glyph set for the F2 quick-action dialog,
// selected by DialogQuickActionBorderStyle ("rounded", the default, or "sharp").
func (t Theme) QuickActionBorderGlyphs() primitive.BorderGlyphs {
	if t.DialogQuickActionBorderStyle == QuickActionBorderSharp {
		return primitive.SharpBorder
	}
	return primitive.RoundedBorder
}

// PanelRowIconForeground returns the foreground for cursor-row adornment icons: file-list
// suffix glyphs, folder icon strip, and file devicons when the matching panel.*.row.cursor
// style defines icon; otherwise the base indicator style foreground is used.
func (t Theme) PanelRowIconForeground(cursorStyleKey string, base tcell.Style) tcell.Color {
	if cursorStyleKey != "" && t.PanelFileIconFG != nil {
		if c, ok := t.PanelFileIconFG[cursorStyleKey]; ok {
			return c
		}
	}
	fg, _, _ := base.Decompose()
	return fg
}

// DialogInputPair returns base (row fill + committed text) and placeholder glyph styles.
func (t Theme) DialogInputPair(focused bool) (base, placeholder tcell.Style) {
	if focused {
		return t.DialogInputActive, t.DialogInputActivePlaceholder
	}
	return t.DialogInputInactive, t.DialogInputInactivePlaceholder
}

// DialogInputBaseStyle returns the row fill + committed text style for a simple one-line input
// (no placeholder split). When invalid is true, uses dialog.input.*.error styles.
func (t Theme) DialogInputBaseStyle(focused, invalid bool) tcell.Style {
	if invalid {
		if focused {
			return t.DialogInputActiveError
		}
		return t.DialogInputInactiveError
	}
	if focused {
		return t.DialogInputActive
	}
	return t.DialogInputInactive
}

// DialogOptionRowStyle returns the resolved style for a checkbox/radio/list option row.
// Foreground and attributes come from dialog.option.inactive, .active, .selected,
// or .active.selected; background always matches dialog.surface.
func (t Theme) DialogOptionRowStyle(focused, selected bool) tcell.Style {
	switch {
	case focused && selected:
		return mergeForegroundOnSurface(t.DialogOptionActiveSelected, t.DialogSurface)
	case focused:
		return mergeForegroundOnSurface(t.DialogOptionActive, t.DialogSurface)
	case selected:
		return mergeForegroundOnSurface(t.DialogOptionSelected, t.DialogSurface)
	default:
		return mergeForegroundOnSurface(t.DialogOptionInactive, t.DialogSurface)
	}
}

// DialogOptionInvalidStyle returns the resolved style for invalid/missing option rows.
func (t Theme) DialogOptionInvalidStyle() tcell.Style {
	return mergeForegroundOnSurface(t.DialogOptionInvalid, t.DialogSurface)
}

// DialogStatusSelectionSizeStyle returns the find-dialog selection count/size label on a horizontal separator.
// Foreground comes from dialog.status.selection_size; background always matches dialog.surface.
func (t Theme) DialogStatusSelectionSizeStyle() tcell.Style {
	src := t.DialogStatusSelectionSize
	if src == (tcell.Style{}) {
		src = t.PanelStatusSelectionSize
	}
	return mergeForegroundOnSurface(src, t.DialogSurface)
}

// DialogProgressLabelOnBar returns label foreground/attributes merged onto the fill or track bar cell.
// Background always comes from dialog.progress.fill or dialog.progress.track (label TOML must not set bg).
func (t Theme) DialogProgressLabelOnBar(onFill bool) tcell.Style {
	if onFill {
		return mergeForegroundOnBar(t.DialogProgressLabelOnFill, t.DialogProgressFill)
	}
	return mergeForegroundOnBar(t.DialogProgressLabelOnTrack, t.DialogProgressTrack)
}

func mergeForegroundOnBar(labelSrc, bar tcell.Style) tcell.Style {
	fg, _, attrs := labelSrc.Decompose()
	out := bar.Foreground(fg)
	if attrs&tcell.AttrBold != 0 {
		out = out.Bold(true)
	}
	if attrs&tcell.AttrUnderline != 0 {
		out = out.Underline(true)
	}
	if attrs&tcell.AttrReverse != 0 {
		out = out.Reverse(true)
	}
	return out
}

func mergeForegroundOnSurface(src, surface tcell.Style) tcell.Style {
	fg, _, attrs := src.Decompose()
	_, bg, _ := surface.Decompose()
	s := tcell.StyleDefault.Foreground(fg).Background(bg)
	if attrs&tcell.AttrBold != 0 {
		s = s.Bold(true)
	}
	if attrs&tcell.AttrUnderline != 0 {
		s = s.Underline(true)
	}
	if attrs&tcell.AttrReverse != 0 {
		s = s.Reverse(true)
	}
	return s
}

// Panel bottom-indicator style keys ([panel.status] in TOML).
const (
	PanelBottomIndicatorKeySelections     = "selections"
	PanelBottomIndicatorKeyDotfilesHidden = "dotfiles_hidden"
	PanelBottomIndicatorKeyGitignore      = "gitignore"
	PanelBottomIndicatorKeyStash          = "stash"
	PanelBottomIndicatorKeySelectionSize  = "selection_size"
	PanelBottomIndicatorKeySync           = "sync"
	PanelBottomIndicatorKeyQuickView      = "quick_view"
	PanelBottomIndicatorKeyOtherPanel     = "other_panel"
)

// PanelBottomIndicator returns the style for a file-panel bottom-border segment.
// id is one of PanelBottomIndicatorKey* constants. When the theme omits a dedicated key,
// selections falls back to panel title styles; dotfiles_hidden and gitignore fall back to panel frame styles.
func (t Theme) PanelBottomIndicator(id string, fileListActive, chromeBlocked bool) tcell.Style {
	switch id {
	case PanelBottomIndicatorKeySelections:
		if t.PanelStatusSelections != (tcell.Style{}) {
			return t.PanelStatusSelections
		}
		if chromeBlocked {
			return t.PanelBlockedTitle
		}
		if fileListActive {
			return t.PanelActiveTitle
		}
		return t.PanelInactiveTitle
	case PanelBottomIndicatorKeyDotfilesHidden:
		if t.PanelStatusDotfilesHidden != (tcell.Style{}) {
			return t.PanelStatusDotfilesHidden
		}
		fallthrough
	case PanelBottomIndicatorKeyGitignore:
		if t.PanelStatusGitignore != (tcell.Style{}) {
			return t.PanelStatusGitignore
		}
		if chromeBlocked {
			return t.PanelBlockedFrame
		}
		if fileListActive {
			return t.PanelActiveFrame
		}
		return t.PanelInactiveFrame
	case PanelBottomIndicatorKeyStash:
		if t.PanelStatusStash != (tcell.Style{}) {
			return t.PanelStatusStash
		}
		if chromeBlocked {
			return t.PanelBlockedTitle
		}
		if fileListActive {
			return t.PanelActiveTitle
		}
		return t.PanelInactiveTitle
	case PanelBottomIndicatorKeySelectionSize:
		if t.PanelStatusSelectionSize != (tcell.Style{}) {
			return t.PanelStatusSelectionSize
		}
		fallthrough
	case PanelBottomIndicatorKeySync:
		if t.PanelStatusSync != (tcell.Style{}) {
			return t.PanelStatusSync
		}
		fallthrough
	case PanelBottomIndicatorKeyQuickView:
		if t.PanelStatusQuickView != (tcell.Style{}) {
			return t.PanelStatusQuickView
		}
		fallthrough
	case PanelBottomIndicatorKeyOtherPanel:
		if t.PanelStatusOtherPanel != (tcell.Style{}) {
			return t.PanelStatusOtherPanel
		}
		fallthrough
	default:
		if chromeBlocked {
			return t.PanelBlockedFrame
		}
		if fileListActive {
			return t.PanelActiveFrame
		}
		return t.PanelInactiveFrame
	}
}

// Symbol keys in the [symbols] table (optional entries — see accessors for defaults).
const (
	SymbolKeyPathPicker               = "path_picker"
	SymbolKeyGit                      = "git"
	SymbolKeyStash                    = "stash"
	SymbolKeyWorking                  = "working"
	SymbolKeyHiddenDotfiles           = "hidden_dotfiles"
	SymbolKeyFilelistSelectionSubtree = "filelist.selection_subtree"
	SymbolKeyFilelistNew              = "filelist.new"
	SymbolKeyFilelistRenamed          = "filelist.renamed"
	SymbolKeyFilelistJob              = "filelist.job"
	SymbolKeyFilelistNoPermission     = "filelist.no_permission"
	SymbolKeyFoldersFolder            = "folders.folder"
	SymbolKeyFoldersOpen              = "folders.open"
	SymbolKeyFoldersScanning          = "folders.scanning"
	SymbolKeyFoldersMount             = "folders.mount"
	SymbolKeyFoldersExcluded          = "folders.excluded"
	SymbolKeyScrollbarThumb           = "scrollbar.thumb"
	SymbolKeyMetaRunning              = "meta.running"
	SymbolKeyTreeExpand               = "tree.expand"
	SymbolKeyTreeCollapse             = "tree.collapse"
	SymbolKeyTreeContinue             = "tree.continue"
	SymbolKeyTreeBranch               = "tree.branch"
	SymbolKeyTreeEnd                  = "tree.end"
	SymbolKeyTreeLeaf                 = "tree.leaf"
	SymbolKeySelectionsMultiLocation  = "selections.multi_location"
)

// Menu-bar jobs strip symbol keys ([symbols] table); optional — see SymbolMenuJob / SymbolMenuProgress*.
const (
	SymbolKeyMenuProgressDone      = "menu.progress.done"
	SymbolKeyMenuProgressRemaining = "menu.progress.remaining"
	SymbolKeyMenuJobScanning       = "menu.job.scanning"
	SymbolKeyMenuJobQueued         = "menu.job.queued"
	SymbolKeyMenuJobRunning        = "menu.job.running"
	SymbolKeyMenuJobPaused         = "menu.job.paused"
	SymbolKeyMenuJobCanceled       = "menu.job.canceled"
	SymbolKeyMenuJobFailed         = "menu.job.failed"
	SymbolKeyMenuJobDecision       = "menu.job.decision"
	SymbolKeyMenuJobCompleted      = "menu.job.completed"
)

// SymbolWorking returns the in-progress glyph appended to selection-size indicators.
func (t Theme) SymbolWorking() string {
	if t.Symbols != nil {
		if s := strings.TrimSpace(t.Symbols[SymbolKeyWorking]); s != "" {
			return s
		}
	}
	return "\uf017"
}

// SymbolHiddenDotfiles returns the dotfiles-hidden bottom-indicator glyph from [symbols] hidden_dotfiles.
func (t Theme) SymbolHiddenDotfiles() string {
	if t.Symbols != nil {
		if s := strings.TrimSpace(t.Symbols[SymbolKeyHiddenDotfiles]); s != "" {
			return s
		}
	}
	return "\U000F06D1" // nf-md-eye_off_outline (Material Design / Nerd Fonts PUA)
}

// SymbolMetaRunning returns the glyph shown in the meta column while a command is executing.
func (t Theme) SymbolMetaRunning() string {
	if t.Symbols != nil {
		if s := strings.TrimSpace(t.Symbols[SymbolKeyMetaRunning]); s != "" {
			return s
		}
	}
	return "\uf141"
}

// SymbolStash returns the selection-stash bottom-indicator glyph from [symbols] stash.
func (t Theme) SymbolStash() string {
	if t.Symbols != nil {
		if s := strings.TrimSpace(t.Symbols[SymbolKeyStash]); s != "" {
			return s
		}
	}
	return "\ue73d"
}

// SymbolGit returns the panel Git column header glyph from [symbols] git.
func (t Theme) SymbolGit() string {
	if t.Symbols != nil {
		if s := strings.TrimSpace(t.Symbols[SymbolKeyGit]); s != "" {
			return s
		}
	}
	return "\uf1d3" // Font Awesome git (Nerd Fonts)
}

// SymbolSelectionsMultiLocation returns the glyph prepended to the selections strip title
// when the panel's selections span multiple parent directories.
func (t Theme) SymbolSelectionsMultiLocation() string {
	if t.Symbols != nil {
		if s := strings.TrimSpace(t.Symbols[SymbolKeySelectionsMultiLocation]); s != "" {
			return s
		}
	}
	return "\uef81"
}

// SymbolFilelistSelectionSubtree returns the directory nested-selection suffix glyph.
func (t Theme) SymbolFilelistSelectionSubtree() rune {
	return t.filelistSymbolRune(SymbolKeyFilelistSelectionSubtree, '\u25cb') // ○
}

// SymbolFilelistNew returns the recently-transferred file suffix glyph.
func (t Theme) SymbolFilelistNew() rune {
	return t.filelistSymbolRune(SymbolKeyFilelistNew, '\uea7f')
}

// SymbolFilelistRenamed returns the recently-renamed file suffix glyph.
func (t Theme) SymbolFilelistRenamed() rune {
	return t.filelistSymbolRune(SymbolKeyFilelistRenamed, '\U000f11e8')
}

// SymbolFilelistJob returns the file-list job mark glyph (role/status colored via
// Theme.PanelJobMarkStyle; see internal/theme/jobs_style.go).
func (t Theme) SymbolFilelistJob() rune {
	return t.filelistSymbolRune(SymbolKeyFilelistJob, '\U000f02ca')
}

// SymbolFilelistNoPermission returns the suffix glyph for entries the current user cannot access.
func (t Theme) SymbolFilelistNoPermission() rune {
	return t.filelistSymbolRune(SymbolKeyFilelistNoPermission, '\uf023')
}

// SymbolTreeExpand returns the collapsed-node expander glyph for tree-style lists
// (duplicates view; the file-list tree layout later).
func (t Theme) SymbolTreeExpand() rune {
	return t.filelistSymbolRune(SymbolKeyTreeExpand, '▶') // ▶
}

// SymbolTreeCollapse returns the expanded-node expander glyph for tree-style lists.
func (t Theme) SymbolTreeCollapse() rune {
	return t.filelistSymbolRune(SymbolKeyTreeCollapse, '▼') // ▼
}

// SymbolTreeLeaf returns the leaf-row gutter glyph for tree-style lists.
func (t Theme) SymbolTreeLeaf() rune {
	return t.filelistSymbolRune(SymbolKeyTreeLeaf, '·') // ·
}

// SymbolTreeContinue returns the ancestor continuation glyph for tree-style lists.
func (t Theme) SymbolTreeContinue() string {
	return t.treeSymbol(SymbolKeyTreeContinue, "│")
}

// SymbolTreeBranch returns the non-last-sibling branch glyph for tree-style lists.
func (t Theme) SymbolTreeBranch() string {
	return t.treeSymbol(SymbolKeyTreeBranch, "├─")
}

// SymbolTreeEnd returns the last-sibling branch glyph for tree-style lists.
func (t Theme) SymbolTreeEnd() string {
	return t.treeSymbol(SymbolKeyTreeEnd, "└─")
}

// SymbolScrollbarThumb returns the panel scrollbar thumb-style position glyph.
func (t Theme) SymbolScrollbarThumb() rune {
	return t.filelistSymbolRune(SymbolKeyScrollbarThumb, '\u25cf') // ●
}

func (t Theme) filelistSymbolRune(key string, fallback rune) rune {
	if t.Symbols != nil {
		if s := strings.TrimSpace(t.Symbols[key]); s != "" {
			for _, r := range s {
				return r
			}
		}
	}
	return fallback
}

func (t Theme) foldersSymbol(key, fallback string) string {
	if t.Symbols != nil {
		if s := strings.TrimSpace(t.Symbols[key]); s != "" {
			return s
		}
	}
	return fallback
}

func (t Theme) treeSymbol(key, fallback string) string {
	if t.Symbols != nil {
		if s := strings.TrimSpace(t.Symbols[key]); s != "" {
			return s
		}
	}
	return fallback
}

// SymbolPathPicker returns the trailing path-picker glyph from the theme, with a default
// Nerd-Font private-use fallback when the key is absent.
func (t Theme) SymbolPathPicker() string {
	if t.Symbols != nil {
		if s := strings.TrimSpace(t.Symbols[SymbolKeyPathPicker]); s != "" {
			return s
		}
	}
	return "\uef0d"
}

// SymbolMenuProgressDone returns the filled segment glyph for the menu-bar progress bar.
func (t Theme) SymbolMenuProgressDone() rune {
	if g := t.menuBarSymbolTrim(SymbolKeyMenuProgressDone); g != 0 {
		return g
	}
	return '\u25cf' // ●
}

// SymbolMenuProgressRemaining returns the empty segment glyph for the menu-bar progress bar.
func (t Theme) SymbolMenuProgressRemaining() rune {
	if g := t.menuBarSymbolTrim(SymbolKeyMenuProgressRemaining); g != 0 {
		return g
	}
	return '\u25cb' // ○
}

func (t Theme) menuBarSymbolTrim(key string) rune {
	if t.Symbols == nil {
		return 0
	}
	s := strings.TrimSpace(t.Symbols[key])
	if s == "" {
		return 0
	}
	for _, r := range s {
		return r
	}
	return 0
}

// SymbolMenuJob returns the queue glyph for a job status string (e.g. "queued", "running").
// Uses [symbols] menu.job.<status> when set, otherwise compact Unicode (does not fall back to
// the jobs-list Nerd Font keys like symbols.running — those are too wide/noisy on the menu bar).
func (t Theme) SymbolMenuJob(status string) rune {
	key := "menu.job." + status
	if g := t.menuBarSymbolTrim(key); g != 0 {
		return g
	}
	switch status {
	case "scanning":
		return '\u25cc'
	case "queued":
		return '\u25cb'
	case "running":
		return '\u25cf'
	case "paused":
		return '\u25d8'
	case "canceled":
		return '\u00d7'
	case "failed":
		return '!'
	case "decision":
		return '?'
	case "completed":
		return '\u2713'
	default:
		return '\u00b7'
	}
}

// MenuJobStyle returns the style for one queue cell by job status string.
func (t Theme) MenuJobStyle(status string) tcell.Style {
	switch status {
	case "scanning":
		return t.MenuJobScanning
	case "queued":
		return t.MenuJobQueued
	case "running":
		return t.MenuJobRunning
	case "paused":
		return t.MenuJobPaused
	case "canceled":
		return t.MenuJobCanceled
	case "failed":
		return t.MenuJobFailed
	case "decision":
		return t.MenuJobDecision
	case "completed":
		return t.MenuJobCompleted
	default:
		return t.MenuDetail
	}
}

type styleSpec struct {
	FG        string
	BG        string
	Icon      string // optional; only certain panel cursor styles, see parse()
	Bold      bool
	Underline bool
	Reverse   bool
}

var requiredStyleKeys = []string{
	"menu.bar.active",
	"menu.bar.inactive",
	"menu.bar.selected",
	"menu.dropdown",
	"menu.dropdown.selected",
	"menu.dropdown.frame",
	"menu.bar.accent",
	"menu.bar.alert",
	"menu.dropdown.accent",
	"menu.detail",
	"panel.active.frame",
	"panel.inactive.frame",
	"panel.target.frame",
	"panel.active.surface",
	"panel.inactive.surface",
	"panel.active.title",
	"panel.inactive.title",
	"panel.active.disk_usage_overview",
	"panel.inactive.disk_usage_overview",
	"panel.active.header",
	"panel.active.header.carousel",
	"panel.inactive.header",
	"panel.inactive.header.carousel",
	"panel.active.row.cursor",
	"panel.active.row.cursor.selected",
	"panel.active.row.cursor.unique",
	"panel.active.usage.cursor",
	"panel.active.usage.cursor.selected",
	"panel.inactive.row.cursor",
	"panel.inactive.row.cursor.selected",
	"panel.carousel.inactive.row.cursor",
	"panel.carousel.inactive.row.cursor.selected",
	"panel.inactive.usage.cursor",
	"panel.inactive.usage.cursor.selected",
	"panel.row.file",
	"panel.row.directory",
	"panel.row.symlink",
	"panel.row.selected",
	"panel.row.info",
	"panel.dedup.row.all_marked",
	"panel.dedup.row.cursor.all_marked",
	"panel.dedup.row.keep",
	"panel.dedup.row.cursor.keep",
	"panel.dedup.row.mark.keep_subtree",
	"panel.row.mark.selection_subtree",
	"panel.row.mark.new",
	"panel.row.mark.new.previous",
	"panel.row.mark.renamed",
	"panel.row.mark.no_permission",
	"panel.row.mark.job",
	"panel.row.mark.job.read",
	"panel.row.mark.job.decision",
	"panel.row.tree.connector",
	"panel.icon.folder.open",
	"panel.icon.folder.mount",
	"panel.text",
	"panel.hint",
	"panel.status.sync",
	"panel.status.quick_view",
	"panel.status.selections",
	"panel.status.dotfiles_hidden",
	"panel.status.gitignore",
	"panel.status.stash",
	"panel.status.selection_size",
	"panel.status.other_panel",
	"panel.scrollbar.track",
	"panel.scrollbar.thumb",
	"panel.blocked.frame",
	"panel.blocked.surface",
	"panel.blocked.title",
	"panel.blocked.disk_usage_overview",
	"panel.blocked.header",
	"panel.blocked.header.carousel",
	"panel.blocked.row.file",
	"panel.blocked.row.directory",
	"panel.blocked.row.symlink",
	"panel.blocked.row.selected",
	"panel.blocked.row.cursor",
	"panel.blocked.row.cursor.selected",
	"panel.blocked.text",
	"panel.icon.folder.scanning",
	"panel.icon.folder.excluded",
	"menu.spinner",
	"panel.usage.normal",
	"panel.usage.selected",
	"panel.git.not_modified",
	"panel.git.new",
	"panel.git.modified",
	"panel.git.deleted",
	"panel.git.renamed",
	"panel.git.typechange",
	"panel.git.ignored",
	"panel.git.conflicted",
	"fuzzy.input",
	"fuzzy.input.nomatch",
	"fuzzy.highlight",
	"fuzzy.highlight.cursor",
	"dialog.frame",
	"dialog.title",
	"dialog.text",
	"dialog.surface",
	"dialog.accent",
	"dialog.input.active",
	"dialog.input.active.placeholder",
	"dialog.input.active.error",
	"dialog.input.inactive",
	"dialog.input.inactive.placeholder",
	"dialog.input.inactive.error",
	"dialog.button.inactive",
	"dialog.button.active",
	"dialog.button.active_destructive",
	"dialog.option.inactive",
	"dialog.option.active",
	"dialog.option.active.selected",
	"dialog.option.selected",
	"dialog.option.invalid",
	"dialog.status.selection_size",
	"dialog.progress.track",
	"dialog.progress.fill",
	"dialog.progress.label.on_fill",
	"dialog.progress.label.on_track",
	"dialog.massrename.before",
	"dialog.massrename.before.removed",
	"dialog.massrename.before.replaced",
	"dialog.massrename.after",
	"dialog.massrename.after.added",
	"dialog.massrename.after.error",
	"dialog.quickaction.frame",
	"dialog.quickaction.surface",
	"dialog.quickaction.text",
	"dialog.quickaction.title",
	"dialog.quickaction.accent",
	"dialog.quickaction.list.selected",
	"message.info",
	"message.warn",
	"message.error",
	"jobs.row",
	"jobs.running",
	"jobs.done",
	"jobs.failed",
	"jobs.progress.track",
	"jobs.progress.fill",
	"jobs.progress.label.on_fill",
	"jobs.progress.label.on_track",
	"jobs.icons.scanning",
	"jobs.icons.queued",
	"jobs.icons.running",
	"jobs.icons.paused",
	"jobs.icons.canceled",
	"jobs.icons.failed",
	"jobs.icons.decision",
	"jobs.icons.completed",
	"menu.progress.done",
	"menu.progress.remaining",
	"menu.job.scanning",
	"menu.job.queued",
	"menu.job.running",
	"menu.job.paused",
	"menu.job.canceled",
	"menu.job.failed",
	"menu.job.decision",
	"menu.job.completed",
	"footer.key",
	"footer.label",
	"footer.label.shift",
	"terminal.text",
}

var requiredStyleKeySet = makeStyleKeySet(requiredStyleKeys)

// styleSectionRoots are top-level TOML tables for semantic styles (keys inside omit this prefix).
var styleSectionRoots = []string{"menu", "panel", "dialog", "jobs", "message", "footer", "fuzzy", "terminal"}

var styleSectionRootSet = makeStyleKeySet(styleSectionRoots)

var builtInThemeOrder = []string{
	defaultName,
}

var builtInThemeLabels = map[string]string{
	defaultName: "Default",
}

// Default returns the embedded default theme. The embedded asset is expected to
// be valid; a panic here means the binary was built with a broken built-in theme.
func Default() Theme {
	value, err := LoadBuiltIn(defaultName)
	if err != nil {
		panic(err)
	}
	return value
}

// LoadBuiltIn loads one embedded built-in theme by stable name.
func LoadBuiltIn(name string) (Theme, error) {
	if strings.TrimSpace(name) == "" {
		return Theme{}, fmt.Errorf("theme name is required")
	}
	data, err := themes.Files.ReadFile(name + ".toml")
	if err != nil {
		return Theme{}, fmt.Errorf("load built-in theme %q: %w", name, err)
	}
	value, err := parse(data)
	if err != nil {
		return Theme{}, fmt.Errorf("load built-in theme %q: %w", name, err)
	}
	if value.Name != name {
		return Theme{}, fmt.Errorf("load built-in theme %q: file declares name %q", name, value.Name)
	}
	return value, nil
}

// BuiltInThemes returns built-in themes in menu-friendly order.
func BuiltInThemes() ([]NamedTheme, error) {
	themes := make([]NamedTheme, 0, len(builtInThemeOrder))
	for _, name := range builtInThemeOrder {
		value, err := LoadBuiltIn(name)
		if err != nil {
			return nil, err
		}
		themes = append(themes, NamedTheme{
			Name:  name,
			Label: themeLabel(name),
			Theme: value,
		})
	}
	return themes, nil
}

// LoadFile parses a TOML theme file from disk.
func LoadFile(path string) (Theme, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Theme{}, fmt.Errorf("load theme %q: %w", path, err)
	}
	value, err := parse(data)
	if err != nil {
		return Theme{}, fmt.Errorf("load theme %q: %w", path, err)
	}
	return value, nil
}

// LoadDir loads all TOML themes from a directory.
func LoadDir(dir string) (Registry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return Registry{}, nil
		}
		return nil, fmt.Errorf("load themes dir %q: %w", dir, err)
	}

	registry := Registry{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".toml" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		value, err := LoadFile(path)
		if err != nil {
			return nil, err
		}
		if _, exists := registry[value.Name]; exists {
			return nil, fmt.Errorf("load themes dir %q: duplicate theme name %q", dir, value.Name)
		}
		registry[value.Name] = value
	}
	return registry, nil
}

// ThemeChoices returns selectable themes: each stable built-in name appears once
// (loaded from userDir when a .toml declares that name, otherwise embedded),
// then any additional themes found only on disk, sorted by name.
func ThemeChoices(userDir string) ([]NamedTheme, error) {
	disk := Registry{}
	if userDir != "" {
		var err error
		disk, err = LoadDir(userDir)
		if err != nil {
			return nil, err
		}
	}

	usedFromDisk := make(map[string]bool, len(disk))
	choices := make([]NamedTheme, 0, len(builtInThemeOrder)+len(disk))

	for _, name := range builtInThemeOrder {
		if t, ok := disk[name]; ok {
			choices = append(choices, NamedTheme{Name: name, Label: themeLabel(name), Theme: t})
			usedFromDisk[name] = true
			continue
		}
		t, err := LoadBuiltIn(name)
		if err != nil {
			return nil, err
		}
		choices = append(choices, NamedTheme{Name: name, Label: themeLabel(name), Theme: t})
	}

	extra := make([]string, 0, len(disk))
	for name := range disk {
		if !usedFromDisk[name] {
			extra = append(extra, name)
		}
	}
	sort.Strings(extra)
	for _, name := range extra {
		choices = append(choices, NamedTheme{Name: name, Label: themeLabel(name), Theme: disk[name]})
	}
	return choices, nil
}

// lookupUserTheme finds the first parsable theme file in dir (sorted by filename) whose declared
// name matches wantName. Invalid or unreadable .toml siblings are skipped so one broken file does
// not block resolving another theme from the same directory.
func lookupUserTheme(dir, wantName string) (Theme, bool, error) {
	if strings.TrimSpace(dir) == "" {
		return Theme{}, false, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return Theme{}, false, nil
		}
		return Theme{}, false, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".toml" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, base := range names {
		path := filepath.Join(dir, base)
		value, err := LoadFile(path)
		if err != nil {
			continue
		}
		if value.Name == wantName {
			return value, true, nil
		}
	}
	return Theme{}, false, nil
}

// Resolve returns a named theme. When userDir is non-empty, the first successfully parsed
// theme file in that directory whose declared name matches takes precedence over the embedded
// theme with the same stable name; unreadable or invalid sibling `.toml` files are skipped.
// Building the full theme list (ThemeChoices / LoadDir) still requires every file in the
// directory to parse — fix or remove broken files there so the dialog can open.
// On resolution failure it returns the built-in default alongside the error so startup can fall back
// while still surfacing the problem.
func Resolve(name, userDir string) (Theme, error) {
	if strings.TrimSpace(name) == "" {
		name = defaultName
	}

	if strings.TrimSpace(userDir) != "" {
		value, ok, err := lookupUserTheme(userDir, name)
		if err != nil {
			return defaultWithError(err)
		}
		if ok {
			return value, nil
		}
	}

	value, err := LoadBuiltIn(name)
	if err == nil {
		return value, nil
	}
	return defaultWithError(fmt.Errorf("theme %q is not available", name))
}

func defaultWithError(err error) (Theme, error) {
	value, defaultErr := LoadBuiltIn(defaultName)
	if defaultErr != nil {
		return Theme{}, fmt.Errorf("%w; additionally failed to load default theme: %v", err, defaultErr)
	}
	return value, err
}

func parse(data []byte) (Theme, error) {
	var raw map[string]any
	if _, err := toml.Decode(string(data), &raw); err != nil {
		return Theme{}, err
	}
	if _, ok := raw["styles"]; ok {
		return Theme{}, fmt.Errorf("[styles] is not supported; use [menu], [panel], [dialog], [jobs], [message], [footer], and [fuzzy] sections")
	}
	for key := range raw {
		if key == "name" || key == "palette" || key == "symbols" {
			continue
		}
		if styleSectionRootSet[key] {
			continue
		}
		return Theme{}, fmt.Errorf("unknown field %q", key)
	}

	name, err := stringField(raw, "name")
	if err != nil {
		return Theme{}, err
	}
	if strings.TrimSpace(name) == "" {
		return Theme{}, fmt.Errorf("name is required")
	}

	palette, err := paletteField(raw)
	if err != nil {
		return Theme{}, err
	}

	quickActionBorderStyle, err := quickActionBorderStyleField(raw)
	if err != nil {
		return Theme{}, err
	}

	specs, err := collectStyleSpecs(raw)
	if err != nil {
		return Theme{}, err
	}

	dialogSurfaceForegroundKeys := map[string]struct{}{
		"dialog.option.inactive":         {},
		"dialog.option.active":           {},
		"dialog.option.active.selected":  {},
		"dialog.option.selected":         {},
		"dialog.option.invalid":          {},
		"dialog.status.selection_size":   {},
		"dialog.progress.label.on_fill":  {},
		"dialog.progress.label.on_track": {},
	}
	for key, spec := range specs {
		if spec.BG != "" {
			if _, ok := dialogSurfaceForegroundKeys[key]; ok {
				return Theme{}, fmt.Errorf(`style %q: field "bg" is not allowed (background is merged at render time)`, key)
			}
		}
	}

	symbols, err := symbolsField(raw)
	if err != nil {
		return Theme{}, err
	}

	styles := map[string]tcell.Style{}
	for _, key := range requiredStyleKeys {
		spec, ok := specs[key]
		if !ok {
			return Theme{}, fmt.Errorf("missing required style %q", key)
		}
		style, err := buildStyle(spec, palette)
		if err != nil {
			return Theme{}, fmt.Errorf("style %q: %w", key, err)
		}
		styles[key] = style
	}

	panelFileIcons := map[string]tcell.Color{}
	allowedPanelIconStyles := map[string]struct{}{
		"panel.active.row.cursor":                     {},
		"panel.active.row.cursor.selected":            {},
		"panel.active.row.cursor.unique":              {},
		"panel.inactive.row.cursor":                   {},
		"panel.inactive.row.cursor.selected":          {},
		"panel.carousel.inactive.row.cursor":          {},
		"panel.carousel.inactive.row.cursor.selected": {},
		"panel.blocked.row.cursor":                    {},
		"panel.blocked.row.cursor.selected":           {},
	}
	for key, spec := range specs {
		if spec.Icon == "" {
			continue
		}
		if _, ok := allowedPanelIconStyles[key]; !ok {
			return Theme{}, fmt.Errorf("style %q: field \"icon\" is only allowed on panel cursor row styles", key)
		}
		iconColor, err := resolveColor(spec.Icon, palette)
		if err != nil {
			return Theme{}, fmt.Errorf("style %q icon: %w", key, err)
		}
		panelFileIcons[key] = iconColor
	}
	var panelFileIconFG map[string]tcell.Color
	if len(panelFileIcons) > 0 {
		panelFileIconFG = panelFileIcons
	}

	return Theme{
		Name: name,

		MenuBarActive:        styles["menu.bar.active"],
		MenuBarInactive:      styles["menu.bar.inactive"],
		MenuBarSelected:      styles["menu.bar.selected"],
		MenuDropdown:         styles["menu.dropdown"],
		MenuDropdownSelected: styles["menu.dropdown.selected"],
		MenuDropdownFrame:    styles["menu.dropdown.frame"],
		MenuBarAccent:        styles["menu.bar.accent"],
		MenuBarAlert:         styles["menu.bar.alert"],
		MenuDropdownAccent:   styles["menu.dropdown.accent"],
		MenuDetail:           styles["menu.detail"],

		PanelActiveFrame:                    styles["panel.active.frame"],
		PanelInactiveFrame:                  styles["panel.inactive.frame"],
		PanelTargetFrame:                    styles["panel.target.frame"],
		PanelActiveSurface:                  styles["panel.active.surface"],
		PanelInactiveSurface:                styles["panel.inactive.surface"],
		PanelActiveTitle:                    styles["panel.active.title"],
		PanelInactiveTitle:                  styles["panel.inactive.title"],
		PanelActiveDiskUsageOverview:        styles["panel.active.disk_usage_overview"],
		PanelInactiveDiskUsageOverview:      styles["panel.inactive.disk_usage_overview"],
		PanelActiveHeader:                   styles["panel.active.header"],
		PanelActiveHeaderCarousel:           styles["panel.active.header.carousel"],
		PanelInactiveHeader:                 styles["panel.inactive.header"],
		PanelInactiveHeaderCarousel:         styles["panel.inactive.header.carousel"],
		PanelRowFile:                        styles["panel.row.file"],
		PanelRowDirectory:                   styles["panel.row.directory"],
		PanelRowSymlink:                     styles["panel.row.symlink"],
		PanelRowSelected:                    styles["panel.row.selected"],
		PanelRowInfo:                        styles["panel.row.info"],
		PanelDedupRowAllMarked:              styles["panel.dedup.row.all_marked"],
		PanelDedupRowCursorAllMarked:        styles["panel.dedup.row.cursor.all_marked"],
		PanelDedupRowKeep:                   styles["panel.dedup.row.keep"],
		PanelDedupRowCursorKeep:             styles["panel.dedup.row.cursor.keep"],
		PanelDedupRowMarkKeepSubtree:        styles["panel.dedup.row.mark.keep_subtree"],
		PanelRowMarkSelectionSubtree:        styles["panel.row.mark.selection_subtree"],
		PanelRowMarkNew:                     styles["panel.row.mark.new"],
		PanelRowMarkNewPrevious:             styles["panel.row.mark.new.previous"],
		PanelRowMarkRenamed:                 styles["panel.row.mark.renamed"],
		PanelRowMarkNoPermission:            styles["panel.row.mark.no_permission"],
		PanelRowMarkJob:                     styles["panel.row.mark.job"],
		PanelRowMarkJobRead:                 styles["panel.row.mark.job.read"],
		PanelRowMarkJobDecision:             styles["panel.row.mark.job.decision"],
		PanelRowTreeConnector:               styles["panel.row.tree.connector"],
		PanelIconFolderOpen:                 styles["panel.icon.folder.open"],
		PanelIconFolderMount:                styles["panel.icon.folder.mount"],
		PanelText:                           styles["panel.text"],
		PanelHint:                           styles["panel.hint"],
		PanelCursorActive:                   styles["panel.active.row.cursor"],
		PanelCursorActiveUnique:             styles["panel.active.row.cursor.unique"],
		PanelCursorInactive:                 styles["panel.inactive.row.cursor"],
		PanelActiveCursorSelected:           styles["panel.active.row.cursor.selected"],
		PanelInactiveCursorSelected:         styles["panel.inactive.row.cursor.selected"],
		PanelCarouselInactiveCursor:         styles["panel.carousel.inactive.row.cursor"],
		PanelCarouselInactiveCursorSelected: styles["panel.carousel.inactive.row.cursor.selected"],
		PanelStatusSync:                     styles["panel.status.sync"],
		PanelStatusQuickView:                styles["panel.status.quick_view"],
		PanelStatusSelections:               styles["panel.status.selections"],
		PanelStatusDotfilesHidden:           styles["panel.status.dotfiles_hidden"],
		PanelStatusGitignore:                styles["panel.status.gitignore"],
		PanelStatusStash:                    styles["panel.status.stash"],
		PanelStatusSelectionSize:            styles["panel.status.selection_size"],
		PanelStatusOtherPanel:               styles["panel.status.other_panel"],
		PanelScrollbarTrack:                 styles["panel.scrollbar.track"],
		PanelScrollbarThumb:                 styles["panel.scrollbar.thumb"],
		PanelFileIconFG:                     panelFileIconFG,

		PanelBlockedFrame:             styles["panel.blocked.frame"],
		PanelBlockedSurface:           styles["panel.blocked.surface"],
		PanelBlockedTitle:             styles["panel.blocked.title"],
		PanelBlockedDiskUsageOverview: styles["panel.blocked.disk_usage_overview"],
		PanelBlockedHeader:            styles["panel.blocked.header"],
		PanelBlockedHeaderCarousel:    styles["panel.blocked.header.carousel"],
		PanelBlockedRowFile:           styles["panel.blocked.row.file"],
		PanelBlockedRowDirectory:      styles["panel.blocked.row.directory"],
		PanelBlockedRowSymlink:        styles["panel.blocked.row.symlink"],
		PanelBlockedRowSelected:       styles["panel.blocked.row.selected"],
		PanelBlockedText:              styles["panel.blocked.text"],
		PanelBlockedCursor:            styles["panel.blocked.row.cursor"],
		PanelBlockedCursorSelected:    styles["panel.blocked.row.cursor.selected"],

		PanelIconFolderScanning:          styles["panel.icon.folder.scanning"],
		PanelIconFolderExcluded:          styles["panel.icon.folder.excluded"],
		MenuSpinner:                      styles["menu.spinner"],
		MenuProgressDone:                 styles["menu.progress.done"],
		MenuProgressRemaining:            styles["menu.progress.remaining"],
		MenuJobScanning:                  styles["menu.job.scanning"],
		MenuJobQueued:                    styles["menu.job.queued"],
		MenuJobRunning:                   styles["menu.job.running"],
		MenuJobPaused:                    styles["menu.job.paused"],
		MenuJobCanceled:                  styles["menu.job.canceled"],
		MenuJobFailed:                    styles["menu.job.failed"],
		MenuJobDecision:                  styles["menu.job.decision"],
		MenuJobCompleted:                 styles["menu.job.completed"],
		PanelUsageNormal:                 styles["panel.usage.normal"],
		PanelUsageSelected:               styles["panel.usage.selected"],
		PanelUsageCursorActive:           styles["panel.active.usage.cursor"],
		PanelUsageCursorInactive:         styles["panel.inactive.usage.cursor"],
		PanelActiveUsageCursorSelected:   styles["panel.active.usage.cursor.selected"],
		PanelInactiveUsageCursorSelected: styles["panel.inactive.usage.cursor.selected"],

		PanelGitNotModified: styles["panel.git.not_modified"],
		PanelGitNew:         styles["panel.git.new"],
		PanelGitModified:    styles["panel.git.modified"],
		PanelGitDeleted:     styles["panel.git.deleted"],
		PanelGitRenamed:     styles["panel.git.renamed"],
		PanelGitTypechange:  styles["panel.git.typechange"],
		PanelGitIgnored:     styles["panel.git.ignored"],
		PanelGitConflicted:  styles["panel.git.conflicted"],

		FuzzyInput:           styles["fuzzy.input"],
		FuzzyInputNomatch:    styles["fuzzy.input.nomatch"],
		FuzzyHighlight:       styles["fuzzy.highlight"],
		FuzzyHighlightCursor: styles["fuzzy.highlight.cursor"],

		DialogFrame:                    styles["dialog.frame"],
		DialogTitle:                    styles["dialog.title"],
		DialogText:                     styles["dialog.text"],
		DialogSurface:                  styles["dialog.surface"],
		DialogAccent:                   styles["dialog.accent"],
		DialogInputActive:              styles["dialog.input.active"],
		DialogInputActivePlaceholder:   styles["dialog.input.active.placeholder"],
		DialogInputActiveError:         styles["dialog.input.active.error"],
		DialogInputInactive:            styles["dialog.input.inactive"],
		DialogInputInactivePlaceholder: styles["dialog.input.inactive.placeholder"],
		DialogInputInactiveError:       styles["dialog.input.inactive.error"],
		DialogButtonInactive:           styles["dialog.button.inactive"],
		DialogButtonActive:             styles["dialog.button.active"],
		DialogButtonActiveDestructive:  styles["dialog.button.active_destructive"],
		DialogOptionInactive:           styles["dialog.option.inactive"],
		DialogOptionActive:             styles["dialog.option.active"],
		DialogOptionActiveSelected:     styles["dialog.option.active.selected"],
		DialogOptionSelected:           styles["dialog.option.selected"],
		DialogOptionInvalid:            styles["dialog.option.invalid"],
		DialogStatusSelectionSize:      styles["dialog.status.selection_size"],
		DialogProgressTrack:            styles["dialog.progress.track"],
		DialogProgressFill:             styles["dialog.progress.fill"],
		DialogProgressLabelOnFill:      styles["dialog.progress.label.on_fill"],
		DialogProgressLabelOnTrack:     styles["dialog.progress.label.on_track"],
		DialogMassRenameBefore:         styles["dialog.massrename.before"],
		DialogMassRenameBeforeRemoved:  styles["dialog.massrename.before.removed"],
		DialogMassRenameBeforeReplaced: styles["dialog.massrename.before.replaced"],
		DialogMassRenameAfter:          styles["dialog.massrename.after"],
		DialogMassRenameAfterAdded:     styles["dialog.massrename.after.added"],
		DialogMassRenameAfterError:     styles["dialog.massrename.after.error"],

		DialogQuickActionFrame:        styles["dialog.quickaction.frame"],
		DialogQuickActionSurface:      styles["dialog.quickaction.surface"],
		DialogQuickActionText:         styles["dialog.quickaction.text"],
		DialogQuickActionTitle:        styles["dialog.quickaction.title"],
		DialogQuickActionAccent:       styles["dialog.quickaction.accent"],
		DialogQuickActionListSelected: styles["dialog.quickaction.list.selected"],
		DialogQuickActionBorderStyle:  quickActionBorderStyle,

		MessageInfo:  styles["message.info"],
		MessageWarn:  styles["message.warn"],
		MessageError: styles["message.error"],

		JobsRow:     styles["jobs.row"],
		JobsRunning: styles["jobs.running"],
		JobsDone:    styles["jobs.done"],
		JobsFailed:  styles["jobs.failed"],

		JobsProgressTrack:        styles["jobs.progress.track"],
		JobsProgressFill:         styles["jobs.progress.fill"],
		JobsProgressLabelOnFill:  styles["jobs.progress.label.on_fill"],
		JobsProgressLabelOnTrack: styles["jobs.progress.label.on_track"],

		JobsIconsScanning:  styles["jobs.icons.scanning"],
		JobsIconsQueued:    styles["jobs.icons.queued"],
		JobsIconsRunning:   styles["jobs.icons.running"],
		JobsIconsPaused:    styles["jobs.icons.paused"],
		JobsIconsCanceled:  styles["jobs.icons.canceled"],
		JobsIconsFailed:    styles["jobs.icons.failed"],
		JobsIconsDecision:  styles["jobs.icons.decision"],
		JobsIconsCompleted: styles["jobs.icons.completed"],

		Symbols: symbols,

		FooterKey:        styles["footer.key"],
		FooterLabel:      styles["footer.label"],
		FooterLabelShift: styles["footer.label.shift"],

		TerminalText: styles["terminal.text"],
	}, nil
}

func stringField(raw map[string]any, key string) (string, error) {
	value, ok := raw[key]
	if !ok {
		return "", fmt.Errorf("%s is required", key)
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", key)
	}
	return text, nil
}

// quickActionBorderStyleField extracts and validates dialog.quickaction.border ("rounded",
// the default, or "sharp"), then deletes it from raw so the generic style-table flattener
// (collectStyleSpecs) doesn't trip over a plain string sitting among dialog.quickaction's
// fg/bg style tables.
func quickActionBorderStyleField(raw map[string]any) (string, error) {
	dialogTable, _ := raw["dialog"].(map[string]any)
	if dialogTable == nil {
		return QuickActionBorderRounded, nil
	}
	quickActionTable, _ := dialogTable["quickaction"].(map[string]any)
	if quickActionTable == nil {
		return QuickActionBorderRounded, nil
	}
	value, ok := quickActionTable["border"]
	if !ok {
		return QuickActionBorderRounded, nil
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("dialog.quickaction.border must be a string")
	}
	switch text {
	case QuickActionBorderRounded, QuickActionBorderSharp:
	default:
		return "", fmt.Errorf("dialog.quickaction.border must be %q or %q, got %q", QuickActionBorderRounded, QuickActionBorderSharp, text)
	}
	delete(quickActionTable, "border")
	return text, nil
}

func symbolsField(raw map[string]any) (map[string]string, error) {
	value, ok := raw["symbols"]
	if !ok {
		return map[string]string{}, nil
	}
	table, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("symbols must be a table")
	}
	symbols := map[string]string{}
	for name, rawValue := range table {
		if err := collectSymbolEntry(name, rawValue, symbols); err != nil {
			return nil, err
		}
	}
	return symbols, nil
}

// collectSymbolEntry stores one leaf string at fullKey, or recurses into nested tables.
// A value of { symbol = "…" } (only key) remains supported as a leaf for a single entry.
func collectSymbolEntry(fullKey string, rawValue any, symbols map[string]string) error {
	switch v := rawValue.(type) {
	case string:
		symbols[fullKey] = v
		return nil
	case map[string]any:
		if len(v) == 0 {
			return fmt.Errorf("symbols.%s: empty table", fullKey)
		}
		if sym, ok := v["symbol"]; ok {
			if len(v) != 1 {
				return fmt.Errorf("symbols.%s: use nested tables when defining multiple keys; { symbol = \"...\" } must be the only entry", fullKey)
			}
			symStr, ok := sym.(string)
			if !ok {
				return fmt.Errorf("symbols.%s.symbol must be a string", fullKey)
			}
			symbols[fullKey] = symStr
			return nil
		}
		for childName, childVal := range v {
			if err := collectSymbolEntry(fullKey+"."+childName, childVal, symbols); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("symbols.%s: must be a string, nested table of strings, or inline { symbol = \"...\" }", fullKey)
	}
}

func paletteField(raw map[string]any) (map[string]tcell.Color, error) {
	value, ok := raw["palette"]
	if !ok {
		return nil, fmt.Errorf("palette is required")
	}
	table, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("palette must be a table")
	}
	palette := map[string]tcell.Color{}
	for name, rawValue := range table {
		color, err := parsePaletteEntry(rawValue)
		if err != nil {
			return nil, fmt.Errorf("palette.%s: %w", name, err)
		}
		palette[name] = color
	}
	return palette, nil
}

func parsePaletteEntry(rawValue any) (tcell.Color, error) {
	switch v := rawValue.(type) {
	case string:
		if strings.EqualFold(strings.TrimSpace(v), "default") {
			return tcell.ColorDefault, nil
		}
		return parseHexColor(v)
	case int64:
		if v < 0 || v > 255 {
			return tcell.ColorDefault, fmt.Errorf("ANSI palette index must be 0-255, got %d", v)
		}
		return tcell.PaletteColor(int(v)), nil
	case int:
		if v < 0 || v > 255 {
			return tcell.ColorDefault, fmt.Errorf("ANSI palette index must be 0-255, got %d", v)
		}
		return tcell.PaletteColor(v), nil
	default:
		return tcell.ColorDefault, fmt.Errorf("must be \"default\", #RRGGBB hex string, or ANSI palette index 0-255")
	}
}

func collectStyleSpecs(raw map[string]any) (map[string]styleSpec, error) {
	specs := map[string]styleSpec{}
	found := false
	for _, root := range styleSectionRoots {
		value, ok := raw[root]
		if !ok {
			continue
		}
		table, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s must be a table", root)
		}
		if err := flattenStyleSpecs(root, table, specs); err != nil {
			return nil, err
		}
		found = true
	}
	if !found {
		return nil, fmt.Errorf("theme must define at least one style section ([menu], [panel], [dialog], [jobs], [message], [footer], [fuzzy], or [terminal])")
	}
	for key := range specs {
		if !requiredStyleKeySet[key] {
			return nil, fmt.Errorf("unknown style %q", key)
		}
	}
	return specs, nil
}

func flattenStyleSpecs(prefix string, table map[string]any, specs map[string]styleSpec) error {
	specFields := map[string]any{}
	for key, value := range table {
		nextKey := key
		if prefix != "" {
			nextKey = prefix + "." + key
		}
		if isStyleField(key) {
			if child, ok := value.(map[string]any); ok && isNestedStyleSubtree(child) {
				if err := flattenStyleSpecs(nextKey, child, specs); err != nil {
					return err
				}
				continue
			}
			specFields[key] = value
			continue
		}
		child, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("style %q must be a table", nextKey)
		}
		if err := flattenStyleSpecs(nextKey, child, specs); err != nil {
			return err
		}
	}
	if len(specFields) > 0 {
		if prefix == "" {
			return fmt.Errorf("styles root cannot contain style fields")
		}
		spec, err := decodeStyleSpec(prefix, specFields)
		if err != nil {
			return err
		}
		specs[prefix] = spec
	}
	return nil
}

func isNestedStyleSubtree(table map[string]any) bool {
	for key, value := range table {
		if isStyleField(key) {
			return false
		}
		if _, ok := value.(map[string]any); !ok {
			return false
		}
	}
	return len(table) > 0
}

func isStyleField(key string) bool {
	switch key {
	case "fg", "bg", "bold", "underline", "reverse", "icon":
		return true
	default:
		return false
	}
}

func decodeStyleSpec(key string, table map[string]any) (styleSpec, error) {
	var spec styleSpec
	for field, value := range table {
		switch field {
		case "fg":
			text, ok := value.(string)
			if !ok {
				return styleSpec{}, fmt.Errorf("style %q fg must be a string", key)
			}
			spec.FG = text
		case "bg":
			text, ok := value.(string)
			if !ok {
				return styleSpec{}, fmt.Errorf("style %q bg must be a string", key)
			}
			spec.BG = text
		case "bold":
			flag, ok := value.(bool)
			if !ok {
				return styleSpec{}, fmt.Errorf("style %q bold must be a bool", key)
			}
			spec.Bold = flag
		case "underline":
			flag, ok := value.(bool)
			if !ok {
				return styleSpec{}, fmt.Errorf("style %q underline must be a bool", key)
			}
			spec.Underline = flag
		case "icon":
			text, ok := value.(string)
			if !ok {
				return styleSpec{}, fmt.Errorf("style %q icon must be a string", key)
			}
			spec.Icon = text
		case "reverse":
			flag, ok := value.(bool)
			if !ok {
				return styleSpec{}, fmt.Errorf("style %q reverse must be a bool", key)
			}
			spec.Reverse = flag
		default:
			return styleSpec{}, fmt.Errorf("style %q has unknown field %q", key, field)
		}
	}
	return spec, nil
}

func buildStyle(spec styleSpec, palette map[string]tcell.Color) (tcell.Style, error) {
	style := tcell.StyleDefault
	if spec.FG != "" {
		color, err := resolveColor(spec.FG, palette)
		if err != nil {
			return tcell.Style{}, fmt.Errorf("fg: %w", err)
		}
		style = style.Foreground(color)
	}
	if spec.BG != "" {
		color, err := resolveColor(spec.BG, palette)
		if err != nil {
			return tcell.Style{}, fmt.Errorf("bg: %w", err)
		}
		style = style.Background(color)
	}
	if spec.Bold {
		style = style.Bold(true)
	}
	if spec.Underline {
		style = style.Underline(true)
	}
	if spec.Reverse {
		style = style.Reverse(true)
	}
	return style, nil
}

func resolveColor(value string, palette map[string]tcell.Color) (tcell.Color, error) {
	if strings.HasPrefix(value, "#") {
		return parseHexColor(value)
	}
	if strings.EqualFold(strings.TrimSpace(value), "default") {
		return tcell.ColorDefault, nil
	}
	color, ok := palette[value]
	if !ok {
		return tcell.ColorDefault, fmt.Errorf("unknown color %q", value)
	}
	return color, nil
}

func parseHexColor(value string) (tcell.Color, error) {
	if len(value) != 7 || value[0] != '#' {
		return tcell.ColorDefault, fmt.Errorf("expected #RRGGBB, got %q", value)
	}
	var rgb [3]int32
	for i := range rgb {
		part := value[1+i*2 : 3+i*2]
		parsed, err := strconv.ParseUint(part, 16, 8)
		if err != nil {
			return tcell.ColorDefault, fmt.Errorf("invalid hex color %q", value)
		}
		rgb[i] = int32(parsed)
	}
	return tcell.NewRGBColor(rgb[0], rgb[1], rgb[2]), nil
}

func makeStyleKeySet(keys []string) map[string]bool {
	set := make(map[string]bool, len(keys))
	for _, key := range keys {
		set[key] = true
	}
	return set
}

func themeLabel(name string) string {
	if label := builtInThemeLabels[name]; label != "" {
		return label
	}
	label := strings.ReplaceAll(name, "-", " ")
	words := strings.Fields(label)
	for i, word := range words {
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	if len(words) == 0 {
		return name
	}
	return strings.Join(words, " ")
}
