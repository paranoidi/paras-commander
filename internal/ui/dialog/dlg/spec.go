// Package dlg is a small declarative dialog engine. A dialog is described once as a plain-data
// Spec (widgets, Tab groups, mnemonics, buttons); Place computes the geometry in a single pass,
// and Draw, HitTest, HandleKey and Click all derive from that Spec and Placement, so painting,
// focus order, mnemonics and mouse hitboxes cannot drift apart. State stays with the caller
// (View(state) -> Spec); the engine reports activations as Events instead of calling back.
//
// The package must not import internal/ui/dialog: dialog builds specs and imports dlg.
package dlg

import "github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"

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
)

// Fit says how a label that is wider than its cell is shortened.
type Fit int

const (
	FitRight  Fit = iota // clip the end with an ellipsis
	FitMiddle            // keep both ends, ellipsis in the middle
	FitPath              // shorten leading directories first, keep the basename (primitive.FitPathForWidth)
)

// TextStyle is the style variant of a KindLabel.
type TextStyle int

const (
	TextNormal TextStyle = iota // DialogText on the surface
	TextError                   // DialogText with the input error foreground
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
	Fit    Fit       // how an overlong Label is shortened
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
}

// focusable reports whether keyboard focus and mouse clicks can land on the item.
func (it Item) focusable() bool { return it.Kind != KindLabel && it.Kind != KindSeparator }

// Group is one Tab segment: columns side by side (a single column is the common case).
type Group struct{ Columns [][]Item }

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
}

// Rect is a screen region in cells.
type Rect = draw.Rect

// PlacedItem is a widget's screen position. Label is the text actually drawn (a label in a later
// column may be truncated to fit). Kind is KindLabel for static text rows and KindSeparator for
// rules, which HitTest skips; a label's Rect spans the whole text cell range it paints, spaces
// included, and a separator's the full dialog width. A KindInput's Rect is its full row.
type PlacedItem struct {
	ID    ID
	Kind  Kind
	Rect  Rect
	Label string
}

// Placement is the result of the single layout pass. Items are the group items (labels included) in
// spec order, then the buttons. SepY and ButtonY
// are the screen rows of the button separator and the button strip (0 when there are no buttons).
type Placement struct {
	Rect          Rect
	Items         []PlacedItem
	SepY, ButtonY int
}

// Event reports an activation. A zero ID means the key was handled but only moved focus.
//
// Col is set only by Click on a KindInput: the clicked cell's offset from the input's left edge
// (so the caller can place its cursor; an input with a LeadingIcon starts its text after the icon).
type Event struct {
	ID  ID
	Col int
}

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
