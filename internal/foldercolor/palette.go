package foldercolor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// colorsDir holds folder-color-switcher's palette files; a var so tests can point it at a fixture.
var colorsDir = "/usr/share/folder-color-switcher/colors.d"

// Choice is one selectable folder colour of an icon-theme style.
type Choice struct {
	Name  string // e.g. "Red"
	Theme string // icon theme directory, e.g. "Mint-Y-Red"
}

type palette struct {
	themeHex map[string]string   // icon theme dir -> hex
	nameHex  map[string]string   // lower-case colour name -> hex (first style defining it wins)
	styles   map[string][]Choice // style name ("Mint-Y") -> colours
}

// paletteCache re-parses colors.d only when the directory's mtime changes (package installs
// replace or add files there), so a reload normally costs one stat.
type paletteCache struct {
	mtime int64 // 0 = colors.d unreadable at the last load: re-parse next time
	pal   *palette
}

func (c *paletteCache) load() *palette {
	var mt int64
	if fi, err := os.Stat(colorsDir); err == nil {
		mt = fi.ModTime().UnixNano()
	}
	if c.pal == nil || mt == 0 || mt != c.mtime {
		c.pal, c.mtime = loadPalette(), mt
	}
	return c.pal
}

func loadPalette() *palette {
	p := &palette{themeHex: map[string]string{}, nameHex: map[string]string{}, styles: map[string][]Choice{}}
	files, _ := filepath.Glob(filepath.Join(colorsDir, "*.json"))
	slices.Sort(files)
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var doc struct {
			Styles []struct {
				Name   string `json:"name"`
				Themes []struct {
					Name  string `json:"name"`
					Color string `json:"color"`
					Theme string `json:"theme"`
				} `json:"icon-themes"`
			} `json:"styles"`
		}
		if json.Unmarshal(data, &doc) != nil {
			continue
		}
		for _, st := range doc.Styles {
			for _, t := range st.Themes {
				p.styles[st.Name] = append(p.styles[st.Name], Choice{Name: t.Name, Theme: t.Theme})
				p.themeHex[t.Theme] = t.Color
				if _, ok := p.nameHex[strings.ToLower(t.Name)]; !ok {
					p.nameHex[strings.ToLower(t.Name)] = t.Color
				}
			}
		}
	}
	return p
}

// color resolves a ref to its colour: the icon URI's theme directory wins over the icon name.
func (p *palette) color(ref iconRef) (tcell.Color, bool) {
	hex := ""
	if _, rest, ok := strings.Cut(ref.icon, "/icons/"); ok {
		theme, _, _ := strings.Cut(rest, "/")
		hex = p.themeHex[theme]
	} else if c, ok := strings.CutPrefix(ref.name, "folder-"); ok {
		hex = p.nameHex[strings.ToLower(c)]
	}
	if hex == "" {
		return tcell.ColorDefault, false
	}
	c := tcell.GetColor(hex)
	return c, c != tcell.ColorDefault
}

// styleFor maps an icon theme such as "Mint-Y-Sand" to its palette style ("Mint-Y"): the longest
// style name that is the theme or its prefix, else Mint-Y, else any style.
func (p *palette) styleFor(theme string) string {
	best := ""
	for name := range p.styles {
		if (theme == name || strings.HasPrefix(theme, name+"-")) && len(name) > len(best) {
			best = name
		}
	}
	if best == "" {
		if _, ok := p.styles["Mint-Y"]; ok {
			return "Mint-Y"
		}
		for name := range p.styles {
			return name
		}
	}
	return best
}
