Source docs/tapes/settings.tape
Output "/tmp/pc-docs-demo/disk-usage.webm"

Hide
Type "clear && HOME=$PC_DEMO_HOME XDG_CONFIG_HOME=$PC_DEMO_HOME/.config ./pc $PC_DEMO_HOME/work $PC_DEMO_HOME/work/harbor-lantern"
Enter
Wait+Screen /QuitNow/
Sleep 3s
Alt+d
Sleep 3s
Show
Sleep 500ms
Screenshot docs/assets/disk-usage.png
Sleep 500ms
Hide
Escape
Sleep 300ms
Escape
Type ":"
Type "q"
Sleep 500ms
Enter
