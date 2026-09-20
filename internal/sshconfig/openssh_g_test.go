package sshconfig

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveEndpointHonorsOpenSSHNegatedHostPatterns(t *testing.T) {
	if _, err := exec.LookPath("ssh"); err != nil {
		t.Skip("ssh not on PATH")
	}

	tests := []struct {
		name   string
		config string
		hosts  []string
	}{
		{
			name: "Host * !production does not apply to production",
			config: `
Host * !production
  User wildcard-user
  Port 2222
  HostName wildcard.example.com

Host production
  User prod-user
  HostName prod.example.com

Host staging
  User staging-user
`,
			hosts: []string{"production", "staging", "otherhost"},
		},
		{
			name: "negation listed before the wildcard",
			config: `
Host !production *
  User star-user
  Port 4444
`,
			hosts: []string{"production", "otherhost"},
		},
		{
			name: "later Host * still applies when an earlier negated wildcard does not",
			config: `
Host * !production
  User alice
  Port 2201

Host *
  Port 2202
`,
			hosts: []string{"production", "staging"},
		},
		{
			name: "wildcard and suffix negations",
			config: `
Host * !prod* !*.internal
  User generic
  Port 5555

Host production
  User prod-user
  Port 22
`,
			hosts: []string{"production", "prod-west", "db.internal", "web.example"},
		},
		{
			name: "positive wildcard minus one name",
			config: `
Host *.example.com !prod.example.com
  User generic
  Port 8888

Host prod.example.com
  User prod
`,
			hosts: []string{"prod.example.com", "web.example.com"},
		},
		{
			name: "quoted negated glob",
			config: `
Host * "!prod*"
  User quoted
  Port 6666
`,
			hosts: []string{"production", "prod-west", "other"},
		},
		{
			name: "positive exact alias still matches",
			config: `
Host rhasspy
  HostName 192.168.50.10
  User pi
  Port 2222
`,
			hosts: []string{"rhasspy"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config")
			if err := os.WriteFile(path, []byte(tt.config), 0o644); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, host := range tt.hosts {
				wantUser, wantHost, wantPort := sshGEndpoint(t, path, host)
				gotUser, gotHost, gotPort := cfg.ResolveEndpoint("", host, "22")
				if gotUser == "" {
					gotUser = localSSHUsername()
				}
				if gotUser != wantUser || gotHost != wantHost || gotPort != wantPort {
					t.Fatalf("ResolveEndpoint(%q) = %q,%q,%q; ssh -G = %q,%q,%q",
						host, gotUser, gotHost, gotPort, wantUser, wantHost, wantPort)
				}
			}
		})
	}
}

func sshGEndpoint(t *testing.T, configPath, host string) (user, hostname, port string) {
	t.Helper()
	cmd := exec.Command("ssh", "-n", "-G", "-F", configPath, host)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("ssh -G -F %s %s: %v\n%s", configPath, host, err, out)
	}
	for _, line := range strings.Split(string(out), "\n") {
		key, val, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		switch key {
		case "user":
			if user == "" {
				user = val
			}
		case "hostname":
			if hostname == "" {
				hostname = val
			}
		case "port":
			if port == "" {
				port = val
			}
		}
	}
	if user == "" || hostname == "" || port == "" {
		t.Fatalf("ssh -G %s: missing user/hostname/port in:\n%s", host, out)
	}
	return user, hostname, port
}
