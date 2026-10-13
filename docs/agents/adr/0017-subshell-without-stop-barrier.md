# Persistent subshell hides injected cd by muting output, not MC's STOP barrier

The Ctrl+O subshell syncs cwd by injecting `cd` and muting the PTY feed until the precmd pipe reports a prompt cycle, instead of Midnight Commander's SIGSTOP-based barrier. Simpler and portable across shells, with a small accepted race that can briefly show a stale prompt. See `subshell.md`.
