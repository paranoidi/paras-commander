package sshconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseHostStanza(t *testing.T) {
	home := t.TempDir()
	content := `
# comment
Host myalias other
  HostName real.example.com
  User alice
  Port 2222
  IdentityFile id_ed25519
  IdentityFile ~/.ssh/backup_rsa

Host *.wildcard
  User ignored

Host plain
  HostName plain.local
`
	entries, err := parse(content, home)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Fatalf("entries = %d, want 4 (myalias, other, plain, *.wildcard)", len(entries))
	}
	var my, other, plain *HostEntry
	for i := range entries {
		switch entries[i].Alias {
		case "myalias":
			my = &entries[i]
		case "other":
			other = &entries[i]
		case "plain":
			plain = &entries[i]
		}
	}
	if my == nil || other == nil || plain == nil {
		t.Fatalf("entries: %+v", entries)
	}
	if my.HostName != "real.example.com" || my.User != "alice" || my.Port != "2222" {
		t.Fatalf("myalias: %+v", my)
	}
	if len(my.IdentityFiles) != 2 || my.IdentityFiles[0] != "id_ed25519" {
		t.Fatalf("identity files = %v, want id_ed25519 first", my.IdentityFiles)
	}
	if my.IdentityFiles[1] != "~/.ssh/backup_rsa" {
		t.Fatalf("backup path = %q", my.IdentityFiles[1])
	}
	ctx := IdentityTokenContext{
		Home: home, LocalUser: "alice", RemoteUser: "alice", DestHost: "real.example.com",
	}
	expanded := expandIdentityFilePaths(my.IdentityFiles, ctx)
	wantKey := filepath.Join(home, ".ssh", "id_ed25519")
	if len(expanded) != 2 || expanded[0] != wantKey {
		t.Fatalf("expanded identity files = %v, want %q first", expanded, wantKey)
	}
	if filepath.Clean(expanded[1]) != filepath.Join(home, ".ssh", "backup_rsa") {
		t.Fatalf("expanded backup path = %q", expanded[1])
	}
	if other.User != "alice" {
		t.Fatalf("other user = %q", other.User)
	}
	uri, err := my.BuildSFTPURI("/var")
	if err != nil {
		t.Fatal(err)
	}
	if uri != "sftp://alice@real.example.com:2222/var" {
		t.Fatalf("uri = %q", uri)
	}
	if !my.MatchesEndpoint("alice", "real.example.com", "2222") {
		t.Fatal("should match endpoint")
	}
	if !my.MatchesHost("", "myalias") {
		t.Fatal("should match host alias")
	}

	cfg := Config{Entries: entries}
	u, h, p := cfg.ResolveEndpoint("", "myalias", "22")
	if u != "alice" || h != "real.example.com" || p != "2222" {
		t.Fatalf("ResolveEndpoint alias = %q,%q,%q", u, h, p)
	}
	u, h, p = cfg.ResolveEndpoint("alice", "real.example.com", "22")
	if u != "alice" || h != "real.example.com" || p != "2222" {
		t.Fatalf("ResolveEndpoint hostname = %q,%q,%q", u, h, p)
	}
	if got := cfg.IdentityFilesFor("", "myalias", "22"); len(got) != 2 {
		t.Fatalf("IdentityFilesFor alias = %v", got)
	}
	if got := cfg.IdentityFilesFor("", "real.example.com", "22"); len(got) != 2 {
		t.Fatalf("IdentityFilesFor resolved host = %v", got)
	}
	if got := cfg.IdentityFilesForConnect("", "myalias", "real.example.com", "22"); len(got) != 2 {
		t.Fatalf("IdentityFilesForConnect = %v", got)
	}
	plainURI, err := plain.BuildSFTPURI("")
	if err != nil {
		t.Fatal(err)
	}
	if plainURI != "sftp://plain.local/~" {
		t.Fatalf("plain uri = %q", plainURI)
	}
}

func TestLoadMissingFile(t *testing.T) {
	t.Parallel()
	cfg, err := Load(filepath.Join(t.TempDir(), "no-such-config"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Entries) != 0 {
		t.Fatalf("entries = %v", cfg.Entries)
	}
}

func TestMergeDuplicateHostStanzas(t *testing.T) {
	t.Parallel()
	content := `
Host rhasspy
  User pi

Host rhasspy
  HostName 192.168.50.10
  Port 2222
`
	entries, err := parse(content, "/home/test")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1 merged rhasspy", len(entries))
	}
	e := entries[0]
	if e.Alias != "rhasspy" || e.User != "pi" || e.HostName != "192.168.50.10" || e.Port != "2222" {
		t.Fatalf("merged entry = %+v", e)
	}
	u, h, p := Config{Entries: entries}.ResolveEndpoint("", "rhasspy", "22")
	if u != "pi" || h != "192.168.50.10" || p != "2222" {
		t.Fatalf("ResolveEndpoint = %q,%q,%q", u, h, p)
	}
}

func TestIncludeMergesHostName(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	incDir := filepath.Join(dir, "config.d")
	if err := os.MkdirAll(incDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(incDir, "rhasspy.conf"), []byte(`
Host rhasspy
  HostName 192.168.50.10
`), 0o644); err != nil {
		t.Fatal(err)
	}
	main := filepath.Join(dir, "config")
	if err := os.WriteFile(main, []byte(`
Host rhasspy
  User pi
Include config.d/*
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(main)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Entries) != 1 {
		t.Fatalf("entries = %+v", cfg.Entries)
	}
	u, h, p := cfg.ResolveEndpoint("", "rhasspy", "22")
	if u != "pi" || h != "192.168.50.10" || p != "22" {
		t.Fatalf("ResolveEndpoint = %q,%q,%q", u, h, p)
	}
}

func TestResolvePathSSHConfigEnv(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "from-env")
	if err := os.WriteFile(cfgPath, []byte("Host x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SSH_CONFIG", cfgPath)
	got, err := ResolvePath("")
	if err != nil {
		t.Fatal(err)
	}
	if got != cfgPath {
		t.Fatalf("ResolvePath = %q, want %q", got, cfgPath)
	}
}

func TestResolveEndpointMergesAllMatchingStanzas(t *testing.T) {
	t.Parallel()
	content := `
Host rhasspy
  User pi

Host RHASSPY
  HostName 192.168.50.10
`
	entries, err := parse(content, "/home/test")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2 distinct aliases", len(entries))
	}
	u, h, p := Config{Entries: entries}.ResolveEndpoint("", "rhasspy", "22")
	if u != "pi" || h != "192.168.50.10" || p != "22" {
		t.Fatalf("ResolveEndpoint = %q,%q,%q", u, h, p)
	}
}

func TestResolveEndpointWildcardHostPattern(t *testing.T) {
	t.Parallel()
	content := `
Host rhas*
  HostName 192.168.50.10
  User pi
`
	entries, err := parse(content, "/home/test")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	u, h, p := Config{Entries: entries}.ResolveEndpoint("", "rhasspy", "22")
	if u != "pi" || h != "192.168.50.10" || p != "22" {
		t.Fatalf("ResolveEndpoint = %q,%q,%q", u, h, p)
	}
}

func TestResolveEndpointMatchHostDirective(t *testing.T) {
	t.Parallel()
	content := `
Match host rhasspy
  HostName 192.168.50.11
  User pi
`
	entries, err := parse(content, "/home/test")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %+v", entries)
	}
	u, h, p := Config{Entries: entries}.ResolveEndpoint("", "rhasspy", "22")
	if u != "pi" || h != "192.168.50.11" || p != "22" {
		t.Fatalf("ResolveEndpoint = %q,%q,%q", u, h, p)
	}
}

func TestDiagnoseEndpointReportsMatchCount(t *testing.T) {
	t.Parallel()
	content := `
Host rhasspy
  User pi

Host RHASSPY
  HostName 192.168.50.10
`
	entries, err := parse(content, "/home/test")
	if err != nil {
		t.Fatal(err)
	}
	d := Config{Path: "/tmp/config", Entries: entries}.DiagnoseEndpoint("", "rhasspy", "22")
	if !d.Matched || d.MatchedStanzas != 2 {
		t.Fatalf("diag = %+v", d)
	}
	if d.DialAddress != "192.168.50.10:22" {
		t.Fatalf("DialAddress = %q", d.DialAddress)
	}
}

func TestHostNameEqualsFormat(t *testing.T) {
	t.Parallel()
	content := `
Host rhasspy
  HostName=192.168.3.26
  User=pi
  Port=22
`
	entries, err := parse(content, "/home/test")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %+v", entries)
	}
	e := entries[0]
	if e.Alias != "rhasspy" || e.HostName != "192.168.3.26" || e.User != "pi" || e.Port != "22" {
		t.Fatalf("entry = %+v", e)
	}
	u, h, p := Config{Entries: entries}.ResolveEndpoint("", "rhasspy", "22")
	if u != "pi" || h != "192.168.3.26" || p != "22" {
		t.Fatalf("ResolveEndpoint = %q,%q,%q", u, h, p)
	}
	d := Config{Entries: entries}.DiagnoseEndpoint("", "rhasspy", "22")
	if !d.Matched || !d.HostNameSet || d.HostName != "192.168.3.26" || d.DialAddress != "192.168.3.26:22" {
		t.Fatalf("diag = %+v", d)
	}
}

func TestHostNameEqualsFormatSeparateStanza(t *testing.T) {
	t.Parallel()
	content := `
Host rhasspy
  User pi

Host rhasspy
  HostName=192.168.3.26
`
	entries, err := parse(content, "/home/test")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1 merged rhasspy", len(entries))
	}
	u, h, p := Config{Entries: entries}.ResolveEndpoint("", "rhasspy", "22")
	if u != "pi" || h != "192.168.3.26" || p != "22" {
		t.Fatalf("ResolveEndpoint = %q,%q,%q", u, h, p)
	}
}

func TestSplitConfigKeyword(t *testing.T) {
	t.Parallel()
	tests := []struct {
		line, wantKey, wantVal string
	}{
		{"HostName=192.168.3.26", "hostname", "192.168.3.26"},
		{"HostName 192.168.3.26", "hostname", "192.168.3.26"},
		{"HostName = 192.168.3.26", "hostname", "192.168.3.26"},
		{`HostName "192.168.3.26"`, "hostname", "192.168.3.26"},
		{"IdentityFile=~/.ssh/id_rsa", "identityfile", "~/.ssh/id_rsa"},
		{"Host rhasspy", "host", "rhasspy"},
	}
	for _, tc := range tests {
		key, val := splitConfigKeyword(tc.line)
		if key != tc.wantKey || val != tc.wantVal {
			t.Errorf("splitConfigKeyword(%q) = %q,%q want %q,%q", tc.line, key, val, tc.wantKey, tc.wantVal)
		}
	}
}

func TestFirstWinsHostNameAcrossStanzas(t *testing.T) {
	t.Parallel()
	content := `
Host rhasspy
  HostName 192.168.1.1

Host rhasspy
  HostName 192.168.1.2
`
	entries, err := parse(content, "/home/test")
	if err != nil {
		t.Fatal(err)
	}
	if entries[0].HostName != "192.168.1.1" {
		t.Fatalf("HostName = %q, want first stanza value", entries[0].HostName)
	}
}

func TestFormatHostListLinesAlignsHostAndHostName(t *testing.T) {
	t.Parallel()
	entries := []HostEntry{
		{Alias: "rhasspy", HostName: "192.168.50.10", User: "pi"},
		{Alias: "alpha-server", HostName: "alpha.example.com"},
		{Alias: "beta-server", HostName: "beta.example.com", Port: "2222"},
	}
	lines := FormatHostListLines(entries)
	if len(lines) != 3 {
		t.Fatalf("lines = %d, want 3", len(lines))
	}
	targetCol := strings.Index(lines[0], "pi@192.168.50.10")
	if targetCol < 0 {
		t.Fatalf("line[0] = %q, want pi@192.168.50.10", lines[0])
	}
	for i, wantPrefix := range []string{"alpha.example.com", "beta.example.com:2222"} {
		col := strings.Index(lines[i+1], wantPrefix)
		if col != targetCol {
			t.Fatalf("line[%d] HostName column at %d, want %d (line=%q)", i+1, col, targetCol, lines[i+1])
		}
	}
	if targetCol != len([]rune("alpha-server"))+1 {
		t.Fatalf("target column = %d, want one past longest alias", targetCol)
	}
	if !strings.HasPrefix(lines[0], "rhasspy") {
		t.Fatalf("line[0] = %q, want rhasspy prefix", lines[0])
	}
	if !strings.HasPrefix(lines[1], "alpha-server") {
		t.Fatalf("line[1] = %q, want alpha-server prefix", lines[1])
	}
}
