package sshconfig

import (
	"path/filepath"
	"testing"
)

func TestExpandIdentityFileTokens(t *testing.T) {
	t.Parallel()
	ctx := IdentityTokenContext{
		Home:       "/home/alice",
		LocalUser:  "alice",
		RemoteUser: "pi",
		DestHost:   "192.168.50.10",
		LocalHost:  "laptop",
	}
	got := ExpandIdentityFileTokens("%r-%h@%l", ctx)
	want := "pi-192.168.50.10@laptop"
	if got != want {
		t.Fatalf("ExpandIdentityFileTokens = %q, want %q", got, want)
	}
}

func TestConnectAuthOptionsForIdentitiesOnly(t *testing.T) {
	home := t.TempDir()
	content := `
Host rhasspy
  User pi
  IdentitiesOnly yes
  IdentityFile ~/.ssh/pi_ed25519
`
	entries, err := parse(content, home)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	opts := Config{Entries: entries}.ConnectAuthOptionsFor("", "rhasspy", "192.168.50.10", "22")
	if !opts.IdentitiesOnly {
		t.Fatal("IdentitiesOnly want true")
	}
	want := filepath.Join(home, ".ssh", "pi_ed25519")
	if len(opts.IdentityFiles) != 1 || opts.IdentityFiles[0] != want {
		t.Fatalf("IdentityFiles = %v, want %q", opts.IdentityFiles, want)
	}
}
