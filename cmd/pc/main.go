package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/paranoidi/paras-commander/internal/app"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
)

func main() {
	if err := run(os.Args[1:], os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "pc: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stderr io.Writer) error {
	flags := flag.NewFlagSet("pc", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configStub := flags.String("config-stub", "", "write default configuration TOML (general settings + [action_keys] + [jobs_action_keys]) to filename and exit")
	keybindingsStub := flags.String("keybindings-stub", "", "write default keybindings TOML to filename and exit")
	devMode := flags.Bool("dev", false, "enable Dev pulldown menu with test helpers")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		return fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}

	if *configStub != "" {
		return writeConfigStub(*configStub)
	}
	if *keybindingsStub != "" {
		return keymap.WriteDefaultStub(*keybindingsStub)
	}
	return app.Run(*devMode)
}

// writeConfigStub renders a single bootstrap file containing the full
// default configuration plus the complete shortcut map under
// [action_keys]. The shortcut block is sourced directly from the keymap
// package, so any ActionSpec added to keymap.DefaultActionSpecs
// automatically appears in --config-stub output without further changes.
func writeConfigStub(filename string) error {
	if filename == "" {
		return fmt.Errorf("config stub filename is required")
	}
	var buf bytes.Buffer
	if err := config.EncodeDefaultStub(&buf); err != nil {
		return err
	}
	if !bytes.HasSuffix(buf.Bytes(), []byte{'\n'}) {
		buf.WriteByte('\n')
	}
	buf.WriteByte('\n')
	if err := keymap.EncodeDefaultStub(&buf); err != nil {
		return err
	}
	if err := os.WriteFile(filename, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write config stub %q: %w", filename, err)
	}
	return nil
}
