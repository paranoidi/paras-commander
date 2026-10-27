package theme

const IconKeySearchIcon = "search.icon"

// IconSearchIcon returns the leading icon painted inside scrollquery-based filter/search
// input rows (find, history, path picker, help, mass-rename pattern picker, run-for-each
// history picker, SFTP connect, F3 style picker).
// When UseNerdfontIcons is false, returns the ASCII "/" marker.
// Otherwise consults the theme's [icons.search] section or uses the Nerd Font search icon.
func (t Theme) IconSearchIcon() string {
	if !t.UseNerdfontIcons {
		return "/"
	}
	return t.iconOr(IconKeySearchIcon, "\uF002")
}
