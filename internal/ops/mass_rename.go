package ops

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/search"
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

// MassRenameRowErrors returns per-row validation errors aligned with rows. A nil entry means the
// row is valid. rows must come from MassRenameCompute with the same panelPath resolution.
func MassRenameRowErrors(rows []MassRenameRow) []error {
	out := make([]error, len(rows))
	if len(rows) == 0 {
		return out
	}
	dir := filepath.Dir(rows[0].SourcePath)
	mixedDir := &Error{Op: "mass-rename", Text: "all selected files must be in the same directory"}
	for i, r := range rows {
		if filepath.Dir(r.SourcePath) != dir {
			out[i] = mixedDir
		}
	}
	if mixedDirErr := firstMassRenameRowError(out); mixedDirErr != nil {
		for i := range out {
			if out[i] == nil {
				out[i] = mixedDirErr
			}
		}
		return out
	}
	sourceSet := make(map[string]struct{}, len(rows))
	for _, r := range rows {
		sourceSet[r.SourcePath] = struct{}{}
	}
	newToIndices := make(map[string][]int, len(rows))
	for i, r := range rows {
		if r.NewBase == r.OldBase {
			continue
		}
		if r.NewBase == "" {
			out[i] = &Error{Op: "mass-rename", Text: fmt.Sprintf("resulting name is empty for %q", r.OldBase)}
			continue
		}
		if filepath.Base(r.NewBase) != r.NewBase || strings.ContainsRune(r.NewBase, filepath.Separator) {
			out[i] = &Error{Op: "mass-rename", Text: fmt.Sprintf("resulting name must be a single path segment: %q", r.NewBase)}
			continue
		}
		if !utf8.ValidString(r.NewBase) {
			out[i] = &Error{Op: "mass-rename", Text: fmt.Sprintf("resulting name is not valid UTF-8: %q", r.NewBase)}
			continue
		}
		newToIndices[r.NewBase] = append(newToIndices[r.NewBase], i)
	}
	for newBase, indices := range newToIndices {
		if len(indices) < 2 {
			continue
		}
		prev := rows[indices[0]].OldBase
		dup := &Error{Op: "mass-rename", Text: fmt.Sprintf("duplicate target name %q (from %q and %q)", newBase, prev, rows[indices[1]].OldBase)}
		for _, i := range indices {
			massRenameSetRowError(out, i, dup)
		}
	}
	for i, r := range rows {
		if r.NewBase == r.OldBase || out[i] != nil {
			continue
		}
		dst := filepath.Join(dir, r.NewBase)
		if fi, err := os.Lstat(dst); err == nil {
			if fi.IsDir() {
				massRenameSetRowError(out, i, &Error{Op: "mass-rename", Text: fmt.Sprintf("target %q already exists", r.NewBase)})
				continue
			}
			if _, ok := sourceSet[dst]; !ok {
				massRenameSetRowError(out, i, &Error{Op: "mass-rename", Text: fmt.Sprintf("target %q already exists", r.NewBase)})
			}
		} else if !os.IsNotExist(err) {
			massRenameSetRowError(out, i, &Error{Op: "mass-rename", Text: fmt.Sprintf("cannot stat %q", r.NewBase), Err: err})
		}
	}
	return out
}

func massRenameSetRowError(out []error, i int, err error) {
	if out[i] == nil {
		out[i] = err
	}
}

func firstMassRenameRowError(errs []error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// MassRenameValidateRows checks duplicate targets, illegal names, and conflicts with paths
// outside the batch. rows must come from MassRenameCompute with the same panelPath resolution.
func MassRenameValidateRows(rows []MassRenameRow) error {
	return firstMassRenameRowError(MassRenameRowErrors(rows))
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

// MassRenameBeforePreviewHighlightRanges splits before-column preview highlights.
// When replace is empty, find/regex match ranges use before.removed (red); otherwise matches
// use before.replaced (yellow) and lcsRemoved continues to use before.removed for other deletions.
func MassRenameBeforePreviewHighlightRanges(lcsRemoved, matchRanges []search.Range, replace string) (removed, replaced []search.Range) {
	if replace == "" {
		return massRenameMergeRanges(append(append([]search.Range(nil), lcsRemoved...), matchRanges...)), nil
	}
	return lcsRemoved, matchRanges
}

func massRenameMergeRanges(ranges []search.Range) []search.Range {
	if len(ranges) == 0 {
		return nil
	}
	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i].Start == ranges[j].Start {
			return ranges[i].End < ranges[j].End
		}
		return ranges[i].Start < ranges[j].Start
	})
	merged := []search.Range{ranges[0]}
	for _, current := range ranges[1:] {
		last := &merged[len(merged)-1]
		if current.Start <= last.End {
			if current.End > last.End {
				last.End = current.End
			}
			continue
		}
		merged = append(merged, current)
	}
	return merged
}

// MassRenameMatchRanges returns rune-index half-open ranges in oldBase matched by find (simple) or rx (regex).
func MassRenameMatchRanges(oldBase string, mode MassRenameMode, find string, simpleCaseFold bool, rx *regexp.Regexp) []search.Range {
	switch mode {
	case MassRenameModeSimple:
		if find == "" {
			return nil
		}
		return massRenameSimpleFindRanges(oldBase, find, simpleCaseFold)
	case MassRenameModeRegex:
		if rx == nil {
			return nil
		}
		return massRenameRegexFindRanges(oldBase, rx)
	default:
		return nil
	}
}

func massRenameSimpleFindRanges(oldBase, find string, caseFold bool) []search.Range {
	h := []rune(oldBase)
	n := []rune(find)
	if len(n) == 0 {
		return nil
	}
	var ranges []search.Range
	pos := 0
	for {
		var rel int
		if caseFold {
			rel = indexFold(h[pos:], n)
		} else {
			rel = indexRunes(h[pos:], n)
		}
		if rel < 0 {
			break
		}
		abs := pos + rel
		ranges = massRenameAppendRange(ranges, abs, abs+len(n))
		pos = abs + len(n)
	}
	return ranges
}

func indexRunes(haystack, needle []rune) int {
	if len(needle) == 0 {
		return 0
	}
	last := len(haystack) - len(needle)
	if last < 0 {
		return -1
	}
	for i := 0; i <= last; i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func massRenameRegexFindRanges(oldBase string, rx *regexp.Regexp) []search.Range {
	idxs := rx.FindAllStringIndex(oldBase, -1)
	if len(idxs) == 0 {
		return nil
	}
	ranges := make([]search.Range, 0, len(idxs))
	for _, pair := range idxs {
		start := utf8.RuneCountInString(oldBase[:pair[0]])
		end := utf8.RuneCountInString(oldBase[:pair[1]])
		ranges = massRenameAppendRange(ranges, start, end)
	}
	return ranges
}

func massRenameAppendRange(ranges []search.Range, start, end int) []search.Range {
	if start >= end {
		return ranges
	}
	if n := len(ranges); n > 0 && ranges[n-1].End == start {
		ranges[n-1].End = end
		return ranges
	}
	return append(ranges, search.Range{Start: start, End: end})
}

// MassRenameReplacementRanges returns rune-index half-open ranges in the transformed basename
// where replacement text was inserted (simple find/replace or expanded regex template).
func MassRenameReplacementRanges(oldBase string, mode MassRenameMode, find, replace string, simpleCaseFold bool, rx *regexp.Regexp) []search.Range {
	switch mode {
	case MassRenameModeSimple:
		if find == "" {
			return nil
		}
		return massRenameSimpleReplacementRanges(oldBase, find, replace, simpleCaseFold)
	case MassRenameModeRegex:
		if rx == nil {
			return nil
		}
		return massRenameRegexReplacementRanges(oldBase, rx, replace)
	default:
		return nil
	}
}

func massRenameSimpleReplacementRanges(oldBase, find, replace string, caseFold bool) []search.Range {
	h := []rune(oldBase)
	n := []rune(find)
	repl := []rune(replace)
	if len(n) == 0 {
		return nil
	}
	var ranges []search.Range
	pos, outPos := 0, 0
	for {
		var rel int
		if caseFold {
			rel = indexFold(h[pos:], n)
		} else {
			rel = indexRunes(h[pos:], n)
		}
		if rel < 0 {
			break
		}
		abs := pos + rel
		outPos += abs - pos
		ranges = massRenameAppendRange(ranges, outPos, outPos+len(repl))
		outPos += len(repl)
		pos = abs + len(n)
	}
	return ranges
}

func massRenameRegexReplacementRanges(oldBase string, rx *regexp.Regexp, replace string) []search.Range {
	locs := rx.FindAllStringSubmatchIndex(oldBase, -1)
	if len(locs) == 0 {
		return nil
	}
	var ranges []search.Range
	outPos := 0
	lastByte := 0
	for _, loc := range locs {
		if len(loc) < 2 {
			continue
		}
		start, end := loc[0], loc[1]
		outPos += utf8.RuneCountInString(oldBase[lastByte:start])
		repl := string(rx.ExpandString(nil, replace, oldBase, loc))
		replRunes := utf8.RuneCountInString(repl)
		ranges = massRenameAppendRange(ranges, outPos, outPos+replRunes)
		outPos += replRunes
		lastByte = end
	}
	return ranges
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
