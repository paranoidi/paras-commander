Source docs/tapes/settings.tape
Output "/tmp/pc-docs-demo/quick-view.webm"

Hide
Type "clear && HOME=$PC_DEMO_HOME XDG_CONFIG_HOME=$PC_DEMO_HOME/.config ./pc $PC_DEMO_HOME/work/copper-garden/src $PC_DEMO_HOME/work/harbor-lantern"
Enter
Wait+Screen /QuitNow/
Sleep 3s
Down 3
Type ":"
Sleep 500ms
Type "V"
Sleep 2s
Show
Sleep 500ms
Screenshot docs/assets/quick-view.png
Sleep 500ms
Hide
Escape
Down 3
Type ":"
Sleep 500ms
Type "V"
Escape
Type ":"
Type "q"
Sleep 500ms
Enter
