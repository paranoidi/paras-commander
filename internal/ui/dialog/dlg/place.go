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
	prefW := spec.Width
	labels := make([]string, len(spec.Buttons))
	for i, b := range spec.Buttons {
		labels[i] = b.Label
	}
	if len(labels) > 0 {
		prefW = max(prefW, draw.ButtonRowWidth(labels)+2) // the strip must fit between the borders
	}
	layout := draw.Layout{Width: screenW, Height: screenH}
	var rect Rect
	if spec.Unclamped {
		w := min(prefW, screenW-4)
		if w < spec.MinWidth {
			w = min(spec.MinWidth, screenW-2)
		}
		rect = draw.CenteredDialogRect(layout, w, prefH)
	} else {
		var ok bool
		if rect, ok = draw.ClampCenteredDialogRect(layout, prefW, prefH, spec.MinWidth, spec.MinHeight); !ok {
			return Placement{}, false
		}
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
				if it.Kind == KindLabel {
					// Text starts where the option marker's glyph does and keeps the one-cell right margin.
					w := max(right-x-2, 0)
					p.Items = append(p.Items, PlacedItem{
						ID: it.ID, Kind: KindLabel,
						Rect:  Rect{X: x + 1, Y: y + ri, Width: w, Height: 1},
						Label: primitive.TruncateRight(it.Label, w),
					})
					continue
				}
				// The marker counts against the column too, or a long label runs past the border.
				label := primitive.TruncateRight(it.Label, max(right-x-draw.DialogOptionWidth(""), 0))
				p.Items = append(p.Items, PlacedItem{
					ID:    it.ID,
					Kind:  it.Kind,
					Rect:  Rect{X: x, Y: y + ri, Width: draw.DialogOptionWidth(label), Height: 1},
					Label: label,
				})
			}
		}
		y += g.height()
	}
	if len(spec.Buttons) > 0 {
		p.SepY, p.ButtonY = y, y+1
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
	_, surface, _ := styles.DialogSurface.Decompose()
	n := 0 // p.Items holds the group items in the same order as this walk
	for _, g := range spec.Groups {
		for _, col := range g.Columns {
			for _, it := range col {
				pi := p.Items[n]
				n++
				switch it.Kind {
				case KindLabel:
					primitive.Text(screen, pi.Rect.X, pi.Rect.Y, pi.Rect.Width, pi.Label, styles.DialogText.Background(surface))
				case KindCheckbox:
					draw.DrawDialogCheckbox(screen, pi.Rect.X, pi.Rect.Y, pi.Label, it.Mnemonic, it.On, focus == it.ID, false, styles)
				default:
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
		if it.Kind == KindLabel {
			continue
		}
		if x >= r.X && x < r.X+r.Width && y >= r.Y && y < r.Y+r.Height {
			return it.ID, true
		}
	}
	return "", false
}
