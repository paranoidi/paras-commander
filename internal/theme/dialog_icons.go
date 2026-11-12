package theme

import (
	"github.com/gdamore/tcell/v2"
)

const (
	IconKeyDialogCheckboxChecked   = "dialog.checkbox.checked"
	IconKeyDialogCheckboxUnchecked = "dialog.checkbox.unchecked"
	IconKeyDialogCheckboxMixed     = "dialog.checkbox.mixed"
	IconKeyDialogRadioSelected     = "dialog.radio.selected"
	IconKeyDialogRadioUnselected   = "dialog.radio.unselected"
	IconKeyDialogInfo              = "dialog.info"
	IconKeyDialogMissing           = "dialog.missing"
	IconKeyDialogZoxide            = "dialog.zoxide"
	IconKeyDialogHistory           = "dialog.history"
	IconKeyDialogDepth             = "dialog.depth"
	IconKeyDialogCriteria          = "dialog.criteria"
	IconKeyMessageSuccess          = "message.success"
	IconKeyDialogButtonStart       = "dialog.button.start"
	IconKeyDialogButtonEnd         = "dialog.button.end"

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
	return t.icon(IconKeyDialogInfo)
}

// IconMessageSuccess returns the prefix of success toasts from [icons.message] success, or ""
// when UseNerdfontIcons is false (success messages then carry no prefix).
func (t Theme) IconMessageSuccess() string {
	if !t.UseNerdfontIcons {
		return ""
	}
	return t.icon(IconKeyMessageSuccess)
}

// IconDialogMissing returns the trailing mark painted on picker rows (history, bookmarks,
// pin) whose path no longer exists. When UseNerdfontIcons is false, returns "!".
// Otherwise consults the theme's [icons.dialog] missing key or uses the Nerd Font default.
func (t Theme) IconDialogMissing() string {
	if !t.UseNerdfontIcons {
		return "!"
	}
	return t.icon(IconKeyDialogMissing)
}

// IconDialogHistory returns the prefix on History rows from the app's own panel history.
// When UseNerdfontIcons is false, returns "h".
func (t Theme) IconDialogHistory() string {
	if !t.UseNerdfontIcons {
		return "h"
	}
	return t.icon(IconKeyDialogHistory)
}

// IconDialogZoxide returns the prefix on History rows that come from zoxide.
// When UseNerdfontIcons is false, returns "z".
func (t Theme) IconDialogZoxide() string {
	if !t.UseNerdfontIcons {
		return "z"
	}
	return t.icon(IconKeyDialogZoxide)
}

// IconDialogDepth returns the marker before the find indexing depth in the find dialog
// title and before the max-depth input. When UseNerdfontIcons is false, returns "Depth".
func (t Theme) IconDialogDepth() string {
	if !t.UseNerdfontIcons {
		return "Depth"
	}
	return t.icon(IconKeyDialogDepth)
}

// IconDialogCriteria returns the marker before the find dialog's advanced-criteria summary.
// When UseNerdfontIcons is false, returns "Criteria:".
func (t Theme) IconDialogCriteria() string {
	if !t.UseNerdfontIcons {
		return "Criteria:"
	}
	return t.icon(IconKeyDialogCriteria)
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
		return t.icon(IconKeyDialogCheckboxChecked)
	}
	return t.icon(IconKeyDialogCheckboxUnchecked)
}

// IconDialogCheckboxMixed returns the marker of a tri-state checkbox set on only some of its
// targets. When UseNerdfontIcons is false, returns the ASCII marker [-].
func (t Theme) IconDialogCheckboxMixed() string {
	if !t.UseNerdfontIcons {
		return "[-]"
	}
	return t.icon(IconKeyDialogCheckboxMixed)
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
		return t.icon(IconKeyDialogRadioSelected)
	}
	return t.icon(IconKeyDialogRadioUnselected)
}

// DialogButtonCaps returns the strings drawn before and after a dialog button's label
// ([icons.dialog.button] start / end). They are not trimmed: their spaces are part of the
// button. Not gated on UseNerdfontIcons; a theme picks glyphs its fonts have.
func (t Theme) DialogButtonCaps() (start, end string) {
	return t.Icons[IconKeyDialogButtonStart], t.Icons[IconKeyDialogButtonEnd]
}
