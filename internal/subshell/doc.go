// Package subshell implements a persistent PTY-backed shell for MC-style Ctrl+O toggle.
//
// Phase 0 (Spike) provides [Spike]: creack/pty child, tcell Suspend/Resume handoff, and a
// stdin↔PTY feed loop that returns on Ctrl+O (0x0f and kitty-style sequences). Later phases
// add cwd sync and app integration.
//
// Manual smoke test (real TTY, Linux):
//
//	go run ./cmd/subshell-spike/
//
// Uses tcell Suspend/Resume (not Fini/Init) to hand off the TTY, same as the external editor.
// Do not install package-wide signal handlers or restore termios from before Suspend — that breaks foot.
// While shell-visible: read/write /dev/tty (not os.Stdin after tcell Suspend), disable kitty keyboard
// protocol (Ctrl+O → 0x0f), restore host raw mode before tcell Resume. Call [SaveLaunchTerminal] before Init
// and [ShutdownTerminal] on exit so kill does not leave the parent shell in raw mode.
// Press Ctrl+O to enter the shell and again to return.
// q quits the spike only from the commander banner, not from inside the shell.
package subshell
