package sftp

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func testSSHPublicKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return signer.PublicKey()
}

func testTCPAddr(t *testing.T, hostPort string) net.Addr {
	t.Helper()
	addr, err := net.ResolveTCPAddr("tcp", hostPort)
	if err != nil {
		t.Fatal(err)
	}
	return addr
}

func TestHostKeyTrustPersistReloadsBase(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")
	key := testSSHPublicKey(t)
	hostname := "127.0.0.1:22"
	remote := testTCPAddr(t, hostname)

	prompts := 0
	store, err := newHostKeyStore(path, Prompts{
		HostKey: func(context.Context, HostKeyPrompt) (HostKeyDecision, error) {
			prompts++
			return HostKeyTrustPersist, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	cb := store.callback()
	if err := cb(hostname, remote, key); err != nil {
		t.Fatalf("first callback: %v", err)
	}
	if prompts != 1 {
		t.Fatalf("prompts = %d, want 1", prompts)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read known_hosts: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("known_hosts not written")
	}

	store.mu.Lock()
	store.sessionTrusted = make(map[string]string)
	store.mu.Unlock()

	if err := cb(hostname, remote, key); err != nil {
		t.Fatalf("second callback without session trust: %v", err)
	}
	if prompts != 1 {
		t.Fatalf("prompts after reload = %d, want 1", prompts)
	}
}

func TestHostKeyTrustPersistNonDefaultPort(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")
	key := testSSHPublicKey(t)
	hostname := "127.0.0.1:2222"
	remote := testTCPAddr(t, hostname)

	store, err := newHostKeyStore(path, Prompts{
		HostKey: func(context.Context, HostKeyPrompt) (HostKeyDecision, error) {
			return HostKeyTrustPersist, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.callback()(hostname, remote, key); err != nil {
		t.Fatalf("persist: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "[127.0.0.1]:2222") {
		t.Fatalf("known_hosts line missing bracketed host:port, got %q", data)
	}

	store2, err := newHostKeyStore(path, Prompts{
		HostKey: func(context.Context, HostKeyPrompt) (HostKeyDecision, error) {
			t.Fatal("should not prompt after persist")
			return HostKeyReject, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store2.callback()(hostname, remote, key); err != nil {
		t.Fatalf("new store verify: %v", err)
	}
}

func TestHostKeyTrustSessionDoesNotWriteKnownHosts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")
	key := testSSHPublicKey(t)
	hostname := "127.0.0.1:22"
	remote := testTCPAddr(t, hostname)

	store, err := newHostKeyStore(path, Prompts{
		HostKey: func(context.Context, HostKeyPrompt) (HostKeyDecision, error) {
			return HostKeyTrustSession, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.callback()(hostname, remote, key); err != nil {
		t.Fatalf("session trust: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("session trust must not create known_hosts")
	}
}

func TestResolveKnownHostsPathTilde(t *testing.T) {
	t.Parallel()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	got, err := resolveKnownHostsPath("~/.ssh/known_hosts")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".ssh", "known_hosts")
	if got != want {
		t.Fatalf("resolveKnownHostsPath = %q, want %q", got, want)
	}
}
