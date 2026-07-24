package commands

import (
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/usermenu"
)

func TestValidateRunForEachCommandRequiresF(t *testing.T) {
	msg := validateRunForEachCommand("gzip -9", []localfs.Entry{{Path: "/a/b.txt", Name: "b.txt"}}, nil, nil)
	if !strings.Contains(msg, usermenu.ErrRunForEachRequiresF) {
		t.Fatalf("msg = %q", msg)
	}
}
