package sshconfig

import (
	"os"
	"os/user"
	"strings"
)

// IdentityTokenContext supplies OpenSSH IdentityFile %-token values at connect time.
type IdentityTokenContext struct {
	Home       string
	LocalUser  string
	RemoteUser string
	DestHost   string
	LocalHost  string
}

func localSSHUsername() string {
	if u, err := user.Current(); err == nil && strings.TrimSpace(u.Username) != "" {
		return u.Username
	}
	if u := strings.TrimSpace(os.Getenv("USER")); u != "" {
		return u
	}
	return strings.TrimSpace(os.Getenv("LOGNAME"))
}

// ExpandIdentityFileTokens replaces OpenSSH IdentityFile tokens (%h, %r, …).
func ExpandIdentityFileTokens(raw string, ctx IdentityTokenContext) string {
	if raw == "" || !strings.Contains(raw, "%") {
		return raw
	}
	home := ctx.Home
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	localUser := ctx.LocalUser
	if localUser == "" {
		localUser = localSSHUsername()
	}
	localHost := ctx.LocalHost
	if localHost == "" {
		localHost, _ = os.Hostname()
	}
	var b strings.Builder
	for i := 0; i < len(raw); i++ {
		if raw[i] != '%' {
			b.WriteByte(raw[i])
			continue
		}
		if i+1 >= len(raw) {
			b.WriteByte('%')
			break
		}
		switch raw[i+1] {
		case '%':
			b.WriteByte('%')
		case 'd':
			b.WriteString(home)
		case 'h':
			b.WriteString(ctx.DestHost)
		case 'l':
			b.WriteString(localHost)
		case 'r':
			b.WriteString(ctx.RemoteUser)
		case 'u':
			b.WriteString(localUser)
		default:
			b.WriteByte('%')
			b.WriteByte(raw[i+1])
		}
		i++
	}
	return b.String()
}

func expandIdentityFilePaths(paths []string, ctx IdentityTokenContext) []string {
	if len(paths) == 0 {
		return nil
	}
	home := ctx.Home
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	out := make([]string, 0, len(paths))
	for _, raw := range paths {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		raw = ExpandIdentityFileTokens(raw, ctx)
		path, err := expandIdentityFile(raw, home)
		if err != nil {
			continue
		}
		out = appendIdentityUnique(out, path)
	}
	return out
}
