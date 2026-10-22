Source docs/tapes/settings.tape
Output "/tmp/pc-docs-demo/compare.webm"

Hide
Type "clear && HOME=$PC_DEMO_HOME XDG_CONFIG_HOME=$PC_DEMO_HOME/.config ./pc $PC_DEMO_HOME/albums/lisbon-card-import $PC_DEMO_HOME/albums/lisbon-album"
Enter
Wait+Screen /QuitNow/
Sleep 3s
Type ":"
Sleep 500ms
Type "C"
Sleep 3s
Show
Sleep 500ms
Screenshot docs/assets/compare.png
Sleep 500ms
Hide
Escape
Sleep 300ms
Escape
Type ":"
Type "q"
Sleep 500ms
Enter
