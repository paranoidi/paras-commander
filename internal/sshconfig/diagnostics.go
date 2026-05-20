package sshconfig

import (
	"fmt"
	"net"
	"strings"
)

// EndpointDiagnostics captures ssh_config resolution for SFTP connect error hints.
type EndpointDiagnostics struct {
	ConfigPath     string
	LoadError      string
	EntryCount     int
	URIUser        string
	URIHost        string
	URIPort        string
	Matched        bool
	MatchedStanzas int
	MatchedDetails []string
	HostNameSet    bool
	HostName       string
	ResolvedUser   string
	ResolvedHost   string
	ResolvedPort   string
	DialAddress    string
}

// DiagnoseEndpoint resolves user@host:port like ResolveEndpoint and records merge details.
func (c Config) DiagnoseEndpoint(user, host, port string) EndpointDiagnostics {
	d := EndpointDiagnostics{
		ConfigPath: c.Path,
		EntryCount: len(c.Entries),
		URIUser:    user,
		URIHost:    host,
		URIPort:    port,
	}
	if port == "" {
		port = "22"
		d.URIPort = port
	}
	merged, matched, count := c.mergedEntryForHostDetails(user, host)
	d.Matched = matched
	d.MatchedStanzas = count
	for _, e := range c.Entries {
		if !e.MatchesHost(user, host) {
			continue
		}
		d.MatchedDetails = append(d.MatchedDetails, fmt.Sprintf(
			"alias=%q hostname=%q user=%q port=%q",
			e.Alias, e.HostName, e.User, e.Port,
		))
	}
	if !matched {
		d.ResolvedUser = user
		d.ResolvedHost = host
		d.ResolvedPort = port
		d.DialAddress = net.JoinHostPort(host, port)
		return d
	}
	if hn := strings.TrimSpace(merged.HostName); hn != "" {
		d.HostNameSet = true
		d.HostName = hn
		host = hn
	}
	if u := strings.TrimSpace(merged.User); u != "" && strings.TrimSpace(user) == "" {
		user = u
	}
	if p := strings.TrimSpace(merged.Port); p != "" && (port == "" || port == "22") {
		port = p
	}
	d.ResolvedUser = user
	d.ResolvedHost = host
	d.ResolvedPort = port
	d.DialAddress = net.JoinHostPort(host, port)
	return d
}

// ConnectErrorHint returns a short suffix for dial errors.
func ConnectErrorHint(d EndpointDiagnostics) string {
	if d.ConfigPath == "" {
		return ""
	}
	return fmt.Sprintf(" (ssh config: %s, matched=%d)", d.ConfigPath, d.MatchedStanzas)
}
