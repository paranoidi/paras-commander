package rsync

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// errBothRemote is returned when neither side of the transfer is local: rsync has one remote side.
var errBothRemote = errors.New("rsync cannot sync between two remote locations")

// Targets are a transfer's paths in rsync's argument form.
type Targets struct {
	Sources []string
	Dest    string
	// SSH is the remote shell command for -e, "" when both sides are local.
	SSH string
}

// Resolve translates the locations into rsync's arguments. Local paths pass through; at most one
// side may be an SFTP location, which becomes [user@]host:path for rsync to reach over ssh (it
// honours ~/.ssh/config like the SFTP panels do). rsync cannot reuse an SFTP session, so a remote
// side needs key or agent authentication: the ssh runs without a terminal and never asks.
func Resolve(sources []pathloc.Path, dest pathloc.Path) (Targets, error) {
	var t Targets
	host := "" // the remote side's host part, once a location on it was seen (then t.SSH is set)
	for i, p := range append(slices.Clone(sources), dest) {
		var arg string
		switch p.Scheme() {
		case pathloc.SchemeFile:
			arg = p.String()
		case pathloc.SchemeSFTP:
			h, hostErr := pathloc.SFTPHostPart(p)
			// Sources may share one remote host; the destination never joins a remote source.
			if t.SSH != "" && (i == len(sources) || hostErr != nil || h != host) {
				return Targets{}, errBothRemote
			}
			var err error
			if arg, err = remoteArg(p); err != nil {
				return Targets{}, err
			}
			if t.SSH == "" {
				host, t.SSH = h, sshCommand(p)
			}
		default:
			return Targets{}, fmt.Errorf("rsync cannot use %s locations", p.Scheme().Label())
		}
		if i < len(sources) {
			t.Sources = append(t.Sources, arg)
		} else {
			t.Dest = arg
		}
	}
	return t, nil
}

// remoteArg is p as [user@]host:path.
func remoteArg(p pathloc.Path) (string, error) {
	user, host, _, remote, err := pathloc.SFTPEndpoint(p)
	if err != nil {
		return "", err
	}
	if strings.Contains(host, ":") { // an IPv6 address
		host = "[" + host + "]"
	}
	if user != "" {
		host = user + "@" + host
	}
	return host + ":" + remote, nil
}

// sshCommand is the remote shell for the SFTP location p: ssh that never asks for a password or
// a host key question (it has no terminal), on p's port.
func sshCommand(p pathloc.Path) string {
	cmd := "ssh -o BatchMode=yes"
	if _, _, port, _, err := pathloc.SFTPEndpoint(p); err == nil && port != "" && port != "22" {
		cmd += " -p " + port
	}
	return cmd
}
