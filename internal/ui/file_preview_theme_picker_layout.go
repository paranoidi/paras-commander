package ui

import (
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
	"unicode/utf8"
)

const (
	filePreviewThemePickerMinWidth = 18
	filePreviewThemePickerSepCols  = 1
)

// SplitFullscreenPreviewRects divides the twin-panel union into a left preview region and
// an optional right theme picker. When the picker is closed, previewRect is the full union.
func SplitFullscreenPreviewRects(union Rect, pickerOpen bool, choices []ThemeChoice) (previewRect, pickerRect Rect) {
	if !pickerOpen || len(choices) == 0 {
		return union, Rect{}
	}
	pickerW := filePreviewThemePickerOuterWidth(union, choices)
	if pickerW <= 0 || pickerW >= union.Width {
		return union, Rect{}
	}
	previewW := union.Width - pickerW - filePreviewThemePickerSepCols
	if previewW < union.Width/2 {
		previewW = union.Width / 2
		pickerW = union.Width - previewW - filePreviewThemePickerSepCols
	}
	if previewW < 8 || pickerW < filePreviewThemePickerMinWidth {
		return union, Rect{}
	}
	previewRect = Rect{X: union.X, Y: union.Y, Width: previewW, Height: union.Height}
	pickerRect = Rect{
		X:      union.X + previewW + filePreviewThemePickerSepCols,
		Y:      union.Y,
		Width:  pickerW,
		Height: union.Height,
	}
	return previewRect, pickerRect
}

func filePreviewThemePickerOuterWidth(union Rect, choices []ThemeChoice) int {
	listInner := 0
	for _, choice := range choices {
		w := utf8.RuneCountInString(choice.Label) + 6
		if w > listInner {
			listInner = w
		}
	}
	if listInner < 12 {
		listInner = 12
	}
	outer := listInner + 4
	if outer < filePreviewThemePickerMinWidth {
		outer = filePreviewThemePickerMinWidth
	}
	maxW := union.Width / 3
	if maxW < filePreviewThemePickerMinWidth {
		maxW = filePreviewThemePickerMinWidth
	}
	if outer > maxW {
		outer = maxW
	}
	return outer
}

// FilePreviewThemePickerListRows returns how many theme rows fit in the picker panel.
func FilePreviewThemePickerListRows(rect Rect) int {
	return dialog.FilePreviewThemePickerListRows(geom.Rect(rect))
}

// FilePreviewThemePickerQueryWidth is the inner width for the fuzzy filter input row.
func FilePreviewThemePickerQueryWidth(rect Rect) int {
	w := rect.Width - 4
	if w < 1 {
		return 1
	}
	return w
}
