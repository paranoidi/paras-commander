package sftp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/paranoidi/paras-commander/internal/sshconfig"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// defaultIdentityKeyNames mirrors common OpenSSH client defaults (after IdentityFile entries).
var defaultIdentityKeyNames = []string{
	"id_ed25519",
	"id_ed25519_sk",
	"id_rsa",
	"id_ecdsa",
}

const (
	identityStatusOK          = "ok"
	identityStatusMissing     = "missing"
	identityStatusEncrypted   = "encrypted_skip"
	identityStatusParseFail   = "parse_fail"
	identityStatusAgentSigner = "agent_signer"
)

type identityPathResult struct {
	Path   string
	Status string
}

type signerLoadReport struct {
	IdentityPaths    []string
	PerPath          []identityPathResult
	SignerCount      int
	AgentSignerCount int
	AuthSock         string
	AgentStatus      string
	DefaultKeysTried bool
	IdentitiesOnly   bool
	IdentityAgent    string
	SignerSummaries  []string
}

// sshAgentSession keeps the agent Unix connection open while agent-backed signers are used.
type sshAgentSession struct {
	conn net.Conn
}

func (s *sshAgentSession) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	err := s.conn.Close()
	s.conn = nil
	return err
}

func buildAuthMethods(user, connectHost, resolvedHost, port string, cfg sshconfig.Config, prompts Prompts, allowPassword bool) ([]ssh.AuthMethod, *sshAgentSession, signerLoadReport, error) {
	opts := cfg.ConnectAuthOptionsFor(user, connectHost, resolvedHost, port)
	signers, agentSess, report := loadSigners(opts)
	report.SignerCount = len(signers)
	for _, s := range signers {
		if s != nil {
			report.SignerSummaries = append(report.SignerSummaries, s.PublicKey().Type())
		}
	}

	var methods []ssh.AuthMethod
	if len(signers) > 0 {
		methods = append(methods, ssh.PublicKeys(signers...))
	}
	if allowPassword && prompts.Password != nil {
		hostLabel := resolvedHost
		if port != "" && port != "22" {
			hostLabel = net.JoinHostPort(resolvedHost, port)
		}
		methods = append(methods, ssh.PasswordCallback(func() (string, error) {
			return prompts.Password(context.Background(), PasswordPrompt{User: user, Host: hostLabel})
		}))
	}
	if len(methods) == 0 {
		return nil, nil, report, fmt.Errorf("no SSH authentication methods configured")
	}
	return methods, agentSess, report, nil
}

func loadSigners(opts sshconfig.ConnectAuthOptions) ([]ssh.Signer, *sshAgentSession, signerLoadReport) {
	report := signerLoadReport{
		IdentityPaths:  append([]string(nil), opts.IdentityFiles...),
		IdentitiesOnly: opts.IdentitiesOnly,
		IdentityAgent:  opts.IdentityAgent,
	}
	var out []ssh.Signer
	seen := make(map[string]struct{})
	add := func(s ssh.Signer) {
		if s == nil {
			return
		}
		fp := s.PublicKey().Type() + string(s.PublicKey().Marshal())
		if _, ok := seen[fp]; ok {
			return
		}
		seen[fp] = struct{}{}
		out = append(out, s)
	}

	for _, path := range opts.IdentityFiles {
		signer, status := loadPrivateKey(path)
		report.PerPath = append(report.PerPath, identityPathResult{Path: path, Status: status})
		if signer != nil {
			add(signer)
		}
	}

	agentBefore := len(out)
	var agentSess *sshAgentSession
	tryAgent := !opts.IdentitiesOnly || len(opts.IdentityFiles) > 0
	if tryAgent {
		agentSigners, sess, sock, status := loadAgentSigners(opts.IdentityAgent)
		report.AuthSock = sock
		report.AgentStatus = status
		if sess != nil {
			agentSess = sess
			for _, s := range agentSigners {
				add(s)
			}
		}
	} else {
		report.AgentStatus = agentStatusSkipped
	}
	report.AgentSignerCount = len(out) - agentBefore

	if !opts.IdentitiesOnly && len(opts.IdentityFiles) == 0 {
		report.DefaultKeysTried = true
		home, err := os.UserHomeDir()
		if err == nil {
			for _, name := range defaultIdentityKeyNames {
				path := filepath.Join(home, ".ssh", name)
				signer, status := loadPrivateKey(path)
				if status == identityStatusMissing {
					continue
				}
				report.PerPath = append(report.PerPath, identityPathResult{Path: path, Status: status})
				if signer != nil {
					add(signer)
				}
			}
		}
	}
	report.SignerCount = len(out)
	return out, agentSess, report
}

const (
	agentStatusOK          = "agent_ok"
	agentStatusEmpty       = "agent_empty"
	agentStatusUnavailable = "agent_unavailable"
	agentStatusSkipped     = "agent_skipped"
)

func resolveSSHAuthSocket(configured string) string {
	candidates := resolveSSHAuthSocketCandidates(configured)
	if len(candidates) == 0 {
		return ""
	}
	return candidates[0]
}

func resolveSSHAuthSocketCandidates(configured string) []string {
	if strings.EqualFold(strings.TrimSpace(configured), "none") {
		return nil
	}
	var out []string
	seen := make(map[string]struct{})
	add := func(sock string) {
		sock = strings.TrimSpace(sock)
		if sock == "" {
			return
		}
		if st, err := os.Stat(sock); err != nil || st.Mode()&os.ModeSocket == 0 {
			return
		}
		if _, ok := seen[sock]; ok {
			return
		}
		seen[sock] = struct{}{}
		out = append(out, sock)
	}

	if _, ok := os.LookupEnv("SSH_AUTH_SOCK"); ok {
		s := strings.TrimSpace(os.Getenv("SSH_AUTH_SOCK"))
		if s == "" {
			return nil
		}
		add(s)
		if p := strings.TrimSpace(configured); p != "" && !strings.EqualFold(p, "none") && p != s {
			add(p)
		}
		return out
	}
	if p := strings.TrimSpace(configured); p != "" && !strings.EqualFold(p, "none") {
		add(p)
	}
	if matches, _ := filepath.Glob("/tmp/ssh-*/agent.*"); len(matches) > 0 {
		sort.Slice(matches, func(i, j int) bool {
			ai, _ := os.Stat(matches[i])
			aj, _ := os.Stat(matches[j])
			if ai == nil || aj == nil {
				return matches[i] < matches[j]
			}
			return ai.ModTime().After(aj.ModTime())
		})
		for _, sock := range matches {
			add(sock)
		}
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		add(filepath.Join(home, ".ssh", "agent.sock"))
	}
	if uid := syscall.Getuid(); uid > 0 {
		add(fmt.Sprintf("/run/user/%d/keyring/ssh", uid))
	}
	return out
}

func loadAgentSigners(configuredAgent string) ([]ssh.Signer, *sshAgentSession, string, string) {
	candidates := resolveSSHAuthSocketCandidates(configuredAgent)
	if len(candidates) == 0 {
		return nil, nil, "", agentStatusSkipped
	}
	var lastStatus string
	for _, sock := range candidates {
		signers, sess, status := probeAgentSigners(sock)
		lastStatus = status
		if sess != nil {
			return signers, sess, sock, status
		}
	}
	return nil, nil, "", lastStatus
}

func probeAgentSigners(sock string) ([]ssh.Signer, *sshAgentSession, string) {
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil, nil, agentStatusUnavailable
	}
	ag := agent.NewClient(conn)
	keys, err := ag.List()
	if err != nil {
		_ = conn.Close()
		return nil, nil, agentStatusUnavailable
	}
	if len(keys) == 0 {
		_ = conn.Close()
		return nil, nil, agentStatusEmpty
	}
	signers, err := ag.Signers()
	if err != nil {
		_ = conn.Close()
		return nil, nil, agentStatusUnavailable
	}
	if len(signers) == 0 {
		_ = conn.Close()
		return nil, nil, agentStatusEmpty
	}
	return signers, &sshAgentSession{conn: conn}, agentStatusOK
}

func loadPrivateKey(path string) (ssh.Signer, string) {
	key, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, identityStatusMissing
		}
		return nil, identityStatusParseFail
	}
	if isEncryptedPrivateKey(key) {
		return nil, identityStatusEncrypted
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "passphrase") {
			return nil, identityStatusEncrypted
		}
		return nil, identityStatusParseFail
	}
	return signer, identityStatusOK
}

func isEncryptedPrivateKey(pemBytes []byte) bool {
	s := string(pemBytes)
	return strings.Contains(s, "ENCRYPTED") ||
		strings.Contains(s, "Proc-Type: 4,ENCRYPTED")
}

func isSSHAuthError(err error) bool {
	if err == nil {
		return false
	}
	var authErr *ssh.ServerAuthError
	if errors.As(err, &authErr) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unable to authenticate") ||
		strings.Contains(msg, "no supported methods remain") ||
		strings.Contains(msg, "authentication failed")
}
