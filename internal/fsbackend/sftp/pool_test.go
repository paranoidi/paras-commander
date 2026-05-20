package sftp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/sshconfig"
)

func TestDialEndpointResolvesSSHHostAlias(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config")
	if err := os.WriteFile(cfgPath, []byte(`
Host rhasspy
  HostName 192.168.50.10
  User pi
`), 0o644); err != nil {
		t.Fatal(err)
	}
	loc := pathloc.MustParse("sftp://rhasspy/")
	user, host, port, err := resolveDialEndpoint(Settings{SSHConfigFile: cfgPath}, loc)
	if err != nil {
		t.Fatal(err)
	}
	if user != "pi" || host != "192.168.50.10" || port != "22" {
		t.Fatalf("resolveDialEndpoint = %q,%q,%q", user, host, port)
	}
}

func TestPoolTouchConnResolvesSSHHostAlias(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config")
	if err := os.WriteFile(cfgPath, []byte(`
Host rhasspy
  HostName 192.168.50.10
  User pi
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Configure(Settings{
		SSHConfigFile: cfgPath,
		DialTimeout:   time.Second,
	}, Prompts{}); err != nil {
		t.Fatal(err)
	}
	loc := pathloc.MustParse("sftp://rhasspy/")
	err := TouchConn(context.Background(), loc)
	if err == nil {
		t.Fatal("expected dial error without a reachable host")
	}
	if !strings.Contains(err.Error(), "192.168.50.10") {
		t.Fatalf("dial should target resolved HostName, got: %v", err)
	}
	if strings.Contains(err.Error(), "lookup rhasspy") {
		t.Fatalf("dial must not DNS-resolve bare alias after ssh config merge: %v", err)
	}
}

func TestCloseHostDefersWhileStreamsActive(t *testing.T) {
	t.Parallel()
	p := &Pool{
		settings: Settings{IdleTimeout: time.Hour},
		conns:    make(map[string]*pooledConn),
	}
	p.conns["user@host:22"] = &pooledConn{hostPart: "user@host:22", activeStreams: 1}
	p.closeHost("user@host:22")
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.conns["user@host:22"]; !ok {
		t.Fatal("connection closed while a transfer stream is open")
	}
}

func TestReleaseStreamAllowsIdleClose(t *testing.T) {
	t.Parallel()
	p := &Pool{
		settings: Settings{IdleTimeout: time.Hour},
		conns:    make(map[string]*pooledConn),
	}
	host := "user@host:22"
	p.conns[host] = &pooledConn{hostPart: host, activeStreams: 1}
	p.releaseStream(host)
	p.closeHost(host)
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.conns[host]; ok {
		t.Fatal("connection should close after stream released")
	}
}

func resolveDialEndpoint(settings Settings, loc pathloc.Path) (user, host, port string, err error) {
	user, host, port, _, err = pathloc.SFTPEndpoint(loc)
	if err != nil {
		return "", "", "", err
	}
	openSSH, err := sshconfig.Load(settings.SSHConfigFile)
	if err != nil {
		return "", "", "", err
	}
	user, host, port = openSSH.ResolveEndpoint(user, host, port)
	return user, host, port, nil
}
