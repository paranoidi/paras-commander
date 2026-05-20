package sftp

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/paranoidi/paras-commander/internal/sshconfig"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func TestLoadSignersIdentityFileBeforeDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("SSH_AUTH_SOCK", "")
	keyPath := filepath.Join(dir, "custom_key")
	writeTestRSAPrivateKey(t, keyPath)

	signers, _, report := loadSigners(sshconfig.ConnectAuthOptions{
		IdentityFiles: []string{keyPath},
		IdentityAgent: "none",
	})
	if len(signers) < 1 {
		t.Fatalf("signers = %d, want at least identity file key", len(signers))
	}
	if report.PerPath[0].Status != identityStatusOK {
		t.Fatalf("identity status = %s, want %s", report.PerPath[0].Status, identityStatusOK)
	}
}

func TestLoadSignersIdentitiesOnlySkipsDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("SSH_AUTH_SOCK", "")
	defaultKey := filepath.Join(dir, ".ssh", "id_ed25519")
	if err := os.MkdirAll(filepath.Dir(defaultKey), 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestRSAPrivateKey(t, defaultKey)

	signers, _, report := loadSigners(sshconfig.ConnectAuthOptions{
		IdentitiesOnly: true,
		IdentityAgent:  "none",
	})
	if len(signers) != 0 {
		t.Fatalf("signers = %d, want 0 without identity paths", len(signers))
	}
	if report.DefaultKeysTried {
		t.Fatal("default keys must be skipped when identitiesonly yes")
	}
}

func TestBuildAuthMethodsPubkeyOnlyPhaseHasSigners(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("SSH_AUTH_SOCK", "")
	keyPath := filepath.Join(dir, "id_test")
	writeTestRSAPrivateKey(t, keyPath)

	cfg := sshconfig.Config{Entries: []sshconfig.HostEntry{{
		Alias: "rhasspy", HostName: "192.168.50.10", User: "pi",
		IdentityFiles: []string{keyPath}, IdentityAgent: "none",
	}}}
	methods, _, report, err := buildAuthMethods("pi", "rhasspy", "192.168.50.10", "22", cfg, Prompts{
		Password: func(context.Context, PasswordPrompt) (string, error) {
			t.Fatal("password must not be registered on pubkey-only handshake")
			return "", nil
		},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if report.SignerCount != 1 || len(methods) != 1 {
		t.Fatalf("signers=%d methods=%d, want one pubkey method", report.SignerCount, len(methods))
	}
}

func TestBuildAuthMethodsPublicKeyBeforePassword(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("SSH_AUTH_SOCK", "")
	keyPath := filepath.Join(dir, "id_test")
	writeTestRSAPrivateKey(t, keyPath)

	cfg := sshconfig.Config{Entries: []sshconfig.HostEntry{{
		Alias: "rhasspy", HostName: "192.168.50.10", User: "pi",
		IdentityFiles: []string{keyPath}, IdentityAgent: "none",
	}}}

	methods, _, report, err := buildAuthMethods("pi", "rhasspy", "192.168.50.10", "22", cfg, Prompts{
		Password: func(context.Context, PasswordPrompt) (string, error) {
			t.Fatal("password callback must not run during method construction")
			return "", nil
		},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if report.SignerCount != 1 {
		t.Fatalf("signerCount = %d, want 1", report.SignerCount)
	}
	if len(methods) != 2 {
		t.Fatalf("methods = %d, want pubkey then password", len(methods))
	}
}

func TestBuildAuthMethodsNoPasswordWithoutPrompt(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("SSH_AUTH_SOCK", "")
	keyPath := filepath.Join(dir, "id_test")
	writeTestRSAPrivateKey(t, keyPath)

	methods, _, _, err := buildAuthMethods("", "host", "host", "22", sshconfig.Config{
		Entries: []sshconfig.HostEntry{{Alias: "host", IdentityAgent: "none", IdentitiesOnly: "yes"}},
	}, Prompts{}, true)
	if err == nil {
		t.Fatal("expected error with no signers and no password prompt")
	}
	_ = methods

	methods, _, report, err := buildAuthMethods("", "host", "host", "22", sshconfig.Config{
		Entries: []sshconfig.HostEntry{{Alias: "host", IdentityAgent: "none", IdentitiesOnly: "yes"}},
	}, Prompts{
		Password: func(context.Context, PasswordPrompt) (string, error) { return "secret", nil },
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if report.SignerCount != 0 || len(methods) != 1 {
		t.Fatalf("signers=%d methods=%d, want password-only", report.SignerCount, len(methods))
	}

	methods, _, report, err = buildAuthMethods("pi", "rhasspy", "192.168.50.10", "22", sshconfig.Config{Entries: []sshconfig.HostEntry{{
		Alias: "rhasspy", HostName: "192.168.50.10", User: "pi",
		IdentityFiles: []string{keyPath}, IdentityAgent: "none",
	}}}, Prompts{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if report.SignerCount != 1 || len(methods) != 1 {
		t.Fatalf("signers=%d methods=%d, want pubkey-only", report.SignerCount, len(methods))
	}
}

func TestResolveSSHAuthSocketFromEnv(t *testing.T) {
	dir := t.TempDir()
	sock := startAgentListener(t, filepath.Join(dir, "agent.sock"), agent.NewKeyring())
	t.Setenv("SSH_AUTH_SOCK", sock)
	if got := resolveSSHAuthSocket(""); got != sock {
		t.Fatalf("resolveSSHAuthSocket = %q, want %q", got, sock)
	}
}

func TestResolveSSHAuthSocketCandidatesPrefersEnv(t *testing.T) {
	dir := t.TempDir()
	envSock := startAgentListener(t, filepath.Join(dir, "env.sock"), agent.NewKeyring())
	keyringSock := startBrokenAgentListener(t, filepath.Join(dir, "keyring.sock"))
	t.Setenv("SSH_AUTH_SOCK", envSock)
	candidates := resolveSSHAuthSocketCandidates(keyringSock)
	if len(candidates) < 2 {
		t.Fatalf("candidates = %v, want env then configured agent path", candidates)
	}
	if candidates[0] != envSock {
		t.Fatalf("first candidate = %q, want env %q", candidates[0], envSock)
	}
	if candidates[1] != keyringSock {
		t.Fatalf("second candidate = %q, want configured %q", candidates[1], keyringSock)
	}
}

func TestLoadAgentSignersSkipsBrokenEnvSocket(t *testing.T) {
	dir := t.TempDir()
	deadEnv := startBrokenAgentListener(t, filepath.Join(dir, "dead-env.sock"))
	kr := agent.NewKeyring()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	if err := kr.Add(agent.AddedKey{PrivateKey: key}); err != nil {
		t.Fatal(err)
	}
	working := startAgentListener(t, filepath.Join(dir, "working.sock"), kr)
	t.Setenv("SSH_AUTH_SOCK", deadEnv)
	signers, sess, sock, status := loadAgentSigners(working)
	if status != agentStatusOK || sess == nil || sock != working {
		t.Fatalf("status=%q sock=%q session=%v", status, sock, sess)
	}
	if len(signers) == 0 {
		t.Fatal("expected signers from second socket")
	}
	_ = sess.Close()
}

func TestLoadSignersDeadAgentFallsBackToIdentityFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	deadSock := startBrokenAgentListener(t, filepath.Join(dir, "dead-agent.sock"))
	t.Setenv("SSH_AUTH_SOCK", deadSock)
	keyPath := filepath.Join(dir, "id_test")
	writeTestRSAPrivateKey(t, keyPath)

	signers, sess, report := loadSigners(sshconfig.ConnectAuthOptions{
		IdentityFiles: []string{keyPath},
	})
	if sess != nil {
		t.Fatal("dead agent must not leave an open session")
	}
	if report.AgentStatus != agentStatusUnavailable {
		t.Fatalf("agent status = %q, want %q", report.AgentStatus, agentStatusUnavailable)
	}
	if len(signers) != 1 {
		t.Fatalf("signers = %d, want 1 file-based signer", len(signers))
	}
	if report.PerPath[0].Status != identityStatusOK {
		t.Fatalf("identity status = %s, want %s", report.PerPath[0].Status, identityStatusOK)
	}
}

func TestProbeAgentSignersRequiresOpenConnection(t *testing.T) {
	dir := t.TempDir()
	kr := agent.NewKeyring()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	if err := kr.Add(agent.AddedKey{PrivateKey: key}); err != nil {
		t.Fatal(err)
	}
	sock := startAgentListener(t, filepath.Join(dir, "agent.sock"), kr)
	signers, sess, status := probeAgentSigners(sock)
	if status != agentStatusOK || sess == nil || len(signers) == 0 {
		t.Fatalf("probe status=%q session=%v signers=%d", status, sess, len(signers))
	}
	if _, signErr := signers[0].Sign(nil, []byte("probe")); signErr != nil {
		t.Fatalf("sign before close: %v", signErr)
	}
	_ = sess.Close()
	if _, signErr := signers[0].Sign(nil, []byte("probe")); signErr == nil {
		t.Fatal("sign after agent session close should fail")
	}
}

func startAgentListener(t *testing.T, sock string, ag agent.Agent) string {
	t.Helper()
	_ = os.Remove(sock)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close(); _ = os.Remove(sock) })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				_ = agent.ServeAgent(ag, conn)
				_ = conn.Close()
			}(c)
		}
	}()
	return sock
}

func startBrokenAgentListener(t *testing.T, sock string) string {
	t.Helper()
	_ = os.Remove(sock)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close(); _ = os.Remove(sock) })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
	return sock
}

func writeTestRSAPrivateKey(t *testing.T, path string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der := x509.MarshalPKCS1PrivateKey(key)
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: der}
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ssh.ParsePrivateKey(pem.EncodeToMemory(block)); err != nil {
		t.Fatalf("test key not parseable: %v", err)
	}
}
