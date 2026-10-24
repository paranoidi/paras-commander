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
)

// Item is a radio or checkbox row, or a static label row (KindLabel).
type Item struct {
	ID       ID
	Kind     Kind
	Label    string
	Mnemonic rune // 0 = none
	On       bool // radio selected / checkbox checked
}

// focusable reports whether keyboard focus and mouse clicks can land on the item.
func (it Item) focusable() bool { return it.Kind != KindLabel }

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
// column may be truncated to fit). Kind is KindLabel for static text rows, which HitTest skips; a
// label's Rect spans the whole text cell range it paints, spaces included.
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
type Event struct{ ID ID }
