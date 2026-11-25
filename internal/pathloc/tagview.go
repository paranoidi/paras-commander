package pathloc

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
)

// A tag view is a fileindex path with a tag query before the "://":
//
//	fileindex(a&b):///host/path   structure mode: entries carrying all of the tags
//	fileindex@(a|b):///host/path  flat mode: a list of the entries carrying any of the tags
//
// Flat mode marks where a list row ends with "//": fileindex@(a):///root/row//rest/inside. The
// host path is the row and the rest joined. Clean host paths never hold "//", so going up from a
// row can lead back to the list (Up) while Parent stays true to the host path.

// TagListRefusal is what an action that would work on the directory of a flat tag view's list says
// instead: the list names no real directory.
const TagListRefusal = "This is a list of tagged entries: select something first"

const (
	indexScheme  = "fileindex"
	flatBoundary = "//"
)

// TagQuery selects index entries by their own tags.
type TagQuery struct {
	// Flat lists the matches as one list (path "@(...)"); else they keep their directory structure.
	Flat bool
	// Any matches entries with at least one of Tags; else all of them. A single tag is never Any.
	Any bool
	// Tags are the unescaped tag names, sorted and unique.
	Tags []string
}

// Text names the tags of q the way the query reads: "beach & holiday", "beach | holiday".
func (q TagQuery) Text() string {
	sep := " & "
	if q.Any {
		sep = " | "
	}
	return strings.Join(q.Tags, sep)
}

// TagView returns the tag view of q at host, an absolute host path ("/" for the top). In a flat
// view host may carry the "//" row boundary. It builds the path the way Parse does.
func TagView(q TagQuery, host string) (Path, error) {
	q, err := q.normalized()
	if err != nil {
		return Path{}, err
	}
	return parseFileIndex(q.prefix() + host)
}

// TagQuery returns the query of a tag view path; false for any other path.
func (p Path) TagQuery() (TagQuery, bool) {
	if p.Scheme() != SchemeFileIndex {
		return TagQuery{}, false
	}
	prefix, _, flat, _ := cutIndex(p.s)
	if prefix == FileIndexPrefix {
		return TagQuery{}, false
	}
	text, ok := queryText(prefix)
	if !ok {
		return TagQuery{}, false
	}
	q, err := parseTagQuery(text, flat)
	return q, err == nil
}

// IsTagList reports whether p is the list of a flat tag view (not a row or anything inside one).
func (p Path) IsTagList() bool {
	if p.Scheme() != SchemeFileIndex {
		return false
	}
	prefix, host, flat, _ := cutIndex(p.s)
	return flat && host == "/" && prefix != FileIndexPrefix
}

// CutIndexPrefix splits any fileindex path form (plain, structure or flat) into its prefix, which
// ends with "://", and the host path. For a flat path the "//" row boundary is collapsed, so host
// is the host path the path names. An empty host of a tag view is "/". ok is false for any other
// string, including a local file name like "fileindex(x)". It is the one recogniser of the prefix
// and does not allocate for a string that does not start with "fileindex".
func CutIndexPrefix(raw string) (prefix, host string, ok bool) {
	prefix, host, flat, ok := cutIndex(raw)
	if !ok {
		return "", "", false
	}
	if prefix != FileIndexPrefix {
		if host == "" {
			host = "/"
		}
		if i := strings.Index(host, flatBoundary); flat && i >= 0 {
			host = host[:i] + host[i+1:]
		}
	}
	return prefix, host, true
}

// cutIndex is CutIndexPrefix without the collapse: host is the raw text after the prefix, flat is
// whether the prefix has the "@" mark.
func cutIndex(raw string) (prefix, host string, flat, ok bool) {
	if !strings.HasPrefix(raw, indexScheme) {
		return "", "", false, false
	}
	rest := raw[len(indexScheme):]
	if strings.HasPrefix(rest, "@") {
		flat, rest = true, rest[1:]
	}
	end := len(raw) - len(rest)
	switch {
	case strings.HasPrefix(rest, "://"):
		end += len("://")
	case strings.HasPrefix(rest, "("):
		i := strings.Index(rest, ")://")
		if i < 0 {
			return "", "", false, false
		}
		end += i + len(")://")
	default:
		return "", "", false, false
	}
	return raw[:end], raw[end:], flat, true
}

// queryText is the text between the parentheses of a prefix; false when it has none.
func queryText(prefix string) (string, bool) {
	rest := strings.TrimPrefix(prefix[len(indexScheme):], "@")
	if !strings.HasPrefix(rest, "(") {
		return "", false
	}
	return rest[1 : len(rest)-len(")://")], true
}

// parseTagQuery reads the text of a prefix's parentheses: tags joined by "&" (all) or "|" (any).
func parseTagQuery(text string, flat bool) (TagQuery, error) {
	if text == "" {
		return TagQuery{}, errors.New("tag query is empty")
	}
	sep := "&"
	hasAnd, hasOr := strings.Contains(text, "&"), strings.Contains(text, "|")
	if hasAnd && hasOr {
		return TagQuery{}, fmt.Errorf("tag query %q mixes & and |", text)
	}
	if hasOr {
		sep = "|"
	}
	q := TagQuery{Flat: flat, Any: hasOr}
	for _, enc := range strings.Split(text, sep) {
		tag, err := url.PathUnescape(enc)
		if err != nil {
			return TagQuery{}, fmt.Errorf("tag %q: %w", enc, err)
		}
		q.Tags = append(q.Tags, tag)
	}
	return q.normalized()
}

// normalized returns q with sorted unique tags (Tags is not shared with q) and no Any on a single tag.
func (q TagQuery) normalized() (TagQuery, error) {
	tags := slices.Clone(q.Tags)
	if len(tags) == 0 {
		return TagQuery{}, errors.New("tag query has no tags")
	}
	if slices.Contains(tags, "") {
		return TagQuery{}, errors.New("tag query has an empty tag")
	}
	slices.Sort(tags)
	q.Tags = slices.Compact(tags)
	q.Any = q.Any && len(q.Tags) > 1
	return q, nil
}

// prefix is the canonical path prefix of a normalized q.
func (q TagQuery) prefix() string {
	sep := "&"
	if q.Any {
		sep = "|"
	}
	enc := make([]string, len(q.Tags))
	for i, tag := range q.Tags {
		enc[i] = escapeTag(tag)
	}
	at := ""
	if q.Flat {
		at = "@"
	}
	return indexScheme + at + "(" + strings.Join(enc, sep) + ")://"
}

// escapeTag writes the characters a path uses as syntax (and the space) as %XX.
func escapeTag(tag string) string {
	const syntax = "%/:()&| "
	if !strings.ContainsAny(tag, syntax) {
		return tag
	}
	var b strings.Builder
	for i := 0; i < len(tag); i++ {
		if c := tag[i]; strings.IndexByte(syntax, c) >= 0 {
			fmt.Fprintf(&b, "%%%02X", c)
		} else {
			b.WriteByte(c)
		}
	}
	return b.String()
}

// parseFileIndex canonicalises raw (any fileindex form) into a path.
func parseFileIndex(raw string) (Path, error) {
	prefix, host, flat, ok := cutIndex(raw)
	if !ok {
		return Path{}, fmt.Errorf("not a fileindex path: %q", raw)
	}
	if prefix != FileIndexPrefix {
		text, hasQuery := queryText(prefix)
		if !hasQuery {
			return Path{}, fmt.Errorf("fileindex path %q marks a flat view but has no tag query", raw)
		}
		q, err := parseTagQuery(text, flat)
		if err != nil {
			return Path{}, fmt.Errorf("fileindex path %q: %w", raw, err)
		}
		prefix = q.prefix()
		if host == "" {
			host = "/"
		}
	}
	if !filepath.IsAbs(host) {
		return Path{}, fmt.Errorf("fileindex path must be absolute: %q", raw)
	}
	if i := strings.Index(host, flatBoundary); flat && i >= 0 {
		row, rest := host[:i], host[i+len(flatBoundary):]
		return newFlatPath(prefix, row, rest), nil
	}
	return Path{scheme: SchemeFileIndex, s: prefix + filepath.Clean(host)}, nil
}

// newFlatPath joins a list row and the relative rest inside it, cleaning both. With no rest the
// boundary goes; a rest below the list root is just the row it names.
func newFlatPath(prefix, row, rest string) Path {
	row = filepath.Clean("/" + row)
	rest = strings.TrimPrefix(filepath.Clean("/"+rest), "/")
	switch {
	case rest == "":
		return Path{scheme: SchemeFileIndex, s: prefix + row}
	case row == "/":
		return Path{scheme: SchemeFileIndex, s: prefix + "/" + rest}
	}
	return Path{scheme: SchemeFileIndex, s: prefix + row + flatBoundary + rest}
}

// indexParts splits a fileindex path into its prefix, the list row (the whole host path outside
// a flat view) and the rest inside the row.
func (p Path) indexParts() (prefix, row, rest string, flat bool) {
	prefix, host, flat, _ := cutIndex(p.s)
	if i := strings.Index(host, flatBoundary); flat && i >= 0 {
		return prefix, host[:i], host[i+len(flatBoundary):], true
	}
	return prefix, host, "", flat
}

// indexHost is the host path of a SchemeFileIndex location.
func (p Path) indexHost() string {
	_, host, ok := CutIndexPrefix(p.s)
	if !ok {
		return p.s
	}
	return host
}

// indexParent is Parent of a fileindex path: the host path's directory under the same prefix.
func (p Path) indexParent() Path {
	prefix, row, rest, _ := p.indexParts()
	if rest != "" {
		if dir := filepath.Dir(rest); dir != "." {
			return Path{scheme: SchemeFileIndex, s: prefix + row + flatBoundary + dir}
		}
		return Path{scheme: SchemeFileIndex, s: prefix + row}
	}
	parent := filepath.Dir(row)
	if parent == row {
		return p
	}
	return Path{scheme: SchemeFileIndex, s: prefix + parent}
}

// indexJoin is Join of a fileindex path; name is a trimmed, valid element.
func (p Path) indexJoin(name string) (Path, error) {
	prefix, row, rest, flat := p.indexParts()
	if !flat {
		return parseFileIndex(prefix + filepath.Join(row, name))
	}
	switch {
	case rest != "":
		return parseFileIndex(prefix + row + flatBoundary + filepath.Join(rest, name))
	case row == "/":
		return parseFileIndex(prefix + "/" + name)
	}
	return parseFileIndex(prefix + row + flatBoundary + name)
}

// Up is where going up from p leads in a panel: Parent, except in a flat tag view, where a row
// leads back to the list, and the rest inside a row leads up inside it.
func (p Path) Up() Path {
	if p.Scheme() != SchemeFileIndex {
		return p.Parent()
	}
	prefix, row, rest, flat := p.indexParts()
	if !flat || rest != "" {
		return p.indexParent()
	}
	if row == "/" {
		return p
	}
	return Path{scheme: SchemeFileIndex, s: prefix + "/"}
}

// Sibling is the entry called name beside p: Parent().Join(name), except that beside a row of a
// flat tag view it is a row too.
func (p Path) Sibling(name string) (Path, error) {
	if up := p.Up(); up.IsTagList() && !p.IsTagList() {
		prefix, row, _, _ := p.indexParts()
		beside, err := Path{scheme: SchemeFileIndex, s: FileIndexPrefix + filepath.Dir(row)}.Join(name)
		if err != nil {
			return Path{}, err
		}
		return parseFileIndex(prefix + beside.indexHost())
	}
	return p.Parent().Join(name)
}

// indexHasPrefix is HasPrefix for two fileindex paths: the same prefix, and the flat list holds
// everything of its view, otherwise the host paths decide.
func (p Path) indexHasPrefix(of Path) bool {
	pPrefix, _, _, _ := p.indexParts()
	ofPrefix, _, _, _ := of.indexParts()
	if pPrefix != ofPrefix {
		return false
	}
	if of.IsTagList() {
		return true
	}
	return hasFilePrefix(p.indexHost(), of.indexHost())
}
