package bookmarks

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// ResolveGTKFile returns $XDG_CONFIG_HOME/gtk-3.0/bookmarks, or ~/.config/gtk-3.0/bookmarks
// when XDG_CONFIG_HOME is unset.
func ResolveGTKFile(homeDir string) (string, error) {
	cfgHome, err := configHomeDir(homeDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(cfgHome, "gtk-3.0", "bookmarks"), nil
}

func configHomeDir(homeDir string) (string, error) {
	if v := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); v != "" {
		return expandHome(v, homeDir), nil
	}
	if homeDir == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("user home for gtk bookmarks: %w", err)
		}
		homeDir = filepath.Clean(h)
	}
	return filepath.Join(homeDir, ".config"), nil
}

// ParseGTKLine parses one GTK 3 bookmarks file line (file:// URI, optional label after a space).
func ParseGTKLine(raw string) (Mark, bool) {
	line := strings.TrimRightFunc(strings.TrimSuffix(raw, "\r"), unicode.IsSpace)
	if line == "" {
		return Mark{}, false
	}
	uri, label, _ := strings.Cut(line, " ")
	uri = strings.TrimSpace(uri)
	label = strings.TrimSpace(label)
	path, ok := fileURIPath(uri)
	if !ok {
		return Mark{}, false
	}
	name := label
	if name == "" {
		name = filepath.Base(path)
		if name == "" || name == "." || name == string(filepath.Separator) {
			name = "root"
		}
	}
	return Mark{
		Name: name,
		Path: path,
		Line: name + lineDelimiter + path,
	}, true
}

func fileURIPath(uri string) (string, bool) {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return "", false
	}
	p := u.Path
	if p == "" {
		return "", false
	}
	if unescaped, err := url.PathUnescape(p); err == nil {
		p = unescaped
	}
	return filepath.Clean(filepath.FromSlash(p)), true
}

// LoadGTK reads a GTK 3 bookmarks file (read-only consumers; the app does not write this file).
func LoadGTK(path string) ([]Mark, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return ParseGTKReader(f)
}

// ParseGTKReader parses GTK 3 bookmark lines from r.
func ParseGTKReader(r io.Reader) ([]Mark, error) {
	return parseLines(r, ParseGTKLine)
}
