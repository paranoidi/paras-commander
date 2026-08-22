package theme

import "strings"

const (
	SymbolKeyDialogCheckboxChecked   = "dialog.checkbox.checked"
	SymbolKeyDialogCheckboxUnchecked = "dialog.checkbox.unchecked"
	SymbolKeyDialogRadioSelected     = "dialog.radio.selected"
	SymbolKeyDialogRadioUnselected   = "dialog.radio.unselected"
)

// SymbolDialogCheckbox returns the checkbox marker glyph for the given state.
// When UseNerdfontIcons is false, returns ASCII markers ([x] / [ ]).
// Otherwise consults the theme's [symbols.dialog.checkbox] section or uses Nerd Font defaults.
func (t Theme) SymbolDialogCheckbox(checked bool) string {
	if !t.UseNerdfontIcons {
		if checked {
			return "[x]"
		}
		return "[ ]"
	}
	if checked {
		return t.dialogSymbol(SymbolKeyDialogCheckboxChecked, "\U000F0856")
	}
	return t.dialogSymbol(SymbolKeyDialogCheckboxUnchecked, "\U000F0131")
}

// SymbolDialogRadio returns the radio marker glyph for the given state.
// When UseNerdfontIcons is false, returns ASCII markers ((*) / ( )).
// Otherwise consults the theme's [symbols.dialog.radio] section or uses Nerd Font defaults.
func (t Theme) SymbolDialogRadio(selected bool) string {
	if !t.UseNerdfontIcons {
		if selected {
			return "(*)"
		}
		return "( )"
	}
	if selected {
		return t.dialogSymbol(SymbolKeyDialogRadioSelected, "\U000F043E")
	}
	return t.dialogSymbol(SymbolKeyDialogRadioUnselected, "\U000F043D")
}

func (t Theme) dialogSymbol(key, fallback string) string {
	if t.Symbols != nil {
		if s := strings.TrimSpace(t.Symbols[key]); s != "" {
			return s
		}
	}
	return fallback
}
