package bookmarks

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

const (
	lineDelimiter = " : "
	envMarksFile  = "FZF_MARKS_FILE"
)

// Origin identifies where a bookmark was loaded from.
type Origin int

const (
	OriginFZFMarks Origin = iota
	OriginGTK
)

// PathPickerSource returns the path-picker Source column label for this origin.
func (o Origin) PathPickerSource() string {
	switch o {
	case OriginGTK:
		return "gnome"
	default:
		return "fzf-marks"
	}
}

// Mark is one fzf-marks entry: name, path, and the canonical line for display/serialization.
type Mark struct {
	Name   string
	Path   string
	Line   string
	Origin Origin
}

// ParseLine parses a single fzf-marks line. It returns false if the line should be skipped
// (empty, comment-only, or missing a delimiter).
func ParseLine(raw string) (Mark, bool) {
	// Trim trailing whitespace only: a leading space before ` : ` is valid for an unlabeled path (` : /abs/path`).
	line := strings.TrimRightFunc(strings.TrimSuffix(raw, "\r"), unicode.IsSpace)
	if line == "" || strings.HasPrefix(strings.TrimLeftFunc(line, unicode.IsSpace), "#") {
		return Mark{}, false
	}
	name, path, ok := strings.Cut(line, lineDelimiter)
	if !ok {
		return Mark{}, false
	}
	name = strings.TrimSpace(name)
	path = strings.TrimSpace(path)
	if path == "" {
		return Mark{}, false
	}
	return Mark{
		Name:   name,
		Path:   path,
		Line:   name + lineDelimiter + path,
		Origin: OriginFZFMarks,
	}, true
}

// Load reads an fzf-marks file and returns parsed marks in file order.
func Load(path string) ([]Mark, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return ParseReader(f)
}

// ParseReader parses marks from a reader (line-based).
func ParseReader(r io.Reader) ([]Mark, error) {
	return parseLines(r, ParseLine)
}

func parseLines(r io.Reader, parse func(string) (Mark, bool)) ([]Mark, error) {
	var out []Mark
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		if m, ok := parse(sc.Text()); ok {
			out = append(out, m)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// LoadAll reads the primary fzf-marks file plus GTK 3 bookmarks from
// $XDG_CONFIG_HOME/gtk-3.0/bookmarks (read-only). Primary entries are listed first;
// GTK paths already present in the primary file are skipped.
func LoadAll(cfgFile, homeDir string) ([]Mark, error) {
	primaryPath, err := ResolveFile(cfgFile, homeDir)
	if err != nil {
		return nil, err
	}
	primary, err := Load(primaryPath)
	if err != nil {
		return nil, err
	}
	gtkPath, err := ResolveGTKFile(homeDir)
	if err != nil {
		return nil, err
	}
	gtkMarks, err := LoadGTK(gtkPath)
	if err != nil {
		return nil, err
	}
	return mergeByPath(primary, gtkMarks), nil
}

func mergeByPath(primary, extra []Mark) []Mark {
	if len(extra) == 0 {
		return primary
	}
	seen := make(map[string]struct{}, len(primary))
	for _, m := range primary {
		seen[filepath.Clean(m.Path)] = struct{}{}
	}
	out := append([]Mark(nil), primary...)
	for _, m := range extra {
		cp := filepath.Clean(m.Path)
		if _, ok := seen[cp]; ok {
			continue
		}
		seen[cp] = struct{}{}
		out = append(out, m)
	}
	return out
}

// ResolveFile returns the path to the marks file: cfgFile if non-empty, else
// FZF_MARKS_FILE, else ~/.fzf-marks. Paths are cleaned; a leading ~ is expanded when home is non-empty.
func ResolveFile(cfgFile, homeDir string) (string, error) {
	if p := strings.TrimSpace(cfgFile); p != "" {
		return expandHome(strings.TrimSpace(p), homeDir), nil
	}
	if env := strings.TrimSpace(os.Getenv(envMarksFile)); env != "" {
		return expandHome(env, homeDir), nil
	}
	if homeDir == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("user home for default fzf-marks: %w", err)
		}
		homeDir = filepath.Clean(h)
	}
	return filepath.Join(homeDir, ".fzf-marks"), nil
}

func expandHome(p, home string) string {
	p = filepath.Clean(p)
	if home == "" {
		return p
	}
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	return p
}

// Append appends a canonical mark line to path using an atomic replace (read-modify-write).
func Append(path string, m Mark) error {
	line := strings.TrimSpace(m.Line)
	if line == "" {
		line = strings.TrimSpace(m.Name) + lineDelimiter + strings.TrimSpace(m.Path)
	}
	if strings.TrimSpace(m.Name) == "" || strings.TrimSpace(m.Path) == "" {
		return fmt.Errorf("bookmark name and path are required")
	}

	var base []byte
	if b, err := os.ReadFile(path); err == nil {
		base = b
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	var sb strings.Builder
	if len(base) > 0 {
		sb.Write(base)
		if base[len(base)-1] != '\n' {
			sb.WriteByte('\n')
		}
	}
	sb.WriteString(line)
	sb.WriteByte('\n')
	return atomicWriteFile(path, []byte(sb.String()), 0o644)
}

func atomicWriteFile(dest string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(dest)
	temp, err := os.CreateTemp(dir, "."+filepath.Base(dest)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := temp.Name()
	_, writeErr := temp.Write(data)
	syncErr := temp.Sync()
	closeErr := temp.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		_ = os.Remove(tmpPath)
		return errors.Join(writeErr, syncErr, closeErr)
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}
