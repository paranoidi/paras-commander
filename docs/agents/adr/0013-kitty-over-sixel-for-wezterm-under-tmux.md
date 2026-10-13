# WezTerm uses cursor-relative Kitty, even under tmux

Under tmux, WezTerm (without Unicode-placeholder support) must choose between cursor-relative Kitty (intermittent misplacement from tmux's cursor-position race) and Sixel (vanish-on-redraw, reduced palette). `auto` picks Kitty for 24-bit colour, deliberately diverging from yazi. Neither failure is fixed; see `terminal-graphics.md` lessons 12–14.

## Considered options

- Sixel under tmux+WezTerm: rejected for palette loss and the unmitigated vanish gap (lesson 11).
