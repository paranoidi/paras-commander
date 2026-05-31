package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/paranoidi/paras-commander/internal/app"
	"github.com/paranoidi/paras-commander/internal/version"
)

func main() {
	if err := run(os.Args[1:], os.Stderr, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "pc: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stderr, stdout io.Writer) error {
	flags := flag.NewFlagSet("pc", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var showVersion bool
	flags.BoolVar(&showVersion, "version", false, "print version and exit")
	flags.BoolVar(&showVersion, "v", false, "print version and exit")
	configStub := flags.Bool("config-stub", false, "write example config files to ~/.config/pc/ (optional directory argument) and exit")
	devMode := flags.Bool("dev", false, "enable Dev pulldown menu with test helpers")
	if err := flags.Parse(args); err != nil {
		return err
	}

	if showVersion {
		if flags.NArg() > 0 {
			return fmt.Errorf("unexpected argument %q", flags.Arg(0))
		}
		_, err := fmt.Fprintln(stdout, version.Line())
		return err
	}

	if *configStub {
		dir, err := resolveConfigStubDir(flags.Arg(0))
		if err != nil {
			return err
		}
		if flags.NArg() > 1 {
			return fmt.Errorf("unexpected argument %q", flags.Arg(1))
		}
		return writeConfigDirStubs(dir, stderr)
	}

	if flags.NArg() > 0 {
		return fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	return app.Run(*devMode)
}
