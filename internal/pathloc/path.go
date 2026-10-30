// Package pathloc identifies filesystem locations across host paths and remote URIs.
package pathloc

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/primitive"
)

// Scheme names a path backend.
type Scheme string

const (
	SchemeFile Scheme = "file"
	SchemeSFTP Scheme = "sftp"
	// SchemeFileIndex browses the fileindex database: fileindex:// + an absolute host path.
	SchemeFileIndex Scheme = "fileindex"
)

// FileIndexPrefix starts every SchemeFileIndex path; the host path follows, so the index
// root is "fileindex:///". The UI may show it shorter (primitive.SetPathDisplayPrefixes).
const FileIndexPrefix = "fileindex://"

// Path is a canonical location (host file path or remote URI).
type Path struct {
	scheme Scheme
	s      string
}

// Parse converts a user or stored path string into a canonical Path.
// Host paths may be relative; they are resolved to an absolute path for scheme file.
// Remote paths must use the sftp:// prefix (MC-compatible).
func Parse(raw string) (Path, error) {
	// Trailing/leading spaces are legal in file names; only reject blank input.
	if strings.TrimSpace(raw) == "" {
		return Path{}, errors.New("empty path")
	}
	if strings.HasPrefix(raw, "sftp://") {
		return parseSFTP(raw)
	}
	if strings.HasPrefix(raw, FileIndexPrefix) {
		return parseFileIndex(raw)
	}
	return parseFile(raw)
}

// MustParse parses raw or panics (tests only).
func MustParse(raw string) Path {
	p, err := Parse(raw)
	if err != nil {
		panic(err)
	}
	return p
}

// File returns a file-scheme path from a host path string (Parse on error).
func File(hostPath string) (Path, error) {
	return Parse(hostPath)
}

// FileMust is File that panics on error (tests).
func FileMust(hostPath string) Path {
	return MustParse(hostPath)
}

// Scheme returns the path backend identifier.
func (p Path) Scheme() Scheme {
	if p.scheme == "" {
		return SchemeFile
	}
	return p.scheme
}

// String returns the canonical serialized form (history, jobs, selection keys).
func (p Path) String() string {
	return p.s
}

// IsZero reports whether p is unset.
func (p Path) IsZero() bool {
	return p.s == ""
}

// IsLocal reports whether the location is served by the local file backend
// (plain host paths). SFTP and fileindex:// are not: their files go through
// their fsbackend.Backend.
func (p Path) IsLocal() bool {
	return p.Scheme() == SchemeFile
}

// Equal reports whether two paths denote the same location.
func (p Path) Equal(other Path) bool {
	return p.Scheme() == other.Scheme() && p.s == other.s
}

// HasPrefix reports whether p is the same as or a descendant of prefix (same scheme).
func (p Path) HasPrefix(prefix Path) bool {
	if p.Scheme() != prefix.Scheme() || prefix.IsZero() {
		return false
	}
	if p.Equal(prefix) {
		return true
	}
	switch p.Scheme() {
	case SchemeFile:
		return hasFilePrefix(p.s, prefix.s)
	case SchemeFileIndex:
		return hasFilePrefix(p.indexHost(), prefix.indexHost())
	case SchemeSFTP:
		return hasSFTPPrefix(p.s, prefix.s)
	default:
		return false
	}
}

// Base returns the final path element (directory or file name).
func (p Path) Base() string {
	switch p.Scheme() {
	case SchemeSFTP:
		_, remote, err := splitSFTP(p.s)
		if err != nil {
			return ""
		}
		return path.Base(remote)
	case SchemeFileIndex:
		return filepath.Base(p.indexHost())
	default:
		return filepath.Base(p.FilePathMust())
	}
}

// Parent returns the parent directory, or zero Path at the root.
func (p Path) Parent() Path {
	switch p.Scheme() {
	case SchemeSFTP:
		return sftpParent(p)
	case SchemeFileIndex:
		host := p.indexHost()
		parent := filepath.Dir(host)
		if parent == host {
			return p
		}
		return Path{scheme: SchemeFileIndex, s: FileIndexPrefix + parent}
	default:
		host := p.FilePathMust()
		parent := filepath.Dir(host)
		if parent == host {
			return p
		}
		return Path{scheme: SchemeFile, s: parent}
	}
}

// CommonAncestor returns the deepest location that is an ancestor of (or equal to)
// both a and b, walking a upward via Parent. False when none exists (mixed schemes/hosts).
func CommonAncestor(a, b Path) (Path, bool) {
	for anc := a; !anc.IsZero(); {
		if b.HasPrefix(anc) {
			return anc, true
		}
		parent := anc.Parent()
		if parent.Equal(anc) {
			break
		}
		anc = parent
	}
	return Path{}, false
}

// CommonParent folds paths into their deepest common ancestor.
// mixed is true when more than one distinct path contributed to the fold.
// ok is false when paths is empty or members mix schemes/hosts.
func CommonParent(paths []Path) (root Path, mixed bool, ok bool) {
	for _, p := range paths {
		if p.IsZero() {
			continue
		}
		switch {
		case root.IsZero():
			root = p
		case !p.Equal(root):
			mixed = true
			anc, ancOK := CommonAncestor(root, p)
			if !ancOK {
				return Path{}, false, false
			}
			root = anc
		}
	}
	return root, mixed, !root.IsZero()
}

// Join appends a single path element (name) under p.
func (p Path) Join(name string) (Path, error) {
	name = strings.Trim(name, "/")
	if name == "" || name == "." || name == ".." {
		return Path{}, fmt.Errorf("invalid path element %q", name)
	}
	switch p.Scheme() {
	case SchemeSFTP:
		return sftpJoin(p, name)
	case SchemeFileIndex:
		return FileIndex(filepath.Join(p.indexHost(), name))
	default:
		return File(filepath.Join(p.FilePathMust(), name))
	}
}

// FilePath returns the host filesystem path for file-scheme locations.
func (p Path) FilePath() (string, error) {
	if p.Scheme() != SchemeFile {
		return "", fmt.Errorf("pathloc: not a file path: %s", p.s)
	}
	return p.s, nil
}

// FilePathMust returns FilePath or panics.
func (p Path) FilePathMust() string {
	s, err := p.FilePath()
	if err != nil {
		panic(err)
	}
	return s
}

// Display truncates the canonical string for status UI (0 = no limit).
func (p Path) Display(maxRunes int) string {
	s := p.s
	runes := []rune(s)
	if maxRunes <= 0 || len(runes) <= maxRunes {
		return s
	}
	if maxRunes <= 3 {
		return string(runes[:maxRunes])
	}
	head := maxRunes / 2
	tail := maxRunes - head - 1
	return string(runes[:head]) + string(primitive.Ellipsis) + string(runes[len(runes)-tail:])
}

func parseFile(raw string) (Path, error) {
	abs, err := filepath.Abs(raw)
	if err != nil {
		return Path{}, fmt.Errorf("resolve path %q: %w", raw, err)
	}
	return Path{scheme: SchemeFile, s: filepath.Clean(abs)}, nil
}

func parseFileIndex(raw string) (Path, error) {
	host := strings.TrimPrefix(raw, FileIndexPrefix)
	if !filepath.IsAbs(host) {
		return Path{}, fmt.Errorf("fileindex path must be absolute: %q", raw)
	}
	return Path{scheme: SchemeFileIndex, s: FileIndexPrefix + filepath.Clean(host)}, nil
}

// FileIndex returns the fileindex:// location of an absolute host path.
func FileIndex(hostPath string) (Path, error) {
	return parseFileIndex(FileIndexPrefix + hostPath)
}

// HasURIPrefix reports whether raw is written as a scheme URI (sftp:// or fileindex://) rather
// than a host path, so it is parsed as given instead of resolved against a directory.
func HasURIPrefix(raw string) bool {
	return strings.HasPrefix(raw, "sftp://") || strings.HasPrefix(raw, FileIndexPrefix)
}

// IndexHostPath returns the host path a fileindex:// location names (the path the index
// records, which may no longer exist on disk).
func IndexHostPath(p Path) (string, error) {
	if p.Scheme() != SchemeFileIndex {
		return "", fmt.Errorf("pathloc: not a fileindex path: %s", p.s)
	}
	return p.indexHost(), nil
}

// indexHost is the host path of a SchemeFileIndex location.
func (p Path) indexHost() string {
	return strings.TrimPrefix(p.s, FileIndexPrefix)
}

func parseSFTP(raw string) (Path, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return Path{}, fmt.Errorf("parse sftp URI: %w", err)
	}
	if u.Scheme != "sftp" {
		return Path{}, fmt.Errorf("expected sftp scheme, got %q", u.Scheme)
	}
	if u.Host == "" {
		return Path{}, errors.New("sftp URI missing host")
	}
	var b strings.Builder
	b.WriteString("sftp://")
	if u.User != nil && u.User.Username() != "" {
		b.WriteString(u.User.Username())
		b.WriteString("@")
	}
	b.WriteString(u.Hostname())
	if port := u.Port(); port != "" {
		b.WriteString(":")
		b.WriteString(port)
	}
	remote := remoteFromURIPath(u.EscapedPath())
	b.WriteString(remoteToURIPath(remote))
	return Path{scheme: SchemeSFTP, s: b.String()}, nil
}

// remoteFromURIPath maps an SFTP URI path (from url.EscapedPath) to the internal remote segment.
// Internal form uses ~ and ~/… for home; / for filesystem root; /var for absolutes.
// Do not use path.Clean on tilde-bearing URI paths — it produces invalid segments like /~.
func remoteFromURIPath(uriPath string) string {
	if uriPath == "" || uriPath == "/" {
		return "/"
	}
	if uriPath == "/~" {
		return "~"
	}
	if strings.HasPrefix(uriPath, "/~/") {
		return strings.TrimPrefix(uriPath, "/")
	}
	return path.Clean(uriPath)
}

// remoteToURIPath maps the internal remote segment to the URI path suffix (always starts with /).
func remoteToURIPath(remote string) string {
	switch {
	case remote == "/":
		return "/"
	case remote == "~":
		return "/~"
	case strings.HasPrefix(remote, "~/"):
		return "/" + remote
	default:
		return path.Clean(remote)
	}
}

// splitSFTP parses canonical sftp://user@host:port/remote/path into host part and remote path.
func splitSFTP(canonical string) (hostPart string, remotePath string, err error) {
	if !strings.HasPrefix(canonical, "sftp://") {
		return "", "", errors.New("not an sftp path")
	}
	rest := strings.TrimPrefix(canonical, "sftp://")
	slash := strings.Index(rest, "/")
	if slash < 0 {
		hostPart = rest
		remotePath = "/"
		return hostPart, remotePath, nil
	}
	hostPart = rest[:slash]
	remotePath = rest[slash:]
	if remotePath == "" {
		remotePath = "/"
	}
	return hostPart, remoteFromURIPath(remotePath), nil
}

func hasFilePrefix(p, prefix string) bool {
	p = filepath.Clean(p)
	prefix = filepath.Clean(prefix)
	if prefix == "" || prefix == "." {
		return false
	}
	if p == prefix {
		return true
	}
	rel, err := filepath.Rel(prefix, p)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func hasSFTPPrefix(p, prefix string) bool {
	pHost, pRemote, err := splitSFTP(p)
	if err != nil {
		return false
	}
	prefixHost, prefixRemote, err := splitSFTP(prefix)
	if err != nil {
		return false
	}
	if pHost != prefixHost {
		return false
	}
	if prefixRemote == "/" {
		return true
	}
	if pRemote == prefixRemote {
		return true
	}
	return strings.HasPrefix(pRemote, prefixRemote+"/")
}

func sftpParent(p Path) Path {
	hostPart, remote, err := splitSFTP(p.s)
	if err != nil {
		return Path{}
	}
	parentRemote := path.Dir(remote)
	switch {
	case remote == "~":
		parentRemote = "/"
	case parentRemote == remote || parentRemote == ".":
		return p
	}
	var b strings.Builder
	b.WriteString("sftp://")
	b.WriteString(hostPart)
	b.WriteString(remoteToURIPath(parentRemote))
	return Path{scheme: SchemeSFTP, s: b.String()}
}

func sftpJoin(p Path, name string) (Path, error) {
	hostPart, remote, err := splitSFTP(p.s)
	if err != nil {
		return Path{}, err
	}
	joined := path.Join(remote, name)
	var b strings.Builder
	b.WriteString("sftp://")
	b.WriteString(hostPart)
	b.WriteString(remoteToURIPath(joined))
	return Path{scheme: SchemeSFTP, s: b.String()}, nil
}

// SFTPHostPart returns the user@host[:port] segment used for connection pooling.
func SFTPHostPart(p Path) (string, error) {
	if p.Scheme() != SchemeSFTP {
		return "", fmt.Errorf("pathloc: not sftp path: %s", p.s)
	}
	host, _, err := splitSFTP(p.s)
	return host, err
}

// SFTPEndpoint returns SSH user, host, port (default 22), and remote directory for loc.
func SFTPEndpoint(p Path) (user, host, port, remoteDir string, err error) {
	if p.Scheme() != SchemeSFTP {
		return "", "", "", "", fmt.Errorf("pathloc: not sftp path: %s", p.s)
	}
	u, err := url.Parse(p.s)
	if err != nil {
		return "", "", "", "", err
	}
	if u.User != nil {
		user = u.User.Username()
	}
	host = u.Hostname()
	port = u.Port()
	if port == "" {
		port = "22"
	}
	remoteDir = remoteFromURIPath(u.EscapedPath())
	return user, host, port, remoteDir, nil
}

// SFTPRemotePath returns the remote filesystem path for an SFTP location.
func SFTPRemotePath(p Path) (string, error) {
	if p.Scheme() != SchemeSFTP {
		return "", fmt.Errorf("pathloc: not sftp path: %s", p.s)
	}
	_, remote, err := splitSFTP(p.s)
	return remote, err
}
