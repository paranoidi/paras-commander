// Package dlg is a small declarative dialog engine. A dialog is described once as a plain-data
// Spec (widgets, Tab groups, mnemonics, buttons); Place computes the geometry in a single pass,
// and Draw, HitTest, HandleKey and Click all derive from that Spec and Placement, so painting,
// focus order, mnemonics and mouse hitboxes cannot drift apart. State stays with the caller
// (View(state) -> Spec); the engine reports activations as Events instead of calling back.
//
// The package must not import internal/ui/dialog: dialog builds specs and imports dlg.
package dlg

import (
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// ID names a widget. OK and Cancel are the conventional button IDs.
type ID string

const (
	OK     ID = "ok"
	Cancel ID = "cancel"
)

// Kind is the widget type of an Item.
type Kind int

const (
	KindRadio Kind = iota
	KindCheckbox
	// KindLabel is a static text row in the dialog's text style. It has no mnemonic and no state
	// and is never focusable or hit-testable, so its ID is optional. An empty Label is a blank
	// spacer row.
	KindLabel
	// KindSeparator is a full-width horizontal rule (border to border) in the frame's border style.
	// A non-empty Label is drawn centered in it, padded with one space on each side. It is never
	// focusable or a hit target, so its ID is optional; a group made only of separators and labels
	// is not a Tab segment. It always spans the dialog, whatever column it sits in.
	KindSeparator
	// KindInput is a single-line text input across the content width (DialogTextX /
	// DialogContentWidth). The engine only draws and places it: it does not edit text, the caller
	// keeps Value/Cursor/Scroll in its own state (see HandleKey for the key contract).
	KindInput
	// KindList is a scrollable list of plain-data rows (Item.Rows) with a cursor. The engine
	// draws it, moves its cursor and scroll offset (ListMove events) and reports the activated
	// row, but the rows, Cursor and Offset stay in the caller's state like an input's text. It is
	// its own Tab segment (unless Driven) and, while it has no rows, is skipped by every focus move.
	// See HandleKey for the key contract.
	//
	// Row features: Text, fuzzy-match highlights (Match), a leading Icon, an invalid-path variant
	// (Missing), trailing Marks (pinned / job / missing icons), a radio-style marker (Marked, with
	// Item.Markers) and the cursor row's focus style.
	KindList
	// KindSpacer reserves Height rows of the column for the caller to paint after Draw (the
	// overlay escape hatch, see dialogs.md). It is never focusable or a hit target and its ID
	// is optional; Placement.Item(id) returns its rect.
	KindSpacer
	// KindRow lays Cells out left to right on one dialog row (see Item.Cells). It has no ID of its
	// own: its cells are the widgets, each with its own placed rect, focus stop and mouse hit rect.
	KindRow
)

// Fit says how a label that is wider than its cell is shortened.
type Fit int

const (
	FitRight  Fit = iota // clip the end with an ellipsis
	FitMiddle            // keep both ends, ellipsis in the middle
	FitPath              // shorten leading directories first, keep the basename (primitive.FitPathForWidth)
	// FitNone is for list rows the caller already fitted to the row width (column grids): the text
	// is drawn as is, clipped, and ListRow.Match ranges are rune offsets into that displayed text.
	FitNone
)

// TextStyle is the style variant of a KindLabel.
type TextStyle int

const (
	TextNormal       TextStyle = iota // DialogText on the surface
	TextError                         // DialogText with the input error foreground
	TextMessageError                  // the theme's MessageError style on the dialog surface (message dialog body)
)

// Item is a radio or checkbox row, a static label row (KindLabel), a separator or a text input.
type Item struct {
	ID       ID
	Kind     Kind
	Label    string
	Mnemonic rune // 0 = none
	On       bool // radio selected / checkbox checked

	// Label rows only.
	Indent int       // extra cells before the text
	Fit    Fit       // how an overlong Label (or list row text) is shortened
	Prefix string    // fixed text drawn before Label; only Label is shortened (FitPath)
	Style  TextStyle // text style variant
	Center bool      // center the text in the available width

	// Input rows only; they mirror draw.ScrollingInputState.
	Value            string
	Cursor, Scroll   int
	CompletionSuffix string // ghost completion drawn at the caret
	LeadingIcon      string
	Invalid          bool // error styling
	Placeholder      bool // Value is a suggested default: drawn in the placeholder style
	// Simple draws draw.DrawSimpleDialogInput (no scrolling, no ghost text, no icon) instead of the
	// scrolling input.
	Simple bool
	// Drives names a KindList that this input filters. While the input is focused and that list
	// has rows, Up/Down/PgUp/PgDn/Ctrl+Home/Ctrl+End move the list's cursor (ListMove events)
	// and focus stays in the input. The list is drawn with its cursor row highlighted while this
	// input (or the list itself) holds focus.
	Drives ID

	// List rows only (KindList). Cursor is the cursor row index (shared with the input's caret
	// field) and Offset the first visible row; both stay in the caller's state.
	Rows   []ListRow
	Offset int
	// Height is the number of visible rows of a list, or the rows a KindSpacer reserves.
	Height int
	// Wrap makes Up/Down/PgUp/PgDn wrap around the ends instead of stopping at them.
	Wrap bool
	// CenterScroll keeps the cursor row centered in the viewport (geom.ScrollOffset, Offset is
	// ignored). Otherwise the viewport scrolls only as far as needed (Offset is sticky).
	CenterScroll bool
	// PageStep is how many rows PgUp/PgDn move; 0 means Height-1 (at least 1).
	PageStep int
	// Driven marks a list whose focus lives in the input that Drives it: the list is never a focus
	// stop or Tab segment itself, its cursor row is lit while that input is focused, and a click on
	// one of its rows focuses that input.
	Driven bool
	// Markers draws a radio glyph before each row (lit when ListRow.Marked) and styles every row
	// as an option row; otherwise rows are plain dialog text and only the focused cursor row is
	// highlighted.
	Markers bool
	// Scrollbar draws draw.DrawDialogListScrollbar on the dialog's right border when the rows
	// overflow Height.
	Scrollbar bool
	// IconOptionFG paints ListRow.Icon in the foreground of the option row style (active under
	// the cursor, inactive elsewhere) on the row's background instead of in the row's own style,
	// so an invalid row keeps a neutral icon.
	IconOptionFG bool
	// MarksAfterText places a row's trailing Marks right after its (fitted) text instead of at the
	// row's right edge.
	MarksAfterText bool
	// MarksWidth reserves this many cells at the row's right edge for Marks on every row, so a
	// column of marks stays aligned however the rows are filtered (0 = each row reserves what its
	// own Marks need). Ignored with MarksAfterText.
	MarksWidth int

	// Disabled (radio and checkbox) draws the item in the disabled style and takes it out of
	// focus, mnemonics and hits.
	Disabled bool
	// Seg names the Tab segment of an item inside its group (default 0). Items of one group that
	// share a Seg are one Tab segment, whatever their layout order, so a grid whose rows hold a
	// radio and its checkboxes can still Tab between the radios and the checkboxes. A segment
	// starts at its first item in focus order; inputs and lists remain segments of their own.
	Seg int

	// Row only (KindRow). Cells are laid out left to right from the column's left edge, each in a
	// box whose width follows its own rule (Width, Grow or its content), followed by the cell's Gap. Cells may be radios, checkboxes, labels, inputs and spacers (no
	// rows, lists or separators). The row ends one cell before the right border, like labels and
	// inputs; a cell that does not fit is clipped, and one that would start past the end is not
	// drawn (its Rect is empty). Left/Right move between the focusable cells of a row (never out of
	// a focused input); Up/Down keep the cell index of the row they leave (clamped to the row, then
	// the nearest focusable cell to the left, then to the right).
	Cells []Item
	// EntryLast makes Up land on the row's last focusable cell, instead of its first, when focus
	// comes from a widget that is not a cell of a row (a button or an item of another kind).
	EntryLast bool

	// Cell box (items inside Cells). The box starts where the previous one (plus its Gap) ends; radios
	// and checkboxes start in the box's first cell, labels and inputs one cell in (the dialog's
	// text column). Width is the box width in cells; 0 means the cell's content: the marker and
	// label of a radio or checkbox, the indent and text of a label. Grow cells split what the fixed
	// and content boxes leave, equally (an input without Width grows too). Width and Grow on an
	// item outside a row are ignored.
	Width int
	Grow  bool
	// Gap is the number of blank cells after the box.
	Gap int
	// Right right-aligns a label's text in its box.
	Right bool
}

// ListRow is one row of a KindList.
type ListRow struct {
	Text   string
	Match  []search.Range // fuzzy-match highlight ranges (rune offsets into Text)
	Marked bool           // lit radio glyph when the list has Markers
	// Icon is drawn before Text followed by one space (empty = none); Match ranges stay keyed to Text.
	Icon string
	// Missing styles the row as an invalid path (unless it is under the cursor) and adds the
	// missing mark to Marks.
	Missing bool
	// Marks are the pinned / job / missing icons drawn after the text (see Item.MarksAfterText,
	// Item.MarksWidth). Marks.Cursor is set by the engine.
	Marks RowMarks
}

// focusable reports whether keyboard focus and mouse clicks can land on the item.
func (it Item) focusable() bool {
	switch it.Kind {
	case KindLabel, KindSeparator, KindSpacer:
		return false
	case KindRadio, KindCheckbox:
		return !it.Disabled
	case KindRow:
		for _, c := range it.Cells {
			if c.focusable() {
				return true
			}
		}
		return false
	case KindList:
		return len(it.Rows) > 0 && !it.Driven // an empty list has nothing to select
	}
	return true
}

// span is how many dialog rows the item occupies.
func (it Item) span() int {
	if (it.Kind == KindList || it.Kind == KindSpacer) && it.Height > 1 {
		return it.Height
	}
	return 1
}

// Group is one Tab segment: columns side by side (a single column is the common case).
//
// ColWidths, when set, are the widths in cells of every column but the last (which takes the rest
// up to the right border); without it the columns share the width equally.
type Group struct {
	Columns   [][]Item
	ColWidths []int
}

// Button is a push button in the strip below the groups.
type Button struct {
	ID          ID
	Label       string
	Mnemonic    rune
	Destructive bool
}

// Spec describes a whole dialog.
//
// Width is the preferred width; Place widens it to fit the button strip between the borders.
type Spec struct {
	Title                      string
	Width, MinWidth, MinHeight int
	Groups                     []Group
	Buttons                    []Button // drawn after a horizontal separator, centered; also its own Tab segment
	PlainButtonKeys            bool     // plain (no Alt) button mnemonic letters press the button
	// Unclamped sizes the dialog like draw.CenteredDialogRect instead of refusing to draw on a small
	// screen: width is min(Width, screen-4), widened to min(MinWidth, screen-2) when that is below
	// MinWidth, and the height only shrinks to the screen. MinHeight still raises the height.
	Unclamped bool
	// ScrollbarStyle is the scrollbar variant for lists with Item.Scrollbar (empty = thumb).
	ScrollbarStyle uiscrollbar.Style
	// MarkerWidth is the width in cells of a radio or checkbox marker (" ( ) ", " <icon> ") for
	// the content width of row cells. Zero means the ASCII marker (5 cells); a caller that knows
	// its theme passes MarkerWidth(styles) so rows follow the theme's narrower icons.
	MarkerWidth int
}

// Rect is a screen region in cells.
type Rect = draw.Rect

// PlacedItem is a widget's screen position. Label is the text actually drawn (a label in a later
// column may be truncated to fit). Kind is KindLabel for static text rows and KindSeparator for
// rules, which HitTest skips; a label's Rect spans the whole text cell range it paints, spaces
// included, and a separator's the full dialog width. A KindInput's Rect is its full row.
//
// A KindList's Rect spans its visible rows and text width (the scrollbar column is outside it);
// Offset is the first visible row and Rows the number of rows in the data, so rows past the end
// are not hits. A KindSpacer's Rect is the whole column cell it reserves.
type PlacedItem struct {
	ID           ID
	Kind         Kind
	Rect         Rect
	Label        string
	Offset, Rows int
	// Disabled items are drawn but are not hit targets.
	Disabled bool
}

// Placement is the result of the single layout pass. Items are the group items (labels included) in
// spec order, then the buttons. SepY and ButtonY
// are the screen rows of the button separator and the button strip (0 when there are no buttons).
type Placement struct {
	Rect          Rect
	Items         []PlacedItem
	SepY, ButtonY int
	// ContentBottom is the first screen row content is not drawn on: the button separator, or
	// the bottom border when there are no buttons.
	ContentBottom int
}

// Event reports an activation. A zero ID means the key was handled but only moved focus.
//
// Col is set only by Click on a KindInput: the clicked cell's offset from the input's left edge
// (so the caller can place its cursor; an input with a LeadingIcon starts its text after the icon).
//
// List events carry the list's ID. EventActivate (the zero Kind) reports the activated row in
// Index. EventListMove reports the list's new Cursor in Index and the new first visible row in
// Offset; the caller stores both in its state.
type Event struct {
	ID     ID
	Col    int
	Kind   EventKind
	Index  int
	Offset int
}

// EventKind says what an Event with a list ID reports.
type EventKind int

const (
	EventActivate EventKind = iota // a widget (or list row) was activated
	EventListMove                  // a list's cursor / scroll offset moved
)

// Item returns the placed widget named id (an input's Rect is where a caller anchors overlays such
// as a completion dropdown or a validation hint).
func (p Placement) Item(id ID) (PlacedItem, bool) {
	for _, it := range p.Items {
		if it.ID == id && id != "" {
			return it, true
		}
	}
	return PlacedItem{}, false
}
