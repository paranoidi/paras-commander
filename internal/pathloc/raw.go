package pathloc

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Panels, dialogs and handlers keep locations as strings that are host paths or scheme URIs
// (sftp://, any fileindex form). The functions here are the path/filepath operations for such a
// string: path/filepath itself would collapse the "//" of a URI ("fileindex:///a" becomes
// "fileindex:/a"). They do no I/O, and on a host path they are path/filepath, behind a prefix
// check that allocates nothing. Use them, not path/filepath, on any string that may be a
// location; path/filepath stays right for a string that is a disk path by construction (a
// config file, a temp file, a path already mapped by HostOf).

// Clean is filepath.Clean of a host path, or the canonical form of a URI (as given when it does
// not parse). It does not resolve a relative path, see Canonical.
func Clean(raw string) string {
	if strings.HasPrefix(raw, "sftp://") {
		return cleanURI(raw)
	}
	prefix, host, _, ok := cutIndex(raw)
	switch {
	case !ok:
		return filepath.Clean(raw)
	case prefix != FileIndexPrefix:
		return cleanURI(raw)
	case !filepath.IsAbs(host):
		return raw
	}
	return cleanPlainIndex(raw, host)
}

// cleanURI is the canonical form of a URI, raw itself when it does not parse.
func cleanURI(raw string) string {
	if p, err := Parse(raw); err == nil {
		return p.String()
	}
	return raw
}

// cleanPlainIndex is raw, a plain fileindex path with the absolute host path host, with the host
// path cleaned. It is raw itself when that is already clean.
func cleanPlainIndex(raw, host string) string {
	if c := filepath.Clean(host); c != host {
		return FileIndexPrefix + c
	}
	return raw
}

// Canonical is the absolute canonical form of a location, as Parse gives it: a host path
// resolved against the working directory and cleaned, a URI in its canonical form. It is "" for
// a blank or unparseable string, so it also keys maps (history, recalled cursors, selections).
func Canonical(raw string) string {
	if filepath.IsAbs(raw) {
		return filepath.Clean(raw)
	}
	if prefix, host, _, ok := cutIndex(raw); ok && prefix == FileIndexPrefix {
		if !filepath.IsAbs(host) {
			return ""
		}
		return cleanPlainIndex(raw, host)
	}
	p, err := Parse(raw)
	if err != nil {
		return ""
	}
	return p.String()
}

// HostOf is the real local path raw names, cleaned: a host path itself (a relative one resolved
// as Canonical does), a fileindex path's host path. False for a path that has none (sftp://, a
// blank or unparseable string).
func HostOf(raw string) (string, bool) {
	if filepath.IsAbs(raw) {
		return filepath.Clean(raw), true
	}
	if prefix, host, _, ok := cutIndex(raw); ok && prefix == FileIndexPrefix {
		if !filepath.IsAbs(host) {
			return "", false
		}
		return filepath.Clean(host), true
	}
	p, err := Parse(raw)
	if err != nil {
		return "", false
	}
	return p.HostPath()
}

// DiskPath is the cleaned disk path raw names: a host path itself, a fileindex path's host path.
// Any other URI has none and comes back as Clean gives it. Unlike HostOf it never resolves a
// relative path, and it always returns something to open.
func DiskPath(raw string) string {
	if _, host, ok := CutIndexPrefix(raw); ok {
		return filepath.Clean(host)
	}
	return Clean(raw)
}

// Dir is filepath.Dir of a host path, or the directory a URI's host path lies in under the same
// prefix (Path.Parent: a flat tag view's row leads to the directory its host path is in, not back
// to the list). The root is its own Dir. A URI that does not parse comes back as given.
func Dir(raw string) string {
	if !HasURIPrefix(raw) {
		return filepath.Dir(raw)
	}
	p, err := Parse(raw)
	if err != nil {
		return raw
	}
	return p.Parent().String()
}

// Base is filepath.Base of a host path, or the final element of a URI's host path ("/" for the
// root, where filepath.Base would give "fileindex:" or the SFTP host).
func Base(raw string) string {
	if !HasURIPrefix(raw) {
		return filepath.Base(raw)
	}
	p, err := Parse(raw)
	if err != nil {
		return raw
	}
	return p.Base()
}

// Join is filepath.Join of a host path and elems, or the canonical URI of the location the
// elements lead to from raw ("" and "." elements are skipped, ".." goes up as Dir does). A URI
// that does not parse comes back as given.
func Join(raw string, elem ...string) string {
	if !HasURIPrefix(raw) {
		return filepath.Join(append([]string{raw}, elem...)...)
	}
	p, err := Parse(raw)
	if err != nil {
		return raw
	}
	for _, e := range elem {
		for _, seg := range strings.Split(e, "/") {
			switch seg {
			case "", ".":
			case "..":
				p = p.Parent()
			default:
				if q, err := p.Join(seg); err == nil {
					p = q
				}
			}
		}
	}
	return p.String()
}

// Rel is filepath.Rel of two host paths. Two URIs of the same scheme and prefix (the same tag
// query, the same SFTP host) give the relative path of their host paths; any other mix is an
// error.
func Rel(base, target string) (string, error) {
	if !HasURIPrefix(base) && !HasURIPrefix(target) {
		return filepath.Rel(base, target)
	}
	errRel := fmt.Errorf("pathloc: cannot make %q relative to %q", target, base)
	b, berr := Parse(base)
	t, terr := Parse(target)
	if berr != nil || terr != nil || b.Scheme() != t.Scheme() {
		return "", errRel
	}
	switch b.Scheme() {
	case SchemeFileIndex:
		bp, _, _, _ := b.indexParts()
		tp, _, _, _ := t.indexParts()
		if bp == tp {
			return filepath.Rel(b.indexHost(), t.indexHost())
		}
	case SchemeSFTP:
		bh, br, berr := splitSFTP(b.s)
		th, tr, terr := splitSFTP(t.s)
		if berr == nil && terr == nil && bh == th {
			return filepath.Rel(br, tr)
		}
	}
	return "", errRel
}
