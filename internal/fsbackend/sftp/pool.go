package sftp

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	gosftp "github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/sshconfig"
)

// Settings holds pool timing and known_hosts path.
type Settings struct {
	KnownHostsFile string
	SSHConfigFile  string
	IdleTimeout    time.Duration
	DialTimeout    time.Duration
}

type pooledConn struct {
	hostPart      string
	sshClient     *ssh.Client
	sftpClient    *gosftp.Client
	lastUsed      time.Time
	idleTimer     *time.Timer
	activeStreams int
}

// Pool reuses SSH/SFTP sessions keyed by sftp host part (user@host:port).
type Pool struct {
	mu       sync.Mutex
	settings Settings
	prompts  Prompts
	hostKeys *hostKeyStore

	conns map[string]*pooledConn
}

// DefaultPool is configured by Configure before use.
var DefaultPool = &Pool{
	conns: make(map[string]*pooledConn),
}

// Configure applies settings and prompts to the process-wide pool.
func Configure(settings Settings, prompts Prompts) error {
	store, err := newHostKeyStore(settings.KnownHostsFile, prompts)
	if err != nil {
		return err
	}
	DefaultPool.mu.Lock()
	defer DefaultPool.mu.Unlock()
	DefaultPool.settings = settings
	DefaultPool.prompts = prompts
	DefaultPool.hostKeys = store
	return nil
}

// Touch ensures a connection exists for loc's host (used before list/stat).
func (p *Pool) Touch(ctx context.Context, loc pathloc.Path) error {
	_, err := p.withSFTP(ctx, loc)
	return err
}

func (p *Pool) withSFTP(ctx context.Context, loc pathloc.Path) (*gosftp.Client, error) {
	hostPart, err := pathloc.SFTPHostPart(loc)
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	if c, ok := p.conns[hostPart]; ok {
		c.lastUsed = time.Now()
		if c.idleTimer != nil {
			c.idleTimer.Reset(p.settings.IdleTimeout)
		}
		client := c.sftpClient
		p.mu.Unlock()
		return client, nil
	}
	p.mu.Unlock()

	client, err := p.dial(ctx, loc, hostPart)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (p *Pool) dial(ctx context.Context, loc pathloc.Path, hostPart string) (*gosftp.Client, error) {
	user, host, port, _, err := pathloc.SFTPEndpoint(loc)
	if err != nil {
		return nil, err
	}

	openSSH, loadErr := sshconfig.Load(p.settings.SSHConfigFile)
	if loadErr != nil {
		return nil, fmt.Errorf("load ssh config: %w", loadErr)
	}
	diag := openSSH.DiagnoseEndpoint(user, host, port)
	user, host, port = diag.ResolvedUser, diag.ResolvedHost, diag.ResolvedPort
	addr := diag.DialAddress
	dialer := &net.Dialer{Timeout: p.settings.DialTimeout}
	raw, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, wrapDialError(addr, err, diag)
	}
	enableTCPKeepAlive(raw)
	sshConn, chans, reqs, err := p.handshakeSSH(raw, addr, user, diag.URIHost, host, port, openSSH, false)
	if err != nil {
		_ = raw.Close()
		if p.prompts.Password != nil && isSSHAuthError(err) {
			raw, err = dialer.DialContext(ctx, "tcp", addr)
			if err != nil {
				return nil, wrapDialError(addr, err, diag)
			}
			enableTCPKeepAlive(raw)
			sshConn, chans, reqs, err = p.handshakeSSH(raw, addr, user, diag.URIHost, host, port, openSSH, true)
		}
		if err != nil {
			if raw != nil {
				_ = raw.Close()
			}
			return nil, wrapDialError(addr, err, diag)
		}
	}
	client := ssh.NewClient(sshConn, chans, reqs)
	sftpClient, err := gosftp.NewClient(client)
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("sftp session %s: %w", addr, err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if existing, ok := p.conns[hostPart]; ok {
		_ = sftpClient.Close()
		_ = client.Close()
		existing.lastUsed = time.Now()
		if existing.idleTimer != nil {
			existing.idleTimer.Reset(p.settings.IdleTimeout)
		}
		return existing.sftpClient, nil
	}
	pc := &pooledConn{
		hostPart:   hostPart,
		sshClient:  client,
		sftpClient: sftpClient,
		lastUsed:   time.Now(),
	}
	if p.settings.IdleTimeout > 0 {
		pc.idleTimer = time.AfterFunc(p.settings.IdleTimeout, func() {
			p.closeHost(hostPart)
		})
	}
	p.conns[hostPart] = pc
	return sftpClient, nil
}

func (p *Pool) leaseStream(hostPart string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	c, ok := p.conns[hostPart]
	if !ok {
		return
	}
	c.activeStreams++
	if c.idleTimer != nil {
		c.idleTimer.Stop()
	}
}

func (p *Pool) releaseStream(hostPart string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	c, ok := p.conns[hostPart]
	if !ok {
		return
	}
	if c.activeStreams > 0 {
		c.activeStreams--
	}
	c.lastUsed = time.Now()
	if c.activeStreams == 0 && p.settings.IdleTimeout > 0 && c.idleTimer != nil {
		c.idleTimer.Reset(p.settings.IdleTimeout)
	}
}

func (p *Pool) handshakeSSH(raw net.Conn, addr, user, connectHost, resolvedHost, port string, openSSH sshconfig.Config, allowPassword bool) (ssh.Conn, <-chan ssh.NewChannel, <-chan *ssh.Request, error) {
	auth, agentSess, _, err := buildAuthMethods(user, connectHost, resolvedHost, port, openSSH, p.prompts, allowPassword)
	if err != nil {
		return nil, nil, nil, err
	}
	if agentSess != nil {
		defer func() { _ = agentSess.Close() }()
	}
	clientCfg := &ssh.ClientConfig{
		User:            user,
		Auth:            auth,
		HostKeyCallback: p.hostKeys.callback(),
		Timeout:         0,
	}
	return ssh.NewClientConn(raw, addr, clientCfg)
}

func wrapDialError(addr string, err error, diag sshconfig.EndpointDiagnostics) error {
	msg := fmt.Errorf("ssh dial %s: %w", addr, err)
	if hint := sshconfig.ConnectErrorHint(diag); hint != "" {
		return fmt.Errorf("%w%s", msg, hint)
	}
	return msg
}

func (p *Pool) closeHost(hostPart string) {
	p.mu.Lock()
	c, ok := p.conns[hostPart]
	if !ok {
		p.mu.Unlock()
		return
	}
	if c.activeStreams > 0 {
		if p.settings.IdleTimeout > 0 {
			if c.idleTimer != nil {
				c.idleTimer.Stop()
			}
			c.idleTimer = time.AfterFunc(p.settings.IdleTimeout, func() {
				p.closeHost(hostPart)
			})
		}
		p.mu.Unlock()
		return
	}
	delete(p.conns, hostPart)
	if c.idleTimer != nil {
		c.idleTimer.Stop()
	}
	if c.sftpClient != nil {
		_ = c.sftpClient.Close()
	}
	if c.sshClient != nil {
		_ = c.sshClient.Close()
	}
	p.mu.Unlock()
}

// CloseAll disconnects every pooled session.
func (p *Pool) CloseAll() {
	p.mu.Lock()
	hosts := make([]string, 0, len(p.conns))
	for h := range p.conns {
		hosts = append(hosts, h)
	}
	p.mu.Unlock()
	for _, h := range hosts {
		p.closeHost(h)
	}
}
