package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

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
	for _, arg := range args {
		if arg == "--chooser-file" {
			return fmt.Errorf("--chooser-file requires a path argument")
		}
		if arg == "--select" {
			return fmt.Errorf("--select requires a path argument")
		}
		const chooserPrefix = "--chooser-file="
		if strings.HasPrefix(arg, chooserPrefix) && strings.TrimSpace(strings.TrimPrefix(arg, chooserPrefix)) == "" {
			return fmt.Errorf("--chooser-file requires a non-empty path")
		}
		const selectPrefix = "--select="
		if strings.HasPrefix(arg, selectPrefix) && strings.TrimSpace(strings.TrimPrefix(arg, selectPrefix)) == "" {
			return fmt.Errorf("--select requires a non-empty path")
		}
	}

	flags := flag.NewFlagSet("pc", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var showVersion bool
	flags.BoolVar(&showVersion, "version", false, "print version and exit")
	flags.BoolVar(&showVersion, "v", false, "print version and exit")
	configStub := flags.Bool("config-stub", false, "write example config files to ~/.config/pc/ (optional directory argument) and exit")
	devMode := flags.Bool("dev", false, "enable Dev pulldown menu with test helpers")
	chooserFile := flags.String("chooser-file", "", "write selected file path on Enter and exit (Helix integration)")
	selectPath := flags.String("select", "", "file or directory to open and highlight at startup (chooser mode)")
	noCarousel := flags.Bool("no-carousel", false, "disable carousel view on the left panel at startup (chooser mode only)")
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
	chooser := strings.TrimSpace(*chooserFile)
	selectArg := strings.TrimSpace(*selectPath)
	if selectArg != "" && chooser == "" {
		return fmt.Errorf("--select requires --chooser-file")
	}
	if *noCarousel && chooser == "" {
		return fmt.Errorf("--no-carousel requires --chooser-file")
	}

	return app.Run(app.LaunchConfig{
		DevMode:           *devMode,
		ChooserFile:       chooser,
		ChooserSelect:     selectArg,
		ChooserNoCarousel: *noCarousel,
	})
}
