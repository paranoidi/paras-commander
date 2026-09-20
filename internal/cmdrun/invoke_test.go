package cmdrun

import (
	"os/exec"
	"strings"
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

func TestBuildInvocationShellHostilePathMacros(t *testing.T) {
	dir := "/tmp/orchard"
	hostile := []struct {
		name string
		path string
	}{
		{"command substitution", dir + "/beacon$(echo INJECTED)"},
		{"backticks", dir + "/lantern`echo INJECTED`"},
		{"dollar home", dir + "/meadow$HOME"},
		{"double quotes", dir + `/harbor "quoted"`},
		{"single quote", dir + "/harbor's-lantern"},
		{"spaces", dir + "/orchard meadow.txt"},
		{"embedded newline", dir + "/meadow\norchard"},
	}
	macros := []struct {
		name     string
		template string
		ctx      func(path string) cmdmacro.Context
	}{
		{
			name:     "percent-f row",
			template: `printf '%s\n' %f`,
			ctx: func(path string) cmdmacro.Context {
				return cmdmacro.Context{RowPath: path}
			},
		},
		{
			name:     "percent-F other current",
			template: `printf '%s\n' %F`,
			ctx: func(path string) cmdmacro.Context {
				return cmdmacro.Context{
					Active: &cmdmacro.PanelSnapshot{Dir: dir, HasCurrent: true, CurrentName: dir + "/cursor"},
					Other:  &cmdmacro.PanelSnapshot{Dir: dir, HasCurrent: true, CurrentName: path},
				}
			},
		},
		{
			name:     "percent-d active dir",
			template: `printf '%s\n' %d`,
			ctx: func(path string) cmdmacro.Context {
				return cmdmacro.Context{
					Active: &cmdmacro.PanelSnapshot{Dir: path, HasCurrent: true, CurrentName: path + "/cursor"},
				}
			},
		},
		{
			name:     "percent-D other dir",
			template: `printf '%s\n' %D`,
			ctx: func(path string) cmdmacro.Context {
				return cmdmacro.Context{
					Active: &cmdmacro.PanelSnapshot{Dir: dir, HasCurrent: true, CurrentName: dir + "/cursor"},
					Other:  &cmdmacro.PanelSnapshot{Dir: path},
				}
			},
		},
		{
			name:     "percent-t tagged",
			template: `printf '%s\n' %t`,
			ctx: func(path string) cmdmacro.Context {
				return cmdmacro.Context{
					Active: &cmdmacro.PanelSnapshot{
						Dir: dir, HasCurrent: true, CurrentName: dir + "/cursor",
						TaggedInDir: []string{path},
					},
				}
			},
		},
		{
			name:     "percent-T other tagged",
			template: `printf '%s\n' %T`,
			ctx: func(path string) cmdmacro.Context {
				return cmdmacro.Context{
					Active: &cmdmacro.PanelSnapshot{Dir: dir, HasCurrent: true, CurrentName: dir + "/cursor"},
					Other: &cmdmacro.PanelSnapshot{
						Dir:         dir,
						TaggedInDir: []string{path},
					},
				}
			},
		},
	}
	for _, mac := range macros {
		for _, h := range hostile {
			t.Run(mac.name+"/"+h.name, func(t *testing.T) {
				built, err := BuildInvocation(InvocationSpec{
					Template:   mac.template,
					Mode:       ModeShellScript,
					ForceShell: true,
					Ctx:        mac.ctx(h.path),
				})
				if err != nil {
					t.Fatal(err)
				}
				if len(built.Argv) != 3 || built.Argv[1] != "-c" {
					t.Fatalf("argv = %#v, want sh -c", built.Argv)
				}
				out, err := exec.Command(built.Argv[0], built.Argv[1:]...).Output()
				if err != nil {
					t.Fatalf("run: %v\nscript=%q", err, built.Argv[2])
				}
				got := strings.TrimRight(string(out), "\n")
				if got != h.path {
					t.Fatalf("stdout = %q want %q (script %q)", got, h.path, built.Argv[2])
				}
			})
		}
	}
}

func TestBuildInvocationArgvHostilePathStaysStructured(t *testing.T) {
	path := `/tmp/orchard/beacon$(echo INJECTED)`
	built, err := BuildInvocation(InvocationSpec{
		Template: `gzip -9 %f`,
		Mode:     ModeExecParsed,
		Ctx:      cmdmacro.Context{RowPath: path},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(built.Argv) != 3 || built.Argv[0] != "gzip" || built.Argv[1] != "-9" || built.Argv[2] != path {
		t.Fatalf("argv = %#v", built.Argv)
	}
	if built.Argv[1] == "-c" {
		t.Fatal("argv mode must not wrap sh -c")
	}
}
