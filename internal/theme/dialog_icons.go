package theme

import (
	"strings"

	"github.com/gdamore/tcell/v2"
)

const (
	IconKeyDialogCheckboxChecked   = "dialog.checkbox.checked"
	IconKeyDialogCheckboxUnchecked = "dialog.checkbox.unchecked"
	IconKeyDialogRadioSelected     = "dialog.radio.selected"
	IconKeyDialogRadioUnselected   = "dialog.radio.unselected"
	IconKeyDialogInfo              = "dialog.info"
	IconKeyDialogMissing           = "dialog.missing"
	IconKeyDialogZoxide            = "dialog.zoxide"
	IconKeyDialogHistory           = "dialog.history"

	// IconDialogInfoASCII is the info marker used when Nerd Font icons are disabled; it is
	// also the widest form, so dialog sizing reserves its width.
	IconDialogInfoASCII = "[i]"
)

// IconDialogInfo returns the info marker that prefixes dialog help hints.
// When UseNerdfontIcons is false, returns IconDialogInfoASCII.
// Otherwise consults the theme's [icons.dialog] info key or uses the Nerd Font default.
func (t Theme) IconDialogInfo() string {
	if !t.UseNerdfontIcons {
		return IconDialogInfoASCII
	}
	return t.dialogIcon(IconKeyDialogInfo, "\uf05a")
}

// IconDialogMissing returns the trailing mark painted on picker rows (history, bookmarks,
// pin) whose path no longer exists. When UseNerdfontIcons is false, returns "!".
// Otherwise consults the theme's [icons.dialog] missing key or uses the Nerd Font default.
func (t Theme) IconDialogMissing() string {
	if !t.UseNerdfontIcons {
		return "!"
	}
	return t.dialogIcon(IconKeyDialogMissing, "\U000F09E7")
}

// IconDialogHistory returns the prefix on History rows from the app's own panel history.
// When UseNerdfontIcons is false, returns "h".
func (t Theme) IconDialogHistory() string {
	if !t.UseNerdfontIcons {
		return "h"
	}
	return t.dialogIcon(IconKeyDialogHistory, "\uf1da")
}

// IconDialogZoxide returns the prefix on History rows that come from zoxide.
// When UseNerdfontIcons is false, returns "z".
func (t Theme) IconDialogZoxide() string {
	if !t.UseNerdfontIcons {
		return "z"
	}
	return t.dialogIcon(IconKeyDialogZoxide, "\uf0e7")
}

// DialogMissingIconForeground is the missing-path mark color: dialog.option.active on the
// cursor row (dialog cursor rows keep the dialog surface background), else dialog.option.invalid.
func (t Theme) DialogMissingIconForeground(cursor bool) tcell.Color {
	style := t.DialogOptionInvalidStyle()
	if cursor {
		style = t.DialogOptionRowStyle(true, false)
	}
	fg, _, _ := style.Decompose()
	return fg
}

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
