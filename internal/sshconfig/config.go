// Package sshconfig parses OpenSSH client configuration for SFTP host selection.
package sshconfig

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

// HostEntry is one connectable Host stanza from ssh_config.
type HostEntry struct {
	Alias          string
	HostName       string
	User           string
	Port           string
	IdentityFiles  []string
	IdentitiesOnly string // "yes", "no", or empty (unset)
	IdentityAgent  string
}

// Config holds parsed host entries from a config file.
type Config struct {
	Path    string
	Entries []HostEntry
}

// DefaultPath returns ~/.ssh/config when home is known.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ssh", "config"), nil
}

// ResolvePath picks the OpenSSH client config file to read.
// Precedence: configured path (non-empty) → SSH_CONFIG env → ~/.ssh/config.
func ResolvePath(configured string) (string, error) {
	home, _ := os.UserHomeDir()
	configured = strings.TrimSpace(configured)
	candidate := configured
	if candidate == "" {
		if env := strings.TrimSpace(os.Getenv("SSH_CONFIG")); env != "" {
			candidate = env
		} else {
			return DefaultPath()
		}
	}
	expanded, err := expandConfigPath(candidate, home)
	if err != nil {
		return "", err
	}
	return filepath.Clean(expanded), nil
}

// Load reads path (empty uses ResolvePath). Missing file yields empty Config, not an error.
func Load(path string) (Config, error) {
	resolved, err := ResolvePath(path)
	if err != nil {
		return Config{}, err
	}
	path = resolved
	home, _ := os.UserHomeDir()
	if expanded, err := expandConfigPath(path, home); err == nil {
		path = expanded
	}
	entries, err := loadEntriesFromFile(path, home)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{Path: path}, nil
		}
		return Config{}, fmt.Errorf("read ssh config %q: %w", path, err)
	}
	return Config{Path: path, Entries: entries}, nil
}

func loadEntriesFromFile(path, home string) ([]HostEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content, err := expandIncludes(string(data), filepath.Dir(path), home, 0)
	if err != nil {
		return nil, err
	}
	return parse(content, home)
}

// EntryFor returns the merged Host stanza matching user@host (alias or HostName).
// Port is ignored so URIs with the default port still match stanzas that set Port.
// All matching stanzas are merged with OpenSSH first-wins per keyword.
func (c Config) EntryFor(user, host string) (HostEntry, bool) {
	return c.mergedEntryForHost(user, host)
}

// ResolveEndpoint merges OpenSSH settings into user@host:port from an SFTP URI.
// host may be a Host alias; the returned host is the TCP/SSH dial target (HostName when set).
func (c Config) ResolveEndpoint(user, host, port string) (string, string, string) {
	if port == "" {
		port = "22"
	}
	e, ok := c.mergedEntryForHost(user, host)
	if !ok {
		return user, host, port
	}
	if hn := strings.TrimSpace(e.HostName); hn != "" {
		host = hn
	}
	if u := strings.TrimSpace(e.User); u != "" && strings.TrimSpace(user) == "" {
		user = u
	}
	if p := strings.TrimSpace(e.Port); p != "" && (port == "" || port == "22") {
		port = p
	}
	return user, host, port
}

// IdentityFilesFor returns IdentityFile paths for user@host (alias or HostName).
func (c Config) IdentityFilesFor(user, host, port string) []string {
	_ = port
	e, ok := c.mergedEntryForHost(user, host)
	if !ok {
		return nil
	}
	return append([]string(nil), e.IdentityFiles...)
}

// IdentityFilesForConnect returns IdentityFile paths for an SFTP dial.
// connectHost is the host from the URI (may be a Host alias); resolvedHost is the
// TCP target after HostName merge. Both are consulted so keys match OpenSSH whether
// the user typed an alias or the resolved address.
func (c Config) IdentityFilesForConnect(user, connectHost, resolvedHost, port string) []string {
	paths := c.IdentityFilesFor(user, connectHost, port)
	if resolvedHost == connectHost {
		return paths
	}
	return appendIdentityPathsUnique(paths, c.IdentityFilesFor(user, resolvedHost, port))
}

// ConnectAuthOptions holds ssh_config keywords used when loading SSH signers.
type ConnectAuthOptions struct {
	IdentityFiles  []string
	IdentitiesOnly bool
	IdentityAgent  string
}

// ConnectAuthOptionsFor returns merged auth options for an SFTP dial (paths expanded).
func (c Config) ConnectAuthOptionsFor(user, connectHost, resolvedHost, port string) ConnectAuthOptions {
	merged, matched, _ := c.mergedEntryForHostDetails(user, connectHost)
	if !matched && resolvedHost != connectHost {
		merged, matched, _ = c.mergedEntryForHostDetails(user, resolvedHost)
	}
	paths := c.IdentityFilesForConnect(user, connectHost, resolvedHost, port)
	home, _ := os.UserHomeDir()
	localUser := localSSHUsername()
	localHost, _ := os.Hostname()
	remoteUser := strings.TrimSpace(user)
	if remoteUser == "" && matched {
		remoteUser = strings.TrimSpace(merged.User)
	}
	ctx := IdentityTokenContext{
		Home:       home,
		LocalUser:  localUser,
		RemoteUser: remoteUser,
		DestHost:   strings.TrimSpace(resolvedHost),
		LocalHost:  localHost,
	}
	if ctx.DestHost == "" {
		ctx.DestHost = strings.TrimSpace(connectHost)
	}
	if !matched {
		return ConnectAuthOptions{IdentityFiles: expandIdentityFilePaths(paths, ctx)}
	}
	agentPath := strings.TrimSpace(merged.IdentityAgent)
	if agentPath != "" {
		if expanded, err := expandConfigPath(agentPath, home); err == nil {
			agentPath = expanded
		}
	}
	return ConnectAuthOptions{
		IdentityFiles:  expandIdentityFilePaths(paths, ctx),
		IdentitiesOnly: strings.EqualFold(strings.TrimSpace(merged.IdentitiesOnly), "yes"),
		IdentityAgent:  agentPath,
	}
}

func appendIdentityPathsUnique(dst, extra []string) []string {
	for _, path := range extra {
		dst = appendIdentityUnique(dst, path)
	}
	return dst
}

func (c Config) mergedEntryForHost(user, host string) (HostEntry, bool) {
	merged, matched, _ := c.mergedEntryForHostDetails(user, host)
	if !matched {
		return HostEntry{}, false
	}
	return merged, true
}

func (c Config) mergedEntryForHostDetails(user, host string) (merged HostEntry, matched bool, matchCount int) {
	for _, e := range c.Entries {
		if !e.MatchesHost(user, host) {
			continue
		}
		matched = true
		matchCount++
		mergeHostEntryFirstWins(&merged, e)
	}
	if !matched {
		return HostEntry{}, false, 0
	}
	if strings.TrimSpace(merged.Alias) == "" {
		merged.Alias = strings.TrimSpace(host)
	}
	return merged, true, matchCount
}

func mergeHostEntryFirstWins(dst *HostEntry, src HostEntry) {
	if strings.TrimSpace(dst.Alias) == "" && strings.TrimSpace(src.Alias) != "" {
		dst.Alias = strings.TrimSpace(src.Alias)
	}
	if strings.TrimSpace(dst.HostName) == "" && strings.TrimSpace(src.HostName) != "" {
		dst.HostName = strings.TrimSpace(src.HostName)
	}
	if strings.TrimSpace(dst.User) == "" && strings.TrimSpace(src.User) != "" {
		dst.User = strings.TrimSpace(src.User)
	}
	if strings.TrimSpace(dst.Port) == "" && strings.TrimSpace(src.Port) != "" {
		dst.Port = strings.TrimSpace(src.Port)
	}
	if strings.TrimSpace(dst.IdentitiesOnly) == "" && strings.TrimSpace(src.IdentitiesOnly) != "" {
		dst.IdentitiesOnly = strings.TrimSpace(src.IdentitiesOnly)
	}
	if strings.TrimSpace(dst.IdentityAgent) == "" && strings.TrimSpace(src.IdentityAgent) != "" {
		dst.IdentityAgent = strings.TrimSpace(src.IdentityAgent)
	}
	for _, path := range src.IdentityFiles {
		dst.IdentityFiles = appendIdentityUnique(dst.IdentityFiles, path)
	}
}

// ResolvedHost returns HostName or Alias when HostName is unset.
func (e HostEntry) ResolvedHost() string {
	if strings.TrimSpace(e.HostName) != "" {
		return strings.TrimSpace(e.HostName)
	}
	return strings.TrimSpace(e.Alias)
}

// MatchesHost reports whether entry applies to user@host (Host pattern, alias, or HostName).
func (e HostEntry) MatchesHost(user, host string) bool {
	host = strings.TrimSpace(host)
	if host == "" {
		return false
	}
	matched := hostMatchesPattern(e.Alias, host)
	if !matched {
		rh := e.ResolvedHost()
		matched = hostEqual(rh, host)
	}
	if !matched {
		return false
	}
	if u := strings.TrimSpace(e.User); u != "" && strings.TrimSpace(user) != "" && !hostEqual(u, user) {
		return false
	}
	return true
}

// MatchesEndpoint reports whether entry applies to user@host:port.
func (e HostEntry) MatchesEndpoint(user, host, port string) bool {
	if port == "" {
		port = "22"
	}
	host = strings.TrimSpace(host)
	matched := hostMatchesPattern(e.Alias, host)
	if !matched {
		rh := e.ResolvedHost()
		matched = hostEqual(rh, host)
	}
	if !matched {
		return false
	}
	if p := strings.TrimSpace(e.Port); p != "" && p != port {
		return false
	}
	if u := strings.TrimSpace(e.User); u != "" && u != user {
		return false
	}
	return true
}

// BuildSFTPURI builds an sftp:// URI for this entry (remoteDir defaults to ~).
func (e HostEntry) BuildSFTPURI(remoteDir string) (string, error) {
	host := e.ResolvedHost()
	if host == "" {
		return "", fmt.Errorf("ssh config host %q: missing HostName", e.Alias)
	}
	user := strings.TrimSpace(e.User)
	port := strings.TrimSpace(e.Port)
	if remoteDir == "" {
		remoteDir = "~"
	}
	// "/~" is the URI path suffix for remote home (sftp://host/~), not an on-server path.
	if remoteDir == "~" {
		remoteDir = "/~"
	} else if !strings.HasPrefix(remoteDir, "/") {
		remoteDir = "/" + remoteDir
	}
	var b strings.Builder
	b.WriteString("sftp://")
	if user != "" {
		b.WriteString(user)
		b.WriteString("@")
	}
	b.WriteString(host)
	if port != "" && port != "22" {
		b.WriteString(":")
		b.WriteString(port)
	}
	b.WriteString(remoteDir)
	return b.String(), nil
}

// FormatHostListLines builds aligned list rows: Host alias column, then dial target (HostName with optional user/port).
func FormatHostListLines(entries []HostEntry) []string {
	if len(entries) == 0 {
		return nil
	}
	aliasPad := 0
	for _, e := range entries {
		if w := utf8.RuneCountInString(strings.TrimSpace(e.Alias)); w > aliasPad {
			aliasPad = w
		}
	}
	lines := make([]string, len(entries))
	for i, e := range entries {
		lines[i] = e.formatHostListRow(aliasPad)
	}
	return lines
}

func (e HostEntry) formatHostListRow(aliasPad int) string {
	alias := strings.TrimSpace(e.Alias)
	target := e.listTargetLabel()
	if alias == "" {
		return target
	}
	col := padRunesRight(alias, aliasPad)
	if utf8.RuneCountInString(alias) < aliasPad {
		col += " "
	}
	return col + target
}

func (e HostEntry) listTargetLabel() string {
	host := e.ResolvedHost()
	user := strings.TrimSpace(e.User)
	port := strings.TrimSpace(e.Port)
	switch {
	case user != "" && port != "" && port != "22":
		return fmt.Sprintf("%s@%s:%s", user, host, port)
	case user != "":
		return fmt.Sprintf("%s@%s", user, host)
	case port != "" && port != "22":
		return net.JoinHostPort(host, port)
	default:
		return host
	}
}

func padRunesRight(s string, minWidth int) string {
	r := []rune(s)
	if len(r) >= minWidth {
		return s + " "
	}
	return s + string(make([]rune, minWidth-len(r)))
}

type stanza struct {
	patterns       []string
	hostName       string
	user           string
	port           string
	identityFiles  []string
	identitiesOnly string
	identityAgent  string
}

func parse(content string, home string) ([]HostEntry, error) {
	var stanzas []stanza
	var cur *stanza

	flush := func() {
		if cur == nil || len(cur.patterns) == 0 {
			cur = nil
			return
		}
		stanzas = append(stanzas, *cur)
		cur = nil
	}

	sc := bufio.NewScanner(strings.NewReader(content))
	for lineNo := 1; sc.Scan(); lineNo++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val := splitConfigKeyword(line)
		if key == "" {
			continue
		}
		if key == "host" || key == "match" {
			flush()
			if key == "match" {
				matchFields := strings.Fields(val)
				if len(matchFields) < 2 || strings.ToLower(matchFields[0]) != "host" {
					continue
				}
				val = strings.TrimSpace(strings.Join(matchFields[1:], " "))
			}
			var patterns []string
			for _, p := range strings.Fields(val) {
				p = strings.TrimSpace(p)
				if p == "" || strings.HasPrefix(p, "!") {
					continue
				}
				patterns = append(patterns, p)
			}
			if len(patterns) > 0 {
				cur = &stanza{patterns: patterns}
			}
			continue
		}
		if cur == nil {
			continue
		}
		switch key {
		case "hostname":
			cur.hostName = val
		case "user":
			cur.user = val
		case "port":
			cur.port = val
		case "identityfile":
			if val = strings.TrimSpace(val); val != "" {
				cur.identityFiles = append(cur.identityFiles, val)
			}
		case "identitiesonly":
			cur.identitiesOnly = strings.TrimSpace(val)
		case "identityagent":
			cur.identityAgent = strings.TrimSpace(val)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	flush()

	return stanzasToEntries(stanzas), nil
}

// splitConfigKeyword splits an ssh_config assignment line into keyword and argument.
// OpenSSH allows "Keyword value" and "Keyword=value".
func splitConfigKeyword(line string) (key, value string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", ""
	}
	keyEnd := 0
	for keyEnd < len(line) && line[keyEnd] != ' ' && line[keyEnd] != '\t' && line[keyEnd] != '=' {
		keyEnd++
	}
	key = strings.ToLower(line[:keyEnd])
	value = strings.TrimSpace(line[keyEnd:])
	if strings.HasPrefix(value, "=") {
		value = strings.TrimSpace(value[1:])
	}
	value = unquoteSSHConfigValue(value)
	return key, value
}

func unquoteSSHConfigValue(val string) string {
	val = strings.TrimSpace(val)
	if len(val) < 2 {
		return val
	}
	if val[0] == '"' && val[len(val)-1] == '"' {
		return val[1 : len(val)-1]
	}
	if val[0] == '\'' && val[len(val)-1] == '\'' {
		return val[1 : len(val)-1]
	}
	return val
}

// stanzasToEntries merges duplicate Host aliases using OpenSSH first-wins per keyword.
func stanzasToEntries(stanzas []stanza) []HostEntry {
	byAlias := make(map[string]*HostEntry)
	order := make([]string, 0, len(stanzas))
	for _, s := range stanzas {
		for _, alias := range s.patterns {
			e, ok := byAlias[alias]
			if !ok {
				e = &HostEntry{Alias: alias}
				byAlias[alias] = e
				order = append(order, alias)
			}
			mergeStanzaFirstWins(e, s)
		}
	}
	out := make([]HostEntry, len(order))
	for i, alias := range order {
		out[i] = *byAlias[alias]
	}
	return out
}

func mergeStanzaFirstWins(e *HostEntry, s stanza) {
	if strings.TrimSpace(e.HostName) == "" && strings.TrimSpace(s.hostName) != "" {
		e.HostName = strings.TrimSpace(s.hostName)
	}
	if strings.TrimSpace(e.User) == "" && strings.TrimSpace(s.user) != "" {
		e.User = strings.TrimSpace(s.user)
	}
	if strings.TrimSpace(e.Port) == "" && strings.TrimSpace(s.port) != "" {
		e.Port = strings.TrimSpace(s.port)
	}
	if strings.TrimSpace(e.IdentitiesOnly) == "" && strings.TrimSpace(s.identitiesOnly) != "" {
		e.IdentitiesOnly = strings.TrimSpace(s.identitiesOnly)
	}
	if strings.TrimSpace(e.IdentityAgent) == "" && strings.TrimSpace(s.identityAgent) != "" {
		e.IdentityAgent = strings.TrimSpace(s.identityAgent)
	}
	for _, raw := range s.identityFiles {
		raw = strings.TrimSpace(raw)
		if raw != "" {
			e.IdentityFiles = appendIdentityUnique(e.IdentityFiles, raw)
		}
	}
}

func appendIdentityUnique(files []string, path string) []string {
	for _, existing := range files {
		if existing == path {
			return files
		}
	}
	return append(files, path)
}

const maxIncludeDepth = 16

func expandIncludes(content, configDir, home string, depth int) (string, error) {
	if depth > maxIncludeDepth {
		return "", fmt.Errorf("ssh config include depth exceeded")
	}
	var out strings.Builder
	sc := bufio.NewScanner(strings.NewReader(content))
	for sc.Scan() {
		line := sc.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			out.WriteString(line)
			out.WriteByte('\n')
			continue
		}
		fields := strings.Fields(trimmed)
		key := strings.ToLower(fields[0])
		if key == "include" && len(fields) > 1 {
			pattern, err := expandConfigPath(strings.TrimSpace(strings.Join(fields[1:], " ")), home)
			if err != nil {
				return "", fmt.Errorf("include path: %w", err)
			}
			if !filepath.IsAbs(pattern) {
				pattern = filepath.Join(configDir, pattern)
			}
			matches, err := filepath.Glob(pattern)
			if err != nil {
				return "", fmt.Errorf("include glob %q: %w", pattern, err)
			}
			sort.Strings(matches)
			for _, incPath := range matches {
				info, err := os.Stat(incPath)
				if err != nil || info.IsDir() {
					continue
				}
				data, err := os.ReadFile(incPath)
				if err != nil {
					continue
				}
				expanded, err := expandIncludes(string(data), filepath.Dir(incPath), home, depth+1)
				if err != nil {
					return "", err
				}
				out.WriteString(expanded)
				if expanded != "" && !strings.HasSuffix(expanded, "\n") {
					out.WriteByte('\n')
				}
			}
			continue
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	return out.String(), nil
}

func expandConfigPath(raw, home string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty path")
	}
	if strings.HasPrefix(raw, "~/") && home != "" {
		return filepath.Clean(filepath.Join(home, raw[2:])), nil
	}
	if strings.HasPrefix(raw, "~") && home != "" {
		return filepath.Clean(filepath.Join(home, strings.TrimPrefix(raw, "~"))), nil
	}
	return filepath.Clean(raw), nil
}

func hostMatchesPattern(pattern, host string) bool {
	pattern = strings.TrimSpace(pattern)
	host = strings.TrimSpace(host)
	if pattern == "" || host == "" {
		return false
	}
	if !strings.ContainsAny(pattern, "*?[") {
		return hostEqual(pattern, host)
	}
	ok, err := filepath.Match(pattern, host)
	return err == nil && ok
}

func hostEqual(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func expandIdentityFile(raw, home string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty IdentityFile")
	}
	if strings.HasPrefix(raw, "~/") && home != "" {
		return filepath.Clean(filepath.Join(home, raw[2:])), nil
	}
	if strings.HasPrefix(raw, "~") && home != "" {
		return filepath.Clean(filepath.Join(home, strings.TrimPrefix(raw, "~"))), nil
	}
	if filepath.IsAbs(raw) {
		return filepath.Clean(raw), nil
	}
	if home != "" {
		return filepath.Clean(filepath.Join(home, ".ssh", raw)), nil
	}
	return filepath.Clean(raw), nil
}
