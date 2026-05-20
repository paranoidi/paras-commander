package config

// SFTPConfig controls SSH/SFTP remote panel connections.
type SFTPConfig struct {
	// KnownHostsFile is the OpenSSH known_hosts path. Empty uses ~/.ssh/known_hosts.
	KnownHostsFile string `toml:"known_hosts_file"`
	// SSHConfigFile is the OpenSSH client config path. Empty uses ~/.ssh/config.
	SSHConfigFile string `toml:"ssh_config_file"`
	// IdleTimeoutSecs closes pooled connections after this many seconds without use (minimum 15 after Validate).
	IdleTimeoutSecs int `toml:"idle_timeout_secs"`
	// DialTimeoutSecs limits TCP+SSH handshake time (minimum 5 after Validate).
	DialTimeoutSecs int `toml:"dial_timeout_secs"`
	// ListTimeoutSecs limits remote panel directory listing via SFTP ReadDir (minimum 5 after Validate).
	ListTimeoutSecs int `toml:"list_timeout_secs"`
}
