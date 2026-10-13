# No yazi-style startup drain of terminal replies

Draining terminal capability replies before the first frame would hide tmux's one-time feature-update redraw, but the tty ignores `SetReadDeadline`: the drain blocked startup ~3 s and swallowed the first keystroke. Rejected until a working read deadline exists. See `terminal-graphics.md` lesson 18.
