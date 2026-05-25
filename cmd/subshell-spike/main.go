// Command subshell-spike is a manual Phase 0 smoke test for internal/subshell.
//
// Run from a real terminal: go run ./cmd/subshell-spike/
//
// Ctrl+O toggles between the subshell and a minimal tcell screen (both directions).
// q quits only from the commander banner, not while the shell is visible. The shell child
// stays alive across toggles until you type exit in the shell.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/subshell"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "subshell-spike: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	subshell.SaveLaunchTerminal()

	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	spike, err := subshell.StartSpike(subshell.StartOptions{Dir: wd})
	if err != nil {
		return err
	}
	defer func() {
		_ = spike.Close()
	}()

	screen, err := tcell.NewScreen()
	if err != nil {
		return err
	}
	if err := screen.Init(); err != nil {
		return err
	}
	defer subshell.ShutdownTerminal(screen)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		<-sigCh
		subshell.ShutdownTerminal(screen)
		os.Exit(130)
	}()

	drawBanner(screen, "paras-commander subshell spike — Ctrl+O enters shell")
	for {
		if !spike.Alive() {
			drawBanner(screen, "shell exited — press q to quit spike")
			if waitQuit(screen) {
				return nil
			}
			continue
		}
		drawBanner(screen, "commander view — Ctrl+O enter shell, q quit spike")
		switch waitCommanderKey(screen) {
		case "shell":
			toggled, err := spike.RunVisible(screen)
			if err != nil {
				return err
			}
			if toggled && spike.Alive() {
				drawBanner(screen, "commander view — Ctrl+O enter shell, q quit spike")
			} else {
				drawBanner(screen, "shell exited")
			}
		case "quit":
			return nil
		}
	}
}

func drawBanner(screen tcell.Screen, msg string) {
	w, h := screen.Size()
	style := tcell.StyleDefault.Foreground(tcell.ColorGreen).Background(tcell.ColorBlack)
	screen.Clear()
	row := h / 2
	col := max(0, (w-len(msg))/2)
	for i, r := range msg {
		screen.SetContent(col+i, row, r, nil, style)
	}
	screen.Show()
}

func waitCommanderKey(screen tcell.Screen) string {
	for {
		switch ev := screen.PollEvent().(type) {
		case *tcell.EventResize:
			drawBanner(screen, "commander view — Ctrl+O enter shell, q quit spike")
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyCtrlO {
				return "shell"
			}
			if ev.Rune() == 'q' || ev.Rune() == 'Q' {
				return "quit"
			}
		}
	}
}

func waitQuit(screen tcell.Screen) bool {
	for {
		switch ev := screen.PollEvent().(type) {
		case *tcell.EventResize:
			drawBanner(screen, "shell exited — press q to quit spike")
		case *tcell.EventKey:
			if ev.Rune() == 'q' || ev.Rune() == 'Q' {
				return true
			}
		}
	}
}
