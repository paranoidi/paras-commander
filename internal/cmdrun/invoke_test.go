package cmdrun

import (
	"testing"

	"github.com/paranoidi/paras-commander/internal/cmdmacro"
)

func TestBuildInvocationExecParsed(t *testing.T) {
	built, err := BuildInvocation(InvocationSpec{
		Template: `gzip -9 %f`,
		Mode:     ModeAuto,
		Ctx: cmdmacro.Context{
			Active: &cmdmacro.PanelSnapshot{
				Dir:         "/tmp",
				HasCurrent:  true,
				CurrentName: "a.txt",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(built.Argv) < 2 || built.Argv[0] != "gzip" {
		t.Fatalf("argv = %#v", built.Argv)
	}
}

func TestBuildInvocationShellOperators(t *testing.T) {
	built, err := BuildInvocation(InvocationSpec{
		Template: `echo %f >> /tmp/out`,
		Mode:     ModeAuto,
		Ctx: cmdmacro.Context{
			Active: &cmdmacro.PanelSnapshot{
				Dir:         "/tmp",
				HasCurrent:  true,
				CurrentName: "a.txt",
			},
			FOverride: "/tmp/a.txt",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(built.Argv) != 3 || built.Argv[1] != "-c" {
		t.Fatalf("argv = %#v, want sh -c", built.Argv)
	}
}

func TestBuildInvocationMetaShell(t *testing.T) {
	built, err := BuildInvocation(InvocationSpec{
		Template: `wc -l < %f`,
		Mode:     ModeShellScript,
		Ctx:      cmdmacro.Context{RowPath: "/tmp/x.go"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(built.Argv) != 3 || built.Argv[1] != "-c" {
		t.Fatalf("argv = %#v", built.Argv)
	}
}
