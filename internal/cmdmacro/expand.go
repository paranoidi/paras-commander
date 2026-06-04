package cmdmacro

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// PanelSnapshot holds panel fields needed for macro expansion (avoids importing panel).
type PanelSnapshot struct {
	Dir         string
	CurrentName string
	HasCurrent  bool
	TaggedInDir []string // absolute paths tagged in Dir
}

// Context carries state for macro expansion.
type Context struct {
	Active    *PanelSnapshot
	Other     *PanelSnapshot
	FOverride string // run-for-each iterated absolute path
	RowPath   string // meta row absolute path
}

// ExpandCommandLine substitutes %% %f %F %d %D %t %T into template.
// Path and name values are shell-quoted for downstream ParseCommandArgv.
func ExpandCommandLine(template string, ctx Context) (string, error) {
	if ctx.RowPath == "" && ctx.Active == nil {
		return "", fmt.Errorf("cmdmacro: no expansion context")
	}
	var b strings.Builder
	for i := 0; i < len(template); i++ {
		if template[i] != '%' || i+1 >= len(template) {
			b.WriteByte(template[i])
			continue
		}
		switch template[i+1] {
		case '%':
			b.WriteByte('%')
			i++
		case 'f':
			v, err := expandF(ctx)
			if err != nil {
				return "", err
			}
			b.WriteString(strconv.Quote(v))
			i++
		case 'F':
			if ctx.Active == nil {
				return "", fmt.Errorf("cmdmacro: %%F: no active panel")
			}
			if ctx.Other == nil || !ctx.Other.HasCurrent {
				return "", fmt.Errorf("cmdmacro: %%F: no current file on other panel")
			}
			b.WriteString(strconv.Quote(ctx.Other.CurrentName))
			i++
		case 'd':
			if ctx.Active == nil {
				return "", fmt.Errorf("cmdmacro: %%d: no active panel")
			}
			b.WriteString(strconv.Quote(filepath.Clean(ctx.Active.Dir)))
			i++
		case 'D':
			if ctx.Active == nil {
				return "", fmt.Errorf("cmdmacro: %%D: no active panel")
			}
			if ctx.Other == nil {
				return "", fmt.Errorf("cmdmacro: %%D: no other panel")
			}
			b.WriteString(strconv.Quote(filepath.Clean(ctx.Other.Dir)))
			i++
		case 't':
			if ctx.Active == nil {
				return "", fmt.Errorf("cmdmacro: %%t: no active panel")
			}
			s, err := quotedTagged(ctx.Active)
			if err != nil {
				return "", err
			}
			b.WriteString(s)
			i++
		case 'T':
			if ctx.Active == nil {
				return "", fmt.Errorf("cmdmacro: %%T: no active panel")
			}
			if ctx.Other == nil {
				return "", fmt.Errorf("cmdmacro: %%T: no other panel")
			}
			s, err := quotedTagged(ctx.Other)
			if err != nil {
				return "", err
			}
			b.WriteString(s)
			i++
		default:
			b.WriteByte('%')
		}
	}
	return b.String(), nil
}

func expandF(ctx Context) (string, error) {
	if strings.TrimSpace(ctx.RowPath) != "" {
		return ctx.RowPath, nil
	}
	if strings.TrimSpace(ctx.FOverride) != "" {
		return ctx.FOverride, nil
	}
	if ctx.Active == nil || !ctx.Active.HasCurrent {
		return "", fmt.Errorf("cmdmacro: %%f: no current file")
	}
	return ctx.Active.CurrentName, nil
}

func quotedTagged(ps *PanelSnapshot) (string, error) {
	if ps == nil || len(ps.TaggedInDir) == 0 {
		return "", fmt.Errorf("cmdmacro: %%t: no tagged files in current directory")
	}
	paths := append([]string(nil), ps.TaggedInDir...)
	sort.Strings(paths)
	var b strings.Builder
	for i, p := range paths {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(strconv.Quote(p))
	}
	return b.String(), nil
}

// CommandRequiresMacro reports whether template contains macro %<letter> (not %%).
func CommandRequiresMacro(template string, letter byte) bool {
	for i := 0; i < len(template)-1; i++ {
		if template[i] != '%' {
			continue
		}
		if template[i+1] == '%' {
			i++
			continue
		}
		if template[i+1] == letter {
			return true
		}
	}
	return false
}

// ErrRunForEachRequiresF is returned when a run-for-each command omits %f.
const ErrRunForEachRequiresF = "command must include %f to represent the selected item"
