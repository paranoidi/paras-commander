# Sixel goes to tmux bare, with no transport setting

Passthrough-wrapped Sixel is invisible to tmux, which erases it on any line repaint, so Sixel is sent bare whenever tmux reports `sixel` in `client_termfeatures` and has image support; passthrough is used only for tmux builds without image support. The `tmux_sixel_transport` setting was removed because its only correct value was the default. Cost: tmux re-encodes the image on every redraw. See `terminal-graphics.md` lessons 9, 17, 19.
