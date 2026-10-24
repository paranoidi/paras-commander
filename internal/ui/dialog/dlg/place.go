package dlg

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
)

func (g Group) height() int {
	h := 0
	for _, col := range g.Columns {
		h = max(h, len(col))
	}
	return h
}

// Place is the one layout pass: it clamps and centers the dialog on a screen of screenW x screenH
// and positions every widget. ok is false when the screen is too small to show the dialog.
func Place(screenW, screenH int, spec Spec) (Placement, bool) {
	prefH := 2
	for _, g := range spec.Groups {
		prefH += g.height()
	}
	if len(spec.Buttons) > 0 {
		prefH += 2 // separator + button row
	}
	prefH = max(prefH, spec.MinHeight)
	rect, ok := draw.ClampCenteredDialogRect(draw.Layout{Width: screenW, Height: screenH}, spec.Width, prefH, spec.MinWidth, spec.MinHeight)
	if !ok {
		return Placement{}, false
	}
	p := Placement{Rect: rect}
	y := rect.Y + 1
	for _, g := range spec.Groups {
		colW := rect.Width / max(len(g.Columns), 1)
		for ci, col := range g.Columns {
			x := draw.DialogOptionX(rect) + ci*colW
			right := rect.X + rect.Width - 1 // stop one cell before the right border
			if ci+1 < len(g.Columns) {
				right = draw.DialogOptionX(rect) + (ci+1)*colW
			}
			for ri, it := range col {
				// The marker counts against the column too, or a long label runs past the border.
				label := primitive.TruncateRight(it.Label, max(right-x-draw.DialogOptionWidth(""), 0))
				p.Items = append(p.Items, PlacedItem{
					ID:    it.ID,
					Rect:  Rect{X: x, Y: y + ri, Width: draw.DialogOptionWidth(label), Height: 1},
					Label: label,
				})
			}
		}
		y += g.height()
	}
	if len(spec.Buttons) > 0 {
		p.SepY, p.ButtonY = y, y+1
		labels := make([]string, len(spec.Buttons))
		for i, b := range spec.Buttons {
			labels[i] = b.Label
		}
		for i, x := range draw.ButtonRowXs(rect, labels) {
			b := spec.Buttons[i]
			p.Items = append(p.Items, PlacedItem{ID: b.ID, Rect: Rect{X: x, Y: p.ButtonY, Width: draw.DialogButtonWidth(b.Label), Height: 1}, Label: b.Label})
		}
	}
	return p, true
}

// Draw paints spec with focus on the widget named by focus (an empty or unknown ID focuses the
// first widget).
func Draw(screen tcell.Screen, screenW, screenH int, spec Spec, focus ID, styles theme.Theme) {
	p, ok := Place(screenW, screenH, spec)
	if !ok {
		return
	}
	focus = resolve(spec, focus)
	borderStyle := draw.DrawDialogFrame(screen, p.Rect, spec.Title, styles)
	placed := map[ID]PlacedItem{}
	for _, pi := range p.Items {
		placed[pi.ID] = pi
	}
	for _, g := range spec.Groups {
		for _, col := range g.Columns {
			for _, it := range col {
				pi := placed[it.ID]
				if it.Kind == KindCheckbox {
					draw.DrawDialogCheckbox(screen, pi.Rect.X, pi.Rect.Y, pi.Label, it.Mnemonic, it.On, focus == it.ID, false, styles)
				} else {
					draw.DrawDialogRadio(screen, pi.Rect.X, pi.Rect.Y, pi.Label, it.Mnemonic, it.On, focus == it.ID, false, styles)
				}
			}
		}
	}
	if len(spec.Buttons) == 0 {
		return
	}
	draw.DrawDialogHSeparator(screen, p.Rect, p.SepY, borderStyle)
	specs := make([]draw.DialogButtonSpec, len(spec.Buttons))
	for i, b := range spec.Buttons {
		specs[i] = draw.DialogButtonSpec{Label: b.Label, Shortcut: b.Mnemonic, Focused: focus == b.ID, Destructive: b.Destructive}
	}
	draw.DrawDialogButtonRowCentered(screen, p.Rect, p.ButtonY, specs, styles)
}

// HitTest returns the widget under screen cell (x, y).
func HitTest(p Placement, x, y int) (ID, bool) {
	for _, it := range p.Items {
		r := it.Rect
		if x >= r.X && x < r.X+r.Width && y >= r.Y && y < r.Y+r.Height {
			return it.ID, true
		}
	}
	return "", false
}
