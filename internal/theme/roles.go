package theme

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// ansiNames are the only [palette] keys, in ANSI index order: a missing key falls back to its own
// index, so the palette is the terminal's own 16 colors unless a theme overrides it.
var ansiNames = [16]string{
	"black", "red", "green", "yellow", "blue", "magenta", "cyan", "white",
	"bright_black", "bright_red", "bright_green", "bright_yellow",
	"bright_blue", "bright_magenta", "bright_cyan", "bright_white",
}

// roleDef is one [roles] entry: the intent a style names and the [palette] name it takes when a
// theme leaves it out. This table is the single source of truth for the role set; themes/default.toml
// lists every role explicitly and a test keeps it equal to this table.
type roleDef struct{ name, palette string }

// roleDefs are the 32 roles in presentation order (backgrounds, text, structure, states).
var roleDefs = []roleDef{
	// Backgrounds.
	{"bg", "default"},
	{"surface", "bright_black"},
	{"header_bg", "blue"},
	{"menu_bg", "blue"},
	{"focus_bg", "cyan"},
	{"file_cursor_bg", "cyan"},
	{"file_cursor_dim", "black"},
	{"highlight_bg", "bright_blue"},
	{"range_bg", "bright_black"},
	{"range_unselect_bg", "red"},
	{"inverse_bg", "white"},
	// Text.
	{"fg", "white"},
	{"fg_strong", "bright_white"},
	{"muted", "bright_black"},
	{"faint", "black"},
	{"ink", "black"},
	{"focus_fg", "black"},
	{"file_cursor_fg", "black"},
	// Structure.
	{"accent", "magenta"},
	{"primary", "blue"},
	{"directory", "blue"},
	{"symlink", "bright_cyan"},
	{"selected", "yellow"},
	// States.
	{"info", "cyan"},
	{"success", "green"},
	{"success_strong", "bright_green"},
	{"warning", "yellow"},
	{"attention", "bright_yellow"},
	{"danger", "red"},
	{"error", "bright_red"},
	{"special", "magenta"},
}

// RoleDefaults returns the role -> default [palette] name table (a copy).
func RoleDefaults() map[string]string {
	out := make(map[string]string, len(roleDefs))
	for _, r := range roleDefs {
		out[r.name] = r.palette
	}
	return out
}

func roleNameList() string {
	names := make([]string, len(roleDefs))
	for i, r := range roleDefs {
		names[i] = r.name
	}
	return strings.Join(names, ", ")
}

// paletteField reads the merged [palette] table: only the 16 ANSI names are allowed; missing names
// keep their ANSI index.
func paletteField(table map[string]any) (map[string]tcell.Color, error) {
	palette := make(map[string]tcell.Color, len(ansiNames))
	for i, name := range ansiNames {
		palette[name] = tcell.PaletteColor(i)
	}
	for name, rawValue := range table {
		if _, known := palette[name]; !known {
			return nil, fmt.Errorf("palette.%s: unknown color name; [palette] holds only %s", name, strings.Join(ansiNames[:], ", "))
		}
		color, err := parsePaletteEntry(rawValue)
		if err != nil {
			return nil, fmt.Errorf("palette.%s: %w", name, err)
		}
		palette[name] = color
	}
	return palette, nil
}

// rolesField reads the merged [roles] table into a complete role table: each value is a [palette]
// name, "#RRGGBB" or "default"; missing roles take their roleDefs palette name.
func rolesField(table map[string]any, palette map[string]tcell.Color) (map[string]tcell.Color, error) {
	roles := make(map[string]tcell.Color, len(roleDefs))
	for _, r := range roleDefs {
		roles[r.name] = palette[r.palette] // "default" is no palette name: the zero colour is tcell.ColorDefault
	}
	for name, rawValue := range table {
		if _, known := roles[name]; !known {
			return nil, fmt.Errorf("roles.%s: unknown role; use one of %s", name, roleNameList())
		}
		text, ok := rawValue.(string)
		if !ok {
			return nil, fmt.Errorf("roles.%s: must be a [palette] name, \"#RRGGBB\" or \"default\"", name)
		}
		text = strings.TrimSpace(text)
		switch {
		case strings.EqualFold(text, "default"):
			roles[name] = tcell.ColorDefault
		case strings.HasPrefix(text, "#"):
			color, err := ParseHexColor(text)
			if err != nil {
				return nil, fmt.Errorf("roles.%s: %w", name, err)
			}
			roles[name] = color
		default:
			color, ok := palette[text]
			if !ok {
				return nil, fmt.Errorf("roles.%s: %q is not a [palette] name, \"#RRGGBB\" or \"default\"", name, text)
			}
			roles[name] = color
		}
	}
	return roles, nil
}

// tableField is the top-level table raw[key] (nil when the file has none).
func tableField(raw map[string]any, key string) (map[string]any, error) {
	value, ok := raw[key]
	if !ok {
		return nil, nil
	}
	table, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be a table", key)
	}
	return table, nil
}

// resolveRole resolves the style field (fg, bg or icon) value, which must name a role.
func resolveRole(field, value string, roles map[string]tcell.Color) (tcell.Color, error) {
	if color, ok := roles[value]; ok {
		return color, nil
	}
	return tcell.ColorDefault, fmt.Errorf("%s %q is not a role; use one of %s", field, value, roleNameList())
}
