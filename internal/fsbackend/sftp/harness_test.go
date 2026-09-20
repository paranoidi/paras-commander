package sftp

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	gosftp "github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

const testSFTPUser = "tester"

type loopbackSFTP struct {
	t    *testing.T
	addr string
	loc  pathloc.Path

	hostSigner ssh.Signer
	clientPub  ssh.PublicKey
	cfgPath    string

	mu    sync.Mutex
	ln    net.Listener
	conns []net.Conn
	stop  chan struct{}
	wg    sync.WaitGroup

	accepts        atomic.Int64
	stallHandshake atomic.Bool
	listGate       <-chan struct{}
	listEntered    chan struct{}
	openGate       <-chan struct{}
	openEntered    chan struct{}
	workDir        string
}

type loopbackSFTPOpts struct {
	stallHandshake bool
	listGate       <-chan struct{}
	listEntered    chan struct{}
	openGate       <-chan struct{}
	openEntered    chan struct{}
	workDir        string
}

func startLoopbackSFTP(t *testing.T, opts loopbackSFTPOpts) *loopbackSFTP {
	t.Helper()
	_, hostPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	hostSigner, err := ssh.NewSignerFromKey(hostPriv)
	if err != nil {
		t.Fatal(err)
	}
	clientPubRaw, clientPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	clientSigner, err := ssh.NewSignerFromKey(clientPriv)
	if err != nil {
		t.Fatal(err)
	}
	_ = clientPubRaw

	dir := t.TempDir()
	keyPath := filepath.Join(dir, "id_ed25519")
	block, err := ssh.MarshalPrivateKey(clientPriv, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatal(err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	cfgPath := filepath.Join(dir, "ssh_config")
	cfg := fmt.Sprintf("Host 127.0.0.1\n  IdentityFile %s\n  IdentitiesOnly yes\n  IdentityAgent none\n", keyPath)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	loc, err := pathloc.Parse(fmt.Sprintf("sftp://%s@%s/", testSFTPUser, addr))
	if err != nil {
		t.Fatal(err)
	}

	s := &loopbackSFTP{
		t:           t,
		addr:        addr,
		loc:         loc,
		hostSigner:  hostSigner,
		clientPub:   clientSigner.PublicKey(),
		cfgPath:     cfgPath,
		ln:          ln,
		stop:        make(chan struct{}),
		listGate:    opts.listGate,
		listEntered: opts.listEntered,
		openGate:    opts.openGate,
		openEntered: opts.openEntered,
		workDir:     opts.workDir,
	}
	s.stallHandshake.Store(opts.stallHandshake)
	s.serve()
	t.Cleanup(s.Close)
	return s
}

func (s *loopbackSFTP) serve() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			tcp, err := s.ln.Accept()
			if err != nil {
				return
			}
			s.mu.Lock()
			s.conns = append(s.conns, tcp)
			s.mu.Unlock()
			s.accepts.Add(1)
			if s.stallHandshake.Load() {
				continue
			}
			s.wg.Add(1)
			go func(c net.Conn) {
				defer s.wg.Done()
				s.handleConn(c)
			}(tcp)
		}
	}()
}

func (s *loopbackSFTP) handleConn(tcp net.Conn) {
	cfg := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if conn.User() != testSFTPUser {
				return nil, fmt.Errorf("bad user %q", conn.User())
			}
			if !bytes.Equal(key.Marshal(), s.clientPub.Marshal()) {
				return nil, fmt.Errorf("unknown key")
			}
			return nil, nil
		},
	}
	cfg.AddHostKey(s.hostSigner)
	sshConn, chans, reqs, err := ssh.NewServerConn(tcp, cfg)
	if err != nil {
		return
	}
	defer func() { _ = sshConn.Close() }()
	go ssh.DiscardRequests(reqs)
	for newCh := range chans {
		if newCh.ChannelType() != "session" {
			_ = newCh.Reject(ssh.UnknownChannelType, "unknown")
			continue
		}
		ch, requests, err := newCh.Accept()
		if err != nil {
			return
		}
		go s.handleSession(ch, requests)
	}
}

func (s *loopbackSFTP) handleSession(ch ssh.Channel, requests <-chan *ssh.Request) {
	defer func() { _ = ch.Close() }()
	for req := range requests {
		ok := false
		if req.Type == "subsystem" && len(req.Payload) >= 4 && string(req.Payload[4:]) == "sftp" {
			ok = true
			_ = req.Reply(true, nil)
			s.serveSFTP(ch)
			return
		}
		_ = req.Reply(ok, nil)
	}
}

func (s *loopbackSFTP) serveSFTP(ch ssh.Channel) {
	if s.workDir != "" && s.listGate == nil && s.openGate == nil {
		srv, err := gosftp.NewServer(ch, gosftp.WithServerWorkingDirectory(s.workDir))
		if err != nil {
			return
		}
		_ = srv.Serve()
		_ = srv.Close()
		return
	}
	inner := gosftp.InMemHandler()
	handlers := gosftp.Handlers{
		FileGet:  &gatedFileReader{inner: inner.FileGet, gate: s.openGate, entered: s.openEntered},
		FilePut:  inner.FilePut,
		FileCmd:  inner.FileCmd,
		FileList: &gatedFileLister{inner: inner.FileList, gate: s.listGate, entered: s.listEntered},
	}
	srv := gosftp.NewRequestServer(ch, handlers)
	_ = srv.Serve()
	_ = srv.Close()
}

func (s *loopbackSFTP) Close() {
	s.mu.Lock()
	if s.stop != nil {
		select {
		case <-s.stop:
		default:
			close(s.stop)
		}
	}
	if s.ln != nil {
		_ = s.ln.Close()
	}
	for _, c := range s.conns {
		_ = c.Close()
	}
	s.conns = nil
	s.mu.Unlock()
	s.wg.Wait()
}

func (s *loopbackSFTP) Restart() {
	t := s.t
	t.Helper()
	s.mu.Lock()
	addr := s.addr
	if s.ln != nil {
		_ = s.ln.Close()
	}
	for _, c := range s.conns {
		_ = c.Close()
	}
	s.conns = nil
	s.mu.Unlock()
	s.wg.Wait()

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("relisten %s: %v", addr, err)
	}
	s.mu.Lock()
	s.ln = ln
	s.stop = make(chan struct{})
	s.mu.Unlock()
	s.serve()
}

func (s *loopbackSFTP) newPool(idle time.Duration) *Pool {
	store, err := newHostKeyStore(filepath.Join(s.t.TempDir(), "known_hosts"), Prompts{
		HostKey: func(context.Context, HostKeyPrompt) (HostKeyDecision, error) {
			return HostKeyTrustSession, nil
		},
	})
	if err != nil {
		s.t.Fatal(err)
	}
	return &Pool{
		settings: Settings{
			SSHConfigFile: s.cfgPath,
			IdleTimeout:   idle,
			DialTimeout:   2 * time.Second,
		},
		prompts: Prompts{
			HostKey: func(context.Context, HostKeyPrompt) (HostKeyDecision, error) {
				return HostKeyTrustSession, nil
			},
		},
		hostKeys: store,
		conns:    make(map[string]*pooledConn),
	}
}

func (s *loopbackSFTP) fileLoc(name string) pathloc.Path {
	s.t.Helper()
	loc, err := s.loc.Join(name)
	if err != nil {
		s.t.Fatal(err)
	}
	return loc
}

type gatedFileLister struct {
	inner   gosftp.FileLister
	gate    <-chan struct{}
	entered chan struct{}
	once    sync.Once
}

func (g *gatedFileLister) Filelist(r *gosftp.Request) (gosftp.ListerAt, error) {
	if g.entered != nil {
		g.once.Do(func() { close(g.entered) })
	}
	if g.gate != nil {
		<-g.gate
	}
	return g.inner.Filelist(r)
}

type gatedFileReader struct {
	inner   gosftp.FileReader
	gate    <-chan struct{}
	entered chan struct{}
	once    sync.Once
}

func (g *gatedFileReader) Fileread(r *gosftp.Request) (io.ReaderAt, error) {
	if g.entered != nil {
		g.once.Do(func() { close(g.entered) })
	}
	if g.gate != nil {
		<-g.gate
	}
	if g.inner != nil {
		if f, err := g.inner.Fileread(r); err == nil {
			return f, nil
		}
	}
	return bytes.NewReader([]byte("hello")), nil
}

var _ gosftp.FileLister = (*gatedFileLister)(nil)
var _ gosftp.FileReader = (*gatedFileReader)(nil)
