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
	activeOps     int
	activeStreams int
}

func (c *pooledConn) inUse() bool {
	return c.activeOps > 0 || c.activeStreams > 0
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
	_, release, err := p.withSFTP(ctx, loc)
	if release != nil {
		release()
	}
	return err
}

func (p *Pool) withSFTP(ctx context.Context, loc pathloc.Path) (*gosftp.Client, func(), error) {
	hostPart, err := pathloc.SFTPHostPart(loc)
	if err != nil {
		return nil, nil, err
	}
	p.mu.Lock()
	if c, ok := p.conns[hostPart]; ok {
		p.holdOpLocked(c)
		client := c.sftpClient
		p.mu.Unlock()
		return client, p.releaseOpFunc(hostPart), nil
	}
	p.mu.Unlock()

	client, err := p.dial(ctx, loc, hostPart)
	if err != nil {
		return nil, nil, err
	}
	return client, p.releaseOpFunc(hostPart), nil
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
	sshConn, chans, reqs, err := p.handshakeSSH(ctx, raw, addr, user, diag.URIHost, host, port, openSSH, false)
	if err != nil {
		_ = raw.Close()
		if p.prompts.Password != nil && isSSHAuthError(err) {
			raw, err = dialer.DialContext(ctx, "tcp", addr)
			if err != nil {
				return nil, wrapDialError(addr, err, diag)
			}
			enableTCPKeepAlive(raw)
			sshConn, chans, reqs, err = p.handshakeSSH(ctx, raw, addr, user, diag.URIHost, host, port, openSSH, true)
		}
		if err != nil {
			if raw != nil {
				_ = raw.Close()
			}
			return nil, wrapDialError(addr, err, diag)
		}
	}
	client := ssh.NewClient(sshConn, chans, reqs)
	sftpClient, err := p.openSFTP(ctx, client)
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("sftp session %s: %w", addr, err)
	}
	_ = raw.SetDeadline(time.Time{})

	p.mu.Lock()
	defer p.mu.Unlock()
	if existing, ok := p.conns[hostPart]; ok {
		_ = sftpClient.Close()
		_ = client.Close()
		p.holdOpLocked(existing)
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
	p.holdOpLocked(pc)
	p.conns[hostPart] = pc
	return sftpClient, nil
}

func (p *Pool) holdOpLocked(c *pooledConn) {
	c.activeOps++
	c.lastUsed = time.Now()
	if c.idleTimer != nil {
		c.idleTimer.Stop()
	}
}

func (p *Pool) releaseOpFunc(hostPart string) func() {
	var once sync.Once
	return func() {
		once.Do(func() { p.releaseOp(hostPart) })
	}
}

func (p *Pool) releaseOp(hostPart string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	c, ok := p.conns[hostPart]
	if !ok {
		return
	}
	if c.activeOps > 0 {
		c.activeOps--
	}
	c.lastUsed = time.Now()
	p.armIdleLocked(c)
}

func (p *Pool) convertOpToStream(hostPart string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	c, ok := p.conns[hostPart]
	if !ok {
		return
	}
	if c.activeOps > 0 {
		c.activeOps--
	}
	c.activeStreams++
	if c.idleTimer != nil {
		c.idleTimer.Stop()
	}
}

func (p *Pool) armIdleLocked(c *pooledConn) {
	if c.inUse() || p.settings.IdleTimeout <= 0 {
		return
	}
	if c.idleTimer == nil {
		hostPart := c.hostPart
		c.idleTimer = time.AfterFunc(p.settings.IdleTimeout, func() {
			p.closeHost(hostPart)
		})
		return
	}
	c.idleTimer.Reset(p.settings.IdleTimeout)
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
	p.armIdleLocked(c)
}

func (p *Pool) handshakeSSH(ctx context.Context, raw net.Conn, addr, user, connectHost, resolvedHost, port string, openSSH sshconfig.Config, allowPassword bool) (ssh.Conn, <-chan ssh.NewChannel, <-chan *ssh.Request, error) {
	auth, agentSess, _, err := buildAuthMethods(ctx, user, connectHost, resolvedHost, port, openSSH, p.prompts, allowPassword)
	if err != nil {
		return nil, nil, nil, err
	}
	if agentSess != nil {
		defer func() { _ = agentSess.Close() }()
	}
	clientCfg := &ssh.ClientConfig{
		User:            user,
		Auth:            auth,
		HostKeyCallback: p.hostKeys.callbackWithContext(ctx),
		Timeout:         p.settings.DialTimeout,
	}
	if dl := handshakeDeadline(ctx, p.settings.DialTimeout); !dl.IsZero() {
		_ = raw.SetDeadline(dl)
	}
	return waitClientConn(ctx, raw, addr, clientCfg)
}

func handshakeDeadline(ctx context.Context, timeout time.Duration) time.Time {
	var deadline time.Time
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	if d, ok := ctx.Deadline(); ok {
		if deadline.IsZero() || d.Before(deadline) {
			deadline = d
		}
	}
	return deadline
}

func waitClientConn(ctx context.Context, raw net.Conn, addr string, cfg *ssh.ClientConfig) (ssh.Conn, <-chan ssh.NewChannel, <-chan *ssh.Request, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, nil, err
	}
	type result struct {
		conn  ssh.Conn
		chans <-chan ssh.NewChannel
		reqs  <-chan *ssh.Request
		err   error
	}
	done := make(chan result, 1)
	go func() {
		c, chans, reqs, err := ssh.NewClientConn(raw, addr, cfg)
		done <- result{conn: c, chans: chans, reqs: reqs, err: err}
	}()
	select {
	case <-ctx.Done():
		_ = raw.Close()
		res := <-done
		if res.conn != nil {
			_ = res.conn.Close()
		}
		if err := ctx.Err(); err != nil {
			return nil, nil, nil, err
		}
		return nil, nil, nil, res.err
	case res := <-done:
		return res.conn, res.chans, res.reqs, res.err
	}
}

func (p *Pool) openSFTP(ctx context.Context, client *ssh.Client) (*gosftp.Client, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	type result struct {
		c   *gosftp.Client
		err error
	}
	done := make(chan result, 1)
	go func() {
		c, err := gosftp.NewClient(client)
		done <- result{c: c, err: err}
	}()
	select {
	case <-ctx.Done():
		_ = client.Close()
		res := <-done
		if res.c != nil {
			_ = res.c.Close()
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return nil, res.err
	case res := <-done:
		return res.c, res.err
	}
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
	if c.inUse() {
		p.armIdleLocked(c)
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

func (p *Pool) evictClient(hostPart string, client *gosftp.Client) {
	if hostPart == "" || client == nil {
		return
	}
	p.mu.Lock()
	c, ok := p.conns[hostPart]
	if !ok || c.sftpClient != client {
		p.mu.Unlock()
		return
	}
	delete(p.conns, hostPart)
	if c.idleTimer != nil {
		c.idleTimer.Stop()
	}
	sftpClient := c.sftpClient
	sshClient := c.sshClient
	p.mu.Unlock()
	if sshClient != nil {
		_ = sshClient.Close()
	}
	if sftpClient != nil {
		_ = sftpClient.Close()
	}
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
