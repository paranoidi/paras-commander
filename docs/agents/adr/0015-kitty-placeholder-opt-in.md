# Kitty Unicode placeholders are opt-in outside Kitty and Ghostty

Placeholder support varies per WezTerm build and cannot be detected through tmux, and a wrong guess renders nothing. Kitty and Ghostty qualify automatically; any other terminal needs `terminal_kitty_placeholder = "yes"`. See `terminal-graphics.md` lesson 16.
