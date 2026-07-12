// Command subshell-spike is a manual smoke test for internal/subshell.
//
// Run from a real terminal: go run ./cmd/subshell-spike/
//
// Ctrl+O toggles between the subshell and a minimal tcell screen (both directions).
// The banner shows the shell's live cwd; 1/2 inject cd to two temp dirs (commander→shell),
// and cd typed inside the shell shows up in the banner (shell→commander).
// q quits only from the commander banner, not while the shell is visible. The shell child
// stays alive across toggles until you type exit in the shell.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	sub, err := subshell.Start(subshell.StartOptions{Dir: wd})
	if err != nil {
		return err
	}
	defer func() {
		_ = sub.Close()
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

	cdTargets := [2]string{}
	for i := range cdTargets {
		dir, err := os.MkdirTemp("", fmt.Sprintf("subshell spike cd%d ", i+1))
		if err != nil {
			return err
		}
		defer func() { _ = os.Remove(dir) }()
		cdTargets[i] = dir
	}

	for {
		if !sub.Alive() {
			drawBanner(screen, "shell exited — press q to quit spike")
			if waitQuit(screen) {
				return nil
			}
			continue
		}
		drawBanner(screen, commanderBanner(sub))
		switch waitCommanderKey(screen, sub) {
		case "shell":
			if _, err := sub.RunVisible(screen); err != nil {
				return err
			}
		case "cd1":
			doChdir(screen, sub, cdTargets[0])
		case "cd2":
			doChdir(screen, sub, cdTargets[1])
		case "quit":
			return nil
		}
	}
}

func commanderBanner(sub *subshell.Subshell) string {
	cwd, err := sub.Cwd()
	if err != nil {
		cwd = "?"
	}
	return fmt.Sprintf("commander — Ctrl+O shell, 1/2 cd tmp, q quit | shell cwd: %s", cwd)
}

// doChdir injects a cd and waits briefly so the banner redraw shows the new cwd.
func doChdir(screen tcell.Screen, sub *subshell.Subshell, target string) {
	if err := sub.Chdir(target); err != nil {
		drawBanner(screen, fmt.Sprintf("chdir refused: %v — any key", err))
		screen.PollEvent()
		return
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if cwd, err := sub.Cwd(); err == nil && cwd == target {
			return
		}
		time.Sleep(20 * time.Millisecond)
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

func waitCommanderKey(screen tcell.Screen, sub *subshell.Subshell) string {
	for {
		switch ev := screen.PollEvent().(type) {
		case *tcell.EventResize:
			drawBanner(screen, commanderBanner(sub))
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyCtrlO {
				return "shell"
			}
			switch ev.Rune() {
			case 'q', 'Q':
				return "quit"
			case '1':
				return "cd1"
			case '2':
				return "cd2"
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
