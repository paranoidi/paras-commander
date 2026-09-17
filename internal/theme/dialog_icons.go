package theme

import "strings"

const (
	IconKeyDialogCheckboxChecked   = "dialog.checkbox.checked"
	IconKeyDialogCheckboxUnchecked = "dialog.checkbox.unchecked"
	IconKeyDialogRadioSelected     = "dialog.radio.selected"
	IconKeyDialogRadioUnselected   = "dialog.radio.unselected"
)

// IconDialogCheckbox returns the checkbox marker icon for the given state.
// When UseNerdfontIcons is false, returns ASCII markers ([x] / [ ]).
// Otherwise consults the theme's [icons.dialog.checkbox] section or uses Nerd Font defaults.
func (t Theme) IconDialogCheckbox(checked bool) string {
	if !t.UseNerdfontIcons {
		if checked {
			return "[x]"
		}
		return "[ ]"
	}
	if checked {
		return t.dialogIcon(IconKeyDialogCheckboxChecked, "\U000F0856")
	}
	return t.dialogIcon(IconKeyDialogCheckboxUnchecked, "\U000F0131")
}

// IconDialogRadio returns the radio marker icon for the given state.
// When UseNerdfontIcons is false, returns ASCII markers ((*) / ( )).
// Otherwise consults the theme's [icons.dialog.radio] section or uses Nerd Font defaults.
func (t Theme) IconDialogRadio(selected bool) string {
	if !t.UseNerdfontIcons {
		if selected {
			return "(*)"
		}
		return "( )"
	}
	if selected {
		return t.dialogIcon(IconKeyDialogRadioSelected, "\U000F043E")
	}
	return t.dialogIcon(IconKeyDialogRadioUnselected, "\U000F043D")
}

func (t Theme) dialogIcon(key, fallback string) string {
	if t.Icons != nil {
		if s := strings.TrimSpace(t.Icons[key]); s != "" {
			return s
		}
	}
	return fallback
}
