package ops

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

// MassRenameMode selects literal vs regexp basename transform.
type MassRenameMode int

const (
	// MassRenameModeSimple replaces all occurrences of Find in each basename with Replace.
	MassRenameModeSimple MassRenameMode = iota
	// MassRenameModeRegex applies regexp.ReplaceAllString(Find as pattern, Replace as template).
	MassRenameModeRegex
)

// MassRenameRow is one source file and its computed new basename after transform.
type MassRenameRow struct {
	SourcePath string
	OldBase    string
	NewBase    string
}

func resolveEntryPath(entry localfs.Entry, panelPath string) string {
	p := entry.Path
	if !filepath.IsAbs(p) {
		p = filepath.Join(panelPath, p)
	}
	return filepath.Clean(p)
}

// MassRenameCompute applies find/replace to each entry basename and returns one row per entry.
// In simple mode, an empty find string leaves each basename unchanged.
// In regex mode, a nil regexp leaves each basename unchanged (caller omits compile for an empty pattern).
// Rows with NewBase == OldBase are no-ops (still listed for preview).
// panelPath is used to resolve non-absolute entry paths (same as PlanRename).
func MassRenameCompute(entries []localfs.Entry, panelPath string, mode MassRenameMode, find, replace string, simpleCaseFold bool, rx *regexp.Regexp) ([]MassRenameRow, error) {
	if len(entries) == 0 {
		return nil, &Error{Op: "mass-rename", Text: "no files to rename"}
	}
	out := make([]MassRenameRow, 0, len(entries))
	for _, e := range entries {
		src := resolveEntryPath(e, panelPath)
		oldBase := filepath.Base(src)
		nb, err := massRenameTransformBase(mode, oldBase, find, replace, simpleCaseFold, rx)
		if err != nil {
			return nil, err
		}
		out = append(out, MassRenameRow{SourcePath: src, OldBase: oldBase, NewBase: nb})
	}
	return out, nil
}

func massRenameTransformBase(mode MassRenameMode, oldBase, find, replace string, simpleCaseFold bool, rx *regexp.Regexp) (string, error) {
	switch mode {
	case MassRenameModeSimple:
		if find == "" {
			return oldBase, nil
		}
		if simpleCaseFold {
			return replaceAllFold(oldBase, find, replace), nil
		}
		return strings.ReplaceAll(oldBase, find, replace), nil
	case MassRenameModeRegex:
		if rx == nil {
			return oldBase, nil
		}
		return rx.ReplaceAllString(oldBase, replace), nil
	default:
		return "", &Error{Op: "mass-rename", Text: "unknown mass rename mode"}
	}
}

func foldEqualRunes(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if unicode.ToLower(a[i]) != unicode.ToLower(b[i]) {
			return false
		}
	}
	return true
}

func indexFold(haystack, needle []rune) int {
	if len(needle) == 0 {
		return 0
	}
	last := len(haystack) - len(needle)
	if last < 0 {
		return -1
	}
	for i := 0; i <= last; i++ {
		if foldEqualRunes(haystack[i:i+len(needle)], needle) {
			return i
		}
	}
	return -1
}

func replaceAllFold(s, old, repl string) string {
	h := []rune(s)
	n := []rune(old)
	if len(n) == 0 {
		return s
	}
	var b strings.Builder
	pos := 0
	for {
		rel := indexFold(h[pos:], n)
		if rel < 0 {
			b.WriteString(string(h[pos:]))
			break
		}
		abs := pos + rel
		b.WriteString(string(h[pos:abs]))
		b.WriteString(repl)
		pos = abs + len(n)
	}
	return b.String()
}

// MassRenameCompileRegex compiles the pattern for MassRenameModeRegex.
func MassRenameCompileRegex(pattern string) (*regexp.Regexp, error) {
	if strings.TrimSpace(pattern) == "" {
		return nil, &Error{Op: "mass-rename", Text: "regexp pattern is empty"}
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, &Error{Op: "mass-rename", Text: "invalid regexp", Err: err}
	}
	return re, nil
}

const regexpParseErrPrefix = "error parsing regexp: "

// MassRenameRegexCompileUserMessage returns a concise regexp compile reason for UI
// (strips the standard "error parsing regexp: " prefix when present).
func MassRenameRegexCompileUserMessage(err error) string {
	if err == nil {
		return ""
	}
	var opErr *Error
	if errors.As(err, &opErr) {
		if opErr.Err != nil {
			return trimRegexpParseDetail(opErr.Err)
		}
		if s := strings.TrimSpace(opErr.Text); s != "" {
			return s
		}
	}
	return strings.TrimSpace(err.Error())
}

func trimRegexpParseDetail(err error) string {
	s := strings.TrimSpace(err.Error())
	if strings.HasPrefix(s, regexpParseErrPrefix) {
		return strings.TrimSpace(s[len(regexpParseErrPrefix):])
	}
	return s
}

// MassRenameValidateRows checks duplicate targets, illegal names, and conflicts with paths
// outside the batch. rows must come from MassRenameCompute with the same panelPath resolution.
func MassRenameValidateRows(rows []MassRenameRow) error {
	if len(rows) == 0 {
		return nil
	}
	dir := filepath.Dir(rows[0].SourcePath)
	for _, r := range rows {
		if filepath.Dir(r.SourcePath) != dir {
			return &Error{Op: "mass-rename", Text: "all selected files must be in the same directory"}
		}
	}
	sourceSet := make(map[string]struct{}, len(rows))
	for _, r := range rows {
		sourceSet[r.SourcePath] = struct{}{}
	}
	// Duplicate new basename?
	seenNew := make(map[string]string, len(rows))
	for _, r := range rows {
		if r.NewBase == r.OldBase {
			continue
		}
		if r.NewBase == "" {
			return &Error{Op: "mass-rename", Text: fmt.Sprintf("resulting name is empty for %q", r.OldBase)}
		}
		if filepath.Base(r.NewBase) != r.NewBase || strings.ContainsRune(r.NewBase, filepath.Separator) {
			return &Error{Op: "mass-rename", Text: fmt.Sprintf("resulting name must be a single path segment: %q", r.NewBase)}
		}
		// Reject NUL etc. (single-segment check already covers path separators).
		if !utf8.ValidString(r.NewBase) {
			return &Error{Op: "mass-rename", Text: fmt.Sprintf("resulting name is not valid UTF-8: %q", r.NewBase)}
		}
		if prev, ok := seenNew[r.NewBase]; ok {
			return &Error{Op: "mass-rename", Text: fmt.Sprintf("duplicate target name %q (from %q and %q)", r.NewBase, prev, r.OldBase)}
		}
		seenNew[r.NewBase] = r.OldBase
	}
	for _, r := range rows {
		if r.NewBase == r.OldBase {
			continue
		}
		dst := filepath.Join(dir, r.NewBase)
		if fi, err := os.Lstat(dst); err == nil {
			if fi.IsDir() {
				return &Error{Op: "mass-rename", Text: fmt.Sprintf("target %q is a directory", r.NewBase)}
			}
			if _, ok := sourceSet[dst]; !ok {
				return &Error{Op: "mass-rename", Text: fmt.Sprintf("target already exists: %q", r.NewBase)}
			}
		} else if !os.IsNotExist(err) {
			return &Error{Op: "mass-rename", Text: fmt.Sprintf("cannot stat %q", r.NewBase), Err: err}
		}
	}
	return nil
}

// MassRenameFindMatchesAny reports whether find (simple) or rx (regex) matches at least one row basename.
// An empty simple find or nil regexp matches all rows (identity preview; no error state).
func MassRenameFindMatchesAny(rows []MassRenameRow, mode MassRenameMode, find string, simpleCaseFold bool, rx *regexp.Regexp) bool {
	switch mode {
	case MassRenameModeSimple:
		if find == "" {
			return true
		}
		for _, r := range rows {
			if massRenameSimpleFindMatches(r.OldBase, find, simpleCaseFold) {
				return true
			}
		}
		return false
	case MassRenameModeRegex:
		if rx == nil {
			return true
		}
		for _, r := range rows {
			if rx.MatchString(r.OldBase) {
				return true
			}
		}
		return false
	default:
		return true
	}
}

func massRenameSimpleFindMatches(oldBase, find string, caseFold bool) bool {
	if caseFold {
		return indexFold([]rune(oldBase), []rune(find)) >= 0
	}
	return strings.Contains(oldBase, find)
}

// MassRenameHasWork returns true if at least one row would change its basename.
func MassRenameHasWork(rows []MassRenameRow) bool {
	for _, r := range rows {
		if r.NewBase != r.OldBase {
			return true
		}
	}
	return false
}

// ExecuteMassRename applies validated rows using a two-phase temp rename so swaps and chains work.
// Caller must pass rows that passed MassRenameValidateRows.
func ExecuteMassRename(rows []MassRenameRow) error {
	type pair struct {
		src     string
		newBase string
	}
	var work []pair
	for _, r := range rows {
		if r.NewBase != r.OldBase {
			work = append(work, pair{src: r.SourcePath, newBase: r.NewBase})
		}
	}
	if len(work) == 0 {
		return nil
	}
	dir := filepath.Dir(work[0].src)
	used := make(map[string]struct{}, len(rows)*3)
	for _, r := range rows {
		used[r.SourcePath] = struct{}{}
	}
	temps := make([]string, len(work))
	for i := range work {
		t, err := pickMassRenameTemp(dir, used)
		if err != nil {
			return err
		}
		temps[i] = t
		used[t] = struct{}{}
	}
	for i := range work {
		if err := localfs.Rename(work[i].src, temps[i]); err != nil {
			for j := i - 1; j >= 0; j-- {
				_ = localfs.Rename(temps[j], work[j].src)
			}
			return fmt.Errorf("mass rename stage1 %q: %w", work[i].src, err)
		}
	}
	for i := range work {
		dst := filepath.Join(dir, work[i].newBase)
		if err := localfs.Rename(temps[i], dst); err != nil {
			_ = localfs.Rename(temps[i], work[i].src)
			for j := i - 1; j >= 0; j-- {
				prevDst := filepath.Join(dir, work[j].newBase)
				_ = localfs.Rename(prevDst, work[j].src)
			}
			return fmt.Errorf("mass rename stage2 %q -> %q: %w", temps[i], dst, err)
		}
	}
	return nil
}

func pickMassRenameTemp(dir string, avoid map[string]struct{}) (string, error) {
	for i := 0; i < 1_000_000; i++ {
		p := filepath.Join(dir, fmt.Sprintf(".paras-massrename-%d", i))
		if _, ok := avoid[p]; ok {
			continue
		}
		_, err := os.Lstat(p)
		if os.IsNotExist(err) {
			return p, nil
		}
		if err != nil {
			return "", &Error{Op: "mass-rename", Text: fmt.Sprintf("stat temp %q", p), Err: err}
		}
	}
	return "", &Error{Op: "mass-rename", Text: "could not allocate a temporary rename name"}
}
