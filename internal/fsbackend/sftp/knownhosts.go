package sftp

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type hostKeyStore struct {
	mu sync.Mutex

	knownHostsPath string
	prompts        Prompts
	base           ssh.HostKeyCallback
	sessionTrusted map[string]string // hostPart -> fingerprint
}

func newHostKeyStore(path string, prompts Prompts) (*hostKeyStore, error) {
	resolved, err := resolveKnownHostsPath(path)
	if err != nil {
		return nil, err
	}
	path = resolved
	var base ssh.HostKeyCallback
	if _, err := os.Stat(path); err == nil {
		cb, err := knownhosts.New(path)
		if err != nil {
			return nil, fmt.Errorf("load known_hosts %q: %w", path, err)
		}
		base = cb
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("stat known_hosts %q: %w", path, err)
	}
	return &hostKeyStore{
		knownHostsPath: path,
		prompts:        prompts,
		base:           base,
		sessionTrusted: make(map[string]string),
	}, nil
}

func resolveKnownHostsPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home for known_hosts: %w", err)
		}
		return filepath.Join(home, ".ssh", "known_hosts"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	if strings.HasPrefix(path, "~/") && home != "" {
		return filepath.Clean(filepath.Join(home, path[2:])), nil
	}
	if strings.HasPrefix(path, "~") && home != "" {
		return filepath.Clean(filepath.Join(home, strings.TrimPrefix(path, "~"))), nil
	}
	return filepath.Clean(path), nil
}

func (s *hostKeyStore) reloadBaseLocked() error {
	if _, err := os.Stat(s.knownHostsPath); err != nil {
		if os.IsNotExist(err) {
			s.base = nil
			return nil
		}
		return fmt.Errorf("stat known_hosts %q: %w", s.knownHostsPath, err)
	}
	cb, err := knownhosts.New(s.knownHostsPath)
	if err != nil {
		return fmt.Errorf("load known_hosts %q: %w", s.knownHostsPath, err)
	}
	s.base = cb
	return nil
}

func (s *hostKeyStore) callback() ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		hostPart := hostname
		if h, err := sshHostPartFromHostname(hostname); err == nil && h != "" {
			hostPart = h
		}
		remoteLabel := hostname
		if remote != nil {
			remoteLabel = remote.String()
		}
		fp := ssh.FingerprintSHA256(key)

		s.mu.Lock()
		if s.sessionTrusted[hostPart] == fp {
			s.mu.Unlock()
			return nil
		}
		s.mu.Unlock()

		if s.base != nil {
			if err := s.base(hostname, remote, key); err == nil {
				return nil
			}
		}

		if s.prompts.HostKey == nil {
			return fmt.Errorf("unknown host key for %s", hostPart)
		}
		decision, err := s.prompts.HostKey(context.Background(), HostKeyPrompt{
			Host:        hostPart,
			RemoteAddr:  remoteLabel,
			KeyType:     key.Type(),
			Fingerprint: fp,
		})
		if err != nil {
			return err
		}
		switch decision {
		case HostKeyTrustSession:
			s.mu.Lock()
			s.sessionTrusted[hostPart] = fp
			s.mu.Unlock()
			return nil
		case HostKeyTrustPersist:
			if err := appendKnownHost(s.knownHostsPath, hostname, key); err != nil {
				return err
			}
			s.mu.Lock()
			if err := s.reloadBaseLocked(); err != nil {
				s.mu.Unlock()
				return err
			}
			s.sessionTrusted[hostPart] = fp
			s.mu.Unlock()
			return nil
		default:
			return fmt.Errorf("host key rejected for %s", hostPart)
		}
	}
}

func appendKnownHost(path string, hostname string, key ssh.PublicKey) error {
	line := knownhosts.Line([]string{hostname}, key)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(line + "\n"); err != nil {
		return err
	}
	return nil
}

func sshHostPartFromHostname(hostname string) (string, error) {
	if strings.HasPrefix(hostname, "[") {
		if i := strings.Index(hostname, "]"); i > 0 {
			return hostname[1:i], nil
		}
	}
	if i := strings.LastIndex(hostname, ":"); i > 0 {
		return hostname[:i], nil
	}
	return hostname, nil
}
