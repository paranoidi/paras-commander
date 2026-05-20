package sftp

import "context"

// HostKeyPrompt describes an unknown or changed SSH host key.
type HostKeyPrompt struct {
	Host        string
	RemoteAddr  string
	KeyType     string
	Fingerprint string
}

// HostKeyDecision is the user's trust choice.
type HostKeyDecision int

const (
	HostKeyReject HostKeyDecision = iota
	HostKeyTrustSession
	HostKeyTrustPersist
)

// PasswordPrompt requests credentials when public-key auth is unavailable.
type PasswordPrompt struct {
	User string
	Host string
}

// Prompts supplies UI callbacks for interactive SSH authentication.
type Prompts struct {
	HostKey  func(ctx context.Context, p HostKeyPrompt) (HostKeyDecision, error)
	Password func(ctx context.Context, p PasswordPrompt) (string, error)
}
